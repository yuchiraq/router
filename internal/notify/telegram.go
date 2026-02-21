package notify

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
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
	n.notifyInternal(eventKey, dedupeKey, message, "")
}

func (n *TelegramNotifier) NotifyWithBanButton(eventKey, dedupeKey, message, ip string) {
	n.notifyInternal(eventKey, dedupeKey, message, ip)
}

func (n *TelegramNotifier) notifyInternal(eventKey, dedupeKey, message, banIP string) {
	cfg := n.store.Get()
	chatIDs := targetChatIDs(cfg)
	if !cfg.Enabled || cfg.Token == "" || len(chatIDs) == 0 {
		return
	}
	if !cfg.Events[eventKey] {
		return
	}
	if inQuietHours(time.Now(), cfg.QuietHoursOn, cfg.QuietHoursStart, cfg.QuietHoursEnd) {
		return
	}
	if dedupeKey != "" && n.shouldSkip(dedupeKey) {
		return
	}

	for _, chatID := range chatIDs {
		values := url.Values{}
		values.Set("chat_id", fmt.Sprintf("%d", chatID))
		values.Set("text", message)
		if banIP != "" {
			markup := map[string]interface{}{
				"inline_keyboard": [][]map[string]string{{
					{"text": "⛔ Ban " + banIP, "callback_data": "ban:" + banIP},
				}},
			}
			payload, _ := json.Marshal(markup)
			values.Set("reply_markup", string(payload))
		}
		if err := n.callBot(cfg.Token, "sendMessage", values); err != nil {
			clog.Warnf("telegram notify error: %v", err)
		}
	}
}

func (n *TelegramNotifier) TestMessage() error {
	cfg := n.store.Get()
	chatIDs := targetChatIDs(cfg)
	if cfg.Token == "" || len(chatIDs) == 0 {
		return fmt.Errorf("token and chat ids are required")
	}
	for _, chatID := range chatIDs {
		values := url.Values{}
		values.Set("chat_id", fmt.Sprintf("%d", chatID))
		values.Set("text", "✅ Router test notification")
		if err := n.callBot(cfg.Token, "sendMessage", values); err != nil {
			return err
		}
	}
	return nil
}

func (n *TelegramNotifier) EnsureWebhook(cfg storage.NotificationConfig, webhookURL string) error {
	if cfg.Token == "" || webhookURL == "" {
		return fmt.Errorf("token and webhook url are required")
	}
	clog.Infof("telegram ensureWebhook: applying url=%s", webhookURL)
	values := url.Values{}
	values.Set("url", webhookURL)
	if cfg.WebhookSecret != "" {
		values.Set("secret_token", cfg.WebhookSecret)
	}
	err := n.callBot(cfg.Token, "setWebhook", values)
	if err != nil {
		clog.Errorf("telegram ensureWebhook: failed: %v", err)
		return err
	}
	clog.Infof("telegram ensureWebhook: success")
	return nil
}

func GenerateWebhookSecret() string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("fallback-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

func (n *TelegramNotifier) HandleCallback(data string, fromChatID int64) (string, string, error) {
	cfg := n.store.Get()
	if cfg.Token == "" || len(cfg.ChatIDs) == 0 {
		return "", "", fmt.Errorf("telegram is not configured")
	}
	if len(cfg.ChatIDs) > 0 {
		allowed := false
		for _, id := range cfg.ChatIDs {
			if id == fromChatID {
				allowed = true
				break
			}
		}
		if !allowed {
			return "", "Unauthorized chat", nil
		}
	}
	if !strings.HasPrefix(data, "ban:") {
		return "", "Unsupported action", nil
	}
	ip := strings.TrimSpace(strings.TrimPrefix(data, "ban:"))
	if netIP := firstValidIP(ip); netIP == "" {
		return "", "Invalid IP", nil
	}
	return ip, "", nil
}

func (n *TelegramNotifier) SendActionResult(text string) {
	cfg := n.store.Get()
	chatIDs := targetChatIDs(cfg)
	if cfg.Token == "" || len(chatIDs) == 0 {
		return
	}
	for _, chatID := range chatIDs {
		values := url.Values{}
		values.Set("chat_id", fmt.Sprintf("%d", chatID))
		values.Set("text", text)
		if err := n.callBot(cfg.Token, "sendMessage", values); err != nil {
			clog.Warnf("telegram action result send error: %v", err)
		}
	}
}

func (n *TelegramNotifier) SendMessageToChat(chatID int64, text string) error {
	cfg := n.store.Get()
	if cfg.Token == "" {
		return fmt.Errorf("telegram token is empty")
	}
	values := url.Values{}
	values.Set("chat_id", fmt.Sprintf("%d", chatID))
	values.Set("text", text)
	return n.callBot(cfg.Token, "sendMessage", values)
}

func (n *TelegramNotifier) callBot(token, method string, values url.Values) error {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/%s", token, method)
	clog.Debugf("telegram callBot: method=%s", method)
	resp, err := n.client.PostForm(apiURL, values)
	if err != nil {
		clog.Errorf("telegram callBot: method=%s network error: %v", method, err)
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("bad status: %s body=%s", resp.Status, string(bytes.TrimSpace(b)))
	}
	clog.Infof("telegram callBot: method=%s status=%s", method, resp.Status)
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

func targetChatIDs(cfg storage.NotificationConfig) []int64 {
	if len(cfg.ChatIDs) > 0 {
		return cfg.ChatIDs
	}
	return cfg.KnownChatIDs
}

func BuildProxyAlert(method, path, host, ip, reason string) string {
	parts := []string{"🚨 Router alert", "reason: " + reason, "ip: " + ip, "host: " + host, "method: " + method, "path: " + path}
	return strings.Join(parts, "\n")
}

func inQuietHours(now time.Time, enabled bool, startHour, endHour int) bool {
	if !enabled {
		return false
	}
	if startHour < 0 || startHour > 23 || endHour < 0 || endHour > 23 {
		return false
	}
	h := now.Hour()
	if startHour == endHour {
		return true
	}
	if startHour < endHour {
		return h >= startHour && h < endHour
	}
	return h >= startHour || h < endHour
}

func firstValidIP(value string) string {
	ip := strings.TrimSpace(value)
	if ip == "" {
		return ""
	}
	if parsed := net.ParseIP(ip); parsed != nil {
		return parsed.String()
	}
	return ""
}
