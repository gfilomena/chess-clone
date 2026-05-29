package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"chess-clone/backend/internal/db"
	"chess-clone/backend/internal/game"
	"chess-clone/backend/internal/matchmaking"
)

// AdminHandler gestisce tutti gli endpoint /api/admin/*.
// Ogni route è protetta da RequireAdmin.
type AdminHandler struct {
	pg  *db.Postgres
	hub *game.Hub
	mm  *matchmaking.Matchmaker
	cfg *AdminConfig
}

func NewAdminHandler(
	pg *db.Postgres,
	hub *game.Hub,
	mm *matchmaking.Matchmaker,
	cfg *AdminConfig,
) *AdminHandler {
	return &AdminHandler{pg: pg, hub: hub, mm: mm, cfg: cfg}
}

// ── Middleware ─────────────────────────────────────────────────────────────────

// RequireAdmin avvolge un handler: JWT → email → AdminConfig.
// 401 se non autenticato, 403 se non admin.
func (h *AdminHandler) RequireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := getUserIDFromCookie(r)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Non autenticato")
			return
		}
		var email string
		if err := h.pg.Pool.QueryRow(r.Context(),
			`SELECT email FROM users WHERE id = $1`, userID,
		).Scan(&email); err != nil {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Utente non trovato")
			return
		}
		if !h.cfg.IsAdmin(email) {
			writeError(w, http.StatusForbidden, "FORBIDDEN", "Accesso negato")
			return
		}
		next(w, r)
	}
}

// ── GET /api/admin/stats ───────────────────────────────────────────────────────

