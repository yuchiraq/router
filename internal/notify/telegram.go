package notify

import (
	"fmt"
	"net/http"
	"net/url"
	"router/internal/clog"
	"router/internal/storage"
	"strings"
	"sync"
	"time"
)

type TelegramNotifier struct {
	store       *storage.NotificationStore
	client      *http.Client
	mu          sync.Mutex
	lastSentKey map[string]time.Time
	cooldown    time.Duration
}

func NewTelegramNotifier(store *storage.NotificationStore) *TelegramNotifier {
	return &TelegramNotifier{
		store:       store,
		client:      &http.Client{Timeout: 10 * time.Second},
		lastSentKey: map[string]time.Time{},
		cooldown:    1 * time.Minute,
	}
}

func (n *TelegramNotifier) Notify(eventKey, dedupeKey, message string) {
	cfg := n.store.Get()
	if !cfg.Enabled || cfg.Token == "" || cfg.ChatID == "" {
		return
	}
	if !cfg.Events[eventKey] {
		return
	}
	if dedupeKey != "" && n.shouldSkip(dedupeKey) {
		return
	}

	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", cfg.Token)
	values := url.Values{}
	values.Set("chat_id", cfg.ChatID)
	values.Set("text", message)

	resp, err := n.client.PostForm(apiURL, values)
	if err != nil {
		clog.Warnf("telegram notify error: %v", err)
		return
	}
	_ = resp.Body.Close()
	if resp.StatusCode >= 300 {
		clog.Warnf("telegram notify bad status: %s", resp.Status)
	}
}

func (n *TelegramNotifier) TestMessage() error {
	cfg := n.store.Get()
	if cfg.Token == "" || cfg.ChatID == "" {
		return fmt.Errorf("token and chat id are required")
	}
	n.Notify("test", "manual-test-"+time.Now().Format(time.RFC3339Nano), "✅ Router test notification")
	return nil
}

func (n *TelegramNotifier) shouldSkip(key string) bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	now := time.Now()
	last, ok := n.lastSentKey[key]
	if ok && now.Sub(last) < n.cooldown {
		return true
	}
	n.lastSentKey[key] = now
	if len(n.lastSentKey) > 5000 {
		for k, t := range n.lastSentKey {
			if now.Sub(t) > 10*n.cooldown {
				delete(n.lastSentKey, k)
			}
		}
	}
	return false
}

func BuildProxyAlert(method, path, host, ip, reason string) string {
	parts := []string{"🚨 Router alert", "reason: " + reason, "ip: " + ip, "host: " + host, "method: " + method, "path: " + path}
	return strings.Join(parts, "\n")
}
