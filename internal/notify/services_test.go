package notify

import (
	"net/http"
	"path/filepath"
	"router/internal/storage"
	"strings"
	"testing"
	"time"
)

func TestServiceUnavailableNotification(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notifications.json")
	store := storage.NewNotificationStore(path)
	cfg := storage.NotificationConfig{Enabled: true, Token: "test-token", ChatIDs: []int64{1, 2}}
	if err := store.Update(cfg); err != nil {
		t.Fatal(err)
	}
	if !store.Get().Events["service_unavailable"] {
		t.Fatal("new event should be enabled by default")
	}
	n := NewTelegramNotifier(store)
	transport := &diskTransport{calls: map[string]int{}}
	n.client = &http.Client{Transport: transport}
	event := storage.ServiceOutage{Host: "site.test", Reason: "HTTP 503", At: time.Now()}
	n.NotifyServiceUnavailable(event)
	if transport.calls["1"] != 1 || transport.calls["2"] != 1 {
		t.Fatal(transport.calls)
	}
	for _, text := range transport.texts {
		for _, part := range []string{"Router by Qoncept", "site.test", "HTTP 503", "сервисный режим"} {
			if !strings.Contains(text, part) {
				t.Fatal("missing message detail", part)
			}
		}
	}
	// A separate outage after recovery must not be swallowed by generic cooldown.
	n.NotifyServiceUnavailable(event)
	if transport.calls["1"] != 2 {
		t.Fatal("new incident suppressed")
	}
	cfg = store.Get()
	cfg.Events["service_unavailable"] = false
	store.Update(cfg)
	n.NotifyServiceUnavailable(event)
	if transport.calls["1"] != 2 {
		t.Fatal("disabled event sent")
	}
	if storage.NewNotificationStore(path).Get().Events["service_unavailable"] {
		t.Fatal("disabled preference lost after reload")
	}
	cfg.Events["service_unavailable"] = true
	cfg.QuietHoursOn = true
	cfg.QuietHoursStart = 0
	cfg.QuietHoursEnd = 0
	store.Update(cfg)
	n.NotifyServiceUnavailable(event)
	if transport.calls["1"] != 2 {
		t.Fatal("quiet hours ignored")
	}
	cfg.QuietHoursOn = false
	cfg.Enabled = false
	store.Update(cfg)
	n.NotifyServiceUnavailable(event)
	if transport.calls["1"] != 2 {
		t.Fatal("global notification switch ignored")
	}
}
