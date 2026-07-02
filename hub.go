package main

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
)

// Simple streaming model:
// - One "streamer" per room (shares screen)
// - Multiple "viewers" per room (watch the stream)
// - Hub relays WebRTC signaling only (SDP, ICE)
// - All media is P2P direct

type Client struct {
	conn     *websocket.Conn
	role     string // "streamer" or "viewer"
	room     string
	send     chan []byte
	network  string // "local" or "external"
	headers  http.Header
}

type Hub struct {
	mu      sync.RWMutex
	rooms   map[string]*Room // roomID -> Room
	register   chan *Client
	unregister chan *Client
}

type Room struct {
	id       string
	streamer *Client           // only one active streamer
	viewers  map[*Client]bool  // multiple viewers
	mu       sync.RWMutex
}

func NewHub() *Hub {
	h := &Hub{
		rooms:      make(map[string]*Room),
		register:   make(chan *Client, 256),
		unregister: make(chan *Client, 256),
	}
	go h.run()
	return h
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.registerClient(client)
		case client := <-h.unregister:
			h.unregisterClient(client)
		}
	}
}

func (h *Hub) registerClient(c *Client) {
	h.mu.Lock()
	room, exists := h.rooms[c.room]
	if !exists {
		room = &Room{
			id:      c.room,
			viewers: make(map[*Client]bool),
		}
		h.rooms[c.room] = room
	}
	h.mu.Unlock()

	room.mu.Lock()
	if c.role == "streamer" {
		// Only one streamer per room
		if room.streamer != nil && room.streamer != c {
			// Close previous streamer
			close(room.streamer.send)
		}
		room.streamer = c
		log.Printf("[ROOM %s] Streamer connected (%s)", c.room, c.network)

		// Tell all viewers about the new streamer
		streamData := map[string]interface{}{
			"type":    "streamer-ready",
			"network": c.network,
		}
		for viewer := range room.viewers {
			if b, err := json.Marshal(streamData); err == nil {
				select {
				case viewer.send <- b:
				default:
				}
			}
		}
	} else if c.role == "viewer" {
		room.viewers[c] = true
		log.Printf("[ROOM %s] Viewer connected (%s) - total viewers: %d", c.room, c.network, len(room.viewers))

		// Tell viewer if streamer is already present
		if room.streamer != nil {
			streamData := map[string]interface{}{
				"type":    "streamer-ready",
				"network": room.streamer.network,
			}
			if b, err := json.Marshal(streamData); err == nil {
				select {
				case c.send <- b:
				default:
				}
			}
		}
	}
	room.mu.Unlock()
}

func (h *Hub) unregisterClient(c *Client) {
	h.mu.RLock()
	room, exists := h.rooms[c.room]
	h.mu.RUnlock()

	if !exists {
		return
	}

	room.mu.Lock()
	defer room.mu.Unlock()

	if c.role == "streamer" && room.streamer == c {
		room.streamer = nil
		log.Printf("[ROOM %s] Streamer disconnected", c.room)

		// Notify all viewers
		msg := []byte(`{"type":"streamer-left"}`)
		for viewer := range room.viewers {
			select {
			case viewer.send <- msg:
			default:
			}
		}
	} else if c.role == "viewer" {
		delete(room.viewers, c)
		log.Printf("[ROOM %s] Viewer disconnected - remaining: %d", c.room, len(room.viewers))
	}

	close(c.send)

	// Delete empty rooms
	h.mu.Lock()
	if room.streamer == nil && len(room.viewers) == 0 {
		delete(h.rooms, c.room)
		log.Printf("[ROOM %s] DELETED (empty)", c.room)
	}
	h.mu.Unlock()
}

// relay handles WebRTC signaling messages
func (h *Hub) relay(c *Client, raw []byte) {
	var msg map[string]interface{}
	if err := json.Unmarshal(raw, &msg); err != nil {
		return
	}

	h.mu.RLock()
	room, exists := h.rooms[c.room]
	h.mu.RUnlock()

	if !exists {
		return
	}

	room.mu.RLock()
	defer room.mu.RUnlock()

	// Streamer sends offer to all viewers
	if c.role == "streamer" && msg["type"] == "offer" {
		log.Printf("[ROOM %s] Streamer sending offer to %d viewers", c.room, len(room.viewers))
		for viewer := range room.viewers {
			select {
			case viewer.send <- raw:
			default:
				log.Printf("[ROOM %s] Viewer send buffer full, dropping message", c.room)
			}
		}
	}

	// Viewer sends answer/candidate to streamer
	if c.role == "viewer" && (msg["type"] == "answer" || msg["type"] == "candidate") {
		if room.streamer != nil {
			select {
			case room.streamer.send <- raw:
			default:
				log.Printf("[ROOM %s] Streamer send buffer full, dropping message", c.room)
			}
		}
	}
}

func getNetworkType(remoteAddr string, headers http.Header) string {
	var ip string

	// Check for X-Forwarded-For header (from ngrok, proxies, etc)
	if forwardedFor := headers.Get("X-Forwarded-For"); forwardedFor != "" {
		ips := strings.Split(forwardedFor, ",")
		ip = strings.TrimSpace(ips[0])
		log.Printf("[NETWORK] Using X-Forwarded-For: %s", ip)
	} else if realIP := headers.Get("X-Real-IP"); realIP != "" {
		ip = realIP
		log.Printf("[NETWORK] Using X-Real-IP: %s", ip)
	} else {
		host, _, err := net.SplitHostPort(remoteAddr)
		if err != nil {
			host = remoteAddr
		}
		ip = host
		log.Printf("[NETWORK] Using RemoteAddr: %s (full: %s)", ip, remoteAddr)
	}

	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		log.Printf("[NETWORK] Could not parse IP: %s", ip)
		return "unknown"
	}

	if parsedIP.IsLoopback() || parsedIP.IsPrivate() {
		log.Printf("[NETWORK] %s -> LOCAL", parsedIP.String())
		return "local"
	}
	log.Printf("[NETWORK] %s -> EXTERNAL", parsedIP.String())
	return "external"
}

func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	room := query.Get("room")
	role := query.Get("role")

	if room == "" || (role != "streamer" && role != "viewer") {
		http.Error(w, "Invalid room or role", http.StatusBadRequest)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	c := &Client{
		conn:    conn,
		role:    role,
		room:    room,
		send:    make(chan []byte, 256),
		network: getNetworkType(r.RemoteAddr, r.Header),
		headers: r.Header,
	}

	h.register <- c

	// Handle client
	go func() {
		defer func() {
			h.unregister <- c
			conn.Close()
		}()

		// Read messages from client
		for {
			_, raw, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Printf("[ROOM %s] WebSocket error: %v", room, err)
				}
				break
			}

			h.relay(c, raw)
		}
	}()

	// Write messages to client
	go func() {
		for message := range c.send {
			conn.WriteMessage(websocket.TextMessage, message)
		}
	}()
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}
