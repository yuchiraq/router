package proxy

import (
	"html/template"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path/filepath"
	"router/internal/clog"
	"router/internal/notify"
	"router/internal/stats"
	"router/internal/storage"
	"strings"
)

// Proxy is a reverse proxy that uses a RuleStore to determine the target.
type Proxy struct {
	store           *storage.RuleStore
	stats           *stats.Stats
	reputation      *storage.IPReputationStore
	notifier        *notify.TelegramNotifier
	maintenanceTmpl *template.Template
}

// NewProxy creates a new Proxy.
func NewProxy(store *storage.RuleStore, stats *stats.Stats, reputation *storage.IPReputationStore, notifier *notify.TelegramNotifier) *Proxy {
	maintenanceTmpl := template.Must(template.ParseFiles("internal/panel/templates/maintenance.html"))
	return &Proxy{
		store:           store,
		stats:           stats,
		reputation:      reputation,
		notifier:        notifier,
		maintenanceTmpl: maintenanceTmpl,
	}
}

// ServeHTTP handles the proxying of requests.
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	socketIP := remoteAddrIP(r.RemoteAddr)
	remoteIP := clientIP(r)
	if p.reputation != nil && p.reputation.IsBanned(remoteIP) {
		clog.Warnf("[blocked-ip] %s %s host=%s remote=%s", r.Method, r.URL.Path, r.Host, remoteIP)
		if p.notifier != nil {
			p.notifier.Notify("blocked_ip_hit", "blocked:"+remoteIP+":"+r.URL.Path, notify.BuildProxyAlert(r.Method, r.URL.Path, r.Host, remoteIP, "blocked IP attempted request"))
		}
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
		if p.notifier != nil {
			p.notifier.Notify("unknown_host", "unknown-host:"+remoteIP+":"+r.Host, notify.BuildProxyAlert(r.Method, r.URL.Path, r.Host, remoteIP, "unknown host"))
		}
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	if p.reputation != nil && suspiciousPath(r.URL.Path) {
		p.reputation.MarkSuspicious(remoteIP, "suspicious path probe")
		if p.notifier != nil {
			p.notifier.Notify("suspicious_probe", "probe:"+remoteIP+":"+r.URL.Path, notify.BuildProxyAlert(r.Method, r.URL.Path, r.Host, remoteIP, "suspicious path probe"))
		}
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
	r.Header.Set("X-Real-IP", remoteIP)
	r.Header.Set("X-Forwarded-Host", r.Header.Get("Host"))
	r.Header.Set("X-Forwarded-For", appendForwardedFor(r.Header.Get("X-Forwarded-For"), socketIP))
	r.Host = targetURL.Host

	clog.Infof("[proxy-forward] %s %s src=%s remote=%s xff=%q host=%s -> %s", r.Method, r.URL.Path, remoteIP, r.RemoteAddr, r.Header.Get("X-Forwarded-For"), r.Header.Get("X-Forwarded-Host"), targetURL.Host)

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

func clientIP(r *http.Request) string {
	socketIP := remoteAddrIP(r.RemoteAddr)

	if ip := firstValidIP(r.Header.Get("CF-Connecting-IP")); ip != "" {
		if isPublicIP(ip) || !isPublicIP(socketIP) {
			return ip
		}
	}

	if ip := firstPublicIPFromXFF(r.Header.Get("X-Forwarded-For")); ip != "" {
		return ip
	}

	if ip := firstValidIP(r.Header.Get("X-Real-IP")); ip != "" {
		if isPublicIP(ip) || !isPublicIP(socketIP) {
			return ip
		}
	}

	if isPublicIP(socketIP) {
		return socketIP
	}

	if ip := firstValidIPFromXFF(r.Header.Get("X-Forwarded-For")); ip != "" {
		return ip
	}

	if ip := firstValidIP(r.Header.Get("X-Real-IP")); ip != "" {
		return ip
	}

	return socketIP
}

func remoteAddrIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return strings.TrimSpace(remoteAddr)
	}
	return strings.TrimSpace(host)
}

func firstPublicIPFromXFF(xff string) string {
	parts := strings.Split(xff, ",")
	for _, part := range parts {
		ip := firstValidIP(part)
		if ip == "" {
			continue
		}
		if isPublicIP(ip) {
			return ip
		}
	}
	return ""
}

func firstValidIPFromXFF(xff string) string {
	parts := strings.Split(xff, ",")
	for _, part := range parts {
		if ip := firstValidIP(part); ip != "" {
			return ip
		}
	}
	return ""
}

func firstValidIP(value string) string {
	ip := strings.TrimSpace(value)
	if ip == "" {
		return ""
	}
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return ""
	}
	return parsed.String()
}

func isPublicIP(ip string) bool {
	parsed := net.ParseIP(strings.TrimSpace(ip))
	if parsed == nil {
		return false
	}
	if parsed.IsLoopback() || parsed.IsPrivate() || parsed.IsLinkLocalMulticast() || parsed.IsLinkLocalUnicast() || parsed.IsMulticast() || parsed.IsUnspecified() {
		return false
	}
	return true
}

func appendForwardedFor(existing, remoteIP string) string {
	if strings.TrimSpace(existing) == "" {
		return remoteIP
	}
	return existing + ", " + remoteIP
}

func appendForwardedFor(existing, remoteIP string) string {
	if strings.TrimSpace(existing) == "" {
		return remoteIP
	}
	return existing + ", " + remoteIP
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
