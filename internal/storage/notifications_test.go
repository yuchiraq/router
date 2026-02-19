package storage

import (
	"path/filepath"
	"testing"
)

func TestNotificationStorePersist(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notifications.json")
	store := NewNotificationStore(path)
	store.Update(NotificationConfig{
		Enabled:         true,
		Token:           "token",
		ChatIDs:         []int64{-100123, 777},
		WebhookURL:      "https://router.example.com/telegram/webhook",
		Events:          map[string]bool{"unknown_host": true, "test": true},
		QuietHoursOn:    true,
		QuietHoursStart: 20,
		QuietHoursEnd:   8,
		WebhookSecret:   "secret",
	})

	reloaded := NewNotificationStore(path)
	cfg := reloaded.Get()
	if !cfg.Enabled || cfg.Token != "token" || len(cfg.ChatIDs) != 2 || cfg.ChatIDs[0] != -100123 {
		t.Fatalf("unexpected config: %+v", cfg)
	}
	if !cfg.Events["unknown_host"] || !cfg.Events["test"] {
		t.Fatalf("unexpected events: %+v", cfg.Events)
	}
	if !cfg.QuietHoursOn || cfg.QuietHoursStart != 20 || cfg.QuietHoursEnd != 8 {
		t.Fatalf("unexpected quiet hours: on=%v start=%d end=%d", cfg.QuietHoursOn, cfg.QuietHoursStart, cfg.QuietHoursEnd)
	}
	if cfg.WebhookSecret != "secret" {
		t.Fatalf("unexpected webhook secret: %s", cfg.WebhookSecret)
	}
	if cfg.WebhookURL != "https://router.example.com/telegram/webhook" {
		t.Fatalf("unexpected webhook url: %s", cfg.WebhookURL)
	}
}
