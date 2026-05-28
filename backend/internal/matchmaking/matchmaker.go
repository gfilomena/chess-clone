package matchmaking

import (
	"context"
	"log"
	"sort"
	"time"

	"chess-clone/backend/internal/db"
)

// Matchmaker gira come goroutine e abbina i giocatori ogni 2 secondi
type Matchmaker struct {
	pg  *db.Postgres
	rdb *db.Redis
}

func NewMatchmaker(pg *db.Postgres, rdb *db.Redis) *Matchmaker {
	return &Matchmaker{pg: pg, rdb: rdb}
}

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
	entries, err := GetAll(ctx, m.rdb)
	if err != nil || len(entries) < 2 {
		return
	}

	// Ordina per tempo di attesa (chi aspetta di più ha priorità)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].JoinedAt < entries[j].JoinedAt
	})

	now := time.Now().UnixMilli()
	matched := make(map[string]bool)

	for i, p1 := range entries {
		if matched[p1.UserID] {
			continue
		}

		// Calcola il range ELO in base al tempo di attesa
		waitMs := now - p1.JoinedAt
		eloRange := eloRangeForWait(waitMs)

		// Cerca il miglior avversario
		for j := i + 1; j < len(entries); j++ {
			p2 := entries[j]
			if matched[p2.UserID] {
				continue
			}

			diff := p1.ELO - p2.ELO
			if diff < 0 {
				diff = -diff
			}

			if diff <= eloRange {
				// Match trovato!
				gameID, err := m.createGame(ctx, p1, p2)
				if err != nil {
					log.Printf("matchmaker: errore creazione partita: %v", err)
					continue
				}

				if err := SetMatch(ctx, m.rdb, p1.UserID, p2.UserID, gameID); err != nil {
					log.Printf("matchmaker: errore SetMatch: %v", err)
					continue
				}

				log.Printf("Match! %s (ELO %d) vs %s (ELO %d) → partita %s",
					p1.UserID, p1.ELO, p2.UserID, p2.ELO, gameID)

				matched[p1.UserID] = true
				matched[p2.UserID] = true
				break
			}
		}
	}
}

// createGame crea la partita nel DB e restituisce l'ID
func (m *Matchmaker) createGame(ctx context.Context, p1, p2 QueueEntry) (string, error) {
	whiteID, blackID := determineColors(ctx, m.pg, p1.UserID, p2.UserID)

	var gameID string
	err := m.pg.Pool.QueryRow(ctx,
		`INSERT INTO games (white_id, black_id, status, time_control, increment)
		 VALUES ($1, $2, 'waiting', 600, 0)
		 RETURNING id`,
		whiteID, blackID,
	).Scan(&gameID)
	return gameID, err
}

// determineColors assegna il bianco a chi ha giocato meno partite come bianco di recente
func determineColors(ctx context.Context, pg *db.Postgres, user1ID, user2ID string) (white, black string) {
	var u1White, u2White int

	pg.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM games WHERE white_id = $1`, user1ID,
	).Scan(&u1White)

	pg.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM games WHERE white_id = $1`, user2ID,
	).Scan(&u2White)

	// Il bianco va a chi ha giocato meno volte come bianco
	if u1White <= u2White {
		return user1ID, user2ID
	}
	return user2ID, user1ID
}

// eloRangeForWait restituisce il range ELO accettabile in base ai ms di attesa
// 0-10s: ±100, 10-20s: ±200, 20-30s: ±300, 30-60s: ±500, 60s+: illimitato
func eloRangeForWait(waitMs int64) int {
	seconds := waitMs / 1000
	switch {
	case seconds < 10:
		return 100
	case seconds < 20:
		return 200
	case seconds < 30:
		return 300
	case seconds < 60:
		return 500
	default:
		return 9999 // qualsiasi avversario
	}
}
