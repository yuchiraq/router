package proxy

import (
	"context"
	"crypto/tls"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"router/internal/storage"
	"strings"

	"golang.org/x/crypto/acme/autocert"
)

func StartProxy(rules *storage.RuleStore) {
	certManager := autocert.Manager{
		Prompt: autocert.AcceptTOS,
		Cache:  autocert.DirCache("certs"),
		HostPolicy: func(ctx context.Context, host string) error {
			if !rules.Exists(host) {
				return context.DeadlineExceeded // Используем DeadlineExceeded, как рекомендуется в документации
			}
			return nil
		},
	}

	// HTTP -> HTTPS redirect
	go func() {
		log.Println("HTTP to HTTPS redirect server running on :80")
		if err := http.ListenAndServe(":80", certManager.HTTPHandler(nil)); err != nil {
			log.Printf("HTTP redirect server error: %v", err)
		}
	}()

	server := &http.Server{
		Addr: ":443",
		TLSConfig: &tls.Config{
			GetCertificate: certManager.GetCertificate,
			MinVersion:     tls.VersionTLS12,
		},
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			target, ok := rules.GetTarget(r.Host)
			if !ok {
				http.Error(w, "Domain not found", http.StatusNotFound)
				return
			}

			if !strings.HasPrefix(target, "http") {
				target = "http://" + target
			}

			targetURL, err := url.Parse(target)
			if err != nil {
				log.Printf("Error parsing target URL for host %s: %v", r.Host, err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}

			proxy := httputil.NewSingleHostReverseProxy(targetURL)
			proxy.ServeHTTP(w, r)
		}),
	}

	log.Println("HTTPS proxy server running on :443 with autocert")
	if err := server.ListenAndServeTLS("", ""); err != nil {
		log.Fatalf("HTTPS proxy server error: %v", err)
	}
}
