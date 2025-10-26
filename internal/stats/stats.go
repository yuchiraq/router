package stats

import (
	"runtime"
	"sync"
	"time"
)

// Request represents a single request entry
type Request struct {
	Time time.Time
}

// Memory represents a single memory usage entry
type Memory struct {
	Time  time.Time
	Alloc uint64
}

// Stats holds the collected statistics
type Stats struct {
	mu       sync.RWMutex
	requests []Request
	memory   []Memory
}

// New creates a new Stats instance
func New() *Stats {
	return &Stats{
		requests: make([]Request, 0),
		memory:   make([]Memory, 0),
	}
}

// AddRequest adds a new request to the stats
func (s *Stats) AddRequest() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests = append(s.requests, Request{Time: time.Now()})
}

// RecordMemory records the current memory usage
func (s *Stats) RecordMemory() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.memory = append(s.memory, Memory{Time: time.Now(), Alloc: m.Alloc / 1024 / 1024}) // MB
}

// GetRequestData returns request data for charting
func (s *Stats) GetRequestData() (labels []string, values []int) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	hourly := make(map[int]int)
	now := time.Now()

	for _, r := range s.requests {
		if r.Time.After(now.Add(-24 * time.Hour)) {
			hourly[r.Time.Hour()]++
		}
	}

	for i := 0; i < 24; i++ {
		hour := (now.Hour() - (23 - i) + 24) % 24
		labels = append(labels, time.Date(0, 0, 0, hour, 0, 0, 0, time.UTC).Format("15:04"))
		values = append(values, hourly[hour])
	}

	return
}

// GetMemoryData returns memory data for charting
func (s *Stats) GetMemoryData() (labels []string, values []uint64) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, m := range s.memory {
		labels = append(labels, m.Time.Format("15:04:05"))
		values = append(values, m.Alloc)
	}

	return
}
