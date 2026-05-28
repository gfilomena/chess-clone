package main

import (
	"bufio"
	"context"
	"log"
	"net/http"
	"os"
	"strings"

	"chess-clone/backend/internal/api"
	"chess-clone/backend/internal/db"
	"chess-clone/backend/internal/matchmaking"
)

func main() {
	// Carica .env prima di tutto
	loadDotEnv(".env")

	// Config da variabili ambiente
	pgURL := getEnv("DATABASE_URL", "postgres://chess:chess_secret@localhost:5433/chessdb")
	redisURL := getEnv("REDIS_URL", "redis://localhost:6380")
	port := getEnv("PORT", "8080")

	// Connessioni DB
	pg, err := db.NewPostgres(pgURL)
	if err != nil {
		log.Fatalf("postgres connection failed: %v", err)
	}
	defer pg.Close()

	rdb, err := db.NewRedis(redisURL)
	if err != nil {
		log.Fatalf("redis connection failed: %v", err)
	}
	defer rdb.Close()

	// Matchmaker (goroutine in background)
	mm := matchmaking.NewMatchmaker(pg, rdb)
	go mm.Run(context.Background())

	// Router
	router := api.NewRouter(pg, rdb)

	log.Printf("Server avviato su :%s", port)
	if err := http.ListenAndServe(":"+port, router); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

// loadDotEnv legge un file .env e setta le variabili d'ambiente
// Non sovrascrive variabili già presenti nell'ambiente di sistema
func loadDotEnv(filename string) {
	f, err := os.Open(filename)
	if err != nil {
		return // .env opzionale, nessun errore se non esiste
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Salta commenti e righe vuote
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		// Non sovrascrive variabili già definite nel sistema
		if os.Getenv(key) == "" {
			os.Setenv(key, val)
		}
	}

	log.Println(".env caricato")
}
