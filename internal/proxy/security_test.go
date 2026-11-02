package proxy

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"router/internal/stats"
	"router/internal/storage"
	"strings"
	"sync/atomic"
	"testing"
)

func TestMaintenanceCannotForwardAlternatePathsOrMethods(t *testing.T) {
	var hits atomic.Int32
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1); w.WriteHeader(204) }))
	defer backend.Close()
	s := storage.NewRuleStore(storage.NewStorage(filepath.Join(t.TempDir(), "rules.json")))
	s.Add("site.test", backend.URL)
	s.SetRuleMaintenance("site.test", true)
	p := &Proxy{store: s, stats: stats.New(), maintenanceTmpl: template.Must(template.New("m").Parse("maintenance"))}
	for _, method := range []string{"GET", "HEAD", "POST", "PUT", "DELETE", "PROPFIND", "CONNECT"} {
		for _, path := range []string{"/", "/login", "/api", "//api", "/%2e%2e/api", "/_router/maintenance/assets/../../api", "/_router/maintenance/assets/panel.js", "/_router/maintenance/assets/game.js", "/_router/maintenance/status"} {
			r := httptest.NewRequest(method, "http://site.test"+path, nil)
			r.Header.Set("X-Original-URL", "/admin")
			r.Header.Set("X-Rewrite-URL", "/admin")
			r.Header.Set("Upgrade", "websocket")
			w := httptest.NewRecorder()
			p.ServeHTTP(w, r)
			if w.Code == 204 {
				t.Fatal("upstream reached", method, path)
			}
		}
	}
	if hits.Load() != 0 {
		t.Fatal("maintenance bypass", hits.Load())
	}
}

func TestProxyReplacesSpoofedHeadersAfterConnectionFiltering(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Forwarded-For") != "192.0.2.5" || r.Header.Get("X-Real-IP") != "192.0.2.5" || r.Header.Get("X-Forwarded-Proto") != "https" || r.Header.Get("X-Forwarded-Host") != "panel.test" || r.Host != "panel.test" || r.Header.Get("Forwarded") != "" || r.Header.Get("CF-Connecting-IP") != "" {
			t.Error("spoofed or stripped trusted headers", r.Header, r.Host)
		}
		w.WriteHeader(204)
	}))
	defer backend.Close()
	s := storage.NewRuleStore(storage.NewStorage(filepath.Join(t.TempDir(), "rules.json")))
	s.SetPanelAddress(strings.TrimPrefix(backend.URL, "http://"))
	s.Add("panel.test", backend.URL)
	p := &Proxy{store: s, stats: stats.New()}
	r := httptest.NewRequest("GET", "https://panel.test/account/data", nil)
	r.RemoteAddr = "192.0.2.5:1234"
	r.Header.Set("Connection", "X-Forwarded-For, X-Forwarded-Proto, X-Real-IP")
	for _, name := range []string{"Forwarded", "X-Forwarded-For", "X-Forwarded-Host", "X-Forwarded-Proto", "X-Real-IP", "CF-Connecting-IP"} {
		r.Header.Set(name, "attacker")
	}
	w := httptest.NewRecorder()
	p.ServeHTTP(w, r)
	if w.Code != 204 {
		t.Fatal(w.Code)
	}
}
