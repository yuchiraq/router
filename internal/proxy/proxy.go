package proxy

import (
	"encoding/json"
	"html/template"
	"net"
	"net/http"
	"net/http/httputil"
	"router/internal/clog"
	"router/internal/notify"
	"router/internal/stats"
	"router/internal/storage"
	"strings"
	"time"
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
	if workServiceHost(r.Host) {
		remoteIP = socketIP
	}
	if p.reputation != nil && p.reputation.IsBanned(remoteIP) {
		clog.Warnf("[blocked-ip] %s %s host=%s remote=%s", r.Method, r.URL.Path, r.Host, remoteIP)
		if p.notifier != nil {
			p.notifier.Notify("blocked_ip_hit", "blocked:"+remoteIP+":"+r.URL.Path, notify.BuildProxyAlert(r.Method, r.URL.Path, r.Host, remoteIP, "blocked IP attempted request"))
		}
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	rule, ok := p.store.GetRule(r.Host)
	panelRoute := ok && p.store.IsPanelTarget(rule.Target)
	if ok && ServeMaintenanceAsset(w, r) {
		return
	}
	if r.URL.Path == "/_router/maintenance/status" {
		if !ok {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		available := panelRoute || !p.store.IsMaintenanceMode() && !rule.Maintenance && !rule.ServiceDown && !rule.HealthCheckedAt.IsZero()
		if r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(map[string]bool{"available": available})
		}
		return
	}
	if p.store.IsMaintenanceMode() && !panelRoute {
		clog.Infof("[maintenance-global] %s %s host=%s", r.Method, r.URL.Path, r.Host)
		p.serveMaintenance(w, r, false)
		return
	}

	if !ok {
		clog.Warnf("[no-rule] %s %s host=%s remote=%s", r.Method, r.URL.Path, r.Host, r.RemoteAddr)
		if p.reputation != nil {
			autoBanned, banUntil := p.reputation.MarkSuspicious(remoteIP, "unknown host")
			if autoBanned && p.notifier != nil {
				p.notifier.Notify("auto_ban", "auto-ban:"+remoteIP, "🤖 Auto-ban activated\nip: "+remoteIP+"\nreason: too many suspicious requests\nuntil: "+banUntil.Format(time.RFC3339))
			}
		}
		if p.notifier != nil {
			p.notifier.NotifyWithBanButton("unknown_host", "unknown-host:"+remoteIP+":"+r.Host, notify.BuildProxyAlert(r.Method, r.URL.Path, r.Host, remoteIP, "unknown host"), remoteIP)
		}
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	if p.reputation != nil && suspiciousPath(r.URL.Path) {
		autoBanned, banUntil := p.reputation.MarkSuspicious(remoteIP, "suspicious path probe")
		if autoBanned && p.notifier != nil {
			p.notifier.Notify("auto_ban", "auto-ban:"+remoteIP, "🤖 Auto-ban activated\nip: "+remoteIP+"\nreason: too many suspicious requests\nuntil: "+banUntil.Format(time.RFC3339))
		}
		if p.notifier != nil {
			p.notifier.NotifyWithBanButton("suspicious_probe", "probe:"+remoteIP+":"+r.URL.Path, notify.BuildProxyAlert(r.Method, r.URL.Path, r.Host, remoteIP, "suspicious path probe"), remoteIP)
		}
	}

	if (rule.Maintenance || rule.ServiceDown) && !panelRoute {
		clog.Infof("[maintenance-rule] %s %s host=%s", r.Method, r.URL.Path, r.Host)
		p.serveMaintenance(w, r, !rule.Maintenance && rule.ServiceDown)
		return
	}

	// Add request to stats with the specific host
	p.stats.AddRequest(r.Host, stats.CountryFromRequest(r))

	targetURL, err := storage.ParseTarget(rule.Target)
	if err != nil {
		clog.Errorf("Error parsing target URL for host %s: %v", r.Host, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		if r.Context().Err() != nil {
			return
		}
		p.store.ReportServiceFailure(rule.Host, rule.Target)
		clog.Warnf("upstream unavailable host=%s", rule.Host)
		p.serveMaintenance(w, r, true)
	}

	// Update the request headers
	incomingHost := r.Host
	r.URL.Host = targetURL.Host
	r.URL.Scheme = targetURL.Scheme
	r.Header.Set("X-Real-IP", remoteIP)
	r.Header.Set("X-Forwarded-Host", r.Header.Get("Host"))
	r.Header.Set("X-Forwarded-For", appendForwardedFor(r.Header.Get("X-Forwarded-For"), socketIP))
	r.Host = targetURL.Host
	setWorkServiceHeaders(r, incomingHost, socketIP)

	clog.Infof("[proxy-forward] %s %s src=%s remote=%s xff=%q host=%s -> %s", r.Method, r.URL.Path, remoteIP, r.RemoteAddr, r.Header.Get("X-Forwarded-For"), r.Header.Get("X-Forwarded-Host"), targetURL.Host)

	proxy.ServeHTTP(w, r)
}

func workServiceHost(host string) bool {
	return strings.EqualFold(host, "appworks.avayusstroi.by") || strings.EqualFold(host, "appworks.avayusstroi.by:443")
}

// WorkService trusts forwarding metadata only from this TLS-terminating proxy.
// Preserve the behavior of unrelated routes until they are migrated separately.
func setWorkServiceHeaders(r *http.Request, incomingHost, socketIP string) {
	if !workServiceHost(incomingHost) {
		return
	}
	r.Host = "appworks.avayusstroi.by"
	r.Header.Set("X-Forwarded-Host", r.Host)
	r.Header.Set("X-Forwarded-Proto", "http")
	if r.TLS != nil {
		r.Header.Set("X-Forwarded-Proto", "https")
	}
	r.Header.Set("X-Forwarded-For", socketIP)
	r.Header.Set("X-Real-IP", socketIP)
	r.Header.Del("Forwarded")
	r.Header.Del("CF-Connecting-IP")
}

func (p *Proxy) serveMaintenance(w http.ResponseWriter, r *http.Request, automatic bool) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Retry-After", "15")
	w.Header().Set("X-Router-Maintenance", "1")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	htmlResponse := (r.Method == http.MethodGet || r.Method == http.MethodHead) && (strings.Contains(r.Header.Get("Accept"), "text/html") || r.Header.Get("Accept") == "")
	if htmlResponse {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	} else {
		w.Header().Set("Content-Type", "application/json")
	}
	w.WriteHeader(http.StatusServiceUnavailable)
	if r.Method == http.MethodHead {
		return
	}
	if htmlResponse {
		if err := p.maintenanceTmpl.Execute(w, map[string]bool{"Automatic": automatic}); err != nil {
			clog.Errorf("maintenance page: %v", err)
		}
	} else {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "service_unavailable", "message": "Сервис временно недоступен"})
	}
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
