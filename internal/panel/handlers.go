package panel

import (
	"embed"
	"html/template"
	"log"
	"net/http"
	"router/internal/config"
	"router/internal/storage"
)

//go:embed templates/*.html
var templatesFS embed.FS

func StartPanel(addr string, cfg *config.Config, store *storage.RuleStore) error {
	mux := http.NewServeMux()

	tmpl, err := template.ParseFS(templatesFS, "templates/layout.html", "templates/index.html")
	if err != nil {
		return err
	}

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if err := tmpl.ExecuteTemplate(w, "layout.html", struct{ Rules map[string]string }{Rules: store.All()}); err != nil {
			log.Printf("Error executing template: %v", err)
		}
	})

	mux.HandleFunc("/add", func(w http.ResponseWriter, r *http.Request) {
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

	mux.HandleFunc("/remove", func(w http.ResponseWriter, r *http.Request) {
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

	// Добавляем обработчик для статических файлов, если они есть.
	// fs := http.FileServer(http.Dir("internal/panel/static"))
	// mux.Handle("/static/", http.StripPrefix("/static/", fs))

	log.Printf("Control panel is available at http://%s", addr)
	return http.ListenAndServe(addr, mux) // Пока без аутентификации для упрощения
}
