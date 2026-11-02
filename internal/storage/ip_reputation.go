package storage

import (
	"context"
	"encoding/json"
	"net/netip"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultAutoBanWindow = 2 * time.Minute
	defaultAutoBanHits   = 3
	autoBanDuration      = 24 * time.Hour
	reputationRetention  = 7 * 24 * time.Hour
	reputationLimit      = 10000
)

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
type IPReputationStore struct {
	mu            sync.RWMutex
	path          string
	entries       map[string]*SuspiciousIP
	nowFn         func() time.Time
	autoBanWindow time.Duration
	autoBanHits   int
	dirty         bool
	lastError     string
}

// Canonical keys prevent IPv4-mapped IPv6 aliases bypassing an existing ban.
func reputationIP(raw string) string {
	a, err := netip.ParseAddr(strings.TrimSpace(raw))
	if err != nil || a.Zone() != "" {
		return ""
	}
	a = a.Unmap()
	if a.IsLoopback() || a.IsUnspecified() || a.IsMulticast() {
		return ""
	}
	return a.String()
}

func ValidReputationIP(raw string) bool { return reputationIP(raw) != "" }

func NewIPReputationStore(path string) *IPReputationStore {
	s := &IPReputationStore{path: path, entries: map[string]*SuspiciousIP{}, nowFn: time.Now, autoBanWindow: envDurationSeconds("IP_AUTO_BAN_WINDOW_SEC", defaultAutoBanWindow), autoBanHits: envInt("IP_AUTO_BAN_HITS", defaultAutoBanHits)}
	s.load()
	return s
}

func (s *IPReputationStore) load() {
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		s.lastError = err.Error()
		return
	}
	var parsed ipReputationData
	if err = json.Unmarshal(data, &parsed); err != nil {
		s.lastError = err.Error()
		return
	}
	for key, e := range parsed.Entries {
		ip := reputationIP(key)
		if ip == "" || e == nil {
			s.dirty = true
			continue
		}
		e.IP = ip
		if e.AutoBanned && e.BanUntil.IsZero() {
			e.BanUntil = e.BannedAt.Add(autoBanDuration)
			s.dirty = true
		}
		old := s.entries[ip]
		if old == nil || (e.Banned && !old.Banned) || (e.Banned && old.Banned && old.AutoBanned && !e.AutoBanned) || (e.Banned == old.Banned && e.AutoBanned == old.AutoBanned && e.LastSeen.After(old.LastSeen)) {
			s.entries[ip] = e
		}
		if ip != key {
			s.dirty = true
		}
	}
	s.pruneLocked(s.nowFn())
	for len(s.entries) > reputationLimit && s.evictObservationLocked() {
	}
	if s.dirty {
		s.saveLocked()
	}
}

func (s *IPReputationStore) saveLocked() {
	data, err := json.Marshal(ipReputationData{Entries: s.entries})
	if err == nil {
		var f *os.File
		f, err = os.CreateTemp(filepath.Dir(s.path), ".ip-reputation-*.tmp")
		if err == nil {
			defer os.Remove(f.Name())
			if _, err = f.Write(data); err == nil {
				err = f.Sync()
			}
			closeErr := f.Close()
			if err == nil {
				err = closeErr
			}
			if err == nil {
				err = os.Rename(f.Name(), s.path)
			}
		}
	}
	if err != nil {
		s.lastError = err.Error()
		return
	}
	s.dirty = false
	s.lastError = ""
}

func (s *IPReputationStore) LastError() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastError
}

func expireBan(e *SuspiciousIP, now time.Time) bool {
	if !e.Banned || e.BanUntil.IsZero() || now.Before(e.BanUntil) {
		return false
	}
	e.Banned = false
	e.AutoBanned = false
	e.BannedAt = time.Time{}
	e.BanUntil = time.Time{}
	e.WindowStart = time.Time{}
	e.WindowCount = 0
	return true
}

func (s *IPReputationStore) pruneLocked(now time.Time) {
	for ip, e := range s.entries {
		if expireBan(e, now) {
			s.dirty = true
		}
		if !e.Banned && !now.Before(e.LastSeen.Add(reputationRetention)) {
			delete(s.entries, ip)
			s.dirty = true
		}
	}
}

// Never evict an active ban just to make space for new observations.
func (s *IPReputationStore) evictObservationLocked() bool {
	oldest := ""
	var seen time.Time
	for ip, e := range s.entries {
		if !e.Banned && (oldest == "" || e.LastSeen.Before(seen)) {
			oldest = ip
			seen = e.LastSeen
		}
	}
	if oldest == "" {
		return false
	}
	delete(s.entries, oldest)
	s.dirty = true
	return true
}

