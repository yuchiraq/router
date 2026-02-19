package panel

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"html/template"
	"net/http"
	"net/netip"
	"net/url"
	"router/internal/clog"
	"strconv"
	"strings"
	"sync"
	"time"

	"router/internal/gpt"
	"router/internal/logstream"
	"router/internal/notify"
	"router/internal/stats"
	"router/internal/storage"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true // non-browser clients
		}

		originURL, err := url.Parse(origin)
		if err != nil {
			return false
		}

		host := r.Host
		if host == "" {
			return false
		}

		return strings.EqualFold(originURL.Host, host)
	},
}

type loginAttempt struct {
	Count       int
	BlockedTill time.Time
}

// Handler holds all dependencies for the web panel
type Handler struct {
	store       *storage.RuleStore
	adminStore  *storage.AdminStore
	auth        *authState
	templates   map[string]*template.Template
	stats       *stats.Stats
	broadcaster *logstream.Broadcaster
	ipStore     *storage.IPReputationStore
	backupStore *storage.BackupStore
	notifyStore *storage.NotificationStore
	gptStore    *storage.GPTStore
	gptClient   *gpt.Client
	notifier    *notify.TelegramNotifier
}

// NewHandler creates a new panel handler
func NewHandler(store *storage.RuleStore, adminStore *storage.AdminStore, stats *stats.Stats, broadcaster *logstream.Broadcaster, ipStore *storage.IPReputationStore, backupStore *storage.BackupStore, notifyStore *storage.NotificationStore, gptStore *storage.GPTStore, gptClient *gpt.Client, notifier *notify.TelegramNotifier) *Handler {
	templates := make(map[string]*template.Template)

	// Parse templates
	templates["index"] = template.Must(template.ParseFiles(
		"internal/panel/templates/layout.html",
		"internal/panel/templates/index.html",
	))
	templates["maintenance"] = template.Must(template.ParseFiles(
		"internal/panel/templates/maintenance.html",
	))

	return &Handler{
		store:       store,
		adminStore:  adminStore,
		auth:        newAuthState(),
		templates:   templates,
		stats:       stats,
		broadcaster: broadcaster,
		ipStore:     ipStore,
		backupStore: backupStore,
		notifyStore: notifyStore,
		gptStore:    gptStore,
		gptClient:   gptClient,
		notifier:    notifier,
	}
}

// basicAuth is a middleware for session-based authentication
func (h *Handler) basicAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.adminStore == nil || h.isAuthenticated(r) {
			next.ServeHTTP(w, r)
			return
		}
		if strings.Contains(r.Header.Get("Accept"), "application/json") || strings.HasPrefix(r.URL.Path, "/ws/") {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		http.Redirect(w, r, "/login", http.StatusFound)
	}
	h.sessionsMu.Lock()
	delete(h.sessions, token)
	h.sessionsMu.Unlock()
}

// render executes the correct template, ensuring page data is passed
func (h *Handler) render(w http.ResponseWriter, _ *http.Request, name string, data interface{}) {
	tmpl, ok := h.templates[name]
	if !ok {
		http.Error(w, "Template not found", http.StatusInternalServerError)
		return
	}

	// This structure is passed to the template
	templateData := map[string]interface{}{
		"Page": name,
		"Data": data,
	}

	if err := tmpl.ExecuteTemplate(w, "layout", templateData); err != nil {
		clog.Errorf("Error executing template %s: %v", name, err)
		http.Error(w, "Error rendering page", http.StatusInternalServerError)
	}
}

// Index serves the main page with the list of rules
func (h *Handler) Index(w http.ResponseWriter, r *http.Request) {
	h.basicAuth(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			maintenance := r.FormValue("maintenance") == "on"
			h.store.SetMaintenanceMode(maintenance)
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}

		data := map[string]interface{}{
			"Rules":           h.store.All(),
			"MaintenanceMode": h.store.MaintenanceMode,
		}
		h.render(w, r, "index", data)
	}).ServeHTTP(w, r)
}

// Stats serves the statistics page by sending the static HTML file
func (h *Handler) Stats(w http.ResponseWriter, r *http.Request) {
	h.basicAuth(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "internal/panel/static/stats.html")
	}).ServeHTTP(w, r)
}

