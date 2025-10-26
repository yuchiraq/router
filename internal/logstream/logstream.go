
package logstream

import (
	"sync"
)

const (
	maxBufferSize = 100 // Keep the last 100 log messages
)

// Broadcaster distributes log messages to multiple listeners.
type Broadcaster struct {
	mu        sync.RWMutex
	listeners map[chan<- []byte]struct{}
	buffer    [][]byte // Stores recent messages
}

// New returns a new Broadcaster.
func New() *Broadcaster {
	return &Broadcaster{
		listeners: make(map[chan<- []byte]struct{}),
		buffer:    make([][]byte, 0, maxBufferSize),
	}
}

// AddListener adds a new listener for log messages.
// It immediately sends the buffered historical logs to the new listener.
func (b *Broadcaster) AddListener(ch chan<- []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.listeners[ch] = struct{}{}

	// Send buffer to the new listener
	for _, msg := range b.buffer {
		ch <- msg
	}
}

// RemoveListener removes a listener.
func (b *Broadcaster) RemoveListener(ch chan<- []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()

	delete(b.listeners, ch)
}

// Write implements the io.Writer interface.
// It broadcasts the message to all listeners and adds it to the buffer.
func (b *Broadcaster) Write(p []byte) (n int, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Create a copy of the message, as the original buffer p can be reused.
	msg := make([]byte, len(p))
	copy(msg, p)

	// Add to buffer
	if len(b.buffer) >= maxBufferSize {
		// Shift buffer to make space
		copy(b.buffer, b.buffer[1:])
		b.buffer[len(b.buffer)-1] = msg
	} else {
		b.buffer = append(b.buffer, msg)
	}

	// Broadcast to listeners
	for ch := range b.listeners {
		// Use a non-blocking send to prevent a slow listener
		// from blocking the log system.
		select {
		case ch <- msg:
		default:
			// Listener channel is full, message dropped for this listener.
		}
	}

	return len(p), nil
}
