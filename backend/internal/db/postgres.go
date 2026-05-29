package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Postgres struct {
	Pool *pgxpool.Pool
}

func NewPostgres(url string) (*Postgres, error) {
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		return nil, fmt.Errorf("unable to create pool: %w", err)
	}

	if err := pool.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("unable to ping postgres: %w", err)
	}

	pg := &Postgres{Pool: pool}
	pg.runBootstrapMigrations(context.Background())
	return pg, nil
}

// runBootstrapMigrations applica operazioni idempotenti necessarie all'avvio.
// Non sostituisce un migration runner completo, ma garantisce l'esistenza
// di dati di sistema come l'utente-bot e nuovi valori di enum.
func (p *Postgres) runBootstrapMigrations(ctx context.Context) {
	// Utente speciale che rappresenta il bot Stockfish nelle partite.
	// UUID fisso (nil UUID) — ON CONFLICT DO NOTHING → idempotente.
	p.Pool.Exec(ctx, `
		INSERT INTO users (id, username, email, elo_rapid, elo_blitz, elo_bullet)
		VALUES ('00000000-0000-0000-0000-000000000000', '(bot)', 'bot@chess.internal', 0, 0, 0)
		ON CONFLICT (id) DO NOTHING
	`)

	// Aggiunge il valore per patta per timeout con materiale insufficiente.
	// IF NOT EXISTS è idempotente (PostgreSQL 9.6+).
	p.Pool.Exec(ctx, `
		ALTER TYPE finish_reason ADD VALUE IF NOT EXISTS 'timeout_vs_insufficient_material'
	`)
}

func (p *Postgres) Close() {
	p.Pool.Close()
}
