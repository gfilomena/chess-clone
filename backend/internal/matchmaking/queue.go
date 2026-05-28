package matchmaking

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"chess-clone/backend/internal/db"
)

const (
	queueKey    = "queue:rapid"
	matchPrefix = "match:"
	matchTTL    = 60 * time.Second
	queueTTL    = 5 * time.Minute
)

// QueueEntry rappresenta un giocatore in attesa
type QueueEntry struct {
	UserID   string `json:"user_id"`
	ELO      int    `json:"elo"`
	JoinedAt int64  `json:"joined_at"` // unix ms
}

// Join aggiunge un utente alla coda rapid
func Join(ctx context.Context, rdb *db.Redis, userID string, elo int) error {
	entry := QueueEntry{
		UserID:   userID,
		ELO:      elo,
		JoinedAt: time.Now().UnixMilli(),
	}
	raw, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	// Usiamo un Hash: field=userID, value=JSON entry
	pipe := rdb.Client.Pipeline()
	pipe.HSet(ctx, queueKey, userID, raw)
	// TTL dedicato per auto-pulizia se il client non fa leave
	pipe.Set(ctx, queueTTLKey(userID), "1", queueTTL)
	_, err = pipe.Exec(ctx)
	return err
}

// Leave rimuove un utente dalla coda
func Leave(ctx context.Context, rdb *db.Redis, userID string) error {
	pipe := rdb.Client.Pipeline()
	pipe.HDel(ctx, queueKey, userID)
	pipe.Del(ctx, queueTTLKey(userID))
	_, err := pipe.Exec(ctx)
	return err
}

// IsInQueue verifica se l'utente è in coda
func IsInQueue(ctx context.Context, rdb *db.Redis, userID string) (bool, error) {
	exists, err := rdb.Client.HExists(ctx, queueKey, userID).Result()
	return exists, err
}

// GetAll restituisce tutti i giocatori in coda (pulendo gli scaduti)
func GetAll(ctx context.Context, rdb *db.Redis) ([]QueueEntry, error) {
	raw, err := rdb.Client.HGetAll(ctx, queueKey).Result()
	if err != nil {
		return nil, err
	}

	var entries []QueueEntry
	var expired []string

	for userID, val := range raw {
		var entry QueueEntry
		if err := json.Unmarshal([]byte(val), &entry); err != nil {
			continue
		}
		// Rimuovi entrate scadute (TTL key non esiste più)
		ttlExists, _ := rdb.Client.Exists(ctx, queueTTLKey(userID)).Result()
		if ttlExists == 0 {
			expired = append(expired, userID)
			continue
		}
		entries = append(entries, entry)
	}

	// Pulizia asincrona degli scaduti
	if len(expired) > 0 {
		rdb.Client.HDel(ctx, queueKey, expired...)
	}

	return entries, nil
}

// SetMatch salva l'abbinamento per entrambi i giocatori
func SetMatch(ctx context.Context, rdb *db.Redis, userID1, userID2, gameID string) error {
	pipe := rdb.Client.Pipeline()
	pipe.Set(ctx, matchPrefix+userID1, gameID, matchTTL)
	pipe.Set(ctx, matchPrefix+userID2, gameID, matchTTL)
	// Rimuovi dalla coda
	pipe.HDel(ctx, queueKey, userID1, userID2)
	pipe.Del(ctx, queueTTLKey(userID1), queueTTLKey(userID2))
	_, err := pipe.Exec(ctx)
	return err
}

// GetMatch controlla se c'è un match pronto per l'utente
func GetMatch(ctx context.Context, rdb *db.Redis, userID string) (string, bool) {
	gameID, err := rdb.Client.GetDel(ctx, matchPrefix+userID).Result()
	if err != nil || gameID == "" {
		return "", false
	}
	return gameID, true
}

func queueTTLKey(userID string) string {
	return fmt.Sprintf("queue:ttl:%s", userID)
}