func (h *AdminHandler) Stats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	botID := "00000000-0000-0000-0000-000000000000"

	var totalUsers int
	h.pg.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM users WHERE id != $1`, botID,
	).Scan(&totalUsers)

	var totalGames int
	h.pg.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM games WHERE white_id != $1 AND black_id != $1`, botID,
	).Scan(&totalGames)

	var onlineUsers int
	h.pg.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM users
		WHERE last_seen > NOW() - INTERVAL '5 minutes' AND id != $1`, botID,
	).Scan(&onlineUsers)

	// Partite per giorno — ultimi 7 giorni (per sparkline)
	type DayCount struct {
		Day   string `json:"day"`
		Count int    `json:"count"`
	}
	dailyGames := []DayCount{}
	dRows, err := h.pg.Pool.Query(ctx, `
		SELECT DATE(created_at)::text, COUNT(*)
		FROM games
		WHERE created_at > NOW() - INTERVAL '7 days'
		  AND white_id != $1 AND black_id != $1
		GROUP BY DATE(created_at) ORDER BY DATE(created_at)
	`, botID)
	if err == nil {
		defer dRows.Close()
		for dRows.Next() {
			var d DayCount
			dRows.Scan(&d.Day, &d.Count)
			dailyGames = append(dailyGames, d)
		}
	}

	// Distribuzione esiti delle partite finite
	finishReasons := map[string]int{}
	fRows, err := h.pg.Pool.Query(ctx, `
		SELECT COALESCE(finish_reason::text, 'unknown'), COUNT(*)
		FROM games
		WHERE status = 'finished' AND white_id != $1 AND black_id != $1
		GROUP BY finish_reason
	`, botID)
	if err == nil {
		defer fRows.Close()
		for fRows.Next() {
			var reason string
			var cnt int
			fRows.Scan(&reason, &cnt)
			finishReasons[reason] = cnt
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"total_users":    totalUsers,
		"total_games":    totalGames,
		"online_users":   onlineUsers,
		"active_rooms":   h.hub.Count(),
		"queue_size":     len(h.mm.QueueEntries()),
		"daily_games":    dailyGames,
		"finish_reasons": finishReasons,
	})
}

// ── GET /api/admin/users?q=&limit=&offset= ────────────────────────────────────

func (h *AdminHandler) Users(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	limit := adminQueryInt(r, "limit", 50, 1, 200)
	offset := adminQueryInt(r, "offset", 0, 0, 1<<30)
	botID := "00000000-0000-0000-0000-000000000000"

	base := `
		SELECT u.id, u.username, u.email, u.elo_rapid,
		       (SELECT COUNT(*) FROM games g
		        WHERE (g.white_id = u.id OR g.black_id = u.id)
		          AND g.white_id != $1 AND g.black_id != $1)::int,
		       u.created_at::text,
		       COALESCE(u.last_seen::text, ''),
		       COALESCE(u.is_banned, false)
		FROM users u WHERE u.id != $1`

	var query string
	var args []any
	if q != "" {
		query = base + ` AND (u.username ILIKE $2 OR u.email ILIKE $2)
			ORDER BY u.created_at DESC LIMIT $3 OFFSET $4`
		args = []any{botID, "%" + q + "%", limit, offset}
	} else {
		query = base + ` ORDER BY u.created_at DESC LIMIT $2 OFFSET $3`
		args = []any{botID, limit, offset}
	}

	rows, err := h.pg.Pool.Query(r.Context(), query, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "SERVER_ERROR", "Errore query utenti")
		return
	}
	defer rows.Close()

	type UserRow struct {
		ID         string `json:"id"`
		Username   string `json:"username"`
		Email      string `json:"email"`
		EloRapid   int    `json:"elo_rapid"`
		TotalGames int    `json:"total_games"`
		CreatedAt  string `json:"created_at"`
		LastSeen   string `json:"last_seen"`
		IsBanned   bool   `json:"is_banned"`
	}
	users := []UserRow{}
	for rows.Next() {
		var u UserRow
		rows.Scan(&u.ID, &u.Username, &u.Email, &u.EloRapid,
			&u.TotalGames, &u.CreatedAt, &u.LastSeen, &u.IsBanned)
		users = append(users, u)
	}
	writeJSON(w, http.StatusOK, users)
}

// ── PATCH /api/admin/users/:id ────────────────────────────────────────────────
//
// Body: { "action": "ban" | "unban" | "reset_elo" }

func (h *AdminHandler) PatchUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "MISSING_ID", "ID utente mancante")
		return
	}

	var body struct {
		Action string `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_BODY", "Body non valido")
		return
	}

	ctx := r.Context()
	switch body.Action {
	case "ban":
		h.pg.Pool.Exec(ctx, `UPDATE users SET is_banned = true  WHERE id = $1`, id)
	case "unban":
		h.pg.Pool.Exec(ctx, `UPDATE users SET is_banned = false WHERE id = $1`, id)
	case "reset_elo":
		h.pg.Pool.Exec(ctx,
			`UPDATE users SET elo_rapid = 800, elo_blitz = 800, elo_bullet = 800 WHERE id = $1`, id)
	default:
		writeError(w, http.StatusBadRequest, "INVALID_ACTION", "Azione non valida")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ── GET /api/admin/games?limit=&offset= ───────────────────────────────────────

func (h *AdminHandler) Games(w http.ResponseWriter, r *http.Request) {
	limit := adminQueryInt(r, "limit", 50, 1, 200)
	offset := adminQueryInt(r, "offset", 0, 0, 1<<30)

	rows, err := h.pg.Pool.Query(r.Context(), `
		SELECT g.id,
		       w.username,
		       b.username,
		       COALESCE(g.result::text, ''),
		       COALESCE(g.finish_reason::text, ''),
		       g.time_control,
		       g.created_at::text
		FROM games g
		JOIN users w ON w.id = g.white_id
		JOIN users b ON b.id = g.black_id
		WHERE g.white_id  != '00000000-0000-0000-0000-000000000000'
		  AND g.black_id  != '00000000-0000-0000-0000-000000000000'
		  AND g.status = 'finished'
		ORDER BY g.created_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "SERVER_ERROR", "Errore query partite")
		return
	}
	defer rows.Close()

	type GameRow struct {
		ID            string `json:"id"`
		WhiteUsername string `json:"white_username"`
		BlackUsername string `json:"black_username"`
		Result        string `json:"result"`
		FinishReason  string `json:"finish_reason"`
		TimeControl   int    `json:"time_control"`
		CreatedAt     string `json:"created_at"`
	}
	games := []GameRow{}
	for rows.Next() {
		var g GameRow
		rows.Scan(&g.ID, &g.WhiteUsername, &g.BlackUsername,
			&g.Result, &g.FinishReason, &g.TimeControl, &g.CreatedAt)
		games = append(games, g)
	}
	writeJSON(w, http.StatusOK, games)
}

// ── GET /api/admin/hub ────────────────────────────────────────────────────────

func (h *AdminHandler) Hub(w http.ResponseWriter, r *http.Request) {
	ids := h.hub.ActiveGameIDs()
	writeJSON(w, http.StatusOK, map[string]any{
		"active_rooms": len(ids),
		"game_ids":     ids,
	})
}

// ── GET /api/admin/queue ──────────────────────────────────────────────────────

func (h *AdminHandler) Queue(w http.ResponseWriter, r *http.Request) {
	entries := h.mm.QueueEntries()

	type QueueRow struct {
		UserID      string `json:"user_id"`
		ELO         int    `json:"elo"`
		TimeControl int    `json:"time_control"`
		GameType    string `json:"game_type"`
		WaitingSecs int    `json:"waiting_secs"`
	}
	out := make([]QueueRow, 0, len(entries))
	for _, e := range entries {
		out = append(out, QueueRow{
			UserID:      e.UserID,
			ELO:         e.ELO,
			TimeControl: e.TimeControl,
			GameType:    e.GameType,
			WaitingSecs: int(time.Since(e.JoinedAt).Seconds()),
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// ── DELETE /api/admin/queue ───────────────────────────────────────────────────

func (h *AdminHandler) ClearQueue(w http.ResponseWriter, r *http.Request) {
	n := h.mm.ClearQueue()
	writeJSON(w, http.StatusOK, map[string]int{"cleared": n})
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// adminQueryInt legge un query param intero con default e clamp min/max.
func adminQueryInt(r *http.Request, key string, def, min, max int) int {
	s := r.URL.Query().Get(key)
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < min {
		return min
	}
	if n > max {
		return max
	}
	return n
}
