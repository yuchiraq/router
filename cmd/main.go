package main

import (
	"log"
	"net/http"

	"router/internal/config"
	"router/internal/panel"
	"router/internal/proxy"
	"router/internal/storage"

	"golang.org/x/crypto/acme/autocert"
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

	// Autocert manager
	m := &autocert.Manager{
		Cache:      autocert.DirCache("certs"),
		Prompt:     autocert.AcceptTOS,
		HostPolicy: rs.HostPolicy,
	}

	// Separate mux for panel and proxy
	proxyMux := http.NewServeMux()
	proxyMux.HandleFunc("/", proxyHandler.ServeHTTP)

	panelMux := http.NewServeMux()
	panelMux.HandleFunc("/", panelHandler.Index)
	panelMux.HandleFunc("/add", panelHandler.AddRule)
	panelMux.HandleFunc("/remove", panelHandler.RemoveRule)
	panelMux.HandleFunc("/styles.css", panelHandler.ServeStyles)

	// HTTP server for ACME challenges
	go func() {
		log.Println("Starting HTTP server for ACME challenges on :80")
		if err := http.ListenAndServe(":80", m.HTTPHandler(nil)); err != nil {
			log.Fatal("HTTP server for ACME challenges failed:", err)
		}
	}()

	// HTTPS server for proxy
	go func() {
		log.Println("Proxy server starting on 0.0.0.0:443")
		server := &http.Server{
			Addr:      ":443",
			Handler:   proxyMux,
			TLSConfig: m.TLSConfig(),
		}
		if err := server.ListenAndServeTLS("", ""); err != nil {
			log.Fatal("Proxy server failed to start:", err)
		}
	}()

	// Panel server
	go func() {
		log.Println("Panel server starting on 0.0.0.0:8162")
		if err := http.ListenAndServe("0.0.0.0:8162", panelMux); err != nil {
			log.Fatal("Panel server failed to start:", err)
		}
	}()

	log.Println("Application started successfully. Servers are running.")
	// Keep the main goroutine alive
	select {}
}
