
package stats

import (
	"runtime"
	"sync"
	"time"

	"github.com/shirou/gopsutil/mem"
)

// Request represents a single request entry with its host
type Request struct {
	Time time.Time
	Host string
}

// Memory represents a single memory usage entry
type Memory struct {
	Time    time.Time
	Used    uint64 // Used memory in MB
	Percent float64 // Used memory in percentage
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
		requests: make([]Request, 0, 10000), // Pre-allocate for performance
		memory:   make([]Memory, 0, 1000),   // Pre-allocate
	}
}

// AddRequest adds a new request to the stats, including the host
func (s *Stats) AddRequest(host string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests = append(s.requests, Request{Time: time.Now(), Host: host})
}

// RecordMemory records the current memory usage (both absolute and percentage)
func (s *Stats) RecordMemory() {
	// Get system memory stats
	v, _ := mem.VirtualMemory()
	// Get process memory stats
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	s.mu.Lock()
	defer s.mu.Unlock()

    // Store memory usage as percentage of total system memory
	s.memory = append(s.memory, Memory{
		Time:    time.Now(),
		Used:    m.Alloc / 1024 / 1024, // MB
		Percent: float64(m.Alloc) / float64(v.Total) * 100,
	})

    // Optional: Keep memory slice from growing indefinitely
    if len(s.memory) > 1000 {
        s.memory = s.memory[len(s.memory)-1000:]
    }
}

// GetRequestData returns request data grouped by host for charting
func (s *Stats) GetRequestData() (map[string]interface{}) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	datasets := make(map[string]map[int]int)
	hosts := []string{}
	now := time.Now()

	// Aggregate requests by host and hour
	for _, r := range s.requests {
		if r.Time.After(now.Add(-24 * time.Hour)) {
			if _, ok := datasets[r.Host]; !ok {
				datasets[r.Host] = make(map[int]int)
                hosts = append(hosts, r.Host)
			}
			datasets[r.Host][r.Time.Hour()]++
		}
	}

	// Prepare data for Chart.js
	chartData := make(map[string]interface{})
	labels := []string{}
	for i := 0; i < 24; i++ {
		hour := (now.Hour() - (23 - i) + 24) % 24
		labels = append(labels, time.Date(0, 0, 0, hour, 0, 0, 0, time.UTC).Format("15:04"))
	}
	chartData["labels"] = labels

	chartDatasets := []interface{}{}
	for _, host := range hosts {
		values := []int{}
		for i := 0; i < 24; i++ {
			hour := (now.Hour() - (23 - i) + 24) % 24
			values = append(values, datasets[host][hour])
		}
		chartDatasets = append(chartDatasets, map[string]interface{}{
			"label": host,
			"data":  values,
		})
	}
	chartData["datasets"] = chartDatasets

	return chartData
}

// GetMemoryData returns memory data for charting (absolute and percentage)
func (s *Stats) GetMemoryData() ([]string, []uint64, []float64) {
	s.mu.RLock()
	defer s.mu.RUnlock()

    labels := make([]string, len(s.memory))
    values := make([]uint64, len(s.memory))
    percents := make([]float64, len(s.memory))

	for i, m := range s.memory {
		labels[i] = m.Time.Format("15:04:05")
		values[i] = m.Used
		percents[i] = m.Percent
	}

	return labels, values, percents
}
