
package proxy

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"router/internal/logstream"
	"router/internal/stats"
	"router/internal/storage"
)

// Proxy is a reverse proxy that forwards requests to different targets
type Proxy struct {
	store       *storage.RuleStore
	stats       *stats.Stats
	broadcaster *logstream.Broadcaster
	defaultTarget string
	maintenanceHandler http.HandlerFunc
}

// NewProxy creates a new Proxy
func NewProxy(store *storage.RuleStore, stats *stats.Stats, broadcaster *logstream.Broadcaster, defaultTarget string, maintenanceHandler http.HandlerFunc) *Proxy {
	return &Proxy{
		store:       store,
		stats:       stats,
		broadcaster: broadcaster,
		defaultTarget: defaultTarget,
		maintenanceHandler: maintenanceHandler,
	}
}

// ServeHTTP handles incoming requests
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
	p.broadcaster.Broadcast([]byte(host + r.URL.Path))

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
		return
	}

	// Create the reverse proxy
	proxy := httputil.NewSingleHostReverseProxy(url)
	proxy.ServeHTTP(w, r)
}
