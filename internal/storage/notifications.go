package storage

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"sync"
)

type NotificationConfig struct {
	Enabled         bool            `json:"enabled"`
	Token           string          `json:"token"`
	ChatID          string          `json:"chatId,omitempty"` // legacy migration
	ChatIDs         []int64         `json:"chatIds"`
	Events          map[string]bool `json:"events"`
	QuietHoursStart int             `json:"quietHoursStart"`
	QuietHoursEnd   int             `json:"quietHoursEnd"`
	QuietHoursOn    bool            `json:"quietHoursOn"`
	WebhookSecret   string          `json:"webhookSecret"`
}

type NotificationStore struct {
	mu     sync.RWMutex
	path   string
	config NotificationConfig
}

func NewNotificationStore(path string) *NotificationStore {
	s := &NotificationStore{path: path}
	s.config = NotificationConfig{Events: map[string]bool{}, QuietHoursStart: 20, QuietHoursEnd: 8, ChatIDs: []int64{}}
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
	normalizeNotificationConfig(&cfg)
	s.config = cfg
}

func (s *NotificationStore) saveLocked() {
	cfg := s.config
	cfg.ChatID = ""
	data, err := json.MarshalIndent(cfg, "", "  ")
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
	cfg.ChatIDs = copyChatIDs(s.config.ChatIDs)
	return cfg
}

func (s *NotificationStore) Update(cfg NotificationConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	normalizeNotificationConfig(&cfg)
	s.config = cfg
	s.saveLocked()
}

func normalizeNotificationConfig(cfg *NotificationConfig) {
	if cfg.Events == nil {
		cfg.Events = map[string]bool{}
	}
	if cfg.QuietHoursStart < 0 || cfg.QuietHoursStart > 23 {
		cfg.QuietHoursStart = 20
	}
	if cfg.QuietHoursEnd < 0 || cfg.QuietHoursEnd > 23 {
		cfg.QuietHoursEnd = 8
	}
	chatIDs := copyChatIDs(cfg.ChatIDs)
	if len(chatIDs) == 0 {
		if v := strings.TrimSpace(cfg.ChatID); v != "" {
			if id, err := strconv.ParseInt(v, 10, 64); err == nil {
				chatIDs = append(chatIDs, id)
			}
		}
	}
	cfg.ChatIDs = dedupeChatIDs(chatIDs)
}

func copyEvents(src map[string]bool) map[string]bool {
	out := make(map[string]bool, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

func copyChatIDs(src []int64) []int64 {
	out := make([]int64, len(src))
	copy(out, src)
	return out
}

func dedupeChatIDs(src []int64) []int64 {
	seen := map[int64]struct{}{}
	out := make([]int64, 0, len(src))
	for _, id := range src {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}
