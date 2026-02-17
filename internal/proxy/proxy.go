package proxy

import (
<<<<<<< HEAD
	"html/template"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path/filepath"
	"router/internal/clog"
=======
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"router/internal/logstream"
>>>>>>> main
	"router/internal/stats"
	"router/internal/storage"
)

// Proxy is a reverse proxy that forwards requests to different targets
type Proxy struct {
<<<<<<< HEAD
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
=======
	store              *storage.RuleStore
	stats              *stats.Stats
	broadcaster        *logstream.Broadcaster
	defaultTarget      string
	maintenanceHandler http.HandlerFunc
}

// NewProxy creates a new Proxy
func NewProxy(store *storage.RuleStore, stats *stats.Stats, broadcaster *logstream.Broadcaster, defaultTarget string, maintenanceHandler http.HandlerFunc) *Proxy {
	return &Proxy{
		store:              store,
		stats:              stats,
		broadcaster:        broadcaster,
		defaultTarget:      defaultTarget,
		maintenanceHandler: maintenanceHandler,
>>>>>>> main
	}
}

// ServeHTTP handles incoming requests
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
<<<<<<< HEAD
	if p.store.MaintenanceMode {
		clog.Infof("[maintenance-global] %s %s host=%s", r.Method, r.URL.Path, r.Host)
		if serveMaintenanceStatic(w, r) {
			return
		}

		w.WriteHeader(http.StatusServiceUnavailable)
		p.maintenanceTmpl.Execute(w, nil)
		return
	}

	rule, ok := p.store.GetRule(r.Host)
	if !ok {
		clog.Warnf("[no-rule] %s %s host=%s remote=%s", r.Method, r.URL.Path, r.Host, r.RemoteAddr)
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	if rule.Maintenance {
		clog.Infof("[maintenance-rule] %s %s host=%s", r.Method, r.URL.Path, r.Host)
		if serveMaintenanceStatic(w, r) {
			return
		}

		w.WriteHeader(http.StatusServiceUnavailable)
		p.maintenanceTmpl.Execute(w, nil)
		return
	}

	// Add request to stats with the specific host
	p.stats.AddRequest(r.Host, stats.CountryFromRequest(r))

	targetURL, err := url.Parse("http://" + rule.Target)
	if err != nil {
		clog.Errorf("Error parsing target URL for host %s: %v", r.Host, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
=======
	if p.store.IsMaintenanceMode() {
		p.maintenanceHandler(w, r)
		return
	}

	host := r.Host
	// Strip port from host
	if i := strings.Index(host, ":"); i != -1 {
		host = host[:i]
	}

	// Record request for stats
	p.stats.AddRequest(host)

	// Log the request
	p.broadcaster.Write([]byte(host + r.URL.Path))

	// Look up the target for the host
	target, ok := p.store.Get(host)
	if !ok {
		// If no rule is found, use the default target if it's set
		if p.defaultTarget != "" {
			target = p.defaultTarget
		} else {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}
	}

	// Parse the target URL
	url, err := url.Parse(target)
	if err != nil {
		http.Error(w, "Invalid target URL", http.StatusInternalServerError)
>>>>>>> main
		return
	}

	// Create the reverse proxy
	proxy := httputil.NewSingleHostReverseProxy(url)
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
