package game

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"time"

	"chess-clone/backend/internal/db"

	"github.com/notnil/chess"
)

const disconnectTimeout = 60 * time.Second

type clientMessage struct {
	client *Client
	msg    InboundMsg
}

// Room gestisce una partita: stato, timer, messaggi
type Room struct {
	gameID string
	white  *Client // nil se non connesso
	black  *Client

	chess       *chess.Game
	timeControl int // secondi (es. 600)
	started     bool

	pg  *db.Postgres
	rdb *db.Redis
	hub *Hub

	inbound            chan clientMessage
	clientDisconnected chan *Client

	// Timer disconnessione
	whiteReconnectTimer *time.Timer
	blackReconnectTimer *time.Timer
}

func newRoom(gameID string, pg *db.Postgres, rdb *db.Redis, hub *Hub) *Room {
	return &Room{
		gameID:             gameID,
		chess:              chess.NewGame(),
		pg:                 pg,
		rdb:                rdb,
		hub:                hub,
		inbound:            make(chan clientMessage, 32),
		clientDisconnected: make(chan *Client, 4),
	}
}

// Run è il loop principale della room (una goroutine per partita)
func (r *Room) Run() {
	defer r.hub.Remove(r.gameID)

	// Carica time_control dal DB
	r.loadTimeControl()

	for {
		select {

		// Messaggio da un client
		case cm := <-r.inbound:
			r.handleMessage(cm)

		// Client disconnesso
		case c := <-r.clientDisconnected:
			r.handleDisconnect(c)
		}
	}
}

// Join aggiunge un client alla room (white o black)
func (r *Room) Join(c *Client) error {
	switch c.Color {
	case "white":
		if r.white != nil && r.white.UserID != c.UserID {
			return fmt.Errorf("posto bianco già occupato")
		}
		// Riconnessione
		if r.whiteReconnectTimer != nil {
			r.whiteReconnectTimer.Stop()
			r.whiteReconnectTimer = nil
			r.broadcast(OutboundMsg{Type: "opponent_reconnected"})
		}
		r.white = c
	case "black":
		if r.black != nil && r.black.UserID != c.UserID {
			return fmt.Errorf("posto nero già occupato")
		}
		if r.blackReconnectTimer != nil {
			r.blackReconnectTimer.Stop()
			r.blackReconnectTimer = nil
			r.broadcast(OutboundMsg{Type: "opponent_reconnected"})
		}
		r.black = c
	default:
		return fmt.Errorf("colore non valido: %s", c.Color)
	}

	// Avvia la partita se entrambi connessi
	if r.white != nil && r.black != nil && !r.started {
		r.startGame()
	}

	return nil
}

func (r *Room) startGame() {
	r.started = true
	ctx := context.Background()
	InitTimer(ctx, r.rdb, r.gameID, r.timeControl)

	// Aggiorna status nel DB
	r.pg.Pool.Exec(ctx,
		`UPDATE games SET status = 'active', started_at = NOW() WHERE id = $1`,
		r.gameID,
	)

	// Manda stato iniziale a entrambi
	fen := r.chess.Position().String()
	wMs, bMs := GetTimer(ctx, r.rdb, r.gameID)

	r.white.Send(OutboundMsg{
		Type: "game_start",
		Payload: map[string]any{
			"fen": fen, "your_color": "white",
			"white_ms": wMs, "black_ms": bMs,
		},
	})
	r.black.Send(OutboundMsg{
		Type: "game_start",
		Payload: map[string]any{
			"fen": fen, "your_color": "black",
			"white_ms": wMs, "black_ms": bMs,
		},
	})
}

// ── Handler messaggi ───────────────────────────────────────────────────────

func (r *Room) handleMessage(cm clientMessage) {
	switch cm.msg.Type {
	case "move":
		var p MovePayload
		if err := json.Unmarshal(cm.msg.Payload, &p); err != nil {
			cm.client.Send(OutboundMsg{Type: "move_invalid", Payload: ErrorPayload{Reason: "payload non valido"}})
			return
		}
		r.handleMove(cm.client, p)

	case "resign":
		r.handleResign(cm.client)

	case "offer_draw":
		r.handleDrawOffer(cm.client)

	case "draw_response":
		var p DrawResponsePayload
		if err := json.Unmarshal(cm.msg.Payload, &p); err != nil {
			return
		}
		r.handleDrawResponse(cm.client, p.Accepted)
	}
}

