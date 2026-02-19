package notify

import (
	"io"
	"net/http"
	"strings"
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
	store.Update(storage.NotificationConfig{Token: "t", ChatIDs: []int64{-100123}})
	n := NewTelegramNotifier(store)

	ip, msg, err := n.HandleCallback("ban:203.0.113.10", -100123)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg != "" {
		t.Fatalf("unexpected message: %s", msg)
	}
	if ip != "203.0.113.10" {
		t.Fatalf("unexpected ip: %s", ip)
	}

	ip, msg, err = n.HandleCallback("ban:203.0.113.10", -999)
	if err != nil {
		t.Fatalf("unexpected error for unauthorized: %v", err)
	}
	if ip != "" || msg != "Unauthorized chat" {
		t.Fatalf("expected unauthorized message, got ip=%s msg=%s", ip, msg)
	}
}

func TestNotifyUsesKnownChatIDsWhenChatIDsEmpty(t *testing.T) {
	store := storage.NewNotificationStore(t.TempDir() + "/n.json")
	store.Update(storage.NotificationConfig{
		Enabled:      true,
		Token:        "token",
		KnownChatIDs: []int64{12345},
		Events:       map[string]bool{"manual_ban": true},
	})
	n := NewTelegramNotifier(store)
	rt := &captureTransport{}
	n.client = &http.Client{Transport: rt}

	n.Notify("manual_ban", "k1", "message")

	if rt.calls != 1 {
		t.Fatalf("expected 1 telegram call, got %d", rt.calls)
	}
	if !strings.Contains(rt.lastBody, "chat_id=12345") {
		t.Fatalf("expected known chat id in request body, got %q", rt.lastBody)
	}
}

func TestTestMessageUsesKnownChatIDs(t *testing.T) {
	store := storage.NewNotificationStore(t.TempDir() + "/n.json")
	store.Update(storage.NotificationConfig{Token: "token", KnownChatIDs: []int64{777}})
	n := NewTelegramNotifier(store)
	rt := &captureTransport{}
	n.client = &http.Client{Transport: rt}

	if err := n.TestMessage(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rt.calls != 1 {
		t.Fatalf("expected 1 telegram call, got %d", rt.calls)
	}
	if !strings.Contains(rt.lastBody, "chat_id=777") {
		t.Fatalf("expected known chat id in request body, got %q", rt.lastBody)
	}
}

type captureTransport struct {
	calls    int
	lastBody string
}

func (t *captureTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.calls++
	b, _ := io.ReadAll(req.Body)
	t.lastBody = string(b)
	return &http.Response{
		StatusCode: 200,
		Status:     "200 OK",
		Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
		Header:     make(http.Header),
	}, nil
}