// Backups serves backup management page.
func (h *Handler) Backups(w http.ResponseWriter, r *http.Request) {
	h.basicAuth(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "internal/panel/static/backups.html")
	}).ServeHTTP(w, r)
}

// Notifications serves notification settings page.
func (h *Handler) Notifications(w http.ResponseWriter, r *http.Request) {
	h.basicAuth(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "internal/panel/static/notifications.html")
	}).ServeHTTP(w, r)
}

// Settings serves GPT settings page.
func (h *Handler) Settings(w http.ResponseWriter, r *http.Request) {
	h.basicAuth(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "internal/panel/static/settings.html")
	}).ServeHTTP(w, r)
}

// AddRule adds a new routing rule
func (h *Handler) AddRule(w http.ResponseWriter, r *http.Request) {
	h.basicAuth(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		host := r.FormValue("host")
		target := r.FormValue("target")
		if host == "" || target == "" {
			http.Error(w, "Host and target are required", http.StatusBadRequest)
			return
		}
		h.store.Add(host, target)
		http.Redirect(w, r, "/", http.StatusFound)
	}).ServeHTTP(w, r)
}

// RemoveRule removes a routing rule
func (h *Handler) RemoveRule(w http.ResponseWriter, r *http.Request) {
	h.basicAuth(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		host := r.FormValue("host")
		if host == "" {
			http.Error(w, "Host is required", http.StatusBadRequest)
			return
		}
		h.store.Remove(host)
		http.Redirect(w, r, "/", http.StatusFound)
	}).ServeHTTP(w, r)
}

// RuleMaintenance toggles maintenance mode for a specific rule.
func (h *Handler) RuleMaintenance(w http.ResponseWriter, r *http.Request) {
	h.basicAuth(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		host := r.FormValue("host")
		if host == "" {
			http.Error(w, "Host is required", http.StatusBadRequest)
			return
		}
		maintenance := r.FormValue("maintenance") == "on"
		h.store.SetRuleMaintenance(host, maintenance)
		http.Redirect(w, r, "/", http.StatusFound)
	}).ServeHTTP(w, r)
}

// StatsData provides stats data as JSON
func (h *Handler) StatsData(w http.ResponseWriter, r *http.Request) {
	h.basicAuth(func(w http.ResponseWriter, r *http.Request) {
		h.stats.RecordMemory()
		h.stats.RecordCPU()
		h.stats.RecordDisks()
		h.stats.RecordSSHConnections()
		suspicious := []storage.SuspiciousIP{}
		if h.ipStore != nil {
			suspicious = h.ipStore.List()
		}
		requestData := h.stats.GetRequestData()
		memoryLabels, memoryValues, memoryPercents := h.stats.GetMemoryData()
		cpuLabels, cpuPercents := h.stats.GetCPUData()
		diskData := h.stats.GetDiskData()
		countryData := h.stats.GetCountryData()
		sshData := h.stats.GetSSHData()

		data := map[string]interface{}{
			"requests": requestData,
			"memory": map[string]interface{}{
				"labels":   memoryLabels,
				"values":   memoryValues,
				"percents": memoryPercents,
			},
			"cpu": map[string]interface{}{
				"labels":   cpuLabels,
				"percents": cpuPercents,
			},
			"disks":      diskData,
			"countries":  countryData,
			"ssh":        sshData,
			"suspicious": suspicious,
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(data); err != nil {
			clog.Errorf("Error encoding stats data: %v", err)
		}
	}).ServeHTTP(w, r)
}

// Logs handles the websocket connection for logs
func (h *Handler) Logs(w http.ResponseWriter, r *http.Request) {
	h.basicAuth(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			clog.Errorf("Failed to upgrade to websockets: %v", err)
			return
		}
		defer conn.Close()

		ch := make(chan []byte, 256)
		h.broadcaster.AddListener(ch)
		defer h.broadcaster.RemoveListener(ch)

		for msg := range ch {
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				break
			}
		}
	}).ServeHTTP(w, r)
}

