package proxy

import (
	"html/template"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path/filepath"
	"router/internal/clog"
	"router/internal/stats"
	"router/internal/storage"
	"strings"
)

// Proxy is a reverse proxy that uses a RuleStore to determine the target.
type Proxy struct {
	store           *storage.RuleStore
	stats           *stats.Stats
	reputation      *storage.IPReputationStore
	maintenanceTmpl *template.Template
}

// NewProxy creates a new Proxy.
func NewProxy(store *storage.RuleStore, stats *stats.Stats, reputation *storage.IPReputationStore) *Proxy {
	maintenanceTmpl := template.Must(template.ParseFiles("internal/panel/templates/maintenance.html"))
	return &Proxy{
		store:           store,
		stats:           stats,
		reputation:      reputation,
		maintenanceTmpl: maintenanceTmpl,
	}
}

// ServeHTTP handles the proxying of requests.
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	remoteIP := clientIP(r.RemoteAddr)
	if p.reputation != nil && p.reputation.IsBanned(remoteIP) {
		clog.Warnf("[blocked-ip] %s %s host=%s remote=%s", r.Method, r.URL.Path, r.Host, remoteIP)
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

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
		if p.reputation != nil {
			p.reputation.MarkSuspicious(remoteIP, "unknown host")
		}
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	if p.reputation != nil && suspiciousPath(r.URL.Path) {
		p.reputation.MarkSuspicious(remoteIP, "suspicious path probe")
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

func clientIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return host
}

func suspiciousPath(path string) bool {
	path = strings.ToLower(path)
	probes := []string{".env", "wp-admin", "wp-login", "phpmyadmin", "adminer", "/etc/passwd", "/.git"}
	for _, p := range probes {
		if strings.Contains(path, p) {
			return true
		}
	}
	return false
}
