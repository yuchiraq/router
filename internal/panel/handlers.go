package panel

import (
	"embed"
	"html/template"
	"log"
	"net/http"
	"router/internal/config"
	"router/internal/storage"
)

//go:embed templates/*
var templatesFS embed.FS

func StartPanel(addr string, cfg *config.Config, store *storage.RuleStore) error {
	mux := http.NewServeMux()

	tmpl, err := template.New("").ParseFS(templatesFS, "templates/layout.html", "templates/index.html")
	if err != nil {
		return err
	}

	// Handler for the main page
	mainHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		if err := tmpl.ExecuteTemplate(w, "layout.html", struct{ Rules map[string]*storage.Rule }{Rules: store.All()}); err != nil {
			log.Printf("Error executing template: %v", err)
		}
	})

	// Handler for adding a rule
	addHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		store.Add(host, target)
		http.Redirect(w, r, "/", http.StatusFound)
	})

	// Handler for removing a rule
	removeHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		host := r.FormValue("host")
		if host == "" {
			http.Error(w, "Host is required", http.StatusBadRequest)
			return
		}
		store.Remove(host)
		http.Redirect(w, r, "/", http.StatusFound)
	})

	// Static file server for CSS
	staticFS, err := http.FS(templatesFS)
	if err != nil {
		return err
	}
	fs := http.StripPrefix("/static/", http.FileServer(staticFS))

	mux.Handle("/static/", fs)
	mux.Handle("/add", BasicAuth(addHandler, cfg))
	mux.Handle("/remove", BasicAuth(removeHandler, cfg))
	mux.Handle("/", BasicAuth(mainHandler, cfg))

	log.Printf("Control panel is available at http://%s", addr)
	return http.ListenAndServe(addr, mux)
}
