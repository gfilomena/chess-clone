package api

import (
	"net/http"

	"chess-clone/backend/internal/db"
)

type UsersHandler struct {
	pg *db.Postgres
}

func NewUsersHandler(pg *db.Postgres) *UsersHandler {
	return &UsersHandler{pg: pg}
}

// GET /api/users/:id — profilo pubblico
func (h *UsersHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")

	var user struct {
		ID        string  `json:"id"`
		Username  string  `json:"username"`
		AvatarURL *string `json:"avatar_url"`
		EloRapid  int     `json:"elo_rapid"`
		EloBlitz  int     `json:"elo_blitz"`
		EloBullet int     `json:"elo_bullet"`
		CreatedAt string  `json:"created_at"`
		LastSeen  string  `json:"last_seen"`
	}

	err := h.pg.Pool.QueryRow(r.Context(), `
		SELECT id, username, avatar_url,
		       elo_rapid, elo_blitz, elo_bullet,
		       created_at::text, last_seen::text
		FROM users WHERE id = $1
	`, userID).Scan(
		&user.ID, &user.Username, &user.AvatarURL,
		&user.EloRapid, &user.EloBlitz, &user.EloBullet,
		&user.CreatedAt, &user.LastSeen,
	)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Utente non trovato")
		return
	}

	writeJSON(w, http.StatusOK, user)
}

// GET /api/users/:id/stats — statistiche W/L/D
func (h *UsersHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")

	var stats struct {
		Wins   int `json:"wins"`
		Losses int `json:"losses"`
		Draws  int `json:"draws"`
		Total  int `json:"total"`
	}

	// Conta wins (partite come bianco vinte + partite come nero vinte)
	h.pg.Pool.QueryRow(r.Context(), `
		SELECT COUNT(*) FROM games
		WHERE status = 'finished'
		  AND ((white_id = $1 AND result = 'white')
		    OR (black_id = $1 AND result = 'black'))
	`, userID).Scan(&stats.Wins)

	// Losses
	h.pg.Pool.QueryRow(r.Context(), `
		SELECT COUNT(*) FROM games
		WHERE status = 'finished'
		  AND ((white_id = $1 AND result = 'black')
		    OR (black_id = $1 AND result = 'white'))
	`, userID).Scan(&stats.Losses)

	// Draws
	h.pg.Pool.QueryRow(r.Context(), `
		SELECT COUNT(*) FROM games
		WHERE status = 'finished'
		  AND (white_id = $1 OR black_id = $1)
		  AND result = 'draw'
	`, userID).Scan(&stats.Draws)

	stats.Total = stats.Wins + stats.Losses + stats.Draws

	writeJSON(w, http.StatusOK, stats)
}

// GET /api/users/:id/elo-history — storico ELO per grafico
func (h *UsersHandler) GetEloHistory(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")

	rows, err := h.pg.Pool.Query(r.Context(), `
		SELECT elo_after, created_at::text
		FROM elo_history
		WHERE user_id = $1
		ORDER BY created_at ASC
		LIMIT 100
	`, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "SERVER_ERROR", "Errore query")
		return
	}
	defer rows.Close()

	type Point struct {
		ELO  int    `json:"elo"`
		Date string `json:"date"`
	}

	var history []Point
	for rows.Next() {
		var p Point
		rows.Scan(&p.ELO, &p.Date)
		history = append(history, p)
	}

	if history == nil {
		history = []Point{}
	}
	writeJSON(w, http.StatusOK, history)
}
