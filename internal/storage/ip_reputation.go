package storage

import (
	"encoding/json"
	"os"
	"sort"
	"sync"
	"time"
)

const (
	autoBanWindow   = 2 * time.Minute
	autoBanHits     = 10
	autoBanDuration = 24 * time.Hour
)

// SuspiciousIP describes an IP with suspicious activity metadata.
type SuspiciousIP struct {
	IP          string    `json:"ip"`
	Reason      string    `json:"reason"`
	Count       int       `json:"count"`
	FirstSeen   time.Time `json:"firstSeen"`
	LastSeen    time.Time `json:"lastSeen"`
	Banned      bool      `json:"banned"`
	BannedAt    time.Time `json:"bannedAt,omitempty"`
	BanUntil    time.Time `json:"banUntil,omitempty"`
	AutoBanned  bool      `json:"autoBanned,omitempty"`
	WindowStart time.Time `json:"windowStart,omitempty"`
	WindowCount int       `json:"windowCount,omitempty"`
}

type ipReputationData struct {
	Entries map[string]*SuspiciousIP `json:"entries"`
}

// IPReputationStore stores suspicious and banned IPs in a JSON file.
type IPReputationStore struct {
	mu      sync.RWMutex
	path    string
	entries map[string]*SuspiciousIP
	nowFn   func() time.Time
}

func NewIPReputationStore(path string) *IPReputationStore {
	s := &IPReputationStore{path: path, entries: make(map[string]*SuspiciousIP), nowFn: time.Now}
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

func (s *IPReputationStore) MarkSuspicious(ip, reason string) (bool, time.Time) {
	if ip == "" {
		return false, time.Time{}
	}
	now := s.nowFn()

	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.entries[ip]
	if !ok {
		entry = &SuspiciousIP{
			IP:          ip,
			Reason:      reason,
			Count:       1,
			FirstSeen:   now,
			LastSeen:    now,
			WindowStart: now,
			WindowCount: 1,
		}
		s.entries[ip] = entry
		s.saveLocked()
		return false, time.Time{}
	}

	entry.Count++
	entry.LastSeen = now
	if reason != "" {
		entry.Reason = reason
	}

	if entry.WindowStart.IsZero() || now.Sub(entry.WindowStart) > autoBanWindow {
		entry.WindowStart = now
		entry.WindowCount = 1
	} else {
		entry.WindowCount++
	}

	if !entry.Banned && entry.WindowCount >= autoBanHits {
		entry.Banned = true
		entry.AutoBanned = true
		entry.BannedAt = now
		entry.BanUntil = now.Add(autoBanDuration)
		s.saveLocked()
		return true, entry.BanUntil
	}

	s.saveLocked()
	return false, time.Time{}
}

func (s *IPReputationStore) Ban(ip string) bool {
	if ip == "" {
		return false
	}
	now := s.nowFn()
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
	entry.AutoBanned = false
	entry.BanUntil = time.Time{}
	entry.BannedAt = now
	s.saveLocked()
	return true
}

func (s *IPReputationStore) Unban(ip string) bool {
	if ip == "" {
		return false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.entries[ip]
	if !ok || !entry.Banned {
		return false
	}

	entry.Banned = false
	entry.AutoBanned = false
	entry.BannedAt = time.Time{}
	entry.BanUntil = time.Time{}
	s.saveLocked()
	return true
}

func (s *IPReputationStore) Remove(ip string) bool {
	if ip == "" {
		return false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.entries[ip]; !ok {
		return false
	}

	delete(s.entries, ip)
	s.saveLocked()
	return true
}

func (s *IPReputationStore) IsBanned(ip string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.entries[ip]
	if !ok {
		return false
	}
	if entry.Banned && !entry.BanUntil.IsZero() && s.nowFn().After(entry.BanUntil) {
		entry.Banned = false
		entry.AutoBanned = false
		entry.BannedAt = time.Time{}
		entry.BanUntil = time.Time{}
		s.saveLocked()
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

func (s *IPReputationStore) AutoBannedList() []SuspiciousIP {
	items := s.List()
	out := make([]SuspiciousIP, 0)
	for _, item := range items {
		if item.Banned && item.AutoBanned {
			out = append(out, item)
		}
	}
	return out
}
