package logstream

import (
	"sync"
)

// Broadcaster manages a set of listeners and broadcasts messages to them.
type Broadcaster struct {
	mu        sync.RWMutex
	listeners map[chan []byte]struct{}
}

// NewBroadcaster creates a new Broadcaster.
func New() *Broadcaster {
	return &Broadcaster{
		listeners: make(map[chan []byte]struct{}),
	}
}

// AddListener adds a new listener to the broadcaster.
func (b *Broadcaster) AddListener(ch chan []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.listeners[ch] = struct{}{}
}

// RemoveListener removes a listener from the broadcaster.
func (b *Broadcaster) RemoveListener(ch chan []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.listeners, ch)
	close(ch)
}

// Write broadcasts a message to all listeners.
func (b *Broadcaster) Write(p []byte) (n int, err error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for ch := range b.listeners {
		// Make a copy to avoid race conditions
		msg := make([]byte, len(p))
		copy(msg, p)

		select {
		case ch <- msg:
		default:
			// Don't block if a listener is slow
		}
	}

	return len(p), nil
}
