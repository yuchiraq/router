
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
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize storage, stats, and broadcaster
	store := storage.NewRuleStore(cfg.Rules)
	statistics := stats.New()
	broadcaster := logstream.NewBroadcaster()

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
	proxyHandler := proxy.NewProxy(store, statistics, broadcaster, cfg.Target, panelHandler.Maintenance)

	// Set up routes
	mux := http.NewServeMux()
	mux.HandleFunc("/", panelHandler.Index)
	mux.HandleFunc("/stats", panelHandler.Stats)
	mux.HandleFunc("/add", panelHandler.AddRule)
	mux.HandleFunc("/remove", panelHandler.RemoveRule)
	mux.HandleFunc("/stats/data", panelHandler.StatsData)
	mux.HandleFunc("/ws", panelHandler.Logs)
	mux.HandleFunc("/toggle_maintenance", panelHandler.ToggleMaintenance) // Add this route

	// Serve static files
	fs := http.FileServer(http.Dir("internal/panel/static"))
	mux.Handle("/static/", http.StripPrefix("/static/", fs))

	// Main proxy handler
	mux.HandleFunc("/proxy/", proxyHandler.ServeHTTP)

	// Start server
	log.Printf("Starting server on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, mux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
