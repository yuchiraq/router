
package main

import (
	"log"
	"net/http"
	"time"

	"router/internal/config"
	"router/internal/logstream"
	"router/internal/panel"
	"router/internal/proxy"
	"router/internal/stats"
	"router/internal/storage"
)

func main() {
	// Load configuration
	cfg := config.New()

	// Initialize storage, stats, and broadcaster
	store := storage.NewRuleStore(nil)
	statistics := stats.New()
	broadcaster := logstream.New()

	// Start background tasks
	go func() {
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			statistics.RecordMemory()
			statistics.RecordCPU()
		}
	}()

	// Set up handlers
	panelHandler := panel.NewHandler(store, cfg.Username, cfg.Password, statistics, broadcaster)

	// Set up the proxy
	proxyHandler := proxy.NewProxy(store, statistics, broadcaster, "", panelHandler.Maintenance)

	// Set up routes for the admin panel
	adminMux := http.NewServeMux()
	adminMux.HandleFunc("/", panelHandler.Index)
	adminMux.HandleFunc("/stats", panelHandler.Stats)
	adminMux.HandleFunc("/add", panelHandler.AddRule)
	adminMux.HandleFunc("/remove", panelHandler.RemoveRule)
	adminMux.HandleFunc("/stats/data", panelHandler.StatsData)
	adminMux.HandleFunc("/ws", panelHandler.Logs)
	adminMux.HandleFunc("/toggle_maintenance", panelHandler.ToggleMaintenance)

	// Serve static files for the admin panel
	fs := http.FileServer(http.Dir("internal/panel/static"))
	adminMux.Handle("/static/", http.StripPrefix("/static/", fs))

	// Create a main router
	mainMux := http.NewServeMux()
	mainMux.Handle("/admin/", http.StripPrefix("/admin", adminMux)) // All admin routes are under /admin/
	mainMux.HandleFunc("/", proxyHandler.ServeHTTP) // The proxy is the default handler

	// Start server
	log.Printf("Starting server on :8080")
	if err := http.ListenAndServe(":8080", mainMux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
