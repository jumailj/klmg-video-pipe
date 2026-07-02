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

// A room has multiple "streamer" peers (screen sharers) and one or more "obs"
// viewers (OBS browser sources). The hub just relays SDP/ICE JSON messages
// between the peers - all real WebRTC media negotiation happens P2P.
// Multiple streamers can exist, OBS viewers can connect to any of them.

type client struct {
	conn       *websocket.Conn
	role       string // "streamer" or "obs"
	room       string
	streamerID string // unique ID for this streamer (empty if role is "obs")
	send       chan []byte
	mu         sync.Mutex
	network    string // "local" or "external" based on IP
	headers    http.Header // for detecting real IP through proxies
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

// getNetworkType determines if the peer is on a local or external network
// Checks X-Forwarded-For headers (for ngrok/proxies) first, then direct IP
func getNetworkType(remoteAddr string, headers http.Header) string {
	var ip string

	// Check for X-Forwarded-For header (from ngrok, proxies, etc)
	if forwardedFor := headers.Get("X-Forwarded-For"); forwardedFor != "" {
		// X-Forwarded-For can have multiple IPs, take the first one
		ips := strings.Split(forwardedFor, ",")
		ip = strings.TrimSpace(ips[0])
		log.Printf("getNetworkType: Using X-Forwarded-For: %s", ip)
	} else if realIP := headers.Get("X-Real-IP"); realIP != "" {
		// Fallback to X-Real-IP header
		ip = realIP
		log.Printf("getNetworkType: Using X-Real-IP: %s", ip)
	} else {
		// Use direct connection IP
		// Handle both IPv4 and IPv6 addresses
		host, _, err := net.SplitHostPort(remoteAddr)
		if err != nil {
			// If SplitHostPort fails, try to use the remoteAddr as-is
			// This handles IPv6 addresses that might not have a port
			host = remoteAddr
		}
		ip = host
		log.Printf("getNetworkType: Using RemoteAddr: %s (full: %s)", ip, remoteAddr)
	}

	// Parse the IP
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		log.Printf("getNetworkType: Could not parse IP: %s", ip)
		return "unknown"
	}

	if parsedIP.IsLoopback() || parsedIP.IsPrivate() {
		log.Printf("getNetworkType: %s is PRIVATE/LOOPBACK", parsedIP.String())
		return "local"
	}
	log.Printf("getNetworkType: %s is PUBLIC/EXTERNAL", parsedIP.String())
	return "external"
}

type Streamer struct {
	ID      string `json:"id"`
	Network string `json:"network"` // "local" or "external"
}

type Hub struct {
	mu       sync.Mutex
	rooms    map[string]map[*client]bool         // room -> clients
	streamers map[string]map[string]*Streamer    // room -> streamerID -> Streamer info
}

func NewHub() *Hub {
	return &Hub{
		rooms:     make(map[string]map[*client]bool),
		streamers: make(map[string]map[string]*Streamer),
	}
}

func (h *Hub) join(c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.rooms[c.room] == nil {
		h.rooms[c.room] = make(map[*client]bool)
	}
	if h.streamers[c.room] == nil {
		h.streamers[c.room] = make(map[string]*Streamer)
	}

	// Detect network type (check headers for real IP through proxies)
	c.network = getNetworkType(c.conn.RemoteAddr().String(), c.headers)

	log.Printf("[JOIN] room=%s role=%s network=%s", c.room, c.role, c.network)

	if c.role == "streamer" {
		// Register this streamer
		h.streamers[c.room][c.streamerID] = &Streamer{
			ID:      c.streamerID,
			Network: c.network,
		}
		log.Printf("[STREAMER JOINED] room=%s streamer=%s network=%s", c.room, c.streamerID, c.network)

		// Tell all OBS viewers a new streamer joined
		for other := range h.rooms[c.room] {
			if other.role == "obs" {
				other.writeJSON(map[string]interface{}{
					"type":       "streamer-joined",
					"streamerId": c.streamerID,
					"network":    c.network,
				})
				log.Printf("[NOTIFY OBS] about new streamer %s", c.streamerID)
			}
		}
	}

	// Tell new peer about all existing streamers
	count := 0
	for sid, streamer := range h.streamers[c.room] {
		c.writeJSON(map[string]interface{}{
			"type":       "streamer-list",
			"streamerId": sid,
			"network":    streamer.Network,
		})
		count++
	}
	if count > 0 {
		log.Printf("[STREAMER LIST] sent %d streamers to %s in room %s", count, c.role, c.room)
	}

	h.rooms[c.room][c] = true
	log.Printf("[ROOM STATE] room=%s total_peers=%d total_streamers=%d", c.room, len(h.rooms[c.room]), len(h.streamers[c.room]))
}

