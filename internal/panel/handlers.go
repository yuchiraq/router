
package panel

import (
	"html/template"
	"log"
	"net/http"
<<<<<<< HEAD
	"router/internal/clog"
=======
	"path/filepath"

	"router/internal/logstream"
	"router/internal/stats"
	"router/internal/storage"

	"github.com/gorilla/websocket"
)

// Handler holds the dependencies for the admin panel handlers
type Handler struct {
	store       *storage.RuleStore
	user        string
	pass        string
	stats       *stats.Stats
	broadcaster *logstream.Broadcaster
	// templates   *template.Template
	upgrader    websocket.Upgrader
}

<<<<<<< HEAD
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

=======
// NewHandler creates a new Handler
func NewHandler(store *storage.RuleStore, user, pass string, stats *stats.Stats, broadcaster *logstream.Broadcaster) *Handler {
>>>>>>> main
	return &Handler{
		store:       store,
		user:        user,
		pass:        pass,
		stats:       stats,
		broadcaster: broadcaster,
		// templates:   templates,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
	}
}

// render executes the given template
func (h *Handler) render(w http.ResponseWriter, r *http.Request, name string, data interface{}) {
	templateData := map[string]interface{}{
		"Page": name,
		"Data": data,
	}

<<<<<<< HEAD
	if err := tmpl.ExecuteTemplate(w, "layout", templateData); err != nil {
		clog.Errorf("Error executing template %s: %v", name, err)
=======
	tmpl, err := template.ParseFiles(filepath.Join("internal", "panel", "templates", "layout.html"), filepath.Join("internal", "panel", "templates", name+".html"))
	if err != nil {
		log.Printf("Error parsing template %s: %v", name, err)
		http.Error(w, "Error rendering page", http.StatusInternalServerError)
		return
	}

	if err := tmpl.ExecuteTemplate(w, "layout.html", templateData); err != nil {
		log.Printf("Error executing template %s: %v", name, err)
		http.Error(w, "Error rendering page", http.StatusInternalServerError)
	}
}

// basicAuth performs basic authentication
func (h *Handler) basicAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.user == "" || h.pass == "" {
			next.ServeHTTP(w, r)
			return
		}

		user, pass, ok := r.BasicAuth()
		if !ok || user != h.user || pass != h.pass {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	}
}

// Index is the handler for the dashboard page
func (h *Handler) Index(w http.ResponseWriter, r *http.Request) {
<<<<<<< HEAD
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
=======
	data := map[string]interface{}{
		"Rules":           h.store.All(),
		"MaintenanceMode": h.store.IsMaintenanceMode(),
	}
	h.render(w, r, "index", data)
>>>>>>> main
}

// AddRule is the handler for adding a new routing rule
func (h *Handler) AddRule(w http.ResponseWriter, r *http.Request) {
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
	http.Redirect(w, r, "/admin/", http.StatusFound)
}

// RemoveRule is the handler for removing a routing rule
func (h *Handler) RemoveRule(w http.ResponseWriter, r *http.Request) {
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
	http.Redirect(w, r, "/admin/", http.StatusFound)
}

<<<<<<< HEAD
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
=======
// Stats is the handler for the stats page
func (h *Handler) Stats(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, "stats", nil)
}

// StatsData is the handler for providing stats data via SSE
func (h *Handler) StatsData(w http.ResponseWriter, r *http.Request) {
	// Implementation for SSE with stats data
>>>>>>> main
}

// Logs is the handler for the logs WebSocket
func (h *Handler) Logs(w http.ResponseWriter, r *http.Request) {
<<<<<<< HEAD
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
=======
	// Implementation for WebSocket with logs
}

// ToggleMaintenance is the handler for toggling maintenance mode
func (h *Handler) ToggleMaintenance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	enabled := r.FormValue("maintenance") == "on"
	h.store.SetMaintenanceMode(enabled)
	http.Redirect(w, r, "/admin/", http.StatusFound)
}

// Maintenance is the handler for the maintenance page
func (h *Handler) Maintenance(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, "maintenance", nil)
>>>>>>> main
}
