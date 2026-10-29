package proxy

import (
	"context"
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

func TestMaintenanceKeepsPanelReachable(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	defer upstream.Close()
	s := storage.NewRuleStore(storage.NewStorage(filepath.Join(t.TempDir(), "rules.json")))
	s.SetPanelAddress(strings.TrimPrefix(upstream.URL, "http://"))
	s.Add("panel.test", upstream.URL)
	s.Add("site.test", "localhost:8099")
	p := &Proxy{store: s, stats: stats.New(), maintenanceTmpl: template.Must(template.New("maintenance").Parse("Maintenance"))}
	s.SetMaintenanceMode(true)
	for host, want := range map[string]int{"panel.test": 204, "site.test": 503} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "http://"+host+"/", nil)
		p.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatal(host, w.Code, w.Body.String())
		}
	}
	s.SetMaintenanceMode(false)
	s.SetRuleMaintenance("site.test", true)
	w := httptest.NewRecorder()
	p.ServeHTTP(w, httptest.NewRequest("GET", "http://site.test/", nil))
	if w.Code != 503 {
		t.Fatal("per-route maintenance lost", w.Code)
	}
}

func TestAutomaticMaintenanceAndRecovery(t *testing.T) {
	var code atomic.Int32
	code.Store(200)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(int(code.Load()))
		w.Write([]byte("service"))
	}))
	defer upstream.Close()
	s := storage.NewRuleStore(storage.NewStorage(filepath.Join(t.TempDir(), "rules.json")))
	s.Add("app.test", upstream.URL)
	p := &Proxy{store: s, stats: stats.New(), maintenanceTmpl: template.Must(template.New("maintenance").Parse("Automatic={{.Automatic}}"))}
	request := func(method, path, accept string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "http://app.test"+path, nil)
		r.Header.Set("Accept", accept)
		w := httptest.NewRecorder()
		p.ServeHTTP(w, r)
		return w
	}
	s.CheckServices(context.Background())
	if w := request("GET", "/", "text/html"); w.Code != 200 {
		t.Fatal(w.Code)
	}
	code.Store(503)
	s.CheckServices(context.Background())
	s.CheckServices(context.Background())
	w := request("GET", "/", "text/html")
	if w.Code != 503 || w.Header().Get("Retry-After") == "" || !strings.Contains(w.Body.String(), "Automatic=true") {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := request("POST", "/api", "application/json"); w.Code != 503 || strings.Contains(w.Body.String(), "Automatic=") {
		t.Fatal("HTML served to API", w.Body.String())
	}
	if w := request("GET", "/_router/maintenance/status", "application/json"); !strings.Contains(w.Body.String(), `"available":false`) {
		t.Fatal(w.Body.String())
	}
	code.Store(200)
	s.CheckServices(context.Background())
	s.CheckServices(context.Background())
	if w := request("GET", "/", "text/html"); w.Code != 200 {
		t.Fatal("not recovered", w.Code)
	}
	if w := request("GET", "/_router/maintenance/status", "application/json"); !strings.Contains(w.Body.String(), `"available":true`) {
		t.Fatal(w.Body.String())
	}
	upstream.Close()
	if w := request("GET", "/deep/path", "text/html"); w.Code != 503 || !strings.Contains(w.Body.String(), "Automatic=true") {
		t.Fatal("connection error not handled", w.Code)
	}
	rule, _ := s.GetRule("app.test")
	if !rule.ServiceDown || rule.Maintenance {
		t.Fatal("incorrect outage state")
	}
}

func TestMaintenanceAssetAllowlist(t *testing.T) {
	for _, path := range []string{"/_router/maintenance/assets/panel.js", "/_router/maintenance/assets/../../rules.json", "/_router/maintenance/assets/vendor/2048/game_manager.js"} {
		w := httptest.NewRecorder()
		if !ServeMaintenanceAsset(w, httptest.NewRequest("GET", path, nil)) || w.Code != 404 {
			t.Fatal("unlisted asset exposed", path, w.Code)
		}
	}
	w := httptest.NewRecorder()
	ServeMaintenanceAsset(w, httptest.NewRequest("POST", "/_router/maintenance/assets/game.js", nil))
	if w.Code != 405 {
		t.Fatal(w.Code)
	}
}
