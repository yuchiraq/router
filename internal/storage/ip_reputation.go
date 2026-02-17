package storage

import (
	"encoding/json"
	"os"
	"sort"
	"sync"
	"time"
)

// SuspiciousIP describes an IP with suspicious activity metadata.
type SuspiciousIP struct {
	IP        string    `json:"ip"`
	Reason    string    `json:"reason"`
	Count     int       `json:"count"`
	FirstSeen time.Time `json:"firstSeen"`
	LastSeen  time.Time `json:"lastSeen"`
	Banned    bool      `json:"banned"`
	BannedAt  time.Time `json:"bannedAt,omitempty"`
}

type ipReputationData struct {
	Entries map[string]*SuspiciousIP `json:"entries"`
}

// IPReputationStore stores suspicious and banned IPs in a JSON file.
type IPReputationStore struct {
	mu      sync.RWMutex
	path    string
	entries map[string]*SuspiciousIP
}

func NewIPReputationStore(path string) *IPReputationStore {
	s := &IPReputationStore{path: path, entries: make(map[string]*SuspiciousIP)}
	s.load()
	return s
}

func (s *IPReputationStore) load() {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path)
	if err != nil {
		return
	}
	if len(data) == 0 {
		return
	}
	var parsed ipReputationData
	if err := json.Unmarshal(data, &parsed); err != nil {
		return
	}
	if parsed.Entries != nil {
		s.entries = parsed.Entries
	}
}

func (s *IPReputationStore) saveLocked() {
	data, err := json.MarshalIndent(ipReputationData{Entries: s.entries}, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(s.path, data, 0644)
}

func (s *IPReputationStore) MarkSuspicious(ip, reason string) {
	if ip == "" {
		return
	}
	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.entries[ip]
	if !ok {
		s.entries[ip] = &SuspiciousIP{
			IP:        ip,
			Reason:    reason,
			Count:     1,
			FirstSeen: now,
			LastSeen:  now,
		}
		s.saveLocked()
		return
	}

	entry.Count++
	entry.LastSeen = now
	if reason != "" {
		entry.Reason = reason
	}
	s.saveLocked()
}

func (s *IPReputationStore) Ban(ip string) bool {
	if ip == "" {
		return false
	}
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.entries[ip]
	if !ok {
		s.entries[ip] = &SuspiciousIP{IP: ip, Reason: "manual ban", Count: 1, FirstSeen: now, LastSeen: now, Banned: true, BannedAt: now}
		s.saveLocked()
		return true
	}
	if entry.Banned {
		return false
	}
	entry.Banned = true
	entry.BannedAt = now
	s.saveLocked()
	return true
}

func (s *IPReputationStore) IsBanned(ip string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.entries[ip]
	if !ok {
		return false
	}
	return entry.Banned
}

func (s *IPReputationStore) List() []SuspiciousIP {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]SuspiciousIP, 0, len(s.entries))
	for _, e := range s.entries {
		out = append(out, *e)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Banned != out[j].Banned {
			return !out[i].Banned
		}
		if out[i].Count == out[j].Count {
			return out[i].LastSeen.After(out[j].LastSeen)
		}
		return out[i].Count > out[j].Count
	})
	return out
}
