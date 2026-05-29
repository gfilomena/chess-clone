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

type forceEndMsg struct {
	result string
	reason string
}

// Room gestisce una partita: stato, timer in-memory, messaggi
type Room struct {
	gameID string
	white  *Client
	black  *Client

	chess       *chess.Game
	timeControl int // secondi
	started     bool
	finished    bool       // impedisce double-endgame
	timer       timerState // in-memory, nessun Redis

	pg  *db.Postgres
	hub *Hub

	inbound            chan clientMessage
	clientDisconnected chan *Client
	forceEnd           chan forceEndMsg

	whiteReconnectTimer *time.Timer
	blackReconnectTimer *time.Timer
}

func newRoom(gameID string, pg *db.Postgres, hub *Hub) *Room {
	r := &Room{
		gameID:             gameID,
		chess:              chess.NewGame(),
		pg:                 pg,
		hub:                hub,
		inbound:            make(chan clientMessage, 32),
		clientDisconnected: make(chan *Client, 4),
		forceEnd:           make(chan forceEndMsg, 1),
	}
	// Carica il time control in modo sincrono prima che Join() possa
	// chiamare startGame(). Se invece lo si fa solo in Run() (goroutine
	// separata) c'è una race: Join() chiama startGame() con r.timeControl=0
	// (zero-value di Go) e il timer parte a zero.
	r.loadTimeControl()
	return r
}

// Run è il loop principale della room (una goroutine per partita)
func (r *Room) Run() {
	defer r.hub.Remove(r.gameID)

	for {
		select {
		case cm := <-r.inbound:
			r.handleMessage(cm)
		case c := <-r.clientDisconnected:
			r.handleDisconnect(c)
		case fe := <-r.forceEnd:
			r.endGame(fe.result, fe.reason)
			return
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
	case "flag":
		r.handleFlag(cm.client)
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
		r.handleTimeout(winner)
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

// handleFlag viene chiamato quando il client segnala che il proprio timer è scaduto.
// Verifica server-side con grace period prima di accettare il flag.
func (r *Room) handleFlag(c *Client) {
	if !r.started || r.finished {
		return
	}
	if r.chess.Outcome() != chess.NoOutcome {
		return
	}

	// Verifica server-side: accettiamo solo se il timer del giocatore è effettivamente
	// scaduto (con la grace period di 800 ms per compensare la latenza).
	turn := r.chess.Position().Turn()
	var activeTurn string
	if turn == chess.White {
		activeTurn = "white"
	} else {
		activeTurn = "black"
	}
	if c.Color != activeTurn {
		// Il flag arriva dal giocatore che non è di turno: ignora.
		return
	}
	timedOut, loser := r.timer.checkTimeout(activeTurn)
	if !timedOut {
		return
	}

	winner := "black"
	if loser == "black" {
		winner = "white"
	}
	r.handleTimeout(winner)
}

// handleTimeout applica le regole chess.com:
// se l'avversario non ha materiale sufficiente per dare scacco matto → patta,
// altrimenti → vittoria per timeout.
func (r *Room) handleTimeout(winner string) {
	if r.finished {
		return
	}
	pos := r.chess.Position()
	var winnerColor chess.Color
	if winner == "white" {
		winnerColor = chess.White
	} else {
		winnerColor = chess.Black
	}

	if hasSufficientMatingMaterial(pos, winnerColor) {
		r.endGame(winner, "timeout")
	} else {
		r.endGame("draw", "timeout_vs_insufficient_material")
	}
}

// hasSufficientMatingMaterial restituisce true se winnerColor ha abbastanza materiale
// per dare potenzialmente scacco matto, secondo le regole chess.com:
//
//   - Solo Re                              → no
//   - Re + Cavallo                         → no
//   - Re + Alfiere                         → no
//   - Re + 2 Cavalli (loser senza pedoni)  → no  (con pedoni → sì, teoricamente)
//   - Qualsiasi altro materiale            → sì
func hasSufficientMatingMaterial(pos *chess.Position, winnerColor chess.Color) bool {
	board := pos.Board()

	loserColor := chess.Black
	if winnerColor == chess.Black {
		loserColor = chess.White
	}

	var wN, wB, wR, wQ, wP int
	var lP int

	for sqIdx := 0; sqIdx < 64; sqIdx++ {
		p := board.Piece(chess.Square(sqIdx))
		if p == chess.NoPiece {
			continue
		}
		if p.Color() == winnerColor {
			switch p.Type() {
			case chess.Knight:
				wN++
			case chess.Bishop:
				wB++
			case chess.Rook:
				wR++
			case chess.Queen:
				wQ++
			case chess.Pawn:
				wP++
			}
		} else if p.Color() == loserColor && p.Type() == chess.Pawn {
			lP++
		}
	}

	// Torre, Donna o Pedone del vincitore → sempre sufficiente
	if wR > 0 || wQ > 0 || wP > 0 {
		return true
	}

	minor := wN + wB
	switch {
	case minor == 0:
		// Solo Re → non sufficiente
		return false
	case minor == 1:
		// Re + pezzo minore (N o B) → non sufficiente
		return false
	case wN == 2 && wB == 0:
		// Re + 2 Cavalli: sufficiente solo se il perdente ha ancora pedoni
		return lP > 0
	default:
		// 2+ pezzi minori con almeno un Alfiere, o 3+ Cavalli → sufficiente
		return true
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
	if r.finished {
		return
	}
	r.finished = true

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
