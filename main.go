package main

import (
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
			time.Sleep(5 * time.Second)
		}
	}()

	// Get admin credentials from environment variables
	adminUser := os.Getenv("ADMIN_USER")
	adminPass := os.Getenv("ADMIN_PASS")

	// Initialize the control panel handler
	panelHandler := panel.NewHandler(store, adminUser, adminPass, stats, broadcaster)

	// Initialize the proxy
	proxyHandler := proxy.NewProxy(store, stats)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			panelHandler.Index(w, r)
		case "/stats":
			panelHandler.Stats(w, r)
		case "/stats/data":
			panelHandler.StatsData(w, r)
		case "/ws/logs":
			panelHandler.Logs(w, r)
		case "/add":
			panelHandler.AddRule(w, r)
		case "/remove":
			panelHandler.RemoveRule(w, r)
		case "/styles.css":
			panelHandler.ServeStyles(w, r)
		default:
			proxyHandler.ServeHTTP(w, r)
		}
	})

	// Start the server
	log.Println("Starting server on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