func (s *IPReputationStore) Cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneLocked(s.nowFn())
	if s.dirty {
		s.saveLocked()
	}
}

// Batch ordinary observations instead of rewriting the entire JSON on every probe.
func (s *IPReputationStore) StartMaintenance(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			s.Cleanup()
			return
		case <-ticker.C:
			s.Cleanup()
		}
	}
}

func (s *IPReputationStore) MarkSuspicious(ip, reason string) (bool, time.Time) {
	ip = reputationIP(ip)
	if ip == "" {
		return false, time.Time{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.nowFn()
	entry := s.entries[ip]
	if entry == nil {
		if len(s.entries) >= reputationLimit {
			s.pruneLocked(now)
			if len(s.entries) >= reputationLimit && !s.evictObservationLocked() {
				return false, time.Time{}
			}
		}
		entry = &SuspiciousIP{IP: ip, FirstSeen: now}
		s.entries[ip] = entry
	}
	if expireBan(entry, now) {
		s.dirty = true
	}
	if entry.Banned {
		return false, entry.BanUntil
	}
	if entry.Count < 1000000000 {
		entry.Count++
	}
	entry.LastSeen = now
	if len(reason) > 256 {
		reason = reason[:256]
	}
	entry.Reason = reason
	if entry.WindowStart.IsZero() || !now.Before(entry.WindowStart.Add(s.autoBanWindow)) {
		entry.WindowStart = now
		entry.WindowCount = 1
	} else {
		entry.WindowCount++
	}
	s.dirty = true
	if entry.WindowCount >= s.autoBanHits {
		entry.Banned = true
		entry.AutoBanned = true
		entry.BannedAt = now
		entry.BanUntil = now.Add(autoBanDuration)
		s.saveLocked()
		return true, entry.BanUntil
	}
	return false, time.Time{}
}

func (s *IPReputationStore) Ban(ip string) bool {
	ip = reputationIP(ip)
	if ip == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.nowFn()
	entry := s.entries[ip]
	if entry == nil {
		if len(s.entries) >= reputationLimit {
			s.pruneLocked(now)
			if len(s.entries) >= reputationLimit && !s.evictObservationLocked() {
				return false
			}
		}
		entry = &SuspiciousIP{IP: ip, Reason: "manual ban", FirstSeen: now, LastSeen: now}
		s.entries[ip] = entry
	}
	if entry.Banned && !entry.AutoBanned {
		return false
	}
	entry.Banned = true
	entry.AutoBanned = false
	entry.BannedAt = now
	entry.BanUntil = time.Time{}
	s.dirty = true
	s.saveLocked()
	return true
}

func (s *IPReputationStore) Unban(ip string) bool {
	ip = reputationIP(ip)
	s.mu.Lock()
	defer s.mu.Unlock()
	e := s.entries[ip]
	if e == nil || !e.Banned {
		return false
	}
	e.Banned = false
	e.AutoBanned = false
	e.BannedAt = time.Time{}
	e.BanUntil = time.Time{}
	e.WindowStart = time.Time{}
	e.WindowCount = 0
	s.dirty = true
	s.saveLocked()
	return true
}

func (s *IPReputationStore) Remove(ip string) bool {
	ip = reputationIP(ip)
	s.mu.Lock()
	defer s.mu.Unlock()
	e := s.entries[ip]
	if e == nil || e.Banned {
		return false
	}
	delete(s.entries, ip)
	s.dirty = true
	s.saveLocked()
	return true
}

func (s *IPReputationStore) IsBanned(ip string) bool {
	ip = reputationIP(ip)
	s.mu.Lock()
	defer s.mu.Unlock()
	e := s.entries[ip]
	if e == nil {
		return false
	}
	if expireBan(e, s.nowFn()) {
		s.dirty = true
	}
	return e.Banned
}

func (s *IPReputationStore) List() []SuspiciousIP {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneLocked(s.nowFn())
	out := make([]SuspiciousIP, 0, len(s.entries))
	for _, e := range s.entries {
		out = append(out, *e)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Banned != out[j].Banned {
			return out[i].Banned
		}
		return out[i].LastSeen.After(out[j].LastSeen)
	})
	return out
}
func (s *IPReputationStore) AutoBannedList() []SuspiciousIP {
	out := []SuspiciousIP{}
	for _, e := range s.List() {
		if e.Banned && e.AutoBanned {
			out = append(out, e)
		}
	}
	return out
}
func envInt(key string, fallback int) int {
	v, err := strconv.Atoi(os.Getenv(key))
	if err != nil || v <= 0 || v > 1000000 {
		return fallback
	}
	return v
}
func envDurationSeconds(key string, fallback time.Duration) time.Duration {
	return time.Duration(envInt(key, int(fallback/time.Second))) * time.Second
}
