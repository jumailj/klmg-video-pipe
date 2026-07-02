package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

// A room has exactly one "player" (screen sharer) and one or more "obs"
// viewers (OBS browser sources). The hub just relays SDP/ICE JSON messages
// between the peers in a room - all real WebRTC media negotiation happens
// browser-to-browser.

type client struct {
	conn *websocket.Conn
	role string // "player" or "obs"
	room string
	send chan []byte
	mu   sync.Mutex
}

func (c *client) writeJSON(v interface{}) {
	b, err := json.Marshal(v)
	if err != nil {
		return
	}
	select {
	case c.send <- b:
	default:
	}
}

type Hub struct {
	mu    sync.Mutex
	rooms map[string]map[*client]bool
}

func NewHub() *Hub {
	return &Hub{rooms: make(map[string]map[*client]bool)}
}

func (h *Hub) join(c *client) {
	h.mu.Lock()
	if h.rooms[c.room] == nil {
		h.rooms[c.room] = make(map[*client]bool)
	}
	// tell existing peers a new peer joined, and tell the new peer about
	// existing peers so it knows whether to start the offer.
	for other := range h.rooms[c.room] {
		other.writeJSON(map[string]string{"type": "peer-joined", "role": c.role})
		c.writeJSON(map[string]string{"type": "peer-joined", "role": other.role})
	}
	h.rooms[c.room][c] = true
	h.mu.Unlock()
}

func (h *Hub) leave(c *client) {
	h.mu.Lock()
	if peers, ok := h.rooms[c.room]; ok {
		delete(peers, c)
		for other := range peers {
			other.writeJSON(map[string]string{"type": "peer-left", "role": c.role})
		}
		if len(peers) == 0 {
			delete(h.rooms, c.room)
		}
	}
	h.mu.Unlock()
	close(c.send)
}

// relay forwards a raw signaling message to every other client in the room.
func (h *Hub) relay(c *client, raw []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for other := range h.rooms[c.room] {
		if other == c {
			continue
		}
		select {
		case other.send <- raw:
		default:
		}
	}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	room := r.URL.Query().Get("room")
	role := r.URL.Query().Get("role")
	if room == "" || (role != "player" && role != "obs") {
		http.Error(w, "room and role=player|obs are required", http.StatusBadRequest)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("ws upgrade:", err)
		return
	}

	c := &client{conn: conn, role: role, room: room, send: make(chan []byte, 16)}
	h.join(c)

	go c.writePump()
	c.readPump(h)
}

func (c *client) readPump(h *Hub) {
	defer func() {
		h.leave(c)
		c.conn.Close()
	}()
	for {
		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		h.relay(c, msg)
	}
}

func (c *client) writePump() {
	defer c.conn.Close()
	for msg := range c.send {
		if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			return
		}
	}
}
