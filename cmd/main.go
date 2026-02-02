
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
	storageSvc := storage.NewStorage("rules.json")
	store := storage.NewRuleStore(storageSvc)
	statistics := stats.New()
	broadcaster := logstream.New()

	// Set log output to the broadcaster
	log.SetOutput(broadcaster)

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
	proxyHandler := proxy.NewProxy(store, statistics)
	panelHandler := panel.NewHandler(store, cfg.Username, cfg.Password, statistics, broadcaster)

	// Set up routes
	mux := http.NewServeMux()

	// Panel routes
	panelSubMux := http.NewServeMux()
	panelSubMux.HandleFunc("/", panelHandler.Index)
	panelSubMux.HandleFunc("/stats", panelHandler.Stats)
	panelSubMux.HandleFunc("/add", panelHandler.AddRule)
	panelSubMux.HandleFunc("/remove", panelHandler.RemoveRule)
	panelSubMux.HandleFunc("/stats/data", panelHandler.StatsData)
	panelSubMux.HandleFunc("/ws", panelHandler.Logs)

	// Serve static files for the panel
	fs := http.FileServer(http.Dir("internal/panel/static"))
	panelSubMux.Handle("/static/", http.StripPrefix("/static/", fs))

	// Add the panel sub-router to the main mux under the "/panel" path
	mux.Handle("/panel/", http.StripPrefix("/panel", panelSubMux))

	// The main proxy handler should handle all other requests
	mux.Handle("/", proxyHandler)

	// Start server
	log.Printf("Starting server on :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
