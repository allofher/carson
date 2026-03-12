package session

import (
	"sync"
	"time"

	"github.com/allofher/carson/internal/llm"
)

const (
	DefaultTTL      = 30 * time.Minute
	cleanupInterval = 5 * time.Minute
)

type entry struct {
	Messages []llm.Message
	LastUsed time.Time
}

type Store struct {
	mu       sync.RWMutex
	sessions map[string]*entry
	ttl      time.Duration
	done     chan struct{}
}

func NewStore(ttl time.Duration) *Store {
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	s := &Store{
		sessions: make(map[string]*entry),
		ttl:      ttl,
		done:     make(chan struct{}),
	}
	go s.cleanup()
	return s
}

// Get returns a copy of the session's message history, or nil if not found.
func (s *Store) Get(id string) []llm.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.sessions[id]
	if !ok {
		return nil
	}
	msgs := make([]llm.Message, len(e.Messages))
	copy(msgs, e.Messages)
	return msgs
}

// Set replaces the session's message history and updates the last-used timestamp.
func (s *Store) Set(id string, messages []llm.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[id] = &entry{
		Messages: messages,
		LastUsed: time.Now(),
	}
}

// Close stops the background cleanup goroutine.
func (s *Store) Close() {
	close(s.done)
}

func (s *Store) cleanup() {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.done:
			return
		case <-ticker.C:
			s.mu.Lock()
			now := time.Now()
			for id, e := range s.sessions {
				if now.Sub(e.LastUsed) > s.ttl {
					delete(s.sessions, id)
				}
			}
			s.mu.Unlock()
		}
	}
}
