package main

import (
	"crypto/tls"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"router/internal/logstream"
	"router/internal/panel"
	"router/internal/proxy"
	"router/internal/stats"
	"router/internal/storage"

	"golang.org/x/crypto/acme/autocert"
)

func main() {
	// Initialize storage
	fileStorage := storage.NewStorage("rules.json")
	store := storage.NewRuleStore(fileStorage)

	// Initialize stats
	stats := stats.New()

	// Initialize log broadcaster
	broadcaster := logstream.New()
	log.SetOutput(io.MultiWriter(os.Stderr, broadcaster))

	// Start memory recording
		go func() {
			for {
				stats.RecordMemory()
				stats.RecordCPU()
				stats.RecordDisks()
				time.Sleep(5 * time.Second)
			}
		}()

	// Get admin credentials from environment variables
	adminUser := os.Getenv("ADMIN_USER")
	adminPass := os.Getenv("ADMIN_PASS")

	// --- Admin Panel (Port 8162) ---
	go func() {
		panelMux := http.NewServeMux()
		panelHandler := panel.NewHandler(store, adminUser, adminPass, stats, broadcaster)
		
		// Serve static files
		staticFS := http.FileServer(http.Dir("internal/panel/static"))
    	panelMux.Handle("/static/", http.StripPrefix("/static/", staticFS))

		panelMux.HandleFunc("/", panelHandler.Index)
		panelMux.HandleFunc("/stats", panelHandler.Stats)
		panelMux.HandleFunc("/stats/data", panelHandler.StatsData)
		panelMux.HandleFunc("/ws/logs", panelHandler.Logs)
		panelMux.HandleFunc("/add", panelHandler.AddRule)
		panelMux.HandleFunc("/remove", panelHandler.RemoveRule)
		log.Println("Starting admin panel on :8162")
		if err := http.ListenAndServe(":8162", panelMux); err != nil {
			log.Fatalf("Failed to start admin panel: %v", err)
		}
	}()

	// --- Proxy (Ports 80 & 443) ---
	proxyHandler := proxy.NewProxy(store, stats)
	proxyMux := http.NewServeMux()
	staticFS := http.FileServer(http.Dir("internal/panel/static"))
	proxyMux.Handle("/static/", http.StripPrefix("/static/", staticFS))
	proxyMux.Handle("/", proxyHandler)

	// Autocert for automatic HTTPS certificates
	certManager := &autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		HostPolicy: store.HostPolicy, // Use the rule store to validate hosts
		Cache:      autocert.DirCache("certs"),
	}

	// HTTPS server
	server := &http.Server{
		Addr:    ":443",
		Handler: proxyMux,
		TLSConfig: &tls.Config{
			GetCertificate: certManager.GetCertificate,
		},
	}

	// HTTP server (for ACME challenge and redirecting to HTTPS)
	go func() {
		log.Println("Starting HTTP server on :80")
		if err := http.ListenAndServe(":80", certManager.HTTPHandler(nil)); err != nil {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	// Start HTTPS server
	log.Println("Starting HTTPS server on :443")
	if err := server.ListenAndServeTLS("", ""); err != nil {
		log.Fatalf("HTTPS server error: %v", err)
	}
}
