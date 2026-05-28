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
	rdb   *db.Redis
}

func NewHub(pg *db.Postgres, rdb *db.Redis) *Hub {
	return &Hub{
		rooms: make(map[string]*Room),
		pg:    pg,
		rdb:   rdb,
	}
}

// GetOrCreate restituisce la room esistente o ne crea una nuova
func (h *Hub) GetOrCreate(gameID string) *Room {
	h.mu.Lock()
	defer h.mu.Unlock()

	if room, ok := h.rooms[gameID]; ok {
		return room
	}

	room := newRoom(gameID, h.pg, h.rdb, h)
	h.rooms[gameID] = room
	go room.Run()

	return room
}

// Remove elimina una room terminata
func (h *Hub) Remove(gameID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.rooms, gameID)
}

// Count restituisce il numero di room attive (utile per monitoring)
func (h *Hub) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.rooms)
}
