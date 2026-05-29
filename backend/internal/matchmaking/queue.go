package matchmaking

import (
	"sync"
	"time"
)

// QueueEntry rappresenta un giocatore in attesa
type QueueEntry struct {
	UserID      string
	ELO         int
	TimeControl int    // secondi base (es. 600 = 10 min)
	Increment   int    // secondi di incremento per mossa
	GameType    string // "bullet" | "blitz" | "rapid"
	JoinedAt    time.Time
}

// queue è la coda in-memory condivisa tra Matchmaker e MatchmakingHandler
type queue struct {
	mu      sync.Mutex
	entries map[string]QueueEntry // userID → entry
	matches map[string]string     // userID → gameID (match pronto, non ancora letto)
}

func newQueue() *queue {
	return &queue{
		entries: make(map[string]QueueEntry),
		matches: make(map[string]string),
	}
}

func (q *queue) join(userID string, elo, timeControl, increment int, gameType string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.entries[userID] = QueueEntry{
		UserID:      userID,
		ELO:         elo,
		TimeControl: timeControl,
		Increment:   increment,
		GameType:    gameType,
		JoinedAt:    time.Now(),
	}
}

func (q *queue) leave(userID string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	delete(q.entries, userID)
}

func (q *queue) isInQueue(userID string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	_, ok := q.entries[userID]
	return ok
}

func (q *queue) getAll() []QueueEntry {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := make([]QueueEntry, 0, len(q.entries))
	for _, e := range q.entries {
		out = append(out, e)
	}
	return out
}

func (q *queue) setMatch(userID1, userID2, gameID string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.matches[userID1] = gameID
	q.matches[userID2] = gameID
	delete(q.entries, userID1)
	delete(q.entries, userID2)
}

// clear svuota tutta la coda e restituisce il numero di entry rimosse (admin).
func (q *queue) clear() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	n := len(q.entries)
	q.entries = make(map[string]QueueEntry)
	return n
}

// getMatch legge e cancella atomicamente il match (usato dall'SSE del client)
func (q *queue) getMatch(userID string) (string, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	gameID, ok := q.matches[userID]
	if ok {
		delete(q.matches, userID)
	}
	return gameID, ok
}
