package main

import (
	"log"
	"net/http"

	"router/internal/config"
	"router/internal/panel"
	"router/internal/proxy"
	"router/internal/storage"
)

func main() {
	c := config.New()

	storageDriver := storage.NewStorage("rules.json")
	rs := storage.NewRuleStore(storageDriver)

	proxyHandler := proxy.NewProxy(rs)

	panelHandler := panel.NewHandler(rs, c.Username, c.Password)

	// Separate mux for panel and proxy
	proxyMux := http.NewServeMux()
	proxyMux.HandleFunc("/", proxyHandler.ServeHTTP)

	panelMux := http.NewServeMux()
	panelMux.HandleFunc("/", panelHandler.Index)
	panelMux.HandleFunc("/add", panelHandler.AddRule)
	panelMux.HandleFunc("/remove", panelHandler.RemoveRule)
	panelMux.Handle("/styles.css", http.FileServer(http.Dir("internal/panel/templates")))

	// Start servers
	go func() {
		log.Println("Proxy server starting on :8080")
		if err := http.ListenAndServe(":8080", proxyMux); err != nil {
			log.Fatal("Proxy server failed to start:", err)
		}
	}()

	go func() {
		log.Println("Panel server starting on :8081")
		if err := http.ListenAndServe(":8081", panelMux); err != nil {
			log.Fatal("Panel server failed to start:", err)
		}
	}()

	// Keep the main goroutine alive
	select {}
}