// BanSuspiciousIP bans suspicious IP manually from admin panel.
func (h *Handler) BanSuspiciousIP(w http.ResponseWriter, r *http.Request) {
	h.basicAuth(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ip := r.FormValue("ip")
		if ip == "" {
			http.Error(w, "ip is required", http.StatusBadRequest)
			return
		}
		if h.ipStore == nil {
			http.Error(w, "ip storage is disabled", http.StatusServiceUnavailable)
			return
		}
		h.ipStore.Ban(ip)
		if h.notifier != nil {
			h.notifier.Notify("manual_ban", "manual-ban:"+ip, "⛔️ Manual ban\nip: "+ip)
		}
		w.WriteHeader(http.StatusNoContent)
	}).ServeHTTP(w, r)
}

// UnbanSuspiciousIP removes IP from ban list manually from admin panel.
func (h *Handler) UnbanSuspiciousIP(w http.ResponseWriter, r *http.Request) {
	h.basicAuth(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ip := r.FormValue("ip")
		if ip == "" {
			http.Error(w, "ip is required", http.StatusBadRequest)
			return
		}
		if h.ipStore == nil {
			http.Error(w, "ip storage is disabled", http.StatusServiceUnavailable)
			return
		}
		h.ipStore.Unban(ip)
		if h.notifier != nil {
			h.notifier.Notify("manual_unban", "manual-unban:"+ip, "✅ Manual unban\nip: "+ip)
		}
		w.WriteHeader(http.StatusNoContent)
	}).ServeHTTP(w, r)
}

// RemoveSuspiciousIP deletes IP from suspicious list manually from admin panel.
func (h *Handler) RemoveSuspiciousIP(w http.ResponseWriter, r *http.Request) {
	h.basicAuth(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ip := r.FormValue("ip")
		if ip == "" {
			http.Error(w, "ip is required", http.StatusBadRequest)
			return
		}
		if h.ipStore == nil {
			http.Error(w, "ip storage is disabled", http.StatusServiceUnavailable)
			return
		}
		if h.ipStore.IsBanned(ip) {
			http.Error(w, "cannot remove banned ip; unban first", http.StatusBadRequest)
			return
		}
		h.ipStore.Remove(ip)
		if h.notifier != nil {
			h.notifier.Notify("manual_remove", "manual-remove:"+ip, "🧹 Removed from suspicious list\nip: "+ip)
		}
		w.WriteHeader(http.StatusNoContent)
	}).ServeHTTP(w, r)
}

// BackupsData returns backup jobs and existing archives.
func (h *Handler) BackupsData(w http.ResponseWriter, r *http.Request) {
	h.basicAuth(func(w http.ResponseWriter, r *http.Request) {
		if h.backupStore == nil {
			http.Error(w, "backup storage is disabled", http.StatusServiceUnavailable)
			return
		}
		jobs, entries, lastError := h.backupStore.Get()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"jobs":      jobs,
			"entries":   entries,
			"lastError": lastError,
		})
	}).ServeHTTP(w, r)
}

// SaveBackupsConfig upserts one backup job configuration.
func (h *Handler) SaveBackupsConfig(w http.ResponseWriter, r *http.Request) {
	h.basicAuth(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if h.backupStore == nil {
			http.Error(w, "backup storage is disabled", http.StatusServiceUnavailable)
			return
		}

		sourcesRaw := r.FormValue("sources")
		sources := []string{}
		for _, line := range strings.Split(sourcesRaw, "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				sources = append(sources, line)
			}
		}

		interval, _ := strconv.Atoi(r.FormValue("intervalMinutes"))
		keep, _ := strconv.Atoi(r.FormValue("keepCopies"))

		job := h.backupStore.UpsertJob(storage.BackupJob{
			ID:              strings.TrimSpace(r.FormValue("id")),
			Name:            strings.TrimSpace(r.FormValue("name")),
			Sources:         sources,
			DestinationDir:  strings.TrimSpace(r.FormValue("destinationDir")),
			IntervalMinutes: interval,
			KeepCopies:      keep,
			Enabled:         r.FormValue("enabled") == "on",
		})

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"job": job})
	}).ServeHTTP(w, r)
}

