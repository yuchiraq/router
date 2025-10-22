package panel

import (
	"html/template"
	"net/http"
	"router/internal/storage"
)

type Handler struct {
	store    *storage.RuleStore
	username string
	password string
	tmpl     *template.Template
}

func NewHandler(store *storage.RuleStore, username, password string) *Handler {
	tmpl := template.Must(template.ParseFiles("internal/panel/templates/layout.html", "internal/panel/templates/index.html"))
	return &Handler{
		store:    store,
		username: username,
		password: password,
		tmpl:     tmpl,
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
	if r.URL.Path != "/" {
		http.NotFound(w,r)
		return
	}
	h.basicAuth(h.indexHandler).ServeHTTP(w,r)
}

func (h *Handler) indexHandler(w http.ResponseWriter, r *http.Request) {
	if err := h.tmpl.ExecuteTemplate(w, "layout.html", struct{ Rules map[string]*storage.Rule }{Rules: h.store.All()}); err != nil {
		http.Error(w, "failed to execute template", http.StatusInternalServerError)
	}
}


func (h *Handler) AddRule(w http.ResponseWriter, r *http.Request) {
	h.basicAuth(h.addRuleHandler).ServeHTTP(w, r)
}

func (h *Handler) addRuleHandler(w http.ResponseWriter, r *http.Request) {
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
}

func (h *Handler) RemoveRule(w http.ResponseWriter, r *http.Request) {
	h.basicAuth(h.removeRuleHandler).ServeHTTP(w, r)
}

func (h *Handler) removeRuleHandler(w http.ResponseWriter, r *http.Request) {
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
}
