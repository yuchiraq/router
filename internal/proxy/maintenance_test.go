package proxy

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"router/internal/stats"
	"router/internal/storage"
	"strings"
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
