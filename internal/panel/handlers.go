package panel

import (
	"encoding/json"
	"html/template"
	"net/http"
	"net/url"
	"router/internal/clog"
	"strconv"
	"strings"

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

// Handler holds all dependencies for the web panel
type Handler struct {
	store       *storage.RuleStore
	username    string
	password    string
	templates   map[string]*template.Template
	stats       *stats.Stats
	broadcaster *logstream.Broadcaster
	ipStore     *storage.IPReputationStore
	backupStore *storage.BackupStore
	notifyStore *storage.NotificationStore
	notifier    *notify.TelegramNotifier
}

// NewHandler creates a new panel handler
func NewHandler(store *storage.RuleStore, username, password string, stats *stats.Stats, broadcaster *logstream.Broadcaster, ipStore *storage.IPReputationStore, backupStore *storage.BackupStore, notifyStore *storage.NotificationStore, notifier *notify.TelegramNotifier) *Handler {
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
		username:    username,
		password:    password,
		templates:   templates,
		stats:       stats,
		broadcaster: broadcaster,
		ipStore:     ipStore,
		backupStore: backupStore,
		notifyStore: notifyStore,
		notifier:    notifier,
	}
}

// basicAuth is a middleware for basic authentication
func (h *Handler) basicAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.username == "" && h.password == "" {
			next.ServeHTTP(w, r)
			return
		}

		user, pass, ok := r.BasicAuth()
		if !ok || user != h.username || pass != h.password {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	}
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

		h.notifyStore.Update(storage.NotificationConfig{
			Enabled:         r.FormValue("enabled") == "on",
			Token:           strings.TrimSpace(r.FormValue("token")),
			ChatID:          strings.TrimSpace(r.FormValue("chatId")),
			Events:          events,
			QuietHoursOn:    r.FormValue("quietEnabled") == "on",
			QuietHoursStart: quietStart,
			QuietHoursEnd:   quietEnd,
		})
		w.WriteHeader(http.StatusNoContent)
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
