package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

//go:embed public/*
var publicFiles embed.FS

var (
	sessions   = map[string]*Session{}
	sessionsMu sync.Mutex
)

type Session struct {
	ID string

	SenderOffer  *SDPMessage
	ViewerAnswer *SDPMessage

	SenderCandidates []ICECandidate
	ViewerCandidates []ICECandidate

	Name    string
	Team    string
	Quality string
	Status  string

	Updated time.Time
}

type SessionRequest struct {
	ID      string `json:"id"`
	Name    string `json:"name,omitempty"`
	Team    string `json:"team,omitempty"`
	Quality string `json:"quality,omitempty"`
	Status  string `json:"status,omitempty"`
}

type SDPMessage struct {
	Type string `json:"type"`
	SDP  string `json:"sdp"`
}

type ICECandidate struct {
	Candidate     string `json:"candidate"`
	SDPMid        string `json:"sdpMid"`
	SDPMLineIndex uint16 `json:"sdpMLineIndex"`
}

type SignalRequest struct {
	Role    string      `json:"role"`
	ID      string      `json:"id"`
	Message interface{} `json:"message"`
	Quality string      `json:"quality,omitempty"`
}

type CandidateRequest struct {
	Role      string       `json:"role"`
	ID        string       `json:"id"`
	Candidate ICECandidate `json:"candidate"`
}

type SignalResponse struct {
	Status string      `json:"status"`
	Data   interface{} `json:"data,omitempty"`
}

func main() {
	port := flag.String("port", "8080", "HTTP port to listen on")
	flag.Parse()

	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/api/stream-links", streamLinksHandler)
	mux.HandleFunc("/signal", signalHandler)
	mux.HandleFunc("/ice", iceHandler)
	mux.HandleFunc("/player/", uiHandler)
	mux.HandleFunc("/view/", uiHandler)
	mux.HandleFunc("/public", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/", http.StatusFound)
	})
	mux.HandleFunc("/public/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/", http.StatusFound)
	})
	mux.HandleFunc("/api/session", sessionHandler)

	publicFS, err := fs.Sub(publicFiles, "public")
	if err != nil {
		log.Fatalf("failed to initialize public filesystem: %v", err)
	}
	mux.Handle("/", http.FileServer(http.FS(publicFS)))

	addr := fmt.Sprintf(":%s", *port)
	log.Printf("Starting KLMG StreamLink server on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func getSession(id string) *Session {
	sessionsMu.Lock()
	defer sessionsMu.Unlock()
	s, ok := sessions[id]
	if !ok {
		s = &Session{ID: id, Updated: time.Now()}
		sessions[id] = s
	}
	s.Updated = time.Now()
	return s
}

func signalHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var req SignalRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		session := getSession(req.ID)

		if req.Quality != "" {
			session.Quality = req.Quality
		}

		switch req.Role {
		case "sender":
			if msg, ok := req.Message.(map[string]interface{}); ok {
				session.SenderOffer = &SDPMessage{Type: msg["type"].(string), SDP: msg["sdp"].(string)}
			}
			session.Status = "connecting"
			respondJSON(w, SignalResponse{Status: "ok"})
			return
		case "viewer":
			if msg, ok := req.Message.(map[string]interface{}); ok {
				session.ViewerAnswer = &SDPMessage{Type: msg["type"].(string), SDP: msg["sdp"].(string)}
			}
			respondJSON(w, SignalResponse{Status: "ok"})
			return
		default:
			http.Error(w, "invalid role", http.StatusBadRequest)
			return
		}
	}

	q := r.URL.Query()
	role := q.Get("role")
	id := q.Get("id")
	msgType := q.Get("type")
	if id == "" || role == "" || msgType == "" {
		http.Error(w, "missing query parameters", http.StatusBadRequest)
		return
	}

	session := getSession(id)
	switch role {
	case "viewer":
		if msgType != "offer" {
			http.Error(w, "viewer only retrieves sender offer", http.StatusBadRequest)
			return
		}
		if session.SenderOffer == nil {
			respondJSON(w, SignalResponse{Status: "pending"})
			return
		}
		respondJSON(w, SignalResponse{Status: "ok", Data: session.SenderOffer})
		return
	case "sender":
		if msgType != "answer" {
			http.Error(w, "sender only retrieves viewer answer", http.StatusBadRequest)
			return
		}
		if session.ViewerAnswer == nil {
			respondJSON(w, SignalResponse{Status: "pending"})
			return
		}
		respondJSON(w, SignalResponse{Status: "ok", Data: session.ViewerAnswer})
		return
	default:
		http.Error(w, "invalid role", http.StatusBadRequest)
		return
	}
}

func iceHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var req CandidateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		session := getSession(req.ID)
		switch req.Role {
		case "sender":
			session.SenderCandidates = append(session.SenderCandidates, req.Candidate)
			respondJSON(w, SignalResponse{Status: "ok"})
			return
		case "viewer":
			session.ViewerCandidates = append(session.ViewerCandidates, req.Candidate)
			respondJSON(w, SignalResponse{Status: "ok"})
			return
		default:
			http.Error(w, "invalid role", http.StatusBadRequest)
			return
		}
	}

	q := r.URL.Query()
	role := q.Get("role")
	id := q.Get("id")
	if id == "" || role == "" {
		http.Error(w, "missing query parameters", http.StatusBadRequest)
		return
	}
	session := getSession(id)
	switch role {
	case "sender":
		respondJSON(w, SignalResponse{Status: "ok", Data: session.ViewerCandidates})
		return
	case "viewer":
		respondJSON(w, SignalResponse{Status: "ok", Data: session.SenderCandidates})
		return
	default:
		http.Error(w, "invalid role", http.StatusBadRequest)
		return
	}
}

func uiHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/player/" || r.URL.Path == "/player" {
		http.Redirect(w, r, "/player.html", http.StatusFound)
		return
	}
	if r.URL.Path == "/view/" || r.URL.Path == "/view" {
		http.Redirect(w, r, "/view.html", http.StatusFound)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/player/") {
		http.ServeFile(w, r, filepath.FromSlash("public/player.html"))
		return
	}
	if strings.HasPrefix(r.URL.Path, "/view/") {
		http.ServeFile(w, r, filepath.FromSlash("public/view.html"))
		return
	}
	http.NotFound(w, r)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}

func streamLinksHandler(w http.ResponseWriter, r *http.Request) {
	sessionsMu.Lock()
	defer sessionsMu.Unlock()

	list := make([]map[string]interface{}, 0, len(sessions))
	for _, session := range sessions {
		if session.Quality == "" {
			session.Quality = "standard"
		}
		status := session.Status
		if status == "" {
			status = "offline"
		}
		name := session.Name
		if name == "" {
			name = fmt.Sprintf("Player %s", session.ID)
		}
		team := session.Team
		list = append(list, map[string]interface{}{
			"id":         session.ID,
			"name":       name,
			"team":       team,
			"quality":    session.Quality,
			"status":     status,
			"streamLink": fmt.Sprintf("/player/%s", session.ID),
			"viewerLink": fmt.Sprintf("/view/%s", session.ID),
		})
	}

	if len(list) == 0 {
		list = append(list, map[string]interface{}{
			"id":         "player01",
			"name":       "Player 01",
			"team":       "Team Alpha",
			"quality":    "standard",
			"status":     "offline",
			"streamLink": "/player/player01",
			"viewerLink": "/view/player01",
		})
	}

	respondJSON(w, list)
}

func sessionHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, "missing id", http.StatusBadRequest)
			return
		}
		session := getSession(id)
		if session.Quality == "" {
			session.Quality = "standard"
		}
		status := session.Status
		if status == "" {
			status = "offline"
		}
		respondJSON(w, map[string]interface{}{
			"id":      session.ID,
			"name":    session.Name,
			"team":    session.Team,
			"quality": session.Quality,
			"status":  status,
		})
	case http.MethodPost:
		var req SessionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if req.ID == "" {
			http.Error(w, "missing id", http.StatusBadRequest)
			return
		}
		session := getSession(req.ID)
		if req.Name != "" {
			session.Name = req.Name
		}
		if req.Team != "" {
			session.Team = req.Team
		}
		if req.Quality != "" {
			session.Quality = req.Quality
		}
		if req.Status != "" {
			session.Status = req.Status
		}
		if session.Quality == "" {
			session.Quality = "standard"
		}
		respondJSON(w, map[string]string{"status": "ok"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func respondJSON(w http.ResponseWriter, value interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(value)
}
