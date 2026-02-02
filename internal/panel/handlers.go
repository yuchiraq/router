
package panel

import (
	"html/template"
	"log"
	"net/http"
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

// NewHandler creates a new Handler
func NewHandler(store *storage.RuleStore, user, pass string, stats *stats.Stats, broadcaster *logstream.Broadcaster) *Handler {
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
	data := map[string]interface{}{
		"Rules":           h.store.All(),
		"MaintenanceMode": h.store.IsMaintenanceMode(),
	}
	h.render(w, r, "index", data)
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

// Stats is the handler for the stats page
func (h *Handler) Stats(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, "stats", nil)
}

// StatsData is the handler for providing stats data via SSE
func (h *Handler) StatsData(w http.ResponseWriter, r *http.Request) {
	// Implementation for SSE with stats data
}

// Logs is the handler for the logs WebSocket
func (h *Handler) Logs(w http.ResponseWriter, r *http.Request) {
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
}