func (r *Room) handleMove(c *Client, p MovePayload) {
	if !r.started {
		return
	}

	// Verifica che sia il turno del client
	turn := r.chess.Position().Turn()
	if (turn == chess.White && c.Color != "white") ||
		(turn == chess.Black && c.Color != "black") {
		c.Send(OutboundMsg{Type: "move_invalid", Payload: ErrorPayload{Reason: "non è il tuo turno"}})
		return
	}

	// Costruisce la stringa UCI (es. "e2e4" o "e7e8q" per promozione)
	uci := p.From + p.To
	if p.Promotion != "" {
		uci += p.Promotion
	}

	// Valida e applica la mossa
	move, err := chess.UCINotation{}.Decode(r.chess.Position(), uci)
	if err != nil {
		c.Send(OutboundMsg{Type: "move_invalid", Payload: ErrorPayload{Reason: "mossa illegale"}})
		return
	}
	if err := r.chess.Move(move); err != nil {
		c.Send(OutboundMsg{Type: "move_invalid", Payload: ErrorPayload{Reason: "mossa non valida"}})
		return
	}

	// Aggiorna timer
	ctx := context.Background()
	wMs, bMs, timedOut, loser, err := RecordMove(ctx, r.rdb, r.gameID)
	if err != nil {
		log.Printf("timer error: %v", err)
	}
	if timedOut {
		winner := "black"
		if loser == "black" {
			winner = "white"
		}
		r.endGame(winner, "timeout")
		return
	}

	// FEN e PGN aggiornati
	newFen := r.chess.Position().String()
	pgn := r.chess.String()
	turnStr := "w"
	if r.chess.Position().Turn() == chess.Black {
		turnStr = "b"
	}

	// Broadcast mossa ai due client
	r.broadcast(OutboundMsg{
		Type: "move_made",
		Payload: MoveMadePayload{
			From: p.From, To: p.To,
			FEN: newFen, PGN: pgn,
			Turn:    turnStr,
			WhiteMs: wMs, BlackMs: bMs,
		},
	})

	// Salva PGN nel DB
	r.pg.Pool.Exec(ctx, `UPDATE games SET pgn = $1 WHERE id = $2`, pgn, r.gameID)

	// Controlla fine partita
	r.checkOutcome()
}

func (r *Room) handleResign(c *Client) {
	winner := "black"
	if c.Color == "black" {
		winner = "white"
	}
	r.endGame(winner, "resigned")
}

func (r *Room) handleDrawOffer(c *Client) {
	opponent := r.opponent(c)
	if opponent != nil {
		opponent.Send(OutboundMsg{Type: "draw_offered"})
	}
}

func (r *Room) handleDrawResponse(c *Client, accepted bool) {
	if accepted {
		r.endGame("draw", "draw_agreed")
	} else {
		// Informiamo chi ha offerto la patta
		opponent := r.opponent(c)
		if opponent != nil {
			opponent.Send(OutboundMsg{Type: "draw_declined"})
		}
	}
}

func (r *Room) handleDisconnect(c *Client) {
	log.Printf("client disconnesso: %s (%s)", c.UserID, c.Color)

	// Notifica avversario
	opponent := r.opponent(c)
	if opponent != nil {
		opponent.Send(OutboundMsg{
			Type:    "opponent_disconnected",
			Payload: DisconnectPayload{TimeoutSeconds: int(disconnectTimeout.Seconds())},
		})
	}

	// Azzera il riferimento
	if c.Color == "white" {
		r.white = nil
	} else {
		r.black = nil
	}

	// Avvia timer abbandono (60s)
	color := c.Color
	timer := time.AfterFunc(disconnectTimeout, func() {
		if r.started && r.chess.Outcome() == chess.NoOutcome {
			winner := "black"
			if color == "black" {
				winner = "white"
			}
			r.endGame(winner, "abandoned")
		}
	})

	if c.Color == "white" {
		r.whiteReconnectTimer = timer
	} else {
		r.blackReconnectTimer = timer
	}
}

// ── Helpers ────────────────────────────────────────────────────────────────

func (r *Room) checkOutcome() {
	outcome := r.chess.Outcome()
	if outcome == chess.NoOutcome {
		return
	}

	method := r.chess.Method()
	reason := methodToReason(method)
	result := outcomeToResult(outcome)
	r.endGame(result, reason)
}

