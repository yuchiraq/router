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
	log.Println("Starting application...")

	log.Println("Loading configuration...")
	c := config.New()
	log.Println("Configuration loaded.")

	log.Println("Initializing storage...")
	storageDriver := storage.NewStorage("rules.json")
	rs := storage.NewRuleStore(storageDriver)
	log.Println("Storage initialized.")

	log.Println("Creating proxy handler...")
	proxyHandler := proxy.NewProxy(rs)
	log.Println("Proxy handler created.")

	log.Println("Creating panel handler...")
	panelHandler := panel.NewHandler(rs, c.Username, c.Password)
	log.Println("Panel handler created.")

	// Separate mux for panel and proxy
	proxyMux := http.NewServeMux()
	proxyMux.HandleFunc("/", proxyHandler.ServeHTTP)

	panelMux := http.NewServeMux()
	panelMux.HandleFunc("/", panelHandler.Index)
	panelMux.HandleFunc("/add", panelHandler.AddRule)
	panelMux.HandleFunc("/remove", panelHandler.RemoveRule)
	panelMux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("internal/panel/templates"))))

	// Start servers
	go func() {
		log.Println("Proxy server starting on 0.0.0.0:8080")
		if err := http.ListenAndServe("0.0.0.0:8080", proxyMux); err != nil {
			log.Fatal("Proxy server failed to start:", err)
		}
	}()

	go func() {
		log.Println("Panel server starting on 0.0.0.0:8182")
		if err := http.ListenAndServe("0.0.0.0:8182", panelMux); err != nil {
			log.Fatal("Panel server failed to start:", err)
		}
	}()

	log.Println("Application started successfully. Servers are running.")
	// Keep the main goroutine alive
	select {}
}
