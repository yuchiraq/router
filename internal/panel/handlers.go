package panel

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"router/internal/stats"
	"router/internal/storage"
)

type Handler struct {
	store    *storage.RuleStore
	username string
	password string
	tmpl     *template.Template
	stats    *stats.Stats
}

func NewHandler(store *storage.RuleStore, username, password string, stats *stats.Stats) *Handler {
	tmpl := template.Must(template.ParseGlob("internal/panel/templates/*.html"))

	return &Handler{
		store:    store,
		username: username,
		password: password,
		tmpl:     tmpl,
		stats:    stats,
	}
}

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

func (h *Handler) Index(w http.ResponseWriter, r *http.Request) {
	h.basicAuth(func(w http.ResponseWriter, r *http.Request) {
		data := map[string]interface{}{
			"Rules": h.store.All(),
		}
		if err := h.tmpl.ExecuteTemplate(w, "layout", data); err != nil {
			log.Printf("Error executing template: %v", err)
		}
	}).ServeHTTP(w,r)
}

func (h *Handler) Stats(w http.ResponseWriter, r *http.Request) {
	h.basicAuth(func(w http.ResponseWriter, r *http.Request) {
		if err := h.tmpl.ExecuteTemplate(w, "layout", nil); err != nil {
			log.Printf("Error executing template: %v", err)
		}
	}).ServeHTTP(w,r)
}

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

func (h *Handler) ServeStyles(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "internal/panel/templates/styles.css")
}

func (h *Handler) StatsData(w http.ResponseWriter, r *http.Request) {
	h.basicAuth(func(w http.ResponseWriter, r *http.Request) {
		requestLabels, requestValues := h.stats.GetRequestData()
		memoryLabels, memoryValues := h.stats.GetMemoryData()

		data := map[string]interface{}{
			"requests": map[string]interface{}{
				"labels": requestLabels,
				"values": requestValues,
			},
			"memory": map[string]interface{}{
				"labels": memoryLabels,
				"values": memoryValues,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(data); err != nil {
			log.Printf("Error encoding stats data: %v", err)
		}
	}).ServeHTTP(w,r)
}
