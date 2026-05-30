package api

import (
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"

	"chess-clone/backend/internal/db"
	"chess-clone/backend/internal/game"
	"chess-clone/backend/internal/matchmaking"

	"github.com/rs/cors"
)

func NewRouter(pg *db.Postgres, mm *matchmaking.Matchmaker, staticFS fs.FS) http.Handler {
	mux := http.NewServeMux()
	hub := game.NewHub(pg)

	// AdminConfig — caricata da ADMIN_EMAILS una volta sola
	adminCfg := NewAdminConfig()
	if adminCfg.Empty() {
		log.Println("⚠️  ADMIN_EMAILS non configurato — pannello admin disabilitato")
	}

	// Health check
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Auth
	mailer := NewMailer()
	if mailer.devMode {
		log.Println("📧  SMTP non configurato — email di verifica stampate a stdout (dev mode)")
	}
	authHandler := NewAuthHandler(pg, adminCfg, mailer)
	mux.HandleFunc("POST /api/auth/register", authHandler.Register)
	mux.HandleFunc("POST /api/auth/login", authHandler.Login)
	mux.HandleFunc("POST /api/auth/logout", authHandler.Logout)
	mux.HandleFunc("GET /api/auth/me", authHandler.Me)
	mux.HandleFunc("POST /api/auth/verify-email", authHandler.VerifyEmail)
	mux.HandleFunc("POST /api/auth/resend-verification", authHandler.ResendVerification)

	if os.Getenv("DEV_MODE") == "true" {
		mux.HandleFunc("POST /api/auth/dev-login", authHandler.DevLogin)
		log.Println("⚠️  DEV_MODE attivo — /api/auth/dev-login abilitato")
	}

	// Google OAuth
	oauthHandler := NewOAuthHandler(pg)
	mux.HandleFunc("GET /api/auth/google", oauthHandler.RedirectToGoogle)
	mux.HandleFunc("GET /api/auth/google/callback", oauthHandler.Callback)

	// WebSocket partite
	wsHandler := NewWSHandler(hub, pg)
	mux.HandleFunc("GET /ws/game/{gameID}", wsHandler.HandleGameWS)

	// Matchmaking
	mmHandler := NewMatchmakingHandler(pg, mm, hub)
	mux.HandleFunc("POST /api/matchmaking/join", mmHandler.Join)
	mux.HandleFunc("DELETE /api/matchmaking/leave", mmHandler.Leave)
	mux.HandleFunc("GET /api/matchmaking/status", mmHandler.Status)
	mux.HandleFunc("GET /api/matchmaking/stream", mmHandler.Stream)

	// Partite
	gamesHandler := NewGamesHandler(pg, hub)
	mux.HandleFunc("GET /api/games/active", gamesHandler.GetActiveGame)
	mux.HandleFunc("GET /api/games/{id}", gamesHandler.GetGame)
	mux.HandleFunc("GET /api/games/{id}/pgn", gamesHandler.GetPGN)
	mux.HandleFunc("GET /api/users/{id}/games", gamesHandler.GetUserGames)
	mux.HandleFunc("POST /api/bot-games", gamesHandler.SaveBotGame)

	// Utenti
	usersHandler := NewUsersHandler(pg)
	mux.HandleFunc("GET /api/leaderboard", usersHandler.GetLeaderboard)
	mux.HandleFunc("GET /api/users/{id}", usersHandler.GetUser)
	mux.HandleFunc("GET /api/users/{id}/stats", usersHandler.GetStats)
	mux.HandleFunc("GET /api/users/{id}/elo-history", usersHandler.GetEloHistory)

	// Presenza online
	onlineHandler := NewOnlineHandler(pg)
	mux.HandleFunc("POST /api/users/heartbeat", onlineHandler.Heartbeat)
	mux.HandleFunc("GET /api/users/online", onlineHandler.GetOnlineUsers)

	// Inviti amico
	invHandler := NewInvitationHandler(pg, hub)
	mux.HandleFunc("POST /api/invitations", invHandler.SendInvite)
	mux.HandleFunc("DELETE /api/invitations/{fromID}", invHandler.DeclineInvite)
	mux.HandleFunc("POST /api/invitations/{fromID}/accept", invHandler.AcceptInvite)
	mux.HandleFunc("GET /api/invitations/stream", invHandler.Stream)

	// Admin
	adminHandler := NewAdminHandler(pg, hub, mm, adminCfg)
	ra := adminHandler.RequireAdmin
	mux.HandleFunc("GET /api/admin/stats",         ra(adminHandler.Stats))
	mux.HandleFunc("GET /api/admin/users",         ra(adminHandler.Users))
	mux.HandleFunc("PUT /api/admin/users/{id}",    ra(adminHandler.EditUser))
	mux.HandleFunc("DELETE /api/admin/users/{id}", ra(adminHandler.DeleteUser))
	mux.HandleFunc("PATCH /api/admin/users/{id}",  ra(adminHandler.PatchUser))
	mux.HandleFunc("GET /api/admin/games",         ra(adminHandler.Games))
	mux.HandleFunc("GET /api/admin/hub",           ra(adminHandler.Hub))
	mux.HandleFunc("GET /api/admin/queue",         ra(adminHandler.Queue))
	mux.HandleFunc("DELETE /api/admin/queue",      ra(adminHandler.ClearQueue))

	// Frontend SPA — catch-all finale
	mux.Handle("/", newSPAHandler(staticFS))

	// CORS
	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:5174"
	}
	c := cors.New(cors.Options{
		AllowedOrigins:   []string{frontendURL},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
	})

	return c.Handler(mux)
}

func newSPAHandler(fsys fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(fsys))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}

		f, err := fsys.Open(path)
		if err != nil {
			indexFile, indexErr := fsys.Open("index.html")
			if indexErr != nil {
				http.NotFound(w, r)
				return
			}
			indexFile.Close()
			r2 := r.Clone(r.Context())
			r2.URL.Path = "/"
			fileServer.ServeHTTP(w, r2)
			return
		}
		f.Close()
		fileServer.ServeHTTP(w, r)
	})
}
