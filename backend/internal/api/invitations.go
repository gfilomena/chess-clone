package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"chess-clone/backend/internal/db"
	"chess-clone/backend/internal/game"
)

const (
	inviteTTL      = 90 * time.Second
	friendMatchTTL = 60 * time.Second
)

// InvitePayload è il dato dell'invito inviato via SSE
type InvitePayload struct {
	FromID       string `json:"from_id"`
	FromUsername string `json:"from_username"`
	FromElo      int    `json:"from_elo"`
	TimeControl  int    `json:"time_control"` // secondi
	Increment    int    `json:"increment"`    // secondi per mossa
}

type inviteEntry struct {
	key       string // "toID:fromID"
	payload   InvitePayload
	expiresAt time.Time
}

type friendMatchEntry struct {
	gameID    string
	expiresAt time.Time
}

// InvitationHandler gestisce gli inviti tra amici in memoria
type InvitationHandler struct {
	pg           *db.Postgres
	hub          *game.Hub
	mu           sync.RWMutex
	invites      map[string]inviteEntry      // key "toID:fromID" → entry
	friendMatch  map[string]friendMatchEntry // fromID → match pendente
}

func NewInvitationHandler(pg *db.Postgres, hub *game.Hub) *InvitationHandler {
	h := &InvitationHandler{
		pg:          pg,
		hub:         hub,
		invites:     make(map[string]inviteEntry),
		friendMatch: make(map[string]friendMatchEntry),
	}
	go h.cleanup()
	return h
}

func (h *InvitationHandler) cleanup() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		h.mu.Lock()
		for k, e := range h.invites {
			if now.After(e.expiresAt) {
				delete(h.invites, k)
			}
		}
		for k, e := range h.friendMatch {
			if now.After(e.expiresAt) {
				delete(h.friendMatch, k)
			}
		}
		h.mu.Unlock()
	}
}

func inviteKey(toID, fromID string) string {
	return fmt.Sprintf("%s:%s", toID, fromID)
}

