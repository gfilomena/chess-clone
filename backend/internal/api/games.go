package api

import (
	"net/http"

	"chess-clone/backend/internal/db"
)

type GamesHandler struct {
	pg *db.Postgres
}

func NewGamesHandler(pg *db.Postgres) *GamesHandler {
	return &GamesHandler{pg: pg}
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
