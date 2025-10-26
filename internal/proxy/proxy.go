package proxy

import (
	"expvar"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"router/internal/storage"
)

// Proxy is a reverse proxy that uses a RuleStore to determine the target.
type Proxy struct {
	store *storage.RuleStore
}

// NewProxy creates a new Proxy.
func NewProxy(store *storage.RuleStore) *Proxy {
	return &Proxy{store: store}
}

// ServeHTTP handles the proxying of requests.
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	target, ok := p.store.Get(r.Host)
	if !ok {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	// Increment request counter for the domain
	requests := expvar.Get("requests_" + r.Host)
	if requests == nil {
		requests = expvar.NewInt("requests_" + r.Host)
	}
	requests.(*expvar.Int).Add(1)

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
