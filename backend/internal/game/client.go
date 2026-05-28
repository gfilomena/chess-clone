package game

import (
	"encoding/json"
	"log"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512
)

// Client rappresenta la connessione WebSocket di un giocatore
type Client struct {
	UserID string
	Color  string // "white" | "black"
	conn   *websocket.Conn
	send   chan []byte
	room   *Room
}

func NewClient(userID, color string, conn *websocket.Conn, room *Room) *Client {
	return &Client{
		UserID: userID,
		Color:  color,
		conn:   conn,
		send:   make(chan []byte, 64),
		room:   room,
	}
}

// ReadPump legge messaggi dal WebSocket e li passa alla room
func (c *Client) ReadPump() {
	defer func() {
		c.room.clientDisconnected <- c
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("ws error [%s]: %v", c.UserID, err)
			}
			break
		}

		var msg InboundMsg
		if err := json.Unmarshal(raw, &msg); err != nil {
			log.Printf("json parse error [%s]: %v", c.UserID, err)
			continue
		}

		c.room.inbound <- clientMessage{client: c, msg: msg}
	}
}

// WritePump scrive messaggi dal canale send al WebSocket
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// Send invia un messaggio al client (non bloccante)
func (c *Client) Send(msg OutboundMsg) {
	raw, err := json.Marshal(msg)
	if err != nil {
		return
	}
	select {
	case c.send <- raw:
	default:
		log.Printf("send buffer pieno per client %s", c.UserID)
	}
}

// Close chiude il canale send
func (c *Client) Close() {
	close(c.send)
}
