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

	// Colonna per ban utenti — aggiunta idempotente (non esiste fino alla v1 admin).
	p.Pool.Exec(ctx, `
		ALTER TABLE users ADD COLUMN IF NOT EXISTS is_banned BOOLEAN DEFAULT FALSE
	`)

	// Colonna per verifica email — DEFAULT TRUE per non bloccare gli utenti esistenti.
	// Le nuove registrazioni email/password impostano esplicitamente is_verified=false.
	// Gli utenti Google ereditano il default TRUE (Google garantisce già l'email).
	p.Pool.Exec(ctx, `
		ALTER TABLE users ADD COLUMN IF NOT EXISTS is_verified BOOLEAN DEFAULT TRUE
	`)

	// Tabella token di verifica email (idempotente).
	p.Pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS email_verifications (
			token      TEXT        PRIMARY KEY,
			user_id    UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			expires_at TIMESTAMPTZ NOT NULL DEFAULT NOW() + INTERVAL '24 hours'
		)
	`)
}

func (p *Postgres) Close() {
	p.Pool.Close()
}
