package matchmaking

import (
	"context"
	"log"
	"sort"
	"time"

	"chess-clone/backend/internal/db"
)

// Matchmaker gira come goroutine e abbina i giocatori ogni 2 secondi.
// La coda è in-memory — nessun Redis necessario.
type Matchmaker struct {
	pg    *db.Postgres
	queue *queue
}

func NewMatchmaker(pg *db.Postgres) *Matchmaker {
	return &Matchmaker{pg: pg, queue: newQueue()}
}

// ── Metodi pubblici usati dall'API handler ─────────────────────────────────

func (m *Matchmaker) Join(userID string, elo, timeControl, increment int, gameType string) {
	m.queue.join(userID, elo, timeControl, increment, gameType)
}
func (m *Matchmaker) Leave(userID string)                   { m.queue.leave(userID) }
func (m *Matchmaker) IsInQueue(userID string) bool          { return m.queue.isInQueue(userID) }
func (m *Matchmaker) GetMatch(userID string) (string, bool) { return m.queue.getMatch(userID) }

// QueueEntries restituisce una copia snapshot delle entries in coda (per admin).
func (m *Matchmaker) QueueEntries() []QueueEntry { return m.queue.getAll() }

// ClearQueue svuota la coda e restituisce il numero di entry rimosse (per admin).
func (m *Matchmaker) ClearQueue() int { return m.queue.clear() }

// ── Loop principale ────────────────────────────────────────────────────────

func (m *Matchmaker) Run(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	log.Println("Matchmaker avviato")

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.tryMatch(ctx)
		}
	}
}

func (m *Matchmaker) tryMatch(ctx context.Context) {
	entries := m.queue.getAll()
	if len(entries) < 2 {
		return
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].JoinedAt.Before(entries[j].JoinedAt)
	})

	now := time.Now()
	matched := make(map[string]bool)

	for i, p1 := range entries {
		if matched[p1.UserID] {
			continue
		}

		eloRange := eloRangeForWait(now.Sub(p1.JoinedAt))

		for j := i + 1; j < len(entries); j++ {
			p2 := entries[j]
			if matched[p2.UserID] {
				continue
			}

			diff := p1.ELO - p2.ELO
			if diff < 0 {
				diff = -diff
			}

			// Stessa modalità (time control esatto) + ELO compatibile
			if p1.TimeControl == p2.TimeControl && p1.Increment == p2.Increment && diff <= eloRange {
				gameID, err := m.createGame(ctx, p1, p2)
				if err != nil {
					log.Printf("matchmaker: errore creazione partita: %v", err)
					continue
				}

				m.queue.setMatch(p1.UserID, p2.UserID, gameID)

				log.Printf("Match! %s (ELO %d) vs %s (ELO %d) → partita %s",
					p1.UserID, p1.ELO, p2.UserID, p2.ELO, gameID)

				matched[p1.UserID] = true
				matched[p2.UserID] = true
				break
			}
		}
	}
}

func (m *Matchmaker) createGame(ctx context.Context, p1, p2 QueueEntry) (string, error) {
	whiteID, blackID := determineColors(ctx, m.pg, p1.UserID, p2.UserID)

	var gameID string
	err := m.pg.Pool.QueryRow(ctx,
		`INSERT INTO games (white_id, black_id, status, time_control, increment)
		 VALUES ($1, $2, 'waiting', $3, $4) RETURNING id`,
		whiteID, blackID, p1.TimeControl, p1.Increment,
	).Scan(&gameID)
	return gameID, err
}

func determineColors(ctx context.Context, pg *db.Postgres, user1ID, user2ID string) (white, black string) {
	var u1White, u2White int
	pg.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM games WHERE white_id = $1`, user1ID).Scan(&u1White)
	pg.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM games WHERE white_id = $1`, user2ID).Scan(&u2White)
	if u1White <= u2White {
		return user1ID, user2ID
	}
	return user2ID, user1ID
}

func eloRangeForWait(wait time.Duration) int {
	switch {
	case wait < 10*time.Second:
		return 100
	case wait < 20*time.Second:
		return 200
	case wait < 30*time.Second:
		return 300
	case wait < 60*time.Second:
		return 500
	default:
		return 9999
	}
}
