package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"chess-clone/backend/internal/db"
	"chess-clone/backend/internal/game"
	"chess-clone/backend/internal/matchmaking"
)

type MatchmakingHandler struct {
	pg  *db.Postgres
	mm  *matchmaking.Matchmaker
	hub *game.Hub
}

func NewMatchmakingHandler(pg *db.Postgres, mm *matchmaking.Matchmaker, hub *game.Hub) *MatchmakingHandler {
	return &MatchmakingHandler{pg: pg, mm: mm, hub: hub}
}

// POST /api/matchmaking/join
// Body: { "time_control": 600, "increment": 0, "game_type": "rapid" }
func (h *MatchmakingHandler) Join(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserIDFromCookie(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Non autenticato")
		return
	}

	// Legge preferenze time control dal body (default: rapid 10 min)
	var body struct {
		TimeControl int    `json:"time_control"`
		Increment   int    `json:"increment"`
		GameType    string `json:"game_type"`
	}
	body.TimeControl = 600
	body.Increment = 0
	body.GameType = "rapid"
	json.NewDecoder(r.Body).Decode(&body) // errore ignorato → usa default

	// Valida game_type
	if body.GameType != "bullet" && body.GameType != "blitz" && body.GameType != "rapid" {
		body.GameType = "rapid"
	}

	// Legge l'ELO della categoria corretta
	eloCol := eloColumn(body.GameType)
	var elo int
	if err := h.pg.Pool.QueryRow(r.Context(),
		`SELECT `+eloCol+` FROM users WHERE id = $1`, userID,
	).Scan(&elo); err != nil {
		writeError(w, http.StatusInternalServerError, "SERVER_ERROR", "Errore interno")
		return
	}

	// Abbandona eventuale partita attiva prima di entrare in coda
	abandonActiveGame(r.Context(), h.pg, h.hub, userID)

	h.mm.Join(userID, elo, body.TimeControl, body.Increment, body.GameType)
	writeJSON(w, http.StatusOK, map[string]string{"status": "in_queue"})
}

// eloColumn restituisce il nome della colonna ELO per il game type dato.
func eloColumn(gameType string) string {
	switch gameType {
	case "bullet":
		return "elo_bullet"
	case "blitz":
		return "elo_blitz"
	default:
		return "elo_rapid"
	}
}

// DELETE /api/matchmaking/leave
func (h *MatchmakingHandler) Leave(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserIDFromCookie(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Non autenticato")
		return
	}
	h.mm.Leave(userID)
	writeJSON(w, http.StatusOK, map[string]string{"status": "left"})
}

// GET /api/matchmaking/status
func (h *MatchmakingHandler) Status(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserIDFromCookie(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Non autenticato")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"in_queue": h.mm.IsInQueue(userID)})
}

// GET /api/matchmaking/stream — SSE: notifica quando il match è pronto
func (h *MatchmakingHandler) Stream(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserIDFromCookie(r)
	if err != nil {
		http.Error(w, "non autenticato", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming non supportato", http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "event: connected\ndata: {}\n\n")
	flusher.Flush()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	pingTicker := time.NewTicker(15 * time.Second)
	defer pingTicker.Stop()

	for {
		select {
		case <-r.Context().Done():
			h.mm.Leave(userID)
			return

		case <-ticker.C:
			if gameID, found := h.mm.GetMatch(userID); found {
				fmt.Fprintf(w, "event: matched\ndata: {\"game_id\":\"%s\"}\n\n", gameID)
				flusher.Flush()
				return
			}

		case <-pingTicker.C:
			fmt.Fprintf(w, ": ping\n\n")
			flusher.Flush()
		}
	}
}
