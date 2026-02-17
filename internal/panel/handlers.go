package panel

import (
	"encoding/json"
	"html/template"
	"net/http"
	"router/internal/clog"

	"router/internal/logstream"
	"router/internal/stats"
	"router/internal/storage"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all connections
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
}

// NewHandler creates a new panel handler
func NewHandler(store *storage.RuleStore, username, password string, stats *stats.Stats, broadcaster *logstream.Broadcaster) *Handler {
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
	h.stats.RecordMemory()
	h.stats.RecordCPU()
	h.stats.RecordDisks()
	h.stats.RecordSSHConnections()
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
		"disks":     diskData,
		"countries": countryData,
		"ssh":       sshData,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		clog.Errorf("Error encoding stats data: %v", err)
	}
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
