package proxy

import (
	"crypto/tls"
	"log"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/acme/autocert"
)

type Route struct {
	Domain string
	Target string
}

type ProxyServer struct {
	Routes []Route
}

func NewProxyServer(routes []Route) *ProxyServer {
	return &ProxyServer{Routes: routes}
}

func (ps *ProxyServer) Start() {
	// Собираем список доменов
	var domains []string
	for _, r := range ps.Routes {
		if !contains(domains, r.Domain) {
			domains = append(domains, r.Domain)
		}
	}

	// Настройка autocert
	certManager := autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		Cache:      autocert.DirCache("certs"), // Папка для сертификатов
		HostPolicy: autocert.HostWhitelist(domains...),
	}

	// HTTP → HTTPS редирект
	go func() {
		log.Println("HTTP to HTTPS redirect server running on :80")
		http.ListenAndServe(":80", certManager.HTTPHandler(nil))
	}()

	// HTTPS сервер с автосертификатами
	server := &http.Server{
		Addr: ":443",
		TLSConfig: &tls.Config{
			GetCertificate: certManager.GetCertificate,
			MinVersion:     tls.VersionTLS12,
		},
		Handler: ps,
	}

	log.Println("HTTPS proxy server running on :443 with autocert")
	log.Fatal(server.ListenAndServeTLS("", "")) // Пустые пути → autocert
}

func (ps *ProxyServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	for _, route := range ps.Routes {
		if strings.EqualFold(r.Host, route.Domain) {
			http.Redirect(w, r, route.Target, http.StatusTemporaryRedirect)
			return
		}
	}
	http.Error(w, "Domain not found", http.StatusNotFound)
}

func contains(slice []string, val string) bool {
	for _, item := range slice {
		if item == val {
			return true
		}
	}
	return false
}
