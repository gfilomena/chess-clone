package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"chess-clone/backend/internal/db"

	"github.com/redis/go-redis/v9"
)

const (
	onlineZSet    = "online_users"
	onlineInfoPfx = "online_info:"
	onlineTTL     = 45 * time.Second
	onlineWindow  = 45 // seconds
)

type OnlineHandler struct {
	pg  *db.Postgres
	rdb *db.Redis
}

func NewOnlineHandler(pg *db.Postgres, rdb *db.Redis) *OnlineHandler {
	return &OnlineHandler{pg: pg, rdb: rdb}
}

// POST /api/users/heartbeat
// Aggiorna la presenza online dell'utente in Redis (ZSET + info cache)
func (h *OnlineHandler) Heartbeat(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserIDFromCookie(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Non autenticato")
		return
	}

	var info struct {
		ID       string `json:"id"`
		Username string `json:"username"`
		EloRapid int    `json:"elo_rapid"`
	}
	if err := h.pg.Pool.QueryRow(r.Context(),
		`SELECT id, username, elo_rapid FROM users WHERE id = $1`, userID,
	).Scan(&info.ID, &info.Username, &info.EloRapid); err != nil {
		writeError(w, http.StatusInternalServerError, "SERVER_ERROR", "Errore interno")
		return
	}

	raw, _ := json.Marshal(info)
	now := float64(time.Now().Unix())

	pipe := h.rdb.Client.Pipeline()
	pipe.ZAdd(r.Context(), onlineZSet, redis.Z{Score: now, Member: userID})
	pipe.Set(r.Context(), onlineInfoPfx+userID, raw, onlineTTL)
	_, _ = pipe.Exec(r.Context())

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// upsertOnline aggiorna la presenza di un utente in Redis.
// Riusato da GetOnlineUsers per eliminare la race condition heartbeat/fetch.
func (h *OnlineHandler) upsertOnline(ctx context.Context, userID string) {
	var info struct {
		ID       string `json:"id"`
		Username string `json:"username"`
		EloRapid int    `json:"elo_rapid"`
	}
	if err := h.pg.Pool.QueryRow(ctx,
		`SELECT id, username, elo_rapid FROM users WHERE id = $1`, userID,
	).Scan(&info.ID, &info.Username, &info.EloRapid); err != nil {
		return
	}
	raw, _ := json.Marshal(info)
	now := float64(time.Now().Unix())
	pipe := h.rdb.Client.Pipeline()
	pipe.ZAdd(ctx, onlineZSet, redis.Z{Score: now, Member: userID})
	pipe.Set(ctx, onlineInfoPfx+userID, raw, onlineTTL)
	_, _ = pipe.Exec(ctx)
}

// GET /api/users/online
// Registra il chiamante come online e restituisce gli altri utenti attivi
// negli ultimi 45 secondi. Chiamarlo equivale anche a fare un heartbeat.
func (h *OnlineHandler) GetOnlineUsers(w http.ResponseWriter, r *http.Request) {
	myID, err := getUserIDFromCookie(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Non autenticato")
		return
	}

	ctx := r.Context()

	// Auto-heartbeat: registra il chiamante prima di leggere la lista,
	// così anche il primo fetch non soffre di race condition col heartbeat.
	h.upsertOnline(ctx, myID)

	minScore := fmt.Sprintf("%d", time.Now().Unix()-onlineWindow)

	members, err := h.rdb.Client.ZRangeByScore(ctx, onlineZSet, &redis.ZRangeBy{
		Min: minScore,
		Max: "+inf",
	}).Result()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "SERVER_ERROR", "Errore interno")
		return
	}

	type UserOnline struct {
		ID       string `json:"id"`
		Username string `json:"username"`
		EloRapid int    `json:"elo_rapid"`
	}

	users := make([]UserOnline, 0)
	for _, uid := range members {
		if uid == myID {
			continue
		}
		raw, err := h.rdb.Client.Get(ctx, onlineInfoPfx+uid).Result()
		if err != nil {
			continue
		}
		var u UserOnline
		if err := json.Unmarshal([]byte(raw), &u); err != nil {
			continue
		}
		users = append(users, u)
	}

	writeJSON(w, http.StatusOK, users)
}
