package logging

import (
	"sync"
)

// Broadcaster distributes log lines to SSE subscribers.
// It implements io.Writer so it can be used as an slog output target.
type Broadcaster struct {
	mu   sync.RWMutex
	subs map[uint64]chan []byte
	next uint64
}

// Subscription is a handle to a broadcast subscription.
type Subscription struct {
	C  <-chan []byte
	id uint64
	bc *Broadcaster
}

// NewBroadcaster creates a new log broadcaster.
func NewBroadcaster() *Broadcaster {
	return &Broadcaster{
		subs: make(map[uint64]chan []byte),
	}
}

// Write implements io.Writer. Each call broadcasts a copy of p to all
// subscribers. Non-blocking — slow subscribers have their messages dropped.
func (b *Broadcaster) Write(p []byte) (int, error) {
	msg := make([]byte, len(p))
	copy(msg, p)

	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, ch := range b.subs {
		select {
		case ch <- msg:
		default:
			// Drop message for slow subscriber.
		}
	}
	return len(p), nil
}

// Subscribe returns a Subscription whose channel receives copies of each
// log line written to the broadcaster. The channel is buffered.
func (b *Broadcaster) Subscribe() *Subscription {
	ch := make(chan []byte, 64)

	b.mu.Lock()
	id := b.next
	b.next++
	b.subs[id] = ch
	b.mu.Unlock()

	return &Subscription{C: ch, id: id, bc: b}
}

// Unsubscribe removes the subscription and closes its channel.
func (s *Subscription) Unsubscribe() {
	s.bc.mu.Lock()
	defer s.bc.mu.Unlock()

	if ch, ok := s.bc.subs[s.id]; ok {
		delete(s.bc.subs, s.id)
		close(ch)
	}
}
