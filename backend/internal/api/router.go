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

	// Health check
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Auth
	authHandler := NewAuthHandler(pg)
	mux.HandleFunc("POST /api/auth/register", authHandler.Register)
	mux.HandleFunc("POST /api/auth/login", authHandler.Login)
	mux.HandleFunc("POST /api/auth/logout", authHandler.Logout)
	mux.HandleFunc("GET /api/auth/me", authHandler.Me)

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
	mmHandler := NewMatchmakingHandler(pg, mm)
	mux.HandleFunc("POST /api/matchmaking/join", mmHandler.Join)
	mux.HandleFunc("DELETE /api/matchmaking/leave", mmHandler.Leave)
	mux.HandleFunc("GET /api/matchmaking/status", mmHandler.Status)
	mux.HandleFunc("GET /api/matchmaking/stream", mmHandler.Stream)

	// Partite
	gamesHandler := NewGamesHandler(pg)
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
	invHandler := NewInvitationHandler(pg)
	mux.HandleFunc("POST /api/invitations", invHandler.SendInvite)
	mux.HandleFunc("DELETE /api/invitations/{fromID}", invHandler.DeclineInvite)
	mux.HandleFunc("POST /api/invitations/{fromID}/accept", invHandler.AcceptInvite)
	mux.HandleFunc("GET /api/invitations/stream", invHandler.Stream)

	// Frontend SPA — catch-all finale
	mux.Handle("/", newSPAHandler(staticFS))

	// CORS
	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:5174"
	}
	c := cors.New(cors.Options{
		AllowedOrigins:   []string{frontendURL},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
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
