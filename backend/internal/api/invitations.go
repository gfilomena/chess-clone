package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"chess-clone/backend/internal/db"
)

// Importiamo solo il necessario — matchmaking.GetMatch NON viene usato qui
// per evitare conflitti con /api/matchmaking/stream (entrambi userebbero GetDel)

const (
	inviteKeyPfx   = "invite:"
	inviteTTL      = 90 * time.Second
	friendMatchPfx = "friend_match:"
	friendMatchTTL = 60 * time.Second
)

// InvitePayload è il dato salvato in Redis e inviato al client via SSE
type InvitePayload struct {
	FromID       string `json:"from_id"`
	FromUsername string `json:"from_username"`
	FromElo      int    `json:"from_elo"`
}

type InvitationHandler struct {
	pg  *db.Postgres
	rdb *db.Redis
}

func NewInvitationHandler(pg *db.Postgres, rdb *db.Redis) *InvitationHandler {
	return &InvitationHandler{pg: pg, rdb: rdb}
}

func inviteRedisKey(toID, fromID string) string {
	return fmt.Sprintf("%s%s:%s", inviteKeyPfx, toID, fromID)
}

// POST /api/invitations
// Body: {"to_user_id": "..."}
func (h *InvitationHandler) SendInvite(w http.ResponseWriter, r *http.Request) {
	fromID, err := getUserIDFromCookie(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Non autenticato")
		return
	}

	var body struct {
		ToUserID string `json:"to_user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ToUserID == "" {
		writeError(w, http.StatusBadRequest, "INVALID_BODY", "to_user_id richiesto")
		return
	}
	if body.ToUserID == fromID {
		writeError(w, http.StatusBadRequest, "INVALID", "Non puoi invitarti da solo")
		return
	}

	var fromUsername string
	var fromElo int
	if err := h.pg.Pool.QueryRow(r.Context(),
		`SELECT username, elo_rapid FROM users WHERE id = $1`, fromID,
	).Scan(&fromUsername, &fromElo); err != nil {
		writeError(w, http.StatusInternalServerError, "SERVER_ERROR", "Errore interno")
		return
	}

	payload := InvitePayload{
		FromID:       fromID,
		FromUsername: fromUsername,
		FromElo:      fromElo,
	}
	raw, _ := json.Marshal(payload)
	h.rdb.Client.Set(r.Context(), inviteRedisKey(body.ToUserID, fromID), raw, inviteTTL)

	writeJSON(w, http.StatusOK, map[string]string{"status": "invited"})
}

// DELETE /api/invitations/{fromID}
// Rifiuta (o cancella) un invito ricevuto/inviato
func (h *InvitationHandler) DeclineInvite(w http.ResponseWriter, r *http.Request) {
	toID, err := getUserIDFromCookie(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Non autenticato")
		return
	}
	fromID := r.PathValue("fromID")
	h.rdb.Client.Del(r.Context(), inviteRedisKey(toID, fromID))
	writeJSON(w, http.StatusOK, map[string]string{"status": "declined"})
}

// POST /api/invitations/{fromID}/accept
// Accetta un invito → crea partita → notifica invitante tramite friend_match key
func (h *InvitationHandler) AcceptInvite(w http.ResponseWriter, r *http.Request) {
	toID, err := getUserIDFromCookie(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Non autenticato")
		return
	}
	fromID := r.PathValue("fromID")

	ctx := r.Context()
	key := inviteRedisKey(toID, fromID)

	// Atomically get & delete (evita race se due richieste arrivano insieme)
	raw, err := h.rdb.Client.GetDel(ctx, key).Result()
	if err != nil || raw == "" {
		writeError(w, http.StatusNotFound, "INVITE_NOT_FOUND", "Invito non trovato o scaduto")
		return
	}

	gameID, err := h.createFriendGame(ctx, fromID, toID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "SERVER_ERROR", "Errore creazione partita")
		return
	}

	// Notifica l'invitante: il suo SSE stream leggerà questo key e farà redirect
	h.rdb.Client.Set(ctx, friendMatchPfx+fromID, gameID, friendMatchTTL)

	// L'invitato riceve il game_id direttamente in risposta HTTP → redirect immediato
	writeJSON(w, http.StatusOK, map[string]string{"game_id": gameID})
}

func (h *InvitationHandler) createFriendGame(ctx context.Context, fromID, toID string) (string, error) {
	whiteID, blackID := determineFriendColors(ctx, h.pg, fromID, toID)
	var gameID string
	err := h.pg.Pool.QueryRow(ctx,
		`INSERT INTO games (white_id, black_id, status, time_control, increment)
		 VALUES ($1, $2, 'waiting', 600, 0) RETURNING id`,
		whiteID, blackID,
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

// GET /api/invitations/stream
// SSE sempre aperto nel layout:
//   - emette "invited" quando arriva un invito indirizzato a questo utente
//   - emette "matched" quando un invito inviato da questo utente viene accettato
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

	// Traccia gli inviti già notificati per evitare duplicati nella sessione
	notified := make(map[string]bool)

	for {
		select {
		case <-r.Context().Done():
			return

		case <-pingTicker.C:
			fmt.Fprintf(w, ": ping\n\n")
			flusher.Flush()

		case <-ticker.C:
			// 1. Controlla se qualcuno ha accettato un invito che abbiamo inviato
			//    (usa chiave diversa da matchmaking per evitare conflitti)
			if gameID, err := h.rdb.Client.GetDel(r.Context(), friendMatchPfx+myID).Result(); err == nil && gameID != "" {
				fmt.Fprintf(w, "event: matched\ndata: {\"game_id\":\"%s\"}\n\n", gameID)
				flusher.Flush()
				return
			}

			// 2. Controlla se abbiamo ricevuto nuovi inviti (pattern invite:{myID}:*)
			pattern := fmt.Sprintf("%s%s:*", inviteKeyPfx, myID)
			keys, err := h.rdb.Client.Keys(r.Context(), pattern).Result()
			if err != nil {
				continue
			}
			for _, key := range keys {
				if notified[key] {
					continue
				}
				raw, err := h.rdb.Client.Get(r.Context(), key).Result()
				if err != nil {
					continue
				}
				var payload InvitePayload
				if err := json.Unmarshal([]byte(raw), &payload); err != nil {
					continue
				}
				data, _ := json.Marshal(payload)
				fmt.Fprintf(w, "event: invited\ndata: %s\n\n", data)
				flusher.Flush()
				notified[key] = true
			}

			// 3. Rimuovi dal notified le chiavi che non esistono più (invito rifiutato/scaduto)
			for k := range notified {
				exists, _ := h.rdb.Client.Exists(r.Context(), k).Result()
				if exists == 0 {
					delete(notified, k)
				}
			}
		}
	}
}
