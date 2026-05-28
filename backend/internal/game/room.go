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

// Room gestisce una partita: stato, timer in-memory, messaggi
type Room struct {
	gameID string
	white  *Client
	black  *Client

	chess       *chess.Game
	timeControl int // secondi
	started     bool
	timer       timerState // in-memory, nessun Redis

	pg  *db.Postgres
	hub *Hub

	inbound            chan clientMessage
	clientDisconnected chan *Client

	whiteReconnectTimer *time.Timer
	blackReconnectTimer *time.Timer
}

func newRoom(gameID string, pg *db.Postgres, hub *Hub) *Room {
	return &Room{
		gameID:             gameID,
		chess:              chess.NewGame(),
		pg:                 pg,
		hub:                hub,
		inbound:            make(chan clientMessage, 32),
		clientDisconnected: make(chan *Client, 4),
	}
}

// Run è il loop principale della room (una goroutine per partita)
func (r *Room) Run() {
	defer r.hub.Remove(r.gameID)
	r.loadTimeControl()

	for {
		select {
		case cm := <-r.inbound:
			r.handleMessage(cm)
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

	if r.white != nil && r.black != nil && !r.started {
		r.startGame()
	}
	return nil
}

func (r *Room) startGame() {
	r.started = true
	r.timer = newTimerState(r.timeControl)

	ctx := context.Background()
	r.pg.Pool.Exec(ctx,
		`UPDATE games SET status = 'active', started_at = NOW() WHERE id = $1`,
		r.gameID,
	)

	fen := r.chess.Position().String()
	wMs, bMs := r.timer.currentTimes("white")

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

	turn := r.chess.Position().Turn()
	if (turn == chess.White && c.Color != "white") ||
		(turn == chess.Black && c.Color != "black") {
		c.Send(OutboundMsg{Type: "move_invalid", Payload: ErrorPayload{Reason: "non è il tuo turno"}})
		return
	}

	uci := p.From + p.To
	if p.Promotion != "" {
		uci += p.Promotion
	}

	move, err := chess.UCINotation{}.Decode(r.chess.Position(), uci)
	if err != nil {
		c.Send(OutboundMsg{Type: "move_invalid", Payload: ErrorPayload{Reason: "mossa illegale"}})
		return
	}
	if err := r.chess.Move(move); err != nil {
		c.Send(OutboundMsg{Type: "move_invalid", Payload: ErrorPayload{Reason: "mossa non valida"}})
		return
	}

	// Aggiorna timer in-memory
	wMs, bMs, timedOut, loser := r.timer.recordMove(c.Color)
	if timedOut {
		winner := "black"
		if loser == "black" {
			winner = "white"
		}
		r.endGame(winner, "timeout")
		return
	}

	newFen := r.chess.Position().String()
	pgn := r.chess.String()
	turnStr := "w"
	if r.chess.Position().Turn() == chess.Black {
		turnStr = "b"
	}

	r.broadcast(OutboundMsg{
		Type: "move_made",
		Payload: MoveMadePayload{
			From: p.From, To: p.To,
			FEN: newFen, PGN: pgn,
			Turn:    turnStr,
			WhiteMs: wMs, BlackMs: bMs,
		},
	})

	ctx := context.Background()
	r.pg.Pool.Exec(ctx, `UPDATE games SET pgn = $1 WHERE id = $2`, pgn, r.gameID)

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
	if opp := r.opponent(c); opp != nil {
		opp.Send(OutboundMsg{Type: "draw_offered"})
	}
}

func (r *Room) handleDrawResponse(c *Client, accepted bool) {
	if accepted {
		r.endGame("draw", "draw_agreed")
	} else {
		if opp := r.opponent(c); opp != nil {
			opp.Send(OutboundMsg{Type: "draw_declined"})
		}
	}
}

func (r *Room) handleDisconnect(c *Client) {
	log.Printf("client disconnesso: %s (%s)", c.UserID, c.Color)

	if opp := r.opponent(c); opp != nil {
		opp.Send(OutboundMsg{
			Type:    "opponent_disconnected",
			Payload: DisconnectPayload{TimeoutSeconds: int(disconnectTimeout.Seconds())},
		})
	}

	if c.Color == "white" {
		r.white = nil
	} else {
		r.black = nil
	}

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
	r.endGame(outcomeToResult(outcome), methodToReason(r.chess.Method()))
}

func (r *Room) endGame(result, reason string) {
	pgn := r.chess.String()
	ctx := context.Background()

	r.pg.Pool.Exec(ctx,
		`UPDATE games SET status='finished', result=$1, finish_reason=$2,
		 finished_at=NOW(), pgn=$3 WHERE id=$4`,
		result, reason, pgn, r.gameID,
	)

	r.updateELO(result, ctx)

	r.broadcast(OutboundMsg{
		Type: "game_over",
		Payload: GameOverPayload{Result: result, Reason: reason, PGN: pgn},
	})

	if r.whiteReconnectTimer != nil {
		r.whiteReconnectTimer.Stop()
	}
	if r.blackReconnectTimer != nil {
		r.blackReconnectTimer.Stop()
	}

	log.Printf("partita %s terminata: %s per %s", r.gameID, result, reason)
}

func (r *Room) updateELO(result string, ctx context.Context) {
	// Legge IDs, time_control e ELO della categoria corretta
	var whiteID, blackID string
	var timeControl int
	err := r.pg.Pool.QueryRow(ctx,
		`SELECT white_id, black_id, time_control FROM games WHERE id = $1`, r.gameID,
	).Scan(&whiteID, &blackID, &timeControl)
	if err != nil {
		log.Printf("updateELO: errore lettura game: %v", err)
		return
	}

	gameType := gameTypeFromTC(timeControl)
	eloCol := eloColFromType(gameType)

	var whiteElo, blackElo int
	r.pg.Pool.QueryRow(ctx, `SELECT `+eloCol+` FROM users WHERE id = $1`, whiteID).Scan(&whiteElo)
	r.pg.Pool.QueryRow(ctx, `SELECT `+eloCol+` FROM users WHERE id = $1`, blackID).Scan(&blackElo)

	// Numero di partite già giocate (per K-factor provvisorio)
	var whiteGames, blackGames int
	r.pg.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM elo_history WHERE user_id = $1 AND game_type = $2`,
		whiteID, gameType,
	).Scan(&whiteGames)
	r.pg.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM elo_history WHERE user_id = $1 AND game_type = $2`,
		blackID, gameType,
	).Scan(&blackGames)

	newWhiteElo, newBlackElo := calculateELO(whiteElo, blackElo, whiteGames, blackGames, result)

	r.pg.Pool.Exec(ctx, `UPDATE users SET `+eloCol+`=$1 WHERE id=$2`, newWhiteElo, whiteID)
	r.pg.Pool.Exec(ctx, `UPDATE users SET `+eloCol+`=$1 WHERE id=$2`, newBlackElo, blackID)
	r.pg.Pool.Exec(ctx,
		`INSERT INTO elo_history (user_id, game_id, game_type, elo_before, elo_after)
		 VALUES ($1,$2,$3,$4,$5),($6,$2,$3,$7,$8)`,
		whiteID, r.gameID, gameType, whiteElo, newWhiteElo,
		blackID, blackElo, newBlackElo,
	)

	log.Printf("ELO [%s] — bianco %d→%d (K=%d) | nero %d→%d (K=%d)",
		gameType,
		whiteElo, newWhiteElo, int(kFactor(whiteElo, whiteGames)),
		blackElo, newBlackElo, int(kFactor(blackElo, blackGames)),
	)
}

// gameTypeFromTC classifica il time control come bullet/blitz/rapid.
func gameTypeFromTC(tc int) string {
	switch {
	case tc <= 179:
		return "bullet"
	case tc <= 600:
		return "blitz"
	default:
		return "rapid"
	}
}

// eloColFromType restituisce il nome della colonna ELO nel DB.
func eloColFromType(gameType string) string {
	switch gameType {
	case "bullet":
		return "elo_bullet"
	case "blitz":
		return "elo_blitz"
	default:
		return "elo_rapid"
	}
}

// kFactor restituisce il K-factor chess.com per il giocatore.
//
//	< 25 partite    → K = 40  (periodo provvisorio, aggiustamento rapido)
//	ELO  < 2000     → K = 32  (giocatore normale)
//	ELO 2000–2399   → K = 24  (giocatore avanzato)
//	ELO ≥ 2400      → K = 16  (élite)
func kFactor(elo, games int) float64 {
	if games < 25 {
		return 40
	}
	if elo >= 2400 {
		return 16
	}
	if elo >= 2000 {
		return 24
	}
	return 32
}

// calculateELO calcola i nuovi rating dopo una partita.
// Formula standard ELO con K-factor dinamico e floor a 100.
//
// Expected score: E = 1 / (1 + 10^((Rb-Ra)/400))
// Nuovo rating:   R' = max(100, R + round(K * (score - expected)))
func calculateELO(whiteElo, blackElo, whiteGames, blackGames int, result string) (int, int) {
	expected := func(a, b int) float64 {
		return 1.0 / (1.0 + math.Pow(10, float64(b-a)/400.0))
	}

	eW := expected(whiteElo, blackElo)
	eB := expected(blackElo, whiteElo)

	kW := kFactor(whiteElo, whiteGames)
	kB := kFactor(blackElo, blackGames)

	var sW, sB float64
	switch result {
	case "white":
		sW, sB = 1, 0
	case "black":
		sW, sB = 0, 1
	default: // draw
		sW, sB = 0.5, 0.5
	}

	const floor = 100
	newW := max(floor, whiteElo+int(math.Round(kW*(sW-eW))))
	newB := max(floor, blackElo+int(math.Round(kB*(sB-eB))))
	return newW, newB
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
	if err := r.pg.Pool.QueryRow(context.Background(),
		`SELECT time_control FROM games WHERE id = $1`, r.gameID,
	).Scan(&tc); err != nil {
		tc = 600
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