func (h *Hub) leave(c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	log.Printf("[LEAVE] room=%s role=%s", c.room, c.role)

	if peers, ok := h.rooms[c.room]; ok {
		delete(peers, c)

		if c.role == "streamer" {
			// Remove from streamers list
			delete(h.streamers[c.room], c.streamerID)
			log.Printf("[STREAMER LEFT] room=%s streamer=%s", c.room, c.streamerID)

			// Notify all OBS viewers that this streamer left
			for other := range peers {
				if other.role == "obs" {
					other.writeJSON(map[string]interface{}{
						"type":       "streamer-left",
						"streamerId": c.streamerID,
					})
					log.Printf("[NOTIFY OBS] streamer %s left", c.streamerID)
				}
			}
		}

		if len(peers) == 0 {
			delete(h.rooms, c.room)
			delete(h.streamers, c.room)
			log.Printf("[ROOM DELETED] room=%s (no more peers)", c.room)
		}
	}
	close(c.send)
}

// relay forwards a signaling message to target peer(s).
// Messages from OBS targeting a streamer are routed to that streamer.
// Messages from streamers are routed to all connected OBS viewers.
func (h *Hub) relay(c *client, raw []byte) {
	var msg map[string]interface{}
	if err := json.Unmarshal(raw, &msg); err != nil {
		log.Printf("relay: unmarshal error: %v", err)
		return
	}

	msgType, _ := msg["type"].(string)
	h.mu.Lock()
	defer h.mu.Unlock()

	peers := h.rooms[c.room]
	if len(peers) == 0 {
		log.Printf("relay: no peers in room %s", c.room)
		return
	}

	if c.role == "obs" {
		// OBS sending to a specific streamer
		targetStreamerID, _ := msg["targetStreamerId"].(string)
		if targetStreamerID == "" {
			log.Printf("relay: OBS message missing targetStreamerId, type=%s", msgType)
			return
		}

		log.Printf("relay: OBS->Streamer, type=%s, target=%s", msgType, targetStreamerID)

		found := false
		for other := range peers {
			if other.streamerID == targetStreamerID {
				found = true
				select {
				case other.send <- raw:
					log.Printf("relay: sent to streamer %s", targetStreamerID)
				default:
					log.Printf("relay: streamer %s send channel full", targetStreamerID)
				}
				break
			}
		}
		if !found {
			log.Printf("relay: streamer %s not found in room %s", targetStreamerID, c.room)
		}
	} else if c.role == "streamer" {
		// Streamer sending ICE/SDP to all connected OBS viewers
		msg["streamerId"] = c.streamerID
		enriched, _ := json.Marshal(msg)

		log.Printf("relay: Streamer->OBS, type=%s, streamer=%s", msgType, c.streamerID)

		count := 0
		for other := range peers {
			if other.role == "obs" {
				count++
				select {
				case other.send <- enriched:
					log.Printf("relay: sent to OBS viewer %d", count)
				default:
					log.Printf("relay: OBS viewer %d send channel full", count)
				}
			}
		}
		if count == 0 {
			log.Printf("relay: no OBS viewers in room %s", c.room)
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
	streamerID := r.URL.Query().Get("streamerId")

	if room == "" || (role != "streamer" && role != "obs") {
		http.Error(w, "room and role=streamer|obs are required", http.StatusBadRequest)
		return
	}

	if role == "streamer" && streamerID == "" {
		http.Error(w, "streamerId is required for role=streamer", http.StatusBadRequest)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("ws upgrade:", err)
		return
	}

	c := &client{
		conn:       conn,
		role:       role,
		room:       room,
		streamerID: streamerID,
		send:       make(chan []byte, 16),
		headers:    r.Header,
	}
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
