package proxy

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"router/internal/storage"

	"golang.org/x/crypto/acme/autocert"
)

func StartProxy(store *storage.RuleStore) {
	// HTTP to HTTPS redirection
	go func() {
		if err := http.ListenAndServe(":80", http.HandlerFunc(redirectToHTTPS)); err != nil {
			log.Fatalf("ListenAndServe error: %v", err)
		}
	}()

	// HTTPS proxy
	m := &autocert.Manager{
		Prompt: autocert.AcceptTOS,
		HostPolicy: func(host string) error {
			// Allow any host that has a rule
			_, ok := store.Get(host)
			if !ok {
				return log.Output(2, "host not allowed: "+host)
			}
			return nil
		},
		Cache: autocert.DirCache("certs"),
	}

	s := &http.Server{
		Addr:      ":443",
		TLSConfig: m.TLSConfig(),
		Handler:   http.HandlerFunc(proxyHandler(store)),
	}

	log.Println("HTTPS proxy server running on :443 with autocert")
	log.Fatal(s.ListenAndServeTLS("", ""))
}

func redirectToHTTPS(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "https://"+r.Host+r.URL.String(), http.StatusMovedPermanently)
}

func proxyHandler(store *storage.RuleStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		target, ok := store.Get(r.Host)
		if !ok {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}

		// Create a new URL for the target
		url, err := url.Parse("http://" + target)
		if err != nil {
			log.Printf("Error parsing target URL: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Create a reverse proxy
		proxy := httputil.NewSingleHostReverseProxy(url)

		// Update the headers to allow for SSL redirection
		r.URL.Host = url.Host
		r.URL.Scheme = url.Scheme
		r.Header.Set("X-Forwarded-Host", r.Header.Get("Host"))
		r.Host = url.Host

		// Serve the request
		proxy.ServeHTTP(w, r)
	}
}
