package notify

import (
	"testing"
	"time"

	"router/internal/storage"
)

func TestInQuietHours(t *testing.T) {
	now := time.Date(2026, 1, 1, 21, 0, 0, 0, time.UTC)
	if !inQuietHours(now, true, 20, 8) {
		t.Fatalf("expected quiet hours at 21:00 for range 20-8")
	}
	if inQuietHours(time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC), true, 20, 8) {
		t.Fatalf("did not expect quiet hours at 12:00 for range 20-8")
	}
	if !inQuietHours(time.Date(2026, 1, 1, 7, 0, 0, 0, time.UTC), true, 20, 8) {
		t.Fatalf("expected quiet hours at 07:00 for range 20-8")
	}
	if inQuietHours(now, false, 20, 8) {
		t.Fatalf("quiet hours should be disabled")
	}
}

func TestHandleCallbackBanAction(t *testing.T) {
	store := storage.NewNotificationStore(t.TempDir() + "/n.json")
	store.Update(storage.NotificationConfig{Token: "t", ChatID: "c", AllowedUserIDs: []int64{123}})
	n := NewTelegramNotifier(store)

	ip, msg, err := n.HandleCallback("ban:203.0.113.10", 123)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg != "" {
		t.Fatalf("unexpected message: %s", msg)
	}
	if ip != "203.0.113.10" {
		t.Fatalf("unexpected ip: %s", ip)
	}

	ip, msg, err = n.HandleCallback("ban:203.0.113.10", 999)
	if err != nil {
		t.Fatalf("unexpected error for unauthorized: %v", err)
	}
	if ip != "" || msg != "Unauthorized user" {
		t.Fatalf("expected unauthorized message, got ip=%s msg=%s", ip, msg)
	}
}
