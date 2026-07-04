package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"

	"nhooyr.io/websocket"
)

type signalPayload struct {
	SDP            string  `json:"sdp,omitempty"`
	Candidate      string  `json:"candidate,omitempty"`
	SDPMid         string  `json:"sdpMid,omitempty"`
	SDPMLineIndex  *uint16 `json:"sdpMLineIndex,omitempty"`
}

type signalMessage struct {
	Type     string        `json:"type"`
	StreamID string        `json:"stream_id,omitempty"`
	From     string        `json:"from,omitempty"`
	To       string        `json:"to,omitempty"`
	Payload  signalPayload `json:"payload"`
}

type peer struct {
	id       string
	role     string
	streamID string
	conn     *websocket.Conn
}

type room struct {
	producer *peer
	viewers  map[string]*peer
	lock     sync.RWMutex
}

type hub struct {
	rooms map[string]*room
	lock  sync.RWMutex
}

func newHub() *hub {
	return &hub{rooms: make(map[string]*room)}
}

func newRoom() *room {
	return &room{viewers: make(map[string]*peer)}
}

func (h *hub) getRoom(streamID string) *room {
	streamID = strings.TrimSpace(streamID)
	if streamID == "" {
		streamID = "default"
	}

	h.lock.RLock()
	room, ok := h.rooms[streamID]
	h.lock.RUnlock()
	if ok {
		return room
	}

	h.lock.Lock()
	defer h.lock.Unlock()
	if room, ok = h.rooms[streamID]; ok {
		return room
	}
	room = newRoom()
	h.rooms[streamID] = room
	return room
}

func (h *hub) addPeer(p *peer) {
	room := h.getRoom(p.streamID)
	room.lock.Lock()
	defer room.lock.Unlock()
	if p.role == "producer" {
		room.producer = p
		return
	}
	room.viewers[p.id] = p
}

func (h *hub) removePeer(p *peer) {
	room := h.getRoom(p.streamID)
	room.lock.Lock()
	defer room.lock.Unlock()
	if p.role == "producer" {
		if room.producer == p {
			room.producer = nil
		}
		return
	}
	delete(room.viewers, p.id)
}

func (h *hub) relay(p *peer, msg signalMessage) {
	room := h.getRoom(p.streamID)
	if p.role == "viewer" {
		room.lock.RLock()
		producer := room.producer
		room.lock.RUnlock()
		if producer == nil {
			return
		}
		msg.From = p.id
		_ = sendSignal(producer.conn, msg)
		return
	}

	if p.role == "producer" {
		room.lock.RLock()
		viewer, ok := room.viewers[msg.To]
		room.lock.RUnlock()
		if !ok {
			return
		}
		msg.From = p.id
		_ = sendSignal(viewer.conn, msg)
	}
}

func sendSignal(conn *websocket.Conn, msg signalMessage) error {
	payload, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return conn.Write(context.Background(), websocket.MessageText, payload)
}

func generateID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func main() {
	hub := newHub()
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/view/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "viewer.html")
	})
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			log.Printf("accept failed: %v", err)
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "server shutdown")
		conn.SetReadLimit(64 << 20)

		role := r.URL.Query().Get("role")
		if role == "" {
			role = "viewer"
		}
		streamID := r.URL.Query().Get("stream")
		if streamID == "" {
			streamID = "default"
		}
		peerID := r.URL.Query().Get("id")
		if peerID == "" {
			peerID = generateID()
		}

		p := &peer{id: peerID, role: role, streamID: streamID, conn: conn}
		hub.addPeer(p)
		defer hub.removePeer(p)

		log.Printf("peer connected role=%s id=%s stream=%s", role, peerID, streamID)
		for {
			msgType, payload, err := conn.Read(context.Background())
			if err != nil {
				log.Printf("read error for %s: %v", peerID, err)
				return
			}

			if msgType != websocket.MessageText {
				continue
			}

			var msg signalMessage
			if err := json.Unmarshal(payload, &msg); err != nil {
				log.Printf("unmarshal error: %v", err)
				continue
			}

			hub.relay(p, msg)
		}
	})

	log.Println("signaling server listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
