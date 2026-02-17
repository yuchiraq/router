package storage

import (
	"encoding/json"
	"os"
	"sync"
)

type NotificationConfig struct {
	Enabled         bool            `json:"enabled"`
	Token           string          `json:"token"`
	ChatID          string          `json:"chatId"`
	Events          map[string]bool `json:"events"`
	QuietHoursStart int             `json:"quietHoursStart"`
	QuietHoursEnd   int             `json:"quietHoursEnd"`
	QuietHoursOn    bool            `json:"quietHoursOn"`
}

type NotificationStore struct {
	mu     sync.RWMutex
	path   string
	config NotificationConfig
}

func NewNotificationStore(path string) *NotificationStore {
	s := &NotificationStore{path: path}
	s.config = NotificationConfig{Events: map[string]bool{}, QuietHoursStart: 20, QuietHoursEnd: 8}
	s.load()
	return s
}

func (s *NotificationStore) load() {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(s.path)
	if err != nil || len(data) == 0 {
		return
	}
	var cfg NotificationConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return
	}
	if cfg.Events == nil {
		cfg.Events = map[string]bool{}
	}
	if cfg.QuietHoursStart < 0 || cfg.QuietHoursStart > 23 {
		cfg.QuietHoursStart = 20
	}
	if cfg.QuietHoursEnd < 0 || cfg.QuietHoursEnd > 23 {
		cfg.QuietHoursEnd = 8
	}
	s.config = cfg
}

func (s *NotificationStore) saveLocked() {
	data, err := json.MarshalIndent(s.config, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(s.path, data, 0644)
}

func (s *NotificationStore) Get() NotificationConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cfg := s.config
	cfg.Events = copyEvents(s.config.Events)
	return cfg
}

func (s *NotificationStore) Update(cfg NotificationConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if cfg.Events == nil {
		cfg.Events = map[string]bool{}
	}
	cfg.Events = copyEvents(cfg.Events)
	if cfg.QuietHoursStart < 0 || cfg.QuietHoursStart > 23 {
		cfg.QuietHoursStart = 20
	}
	if cfg.QuietHoursEnd < 0 || cfg.QuietHoursEnd > 23 {
		cfg.QuietHoursEnd = 8
	}
	s.config = cfg
	s.saveLocked()
}

func copyEvents(src map[string]bool) map[string]bool {
	out := make(map[string]bool, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}
