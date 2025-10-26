package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"router/internal/panel"
	"router/internal/proxy"
	"router/internal/stats"
	"router/internal/storage"
)

func main() {
	// Initialize storage
	store := storage.NewRuleStore()

	// Initialize stats
	stats := stats.New()

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
	panelHandler := panel.NewHandler(store, adminUser, adminPass, stats)

	// Initialize the proxy
	proxyHandler := proxy.NewProxy(store, stats)

	// Register panel handlers
	http.HandleFunc("/", panelHandler.Index)
	http.HandleFunc("/stats", panelHandler.Stats)
	http.HandleFunc("/stats/data", panelHandler.StatsData)
	http.HandleFunc("/add", panelHandler.AddRule)
	http.HandleFunc("/remove", panelHandler.RemoveRule)
	http.HandleFunc("/styles.css", panelHandler.ServeStyles)

	// Register the proxy handler for the root path
	http.HandleFunc("/*", func(w http.ResponseWriter, r *http.Request) {
		// Check if the request is for the panel
		if r.URL.Path == "/" || r.URL.Path == "/stats" || r.URL.Path == "/stats/data" || r.URL.Path == "/add" || r.URL.Path == "/remove" || r.URL.Path == "/styles.css" {
			http.DefaultServeMux.ServeHTTP(w, r)
			return
		}
		proxyHandler.ServeHTTP(w, r)
	})

	// Start the server
	log.Println("Starting server on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
