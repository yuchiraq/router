package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

type NotificationConfig struct {
	Enabled           bool            `json:"enabled"`
	Token             string          `json:"token"`
	ChatID            string          `json:"chatId,omitempty"` // legacy migration
	ChatIDs           []int64         `json:"chatIds"`
	KnownChatIDs      []int64         `json:"knownChatIds"`
	WebhookURL        string          `json:"webhookUrl"`
	Events            map[string]bool `json:"events"`
	QuietHoursStart   int             `json:"quietHoursStart"`
	QuietHoursEnd     int             `json:"quietHoursEnd"`
	QuietHoursOn      bool            `json:"quietHoursOn"`
	WebhookSecret     string          `json:"webhookSecret"`
	DiskWarning       int             `json:"diskWarning"`
	DiskCritical      int             `json:"diskCritical"`
	DiskReminderHours int             `json:"diskReminderHours"`
	DiskPaths         []string        `json:"diskPaths"`
}

type NotificationStore struct {
	mu     sync.RWMutex
	path   string
	config NotificationConfig
}

func NewNotificationStore(path string) *NotificationStore {
	s := &NotificationStore{path: path}
	s.config = NotificationConfig{Events: map[string]bool{}, QuietHoursStart: 20, QuietHoursEnd: 8, ChatIDs: []int64{}}
	normalizeNotificationConfig(&s.config)
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

func (s *NotificationStore) saveLocked(cfg NotificationConfig) error {
	cfg.ChatID = ""
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(s.path), ".notifications-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), s.path)
}

func (s *NotificationStore) Get() NotificationConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cfg := s.config
	cfg.Events = copyEvents(s.config.Events)
	cfg.ChatIDs = copyChatIDs(s.config.ChatIDs)
	cfg.KnownChatIDs = copyChatIDs(s.config.KnownChatIDs)
	cfg.DiskPaths = append([]string{}, s.config.DiskPaths...)
	return cfg
}

func (s *NotificationStore) Update(cfg NotificationConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	normalizeNotificationConfig(&cfg)
	if err := s.saveLocked(cfg); err != nil {
		return err
	}
	s.config = cfg
	return nil
}

func normalizeNotificationConfig(cfg *NotificationConfig) {
	cfg.DiskPaths, _ = ParseDiskPaths(strings.Join(cfg.DiskPaths, "\n"))
	if cfg.DiskWarning < 50 || cfg.DiskWarning > 98 {
		cfg.DiskWarning = 85
	}
	if cfg.DiskCritical <= cfg.DiskWarning || cfg.DiskCritical > 100 {
		cfg.DiskCritical = 95
		if cfg.DiskCritical <= cfg.DiskWarning {
			cfg.DiskCritical = 100
		}
	}
	if cfg.DiskReminderHours < 1 || cfg.DiskReminderHours > 168 {
		cfg.DiskReminderHours = 6
	}
	if cfg.Events == nil {
		cfg.Events = map[string]bool{}
	}
	cfg.Events = copyEvents(cfg.Events)
	if _, exists := cfg.Events["service_unavailable"]; !exists {
		cfg.Events["service_unavailable"] = true
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
	cfg.KnownChatIDs = dedupeChatIDs(cfg.KnownChatIDs)
	cfg.WebhookURL = strings.TrimSpace(cfg.WebhookURL)
}

// Paths are explicit administrator choices, never an automatic partition scan.
func ParseDiskPaths(value string) ([]string, error) {
	paths := []string{}
	seen := map[string]bool{}
	for _, line := range strings.Split(value, "\n") {
		p := strings.TrimSpace(line)
		if p == "" {
			continue
		}
		if !filepath.IsAbs(p) || strings.ContainsAny(p, "\x00\r") || len(p) > 4096 {
			return nil, fmt.Errorf("Укажите абсолютный путь на сервере: %s", p)
		}
		p = filepath.Clean(p)
		if !seen[p] {
			seen[p] = true
			paths = append(paths, p)
		}
		if len(paths) > 16 {
			return nil, fmt.Errorf("Можно добавить не более 16 путей")
		}
	}
	return paths, nil
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

func (s *NotificationStore) RememberKnownChatID(chatID int64) {
	if chatID == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, id := range s.config.KnownChatIDs {
		if id == chatID {
			return
		}
	}
	cfg := s.config
	cfg.KnownChatIDs = append(copyChatIDs(cfg.KnownChatIDs), chatID)
	if err := s.saveLocked(cfg); err == nil {
		s.config = cfg
	}
}
