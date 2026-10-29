package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiskConfigAndFailedSave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "n.json")
	s := NewNotificationStore(path)
	cfg := s.Get()
	if cfg.DiskWarning != 85 || cfg.DiskCritical != 95 || cfg.DiskReminderHours != 6 || len(cfg.DiskPaths) != 0 {
		t.Fatal("disk defaults", cfg)
	}
	paths, err := ParseDiskPaths(dir + "\n" + dir)
	if err != nil || len(paths) != 1 {
		t.Fatal("dedup paths", paths, err)
	}
	for _, bad := range []string{"relative/path", "../escape", dir + "\x00", strings.Repeat(dir+"/x\n", 1) + "relative"} {
		if _, err := ParseDiskPaths(bad); err == nil {
			t.Fatal("accepted", bad)
		}
	}
	cfg.DiskPaths = paths
	cfg.DiskWarning = 80
	cfg.DiskCritical = 90
	if err := s.Update(cfg); err != nil {
		t.Fatal(err)
	}
	s = NewNotificationStore(path)
	if s.Get().DiskWarning != 80 || len(s.Get().DiskPaths) != 1 {
		t.Fatal("not persisted")
	}
	copy := s.Get()
	copy.DiskPaths[0] = "modified"
	if s.Get().DiskPaths[0] == "modified" {
		t.Fatal("alias")
	}
	s.path = dir
	cfg.DiskWarning = 75
	if err := s.Update(cfg); err == nil {
		t.Fatal("saved over directory")
	}
	if s.Get().DiskWarning != 80 {
		t.Fatal("failed save applied in memory")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("lost previous file")
	}
}

func TestNotificationStorePersist(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notifications.json")
	store := NewNotificationStore(path)
	store.Update(NotificationConfig{
		Enabled:         true,
		Token:           "token",
		ChatIDs:         []int64{-100123, 777},
		KnownChatIDs:    []int64{-100123, 999},
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
	if len(cfg.KnownChatIDs) != 2 || cfg.KnownChatIDs[1] != 999 {
		t.Fatalf("unexpected known chat ids: %+v", cfg.KnownChatIDs)
	}
}

func TestNotificationStoreRememberKnownChatID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notifications.json")
	store := NewNotificationStore(path)
	store.RememberKnownChatID(123)
	store.RememberKnownChatID(123)
	store.RememberKnownChatID(456)
	cfg := store.Get()
	if len(cfg.KnownChatIDs) != 2 {
		t.Fatalf("expected 2 known chat ids, got %d", len(cfg.KnownChatIDs))
	}
}
