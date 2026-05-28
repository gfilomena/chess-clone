package api

import (
	"log"
	"net/http"

	"chess-clone/backend/internal/db"
	"chess-clone/backend/internal/game"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// In produzione: controlla l'origine
	CheckOrigin: func(r *http.Request) bool { return true },
}

type WSHandler struct {
	hub *game.Hub
	pg  *db.Postgres
	rdb *db.Redis
}

func NewWSHandler(hub *game.Hub, pg *db.Postgres, rdb *db.Redis) *WSHandler {
	return &WSHandler{hub: hub, pg: pg, rdb: rdb}
}

// GET /ws/game/{gameID}
func (h *WSHandler) HandleGameWS(w http.ResponseWriter, r *http.Request) {
	gameID := r.PathValue("gameID")
	if gameID == "" {
		http.Error(w, "gameID mancante", http.StatusBadRequest)
		return
	}

	// Verifica JWT
	userID, err := getUserIDFromCookie(r)
	if err != nil {
		http.Error(w, "non autenticato", http.StatusUnauthorized)
		return
	}

	// Carica partita dal DB e verifica che l'utente ne faccia parte
	var whiteID, blackID string
	err = h.pg.Pool.QueryRow(r.Context(),
		`SELECT white_id, black_id FROM games WHERE id = $1 AND status != 'finished'`,
		gameID,
	).Scan(&whiteID, &blackID)
	if err != nil {
		http.Error(w, "partita non trovata", http.StatusNotFound)
		return
	}

	// Determina il colore del giocatore
	var color string
	switch userID {
	case whiteID:
		color = "white"
	case blackID:
		color = "black"
	default:
		http.Error(w, "non sei in questa partita", http.StatusForbidden)
		return
	}

	// Upgrade a WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("websocket upgrade error: %v", err)
		return
	}

	// Ottieni o crea la room
	room := h.hub.GetOrCreate(gameID)

	// Crea il client
	client := game.NewClient(userID, color, conn, room)

	// Unisciti alla room
	if err := room.Join(client); err != nil {
		log.Printf("join error: %v", err)
		conn.Close()
		return
	}

	// Avvia le goroutine del client
	go client.WritePump()
	go client.ReadPump()

	log.Printf("client connesso: %s come %s alla partita %s", userID, color, gameID)
}
