package main

import (
	"encoding/json"
	"flag"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
)

var (
	store     = NewStore()
	hub       = NewHub()
	templates *template.Template
)

func main() {
	addr := flag.String("addr", ":8080", "listen address, e.g. :8080")
	flag.Parse()

	var err error
	templates, err = template.ParseGlob(filepath.Join("web", "templates", "*.html"))
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()

	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(filepath.Join("web", "static")))))

	mux.HandleFunc("/", handleDashboard)
	mux.HandleFunc("/api/players", handlePlayersAPI)
	mux.HandleFunc("/play/", handlePlayerPage)
	mux.HandleFunc("/obs/", handleObsPage)
	mux.HandleFunc("/ws", hub.ServeWS)

	log.Println("VOD dashboard listening on", *addr)
	log.Fatal(http.ListenAndServe(*addr, mux))
}

func handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	data := struct {
		Players []*Player
		Host    string
	}{store.List(), r.Host}
	if err := templates.ExecuteTemplate(w, "dashboard.html", data); err != nil {
		http.Error(w, err.Error(), 500)
	}
}

func handlePlayersAPI(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		var body struct{ Name string }
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
			http.Error(w, "name is required", http.StatusBadRequest)
			return
		}
		p := store.Create(body.Name)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(p)
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(store.List())
	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		store.Delete(id)
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func handlePlayerPage(w http.ResponseWriter, r *http.Request) {
	id := filepath.Base(r.URL.Path)
	p, ok := store.Get(id)
	if !ok {
		http.Error(w, "unknown player link", http.StatusNotFound)
		return
	}
	templates.ExecuteTemplate(w, "player.html", p)
}

func handleObsPage(w http.ResponseWriter, r *http.Request) {
	id := filepath.Base(r.URL.Path)
	p, ok := store.Get(id)
	if !ok {
		http.Error(w, "unknown player link", http.StatusNotFound)
		return
	}
	templates.ExecuteTemplate(w, "obs.html", p)
}