// POST /api/invitations
func (h *InvitationHandler) SendInvite(w http.ResponseWriter, r *http.Request) {
	fromID, err := getUserIDFromCookie(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Non autenticato")
		return
	}

	var body struct {
		ToUserID    string `json:"to_user_id"`
		TimeControl int    `json:"time_control"`
		Increment   int    `json:"increment"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ToUserID == "" {
		writeError(w, http.StatusBadRequest, "INVALID_BODY", "to_user_id richiesto")
		return
	}
	if body.ToUserID == fromID {
		writeError(w, http.StatusBadRequest, "INVALID", "Non puoi invitarti da solo")
		return
	}
	// Default: Rapid 10 min
	if body.TimeControl <= 0 {
		body.TimeControl = 600
	}
	if body.Increment < 0 {
		body.Increment = 0
	}

	var fromUsername string
	var fromElo int
	if err := h.pg.Pool.QueryRow(r.Context(),
		`SELECT username, elo_rapid FROM users WHERE id = $1`, fromID,
	).Scan(&fromUsername, &fromElo); err != nil {
		writeError(w, http.StatusInternalServerError, "SERVER_ERROR", "Errore interno")
		return
	}

	// Abbandona eventuale partita attiva prima di inviare l'invito
	abandonActiveGame(r.Context(), h.pg, h.hub, fromID)

	key := inviteKey(body.ToUserID, fromID)
	h.mu.Lock()
	h.invites[key] = inviteEntry{
		key: key,
		payload: InvitePayload{
			FromID:       fromID,
			FromUsername: fromUsername,
			FromElo:      fromElo,
			TimeControl:  body.TimeControl,
			Increment:    body.Increment,
		},
		expiresAt: time.Now().Add(inviteTTL),
	}
	h.mu.Unlock()

	writeJSON(w, http.StatusOK, map[string]string{"status": "invited"})
}

// DELETE /api/invitations/{fromID}
func (h *InvitationHandler) DeclineInvite(w http.ResponseWriter, r *http.Request) {
	toID, err := getUserIDFromCookie(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Non autenticato")
		return
	}
	fromID := r.PathValue("fromID")
	h.mu.Lock()
	delete(h.invites, inviteKey(toID, fromID))
	h.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]string{"status": "declined"})
}

// POST /api/invitations/{fromID}/accept
func (h *InvitationHandler) AcceptInvite(w http.ResponseWriter, r *http.Request) {
	toID, err := getUserIDFromCookie(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Non autenticato")
		return
	}
	fromID := r.PathValue("fromID")
	key := inviteKey(toID, fromID)

	// Atomico: leggi e cancella
	h.mu.Lock()
	entry, ok := h.invites[key]
	if ok {
		delete(h.invites, key)
	}
	h.mu.Unlock()

	if !ok || time.Now().After(entry.expiresAt) {
		writeError(w, http.StatusNotFound, "INVITE_NOT_FOUND", "Invito non trovato o scaduto")
		return
	}

	// Abbandona eventuali partite attive di entrambi i giocatori
	abandonActiveGame(r.Context(), h.pg, h.hub, toID)
	abandonActiveGame(r.Context(), h.pg, h.hub, fromID)

	gameID, err := h.createFriendGame(r.Context(), fromID, toID, entry.payload.TimeControl, entry.payload.Increment)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "SERVER_ERROR", "Errore creazione partita")
		return
	}

	// Notifica l'invitante via SSE
	h.mu.Lock()
	h.friendMatch[fromID] = friendMatchEntry{gameID: gameID, expiresAt: time.Now().Add(friendMatchTTL)}
	h.mu.Unlock()

	writeJSON(w, http.StatusOK, map[string]string{"game_id": gameID})
}

func (h *InvitationHandler) createFriendGame(ctx context.Context, fromID, toID string, timeControl, increment int) (string, error) {
	whiteID, blackID := determineFriendColors(ctx, h.pg, fromID, toID)
	var gameID string
	err := h.pg.Pool.QueryRow(ctx,
		`INSERT INTO games (white_id, black_id, status, time_control, increment)
		 VALUES ($1, $2, 'waiting', $3, $4) RETURNING id`,
		whiteID, blackID, timeControl, increment,
	).Scan(&gameID)
	return gameID, err
}

func determineFriendColors(ctx context.Context, pg *db.Postgres, u1, u2 string) (white, black string) {
	var c1, c2 int
	pg.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM games WHERE white_id = $1`, u1).Scan(&c1)
	pg.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM games WHERE white_id = $1`, u2).Scan(&c2)
	if c1 <= c2 {
		return u1, u2
	}
	return u2, u1
}

// GET /api/invitations/stream — SSE sempre aperto
func (h *InvitationHandler) Stream(w http.ResponseWriter, r *http.Request) {
	myID, err := getUserIDFromCookie(r)
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

	// Chiavi già notificate in questa sessione (evita duplicati)
	notified := make(map[string]bool)

	for {
		select {
		case <-r.Context().Done():
			return

		case <-pingTicker.C:
			fmt.Fprintf(w, ": ping\n\n")
			flusher.Flush()

		case <-ticker.C:
			// 1. Match da invito accettato (io ero l'invitante)
			h.mu.Lock()
			fm, hasFM := h.friendMatch[myID]
			if hasFM {
				delete(h.friendMatch, myID)
			}
			h.mu.Unlock()

			if hasFM && !time.Now().After(fm.expiresAt) {
				fmt.Fprintf(w, "event: matched\ndata: {\"game_id\":\"%s\"}\n\n", fm.gameID)
				flusher.Flush()
				return
			}

			// 2. Inviti ricevuti (io sono il destinatario)
			now := time.Now()
			h.mu.RLock()
			toNotify := make([]inviteEntry, 0)
			for k, e := range h.invites {
				// Filtra per destinatario: la chiave è "myID:fromID"
				if len(k) > len(myID)+1 && k[:len(myID)] == myID && k[len(myID)] == ':' {
					if !notified[k] && !now.After(e.expiresAt) {
						toNotify = append(toNotify, e)
					}
				}
			}
			h.mu.RUnlock()

			for _, e := range toNotify {
				data, _ := json.Marshal(e.payload)
				fmt.Fprintf(w, "event: invited\ndata: %s\n\n", data)
				flusher.Flush()
				notified[e.key] = true
			}

			// 3. Rimuovi dal notified le chiavi scadute/cancellate
			h.mu.RLock()
			for k := range notified {
				if _, exists := h.invites[k]; !exists {
					delete(notified, k)
				}
			}
			h.mu.RUnlock()
		}
	}
}
