package api

import (
	"context"
	"encoding/json"
	"net/http"

	"chess-clone/backend/internal/db"
	"chess-clone/backend/internal/game"
)

// UUID fisso per l'utente-bot (nil UUID, non generato mai da uuid_generate_v4)
const botUserID = "00000000-0000-0000-0000-000000000000"

type GamesHandler struct {
	pg  *db.Postgres
	hub *game.Hub
}

func NewGamesHandler(pg *db.Postgres, hub *game.Hub) *GamesHandler {
	return &GamesHandler{pg: pg, hub: hub}
}

// GET /api/games/active — restituisce la partita attiva (non-bot) del giocatore corrente
func (h *GamesHandler) GetActiveGame(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserIDFromCookie(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Non autenticato")
		return
	}

	var gameID string
	err = h.pg.Pool.QueryRow(r.Context(), `
		SELECT id FROM games
		WHERE (white_id = $1 OR black_id = $1)
		  AND status IN ('waiting', 'active')
		  AND white_id != $2
		  AND black_id != $2
		ORDER BY created_at DESC
		LIMIT 1
	`, userID, botUserID).Scan(&gameID)

	if err != nil {
		// Nessuna partita attiva
		writeJSON(w, http.StatusOK, nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"game_id": gameID})
}

// abandonActiveGame cerca e abbandona la partita attiva (non-bot) dell'utente.
// Aggiorna il DB e notifica la room in-memory se presente.
// È una no-op se l'utente non ha partite attive.
func abandonActiveGame(ctx context.Context, pg *db.Postgres, hub *game.Hub, userID string) {
	var gameID, whiteID string
	var status string
	err := pg.Pool.QueryRow(ctx, `
		SELECT id, white_id, status FROM games
		WHERE (white_id = $1 OR black_id = $1)
		  AND status IN ('waiting', 'active')
		  AND white_id != $2
		  AND black_id != $2
		ORDER BY created_at DESC
		LIMIT 1
	`, userID, botUserID).Scan(&gameID, &whiteID, &status)
	if err != nil {
		return // nessuna partita attiva
	}

	// Chi abbandona perde: se white abbandona → vince black, e viceversa
	result := "white"
	if whiteID == userID {
		result = "black"
	}

	pg.Pool.Exec(ctx, `
		UPDATE games
		SET status='finished', result=$1, finish_reason='abandoned', finished_at=NOW()
		WHERE id=$2 AND status IN ('waiting','active')
	`, result, gameID)

	hub.ForceEnd(gameID, result, "abandoned")
}

// GET /api/games/:id
func (h *GamesHandler) GetGame(w http.ResponseWriter, r *http.Request) {
	gameID := r.PathValue("id")

	var game struct {
		ID             string  `json:"id"`
		WhiteID        string  `json:"white_id"`
		BlackID        string  `json:"black_id"`
		WhiteUsername  string  `json:"white_username"`
		BlackUsername  string  `json:"black_username"`
		WhiteEloRapid  int     `json:"white_elo"`
		BlackEloRapid  int     `json:"black_elo"`
		Result         *string `json:"result"`
		FinishReason   *string `json:"finish_reason"`
		TimeControl    int     `json:"time_control"`
		PGN            string  `json:"pgn"`
		StartedAt      *string `json:"started_at"`
		FinishedAt     *string `json:"finished_at"`
	}

	err := h.pg.Pool.QueryRow(r.Context(), `
		SELECT
			g.id, g.white_id, g.black_id,
			uw.username, ub.username,
			uw.elo_rapid, ub.elo_rapid,
			g.result, g.finish_reason,
			g.time_control, g.pgn,
			g.started_at::text, g.finished_at::text
		FROM games g
		JOIN users uw ON uw.id = g.white_id
		JOIN users ub ON ub.id = g.black_id
		WHERE g.id = $1
	`, gameID).Scan(
		&game.ID, &game.WhiteID, &game.BlackID,
		&game.WhiteUsername, &game.BlackUsername,
		&game.WhiteEloRapid, &game.BlackEloRapid,
		&game.Result, &game.FinishReason,
		&game.TimeControl, &game.PGN,
		&game.StartedAt, &game.FinishedAt,
	)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Partita non trovata")
		return
	}

	writeJSON(w, http.StatusOK, game)
}

