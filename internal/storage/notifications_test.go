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
		Enabled: true,
		Token:   "token",
		ChatID:  "123",
		Events:  map[string]bool{"unknown_host": true, "test": true},
	})

	reloaded := NewNotificationStore(path)
	cfg := reloaded.Get()
	if !cfg.Enabled || cfg.Token != "token" || cfg.ChatID != "123" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
	if !cfg.Events["unknown_host"] || !cfg.Events["test"] {
		t.Fatalf("unexpected events: %+v", cfg.Events)
	}
}
