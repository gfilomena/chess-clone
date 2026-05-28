package api

import (
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"

	"chess-clone/backend/internal/db"
	"chess-clone/backend/internal/game"

	"github.com/rs/cors"
)

func NewRouter(pg *db.Postgres, rdb *db.Redis, staticFS fs.FS) http.Handler {
	mux := http.NewServeMux()
	hub := game.NewHub(pg, rdb)

	// Health check
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Auth email/password
	authHandler := NewAuthHandler(pg, rdb)
	mux.HandleFunc("POST /api/auth/register", authHandler.Register)
	mux.HandleFunc("POST /api/auth/login", authHandler.Login)
	mux.HandleFunc("POST /api/auth/logout", authHandler.Logout)
	mux.HandleFunc("GET /api/auth/me", authHandler.Me)

	// Dev login — solo se DEV_MODE=true (nessuna password)
	if os.Getenv("DEV_MODE") == "true" {
		mux.HandleFunc("POST /api/auth/dev-login", authHandler.DevLogin)
		log.Println("⚠️  DEV_MODE attivo — /api/auth/dev-login abilitato (non usare in produzione)")
	}

	// Auth Google OAuth
	oauthHandler := NewOAuthHandler(pg)
	mux.HandleFunc("GET /api/auth/google", oauthHandler.RedirectToGoogle)
	mux.HandleFunc("GET /api/auth/google/callback", oauthHandler.Callback)

	// WebSocket partite
	wsHandler := NewWSHandler(hub, pg, rdb)
	mux.HandleFunc("GET /ws/game/{gameID}", wsHandler.HandleGameWS)

	// Matchmaking
	mmHandler := NewMatchmakingHandler(pg, rdb)
	mux.HandleFunc("POST /api/matchmaking/join", mmHandler.Join)
	mux.HandleFunc("DELETE /api/matchmaking/leave", mmHandler.Leave)
	mux.HandleFunc("GET /api/matchmaking/status", mmHandler.Status)
	mux.HandleFunc("GET /api/matchmaking/stream", mmHandler.Stream)

	// Partite
	gamesHandler := NewGamesHandler(pg)
	mux.HandleFunc("GET /api/games/{id}", gamesHandler.GetGame)
	mux.HandleFunc("GET /api/games/{id}/pgn", gamesHandler.GetPGN)
	mux.HandleFunc("GET /api/users/{id}/games", gamesHandler.GetUserGames)

	// Utenti
	usersHandler := NewUsersHandler(pg)
	mux.HandleFunc("GET /api/users/{id}", usersHandler.GetUser)
	mux.HandleFunc("GET /api/users/{id}/stats", usersHandler.GetStats)
	mux.HandleFunc("GET /api/users/{id}/elo-history", usersHandler.GetEloHistory)

	// Presenza online
	onlineHandler := NewOnlineHandler(pg, rdb)
	mux.HandleFunc("POST /api/users/heartbeat", onlineHandler.Heartbeat)
	mux.HandleFunc("GET /api/users/online", onlineHandler.GetOnlineUsers)

	// Inviti amico
	invHandler := NewInvitationHandler(pg, rdb)
	mux.HandleFunc("POST /api/invitations", invHandler.SendInvite)
	mux.HandleFunc("DELETE /api/invitations/{fromID}", invHandler.DeclineInvite)
	mux.HandleFunc("POST /api/invitations/{fromID}/accept", invHandler.AcceptInvite)
	mux.HandleFunc("GET /api/invitations/stream", invHandler.Stream)

	// Frontend SPA — fallback catch-all (deve essere l'ultimo)
	mux.Handle("/", newSPAHandler(staticFS))

	// CORS — legge FRONTEND_URL da env
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

// newSPAHandler serve i file statici del build SvelteKit.
// Se il file non esiste (route SPA tipo /game/xyz) serve index.html.
// Se l'FS è vuoto (sviluppo locale) risponde 204 senza errori.
func newSPAHandler(fsys fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(fsys))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Normalizza il path
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}

		// Controlla se il file esiste nel FS embedded
		f, err := fsys.Open(path)
		if err != nil {
			// File non trovato → SPA fallback: servi index.html
			indexFile, indexErr := fsys.Open("index.html")
			if indexErr != nil {
				// FS vuoto (sviluppo): nessuna risposta statica
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
