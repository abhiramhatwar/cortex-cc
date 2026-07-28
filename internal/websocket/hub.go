package websocket

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/abhiramhatwar/cortex-cc/internal/models"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type client struct {
	conn *websocket.Conn
	send chan []byte
}

type Hub struct {
	mu      sync.RWMutex
	clients map[*client]struct{}
}

func NewHub() *Hub {
	return &Hub{clients: make(map[*client]struct{})}
}

// Broadcast fans an event out to every connected client.
func (h *Hub) Broadcast(evt *models.Event) {
	data, err := json.Marshal(evt)
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		select {
		case c.send <- data:
		default:
			// slow client — drop the message rather than block
		}
	}
}

// Run drains the engine event channel and broadcasts each event.
func (h *Hub) Run(events <-chan *models.Event) {
	for evt := range events {
		h.Broadcast(evt)
	}
}

// ServeWS upgrades an HTTP connection to WebSocket and registers the client.
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ws upgrade: %v", err)
		return
	}
	c := &client{conn: conn, send: make(chan []byte, 64)}
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()

	go c.writePump(h)
	c.readPump(h) // blocks until disconnect
}

func (c *client) writePump(h *Hub) {
	defer c.conn.Close()
	for msg := range c.send {
		if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			break
		}
	}
}

func (c *client) readPump(h *Hub) {
	defer func() {
		h.mu.Lock()
		delete(h.clients, c)
		h.mu.Unlock()
		close(c.send)
		c.conn.Close()
	}()
	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			break
		}
	}
}
