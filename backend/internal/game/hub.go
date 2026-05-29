package game

import (
	"sync"

	"chess-clone/backend/internal/db"
)

// Hub gestisce tutte le room attive
type Hub struct {
	rooms map[string]*Room
	mu    sync.RWMutex
	pg    *db.Postgres
}

func NewHub(pg *db.Postgres) *Hub {
	return &Hub{
		rooms: make(map[string]*Room),
		pg:    pg,
	}
}

func (h *Hub) GetOrCreate(gameID string) *Room {
	h.mu.Lock()
	defer h.mu.Unlock()

	if room, ok := h.rooms[gameID]; ok {
		return room
	}

	room := newRoom(gameID, h.pg, h)
	h.rooms[gameID] = room
	go room.Run()

	return room
}

func (h *Hub) Remove(gameID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.rooms, gameID)
}

func (h *Hub) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.rooms)
}

// ForceEnd termina forzatamente una room (abbandono server-side).
// Thread-safe: usa un canale bufferizzato → non blocca mai.
func (h *Hub) ForceEnd(gameID, result, reason string) {
	h.mu.RLock()
	room, ok := h.rooms[gameID]
	h.mu.RUnlock()
	if !ok {
		return
	}
	select {
	case room.forceEnd <- forceEndMsg{result: result, reason: reason}:
	default: // canale già pieno — endGame già in corso
	}
}
