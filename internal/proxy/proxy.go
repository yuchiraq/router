
package proxy

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"router/internal/stats"
	"router/internal/storage"
)

// Proxy is a reverse proxy that uses a RuleStore to determine the target.
type Proxy struct {
	store *storage.RuleStore
	stats *stats.Stats
}

// NewProxy creates a new Proxy.
func NewProxy(store *storage.RuleStore, stats *stats.Stats) *Proxy {
	return &Proxy{store: store, stats: stats}
}

// ServeHTTP handles the proxying of requests.
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	target, ok := p.store.Get(r.Host)
	if !ok {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	// Add request to stats with the specific host
	p.stats.AddRequest(r.Host)

	targetURL, err := url.Parse("http://" + target)
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
