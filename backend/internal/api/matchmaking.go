package api

import (
	"fmt"
	"net/http"
	"time"

	"chess-clone/backend/internal/db"
	"chess-clone/backend/internal/matchmaking"
)

type MatchmakingHandler struct {
	pg  *db.Postgres
	rdb *db.Redis
}

func NewMatchmakingHandler(pg *db.Postgres, rdb *db.Redis) *MatchmakingHandler {
	return &MatchmakingHandler{pg: pg, rdb: rdb}
}

// POST /api/matchmaking/join
// Aggiunge il giocatore alla coda rapid
func (h *MatchmakingHandler) Join(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserIDFromCookie(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Non autenticato")
		return
	}

	// Recupera ELO rapid del giocatore
	var elo int
	err = h.pg.Pool.QueryRow(r.Context(),
		`SELECT elo_rapid FROM users WHERE id = $1`, userID,
	).Scan(&elo)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "SERVER_ERROR", "Errore interno")
		return
	}

	if err := matchmaking.Join(r.Context(), h.rdb, userID, elo); err != nil {
		writeError(w, http.StatusInternalServerError, "SERVER_ERROR", "Errore join coda")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "in_queue"})
}

// DELETE /api/matchmaking/leave
// Rimuove il giocatore dalla coda
func (h *MatchmakingHandler) Leave(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserIDFromCookie(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Non autenticato")
		return
	}

	matchmaking.Leave(r.Context(), h.rdb, userID)
	writeJSON(w, http.StatusOK, map[string]string{"status": "left"})
}

// GET /api/matchmaking/status
// Restituisce se il giocatore è in coda
func (h *MatchmakingHandler) Status(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserIDFromCookie(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Non autenticato")
		return
	}

	inQueue, _ := matchmaking.IsInQueue(r.Context(), h.rdb, userID)
	writeJSON(w, http.StatusOK, map[string]any{"in_queue": inQueue})
}

// GET /api/matchmaking/stream
// SSE: il server notifica il client quando un match è trovato
func (h *MatchmakingHandler) Stream(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserIDFromCookie(r)
	if err != nil {
		http.Error(w, "non autenticato", http.StatusUnauthorized)
		return
	}

	// Headers SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Nginx: disabilita buffering

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming non supportato", http.StatusInternalServerError)
		return
	}

	// Invia evento iniziale (conferma connessione)
	fmt.Fprintf(w, "event: connected\ndata: {}\n\n")
	flusher.Flush()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	pingTicker := time.NewTicker(15 * time.Second)
	defer pingTicker.Stop()

	for {
		select {
		case <-r.Context().Done():
			// Client disconnesso — rimuovi dalla coda
			matchmaking.Leave(r.Context(), h.rdb, userID)
			return

		case <-ticker.C:
			// Controlla se c'è un match
			gameID, found := matchmaking.GetMatch(r.Context(), h.rdb, userID)
			if found {
				fmt.Fprintf(w, "event: matched\ndata: {\"game_id\":\"%s\"}\n\n", gameID)
				flusher.Flush()
				return
			}

		case <-pingTicker.C:
			// Keep-alive per evitare timeout proxy
			fmt.Fprintf(w, ": ping\n\n")
			flusher.Flush()
		}
	}
}
