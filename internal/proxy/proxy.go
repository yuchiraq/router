package proxy

import (
	"html/template"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path/filepath"
	"router/internal/stats"
	"router/internal/storage"
)

// Proxy is a reverse proxy that uses a RuleStore to determine the target.
type Proxy struct {
	store           *storage.RuleStore
	stats           *stats.Stats
	maintenanceTmpl *template.Template
}

// NewProxy creates a new Proxy.
func NewProxy(store *storage.RuleStore, stats *stats.Stats) *Proxy {
	maintenanceTmpl := template.Must(template.ParseFiles("internal/panel/templates/maintenance.html"))
	return &Proxy{
		store:           store,
		stats:           stats,
		maintenanceTmpl: maintenanceTmpl,
	}
}

// ServeHTTP handles the proxying of requests.
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if p.store.MaintenanceMode {
		if serveMaintenanceStatic(w, r) {
			return
		}

		w.WriteHeader(http.StatusServiceUnavailable)
		p.maintenanceTmpl.Execute(w, nil)
		return
	}

	rule, ok := p.store.GetRule(r.Host)
	if !ok {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	if rule.Maintenance {
		if serveMaintenanceStatic(w, r) {
			return
		}

		w.WriteHeader(http.StatusServiceUnavailable)
		p.maintenanceTmpl.Execute(w, nil)
		return
	}

	// Add request to stats with the specific host
	p.stats.AddRequest(r.Host)

	targetURL, err := url.Parse("http://" + rule.Target)
	if err != nil {
		log.Printf("Error parsing target URL for host %s: %v", r.Host, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	// Update the request headers
	r.URL.Host = targetURL.Host
	r.URL.Scheme = targetURL.Scheme
	r.Header.Set("X-Forwarded-Host", r.Header.Get("Host"))
	r.Host = targetURL.Host

	proxy.ServeHTTP(w, r)
}

func serveMaintenanceStatic(w http.ResponseWriter, r *http.Request) bool {
	if r.URL.Path != "/static/styles.css" {
		return false
	}

	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return true
	}

	http.ServeFile(w, r, filepath.Clean("internal/panel/static/styles.css"))
	return true
}