// DeleteBackupJob removes one backup job.
func (h *Handler) DeleteBackupJob(w http.ResponseWriter, r *http.Request) {
	h.basicAuth(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if h.backupStore == nil {
			http.Error(w, "backup storage is disabled", http.StatusServiceUnavailable)
			return
		}
		id := strings.TrimSpace(r.FormValue("id"))
		if id == "" {
			http.Error(w, "id is required", http.StatusBadRequest)
			return
		}
		h.backupStore.DeleteJob(id)
		w.WriteHeader(http.StatusNoContent)
	}).ServeHTTP(w, r)
}

// RunBackupNow starts backup immediately for selected job.
func (h *Handler) RunBackupNow(w http.ResponseWriter, r *http.Request) {
	h.basicAuth(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if h.backupStore == nil {
			http.Error(w, "backup storage is disabled", http.StatusServiceUnavailable)
			return
		}
		id := strings.TrimSpace(r.FormValue("id"))
		if id == "" {
			http.Error(w, "id is required", http.StatusBadRequest)
			return
		}
		if err := h.backupStore.RunJobNow(id); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}).ServeHTTP(w, r)
}

// TelegramWebhook handles bot callback actions (ban buttons).
func (h *Handler) TelegramWebhook(w http.ResponseWriter, r *http.Request) {
	clog.Infof("Telegram webhook: request received method=%s remote=%s", r.Method, r.RemoteAddr)
	if r.Method != http.MethodPost {
		clog.Warnf("Telegram webhook: invalid method=%s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.notifyStore == nil || h.notifier == nil {
		clog.Warnf("Telegram webhook: notifier is disabled")
		http.Error(w, "notifier is disabled", http.StatusServiceUnavailable)
		return
	}
	cfg := h.notifyStore.Get()
	if cfg.WebhookSecret != "" && r.Header.Get("X-Telegram-Bot-Api-Secret-Token") != cfg.WebhookSecret {
		clog.Warnf("Telegram webhook: invalid secret token")
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	var update struct {
		CallbackQuery struct {
			Data    string `json:"data"`
			Message struct {
				Chat struct {
					ID int64 `json:"id"`
				} `json:"chat"`
			} `json:"message"`
		} `json:"callback_query"`
		Message struct {
			Text string `json:"text"`
			Chat struct {
				ID int64 `json:"id"`
			} `json:"chat"`
		} `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		clog.Errorf("Telegram webhook: failed to decode update: %v", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	if update.Message.Chat.ID != 0 && h.notifyStore != nil {
		h.notifyStore.RememberKnownChatID(update.Message.Chat.ID)
	}
	if update.CallbackQuery.Message.Chat.ID != 0 && h.notifyStore != nil {
		h.notifyStore.RememberKnownChatID(update.CallbackQuery.Message.Chat.ID)
	}

	if update.CallbackQuery.Data == "" {
		if strings.TrimSpace(update.Message.Text) == "" {
			clog.Debugf("Telegram webhook: empty text message ignored")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if h.gptClient == nil || h.notifier == nil {
			clog.Warnf("Telegram webhook: gpt client or notifier is nil")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		chatID := update.Message.Chat.ID
		text := strings.TrimSpace(update.Message.Text)
		clog.Infof("Telegram webhook: incoming message chat_id=%d text_len=%d", chatID, len(text))
		if text == "/start" || text == "/help" {
			clog.Debugf("Telegram webhook: help command chat_id=%d", chatID)
			_ = h.notifier.SendMessageToChat(chatID, "Привет! Я бот Router. Пишите сообщение, и я отвечу через GPT.\nКоманды: /help")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		reply, err := h.gptClient.Reply(chatID, text)
		if err != nil {
			clog.Errorf("Telegram webhook: gpt reply failed chat_id=%d err=%v", chatID, err)
			_ = h.notifier.SendMessageToChat(chatID, "Ошибка GPT: "+err.Error())
			w.WriteHeader(http.StatusNoContent)
			return
		}
		clog.Infof("Telegram webhook: sending reply chat_id=%d reply_len=%d", chatID, len(reply))
		_ = h.notifier.SendMessageToChat(chatID, reply)
		w.WriteHeader(http.StatusNoContent)
		return
	}

	ip, replyText, err := h.notifier.HandleCallback(update.CallbackQuery.Data, update.CallbackQuery.Message.Chat.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if ip == "" {
		if replyText != "" {
			h.notifier.SendActionResult("ℹ️ " + replyText)
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if h.ipStore == nil {
		http.Error(w, "ip storage is disabled", http.StatusServiceUnavailable)
		return
	}
	h.ipStore.Ban(ip)
	h.notifier.SendActionResult("⛔️ Banned from Telegram action\nip: " + ip)
	w.WriteHeader(http.StatusNoContent)
}

// NotificationsData returns telegram settings.
func (h *Handler) NotificationsData(w http.ResponseWriter, r *http.Request) {
	h.basicAuth(func(w http.ResponseWriter, r *http.Request) {
		if h.notifyStore == nil {
			http.Error(w, "notification storage is disabled", http.StatusServiceUnavailable)
			return
		}
		cfg := h.notifyStore.Get()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"config": cfg})
	}).ServeHTTP(w, r)
}

// SaveNotificationsConfig updates telegram notification settings.
func (h *Handler) SaveNotificationsConfig(w http.ResponseWriter, r *http.Request) {
	h.basicAuth(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if h.notifyStore == nil {
			http.Error(w, "notification storage is disabled", http.StatusServiceUnavailable)
			return
		}
		events := map[string]bool{}
		for _, k := range []string{"unknown_host", "suspicious_probe", "blocked_ip_hit", "manual_ban", "manual_unban", "manual_remove", "backup_success", "backup_failure", "test"} {
			events[k] = r.FormValue("event_"+k) == "on"
		}
		quietStart, _ := strconv.Atoi(r.FormValue("quietStart"))
		quietEnd, _ := strconv.Atoi(r.FormValue("quietEnd"))
		chatIDs := []int64{}
		for _, part := range strings.Split(r.FormValue("chatIds"), ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			id, err := strconv.ParseInt(part, 10, 64)
			if err == nil {
				chatIDs = append(chatIDs, id)
			}
		}
		secret := strings.TrimSpace(r.FormValue("webhookSecret"))
		if secret == "" {
			secret = notify.GenerateWebhookSecret()
		}

		cfg := storage.NotificationConfig{
			Enabled:         r.FormValue("enabled") == "on",
			Token:           strings.TrimSpace(r.FormValue("token")),
			ChatIDs:         chatIDs,
			Events:          events,
			QuietHoursOn:    r.FormValue("quietEnabled") == "on",
			QuietHoursStart: quietStart,
			QuietHoursEnd:   quietEnd,
			WebhookSecret:   secret,
			WebhookURL:      strings.TrimSpace(r.FormValue("webhookUrl")),
		}
		h.notifyStore.Update(cfg)

		warning := ""
		if cfg.Token != "" {
			proto := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto"))
			if proto == "" {
				if r.TLS != nil {
					proto = "https"
				} else {
					proto = "http"
				}
			}
			webhookURL := strings.TrimSpace(cfg.WebhookURL)
			if webhookURL == "" {
				webhookURL = proto + "://" + r.Host + "/telegram/webhook"
			}
			if !strings.HasPrefix(strings.ToLower(webhookURL), "https://") {
				warning = "Webhook not configured automatically: Telegram requires a public HTTPS URL. Set Webhook URL to https://... and save again."
				clog.Warnf("Notifications config: skip setWebhook because URL is not HTTPS: %s", webhookURL)
			} else if h.notifier != nil {
				clog.Infof("Notifications config: setting telegram webhook url=%s", webhookURL)
				if err := h.notifier.EnsureWebhook(cfg, webhookURL); err != nil {
					clog.Errorf("Notifications config: set webhook failed: %v", err)
					http.Error(w, "failed to set telegram webhook: "+err.Error(), http.StatusBadRequest)
					return
				}
				clog.Infof("Notifications config: telegram webhook set successfully")
			}
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"config": h.notifyStore.Get(), "warning": warning})
	}).ServeHTTP(w, r)
}

// TestNotification sends a test telegram message.
func (h *Handler) TestNotification(w http.ResponseWriter, r *http.Request) {
	h.basicAuth(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if h.notifier == nil {
			http.Error(w, "notifier is disabled", http.StatusServiceUnavailable)
			return
		}
		if err := h.notifier.TestMessage(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}).ServeHTTP(w, r)
}

// SettingsData returns GPT settings.
func (h *Handler) SettingsData(w http.ResponseWriter, r *http.Request) {
	h.basicAuth(func(w http.ResponseWriter, r *http.Request) {
		if h.gptStore == nil {
			http.Error(w, "gpt storage is disabled", http.StatusServiceUnavailable)
			return
		}
		cfg := h.gptStore.Get()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"config": cfg})
	}).ServeHTTP(w, r)
}

// SaveSettingsConfig updates GPT settings.
func (h *Handler) SaveSettingsConfig(w http.ResponseWriter, r *http.Request) {
	h.basicAuth(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if h.gptStore == nil {
			http.Error(w, "gpt storage is disabled", http.StatusServiceUnavailable)
			return
		}

		maxLogLines, _ := strconv.Atoi(r.FormValue("maxLogLines"))
		onlyChatIDs := []int64{}
		for _, part := range strings.Split(r.FormValue("onlyChatIds"), ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			id, err := strconv.ParseInt(part, 10, 64)
			if err == nil {
				onlyChatIDs = append(onlyChatIDs, id)
			}
		}

		cfg := storage.GPTConfig{
			Enabled:      r.FormValue("enabled") == "on",
			APIKey:       strings.TrimSpace(r.FormValue("apiKey")),
			Model:        strings.TrimSpace(r.FormValue("model")),
			SystemPrompt: r.FormValue("systemPrompt"),
			MaxLogLines:  maxLogLines,
			OnlyChatIDs:  onlyChatIDs,
		}
		h.gptStore.Update(cfg)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"config": h.gptStore.Get()})
	}).ServeHTTP(w, r)
}

// Login serves and handles login form.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		if h.isAuthenticated(r) {
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}
		http.ServeFile(w, r, "internal/panel/static/login.html")
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.adminStore == nil {
		http.Error(w, "admin storage is disabled", http.StatusServiceUnavailable)
		return
	}
	clientIP := clientIPFromRequest(r)
	if retryAfter, blocked := h.checkLoginBlocked(clientIP); blocked {
		w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
		http.Error(w, "Too many failed login attempts. Try later.", http.StatusTooManyRequests)
		return
	}
	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")
	if !h.adminStore.Verify(username, password) {
		h.registerLoginFailure(clientIP)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	h.clearLoginFailures(clientIP)
	token := h.createSession()
	http.SetCookie(w, &http.Cookie{Name: "router_session", Value: token, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: 86400})
	w.WriteHeader(http.StatusNoContent)
}

// Logout deletes active session.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	h.basicAuth(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		cookie, _ := r.Cookie("router_session")
		if cookie != nil {
			h.invalidateSession(cookie.Value)
		}
		http.SetCookie(w, &http.Cookie{Name: "router_session", Value: "", Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: -1})
		w.WriteHeader(http.StatusNoContent)
	}).ServeHTTP(w, r)
}

// Account serves account settings page.
func (h *Handler) Account(w http.ResponseWriter, r *http.Request) {
	h.basicAuth(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "internal/panel/static/account.html")
	}).ServeHTTP(w, r)
}

// AccountData returns current account settings.
func (h *Handler) AccountData(w http.ResponseWriter, r *http.Request) {
	h.basicAuth(func(w http.ResponseWriter, r *http.Request) {
		if h.adminStore == nil {
			http.Error(w, "admin storage is disabled", http.StatusServiceUnavailable)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"username": h.adminStore.Username()})
	}).ServeHTTP(w, r)
}

// SaveAccountConfig updates username/password.
func (h *Handler) SaveAccountConfig(w http.ResponseWriter, r *http.Request) {
	h.basicAuth(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if h.adminStore == nil {
			http.Error(w, "admin storage is disabled", http.StatusServiceUnavailable)
			return
		}
		username := strings.TrimSpace(r.FormValue("username"))
		password := strings.TrimSpace(r.FormValue("password"))
		if username == "" || len(password) < 6 {
			http.Error(w, "username is required and password must be at least 6 chars", http.StatusBadRequest)
			return
		}
		if !h.adminStore.Update(username, password) {
			http.Error(w, "failed to update account", http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"username": h.adminStore.Username()})
	}).ServeHTTP(w, r)
}