func (r *Room) endGame(result, reason string) {
	pgn := r.chess.String()
	ctx := context.Background()

	// Aggiorna DB
	r.pg.Pool.Exec(ctx,
		`UPDATE games SET status='finished', result=$1, finish_reason=$2,
		 finished_at=NOW(), pgn=$3 WHERE id=$4`,
		result, reason, pgn, r.gameID,
	)

	// Aggiorna ELO
	r.updateELO(result, ctx)

	// Notifica entrambi i client
	r.broadcast(OutboundMsg{
		Type: "game_over",
		Payload: GameOverPayload{
			Result: result,
			Reason: reason,
			PGN:    pgn,
		},
	})

	// Pulisci timer Redis
	DeleteTimer(ctx, r.rdb, r.gameID)

	// Ferma i timer disconnessione
	if r.whiteReconnectTimer != nil {
		r.whiteReconnectTimer.Stop()
	}
	if r.blackReconnectTimer != nil {
		r.blackReconnectTimer.Stop()
	}

	log.Printf("partita %s terminata: %s per %s", r.gameID, result, reason)
}

func (r *Room) updateELO(result string, ctx context.Context) {
	// Recupera ELO attuali
	var whiteElo, blackElo int
	var whiteID, blackID string

	err := r.pg.Pool.QueryRow(ctx,
		`SELECT g.white_id, g.black_id, u1.elo_rapid, u2.elo_rapid
		 FROM games g
		 JOIN users u1 ON u1.id = g.white_id
		 JOIN users u2 ON u2.id = g.black_id
		 WHERE g.id = $1`, r.gameID,
	).Scan(&whiteID, &blackID, &whiteElo, &blackElo)
	if err != nil {
		log.Printf("updateELO: errore lettura: %v", err)
		return
	}

	newWhiteElo, newBlackElo := calculateELO(whiteElo, blackElo, result)

	r.pg.Pool.Exec(ctx, `UPDATE users SET elo_rapid=$1 WHERE id=$2`, newWhiteElo, whiteID)
	r.pg.Pool.Exec(ctx, `UPDATE users SET elo_rapid=$1 WHERE id=$2`, newBlackElo, blackID)

	r.pg.Pool.Exec(ctx,
		`INSERT INTO elo_history (user_id, game_id, game_type, elo_before, elo_after)
		 VALUES ($1,$2,'rapid',$3,$4),($5,$2,'rapid',$6,$7)`,
		whiteID, r.gameID, whiteElo, newWhiteElo,
		blackID, blackElo, newBlackElo,
	)
}

// Algoritmo ELO standard (K=32)
func calculateELO(whiteElo, blackElo int, result string) (int, int) {
	const K = 32
	expected := func(a, b int) float64 {
		return 1.0 / (1.0 + pow10(float64(b-a)/400.0))
	}
	eW := expected(whiteElo, blackElo)
	eB := expected(blackElo, whiteElo)

	var sW, sB float64
	switch result {
	case "white":
		sW, sB = 1, 0
	case "black":
		sW, sB = 0, 1
	default:
		sW, sB = 0.5, 0.5
	}

	newW := whiteElo + int(K*(sW-eW))
	newB := blackElo + int(K*(sB-eB))
	return newW, newB
}

func pow10(x float64) float64 {
	return math.Pow(10, x)
}

func (r *Room) broadcast(msg OutboundMsg) {
	if r.white != nil {
		r.white.Send(msg)
	}
	if r.black != nil {
		r.black.Send(msg)
	}
}

func (r *Room) opponent(c *Client) *Client {
	if c.Color == "white" {
		return r.black
	}
	return r.white
}

func (r *Room) loadTimeControl() {
	var tc int
	err := r.pg.Pool.QueryRow(context.Background(),
		`SELECT time_control FROM games WHERE id = $1`, r.gameID,
	).Scan(&tc)
	if err != nil {
		tc = 600 // default 10 min
	}
	r.timeControl = tc
}

func methodToReason(m chess.Method) string {
	switch m {
	case chess.Checkmate:
		return "checkmate"
	case chess.Stalemate:
		return "stalemate"
	case chess.FiftyMoveRule:
		return "fifty_moves"
	case chess.ThreefoldRepetition:
		return "threefold"
	case chess.InsufficientMaterial:
		return "insufficient_material"
	default:
		return "unknown"
	}
}

func outcomeToResult(o chess.Outcome) string {
	switch o {
	case chess.WhiteWon:
		return "white"
	case chess.BlackWon:
		return "black"
	default:
		return "draw"
	}
}