// GET /api/games/:id/pgn — scarica PGN puro
func (h *GamesHandler) GetPGN(w http.ResponseWriter, r *http.Request) {
	gameID := r.PathValue("id")

	var pgn, whiteUser, blackUser string
	var result *string
	err := h.pg.Pool.QueryRow(r.Context(), `
		SELECT g.pgn, uw.username, ub.username, g.result
		FROM games g
		JOIN users uw ON uw.id = g.white_id
		JOIN users ub ON ub.id = g.black_id
		WHERE g.id = $1
	`, gameID).Scan(&pgn, &whiteUser, &blackUser, &result)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Partita non trovata")
		return
	}

	w.Header().Set("Content-Type", "application/x-chess-pgn")
	w.Header().Set("Content-Disposition", `attachment; filename="game.pgn"`)
	w.Write([]byte(pgn))
}

// GET /api/users/:id/games
func (h *GamesHandler) GetUserGames(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")

	rows, err := h.pg.Pool.Query(r.Context(), `
		SELECT
			g.id,
			uw.username AS white_username,
			ub.username AS black_username,
			g.white_id,
			g.black_id,
			g.result,
			g.finish_reason,
			g.time_control,
			g.finished_at::text,
			COALESCE(eh.elo_before, 0),
			COALESCE(eh.elo_after, 0)
		FROM games g
		JOIN users uw ON uw.id = g.white_id
		JOIN users ub ON ub.id = g.black_id
		LEFT JOIN elo_history eh ON eh.game_id = g.id AND eh.user_id = $1
		WHERE (g.white_id = $1 OR g.black_id = $1)
		  AND g.status = 'finished'
		ORDER BY g.finished_at DESC
		LIMIT 30
	`, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "SERVER_ERROR", "Errore query")
		return
	}
	defer rows.Close()

	type GameRow struct {
		ID            string  `json:"id"`
		WhiteUsername string  `json:"white_username"`
		BlackUsername string  `json:"black_username"`
		WhiteID       string  `json:"white_id"`
		BlackID       string  `json:"black_id"`
		Result        *string `json:"result"`
		FinishReason  *string `json:"finish_reason"`
		TimeControl   int     `json:"time_control"`
		FinishedAt    *string `json:"finished_at"`
		EloBefore     int     `json:"elo_before"`
		EloAfter      int     `json:"elo_after"`
	}

	var games []GameRow
	for rows.Next() {
		var g GameRow
		rows.Scan(
			&g.ID, &g.WhiteUsername, &g.BlackUsername,
			&g.WhiteID, &g.BlackID,
			&g.Result, &g.FinishReason,
			&g.TimeControl, &g.FinishedAt,
			&g.EloBefore, &g.EloAfter,
		)
		games = append(games, g)
	}

	if games == nil {
		games = []GameRow{}
	}
	writeJSON(w, http.StatusOK, games)
}

// POST /api/bot-games — salva una partita giocata contro il bot
// Le partite bot non generano variazioni ELO.
func (h *GamesHandler) SaveBotGame(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserIDFromCookie(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Non autenticato")
		return
	}

	var body struct {
		PGN          string `json:"pgn"`
		Outcome      string `json:"outcome"`       // "win" | "loss" | "draw"
		FinishReason string `json:"finish_reason"` // "checkmate" | "stalemate" | "threefold" | "resigned" | ""
		PlayerColor  string `json:"player_color"`  // "white" | "black"
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_BODY", "Richiesta non valida")
		return
	}

	// Determina white_id / black_id in base al colore del giocatore
	var whiteID, blackID string
	if body.PlayerColor == "white" {
		whiteID = userID
		blackID = botUserID
	} else {
		whiteID = botUserID
		blackID = userID
	}

	// Mappa outcome → enum game_result
	var dbResult string
	switch body.Outcome {
	case "win":
		if body.PlayerColor == "white" {
			dbResult = "white"
		} else {
			dbResult = "black"
		}
	case "loss":
		if body.PlayerColor == "white" {
			dbResult = "black"
		} else {
			dbResult = "white"
		}
	default:
		dbResult = "draw"
	}

	// Normalizza finish_reason (es. materiale insufficiente → draw_agreed)
	finishReason := body.FinishReason
	if finishReason == "" {
		finishReason = "draw_agreed"
	}

	var gameID string
	err = h.pg.Pool.QueryRow(r.Context(), `
		INSERT INTO games (white_id, black_id, status, result, finish_reason, pgn, started_at, finished_at)
		VALUES ($1, $2, 'finished', $3, $4, $5, NOW(), NOW())
		RETURNING id
	`, whiteID, blackID, dbResult, finishReason, body.PGN).Scan(&gameID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "SERVER_ERROR", "Errore salvataggio partita bot")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"game_id": gameID})
}
