package panel

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"

	"router/internal/config"
	"router/internal/storage"
)

func StartPanel(addr string, cfg *config.Config, store *storage.RuleStore) error {
	mux := http.NewServeMux()

	// HTML
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		tmpl, err := template.ParseFiles(
			"internal/panel/templates/layout.html",
			"internal/panel/templates/index.html",
		)
		if err != nil {
			log.Println("template error:", err)
			http.Error(w, "Template error", 500)
			return
		}
		if err := tmpl.ExecuteTemplate(w, "layout", nil); err != nil {
			log.Println("execute template:", err)
		}
	})

	// API
	mux.HandleFunc("/api/routes", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		respondJSON(w, store.All())
	})

	mux.HandleFunc("/api/add", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		domain := r.FormValue("domain")
		target := r.FormValue("target")
		if domain == "" || target == "" {
			http.Error(w, "domain and target required", http.StatusBadRequest)
			return
		}
		store.Add(domain, target)
		respondJSON(w, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("/api/toggle", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		domain := r.URL.Query().Get("domain")
		if domain == "" {
			http.Error(w, "domain required", http.StatusBadRequest)
			return
		}
		store.Toggle(domain)
		respondJSON(w, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("/api/delete", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		domain := r.URL.Query().Get("domain")
		if domain == "" {
			http.Error(w, "domain required", http.StatusBadRequest)
			return
		}
		store.Remove(domain)
		respondJSON(w, map[string]string{"status": "ok"})
	})

	protected := basicAuth(cfg, mux)
	log.Printf("Панель доступна на %s", addr)
	return http.ListenAndServe(addr, protected)
}

func respondJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}
