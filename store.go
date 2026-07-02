package main

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

type Player struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
}

type Store struct {
	mu      sync.Mutex
	players map[string]*Player
}

func NewStore() *Store {
	return &Store{players: make(map[string]*Player)}
}

func newID() string {
	b := make([]byte, 6)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (s *Store) Create(name string) *Player {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := &Player{ID: newID(), Name: name, CreatedAt: time.Now()}
	s.players[p.ID] = p
	return p
}

func (s *Store) Get(id string) (*Player, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.players[id]
	return p, ok
}

func (s *Store) List() []*Player {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*Player, 0, len(s.players))
	for _, p := range s.players {
		out = append(out, p)
	}
	return out
}

func (s *Store) Delete(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.players, id)
}
