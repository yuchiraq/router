package panel

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"router/internal/storage"
	"strings"
	"testing"
)

func TestMaintenanceHTTP(t *testing.T) {
	dir := t.TempDir()
	store := storage.NewRuleStore(storage.NewStorage(filepath.Join(dir, "rules.json")))
	store.Add("example.test", "localhost:8099")
	h := &Handler{store: store, auth: newAuthState(), adminStore: storage.NewAdminStore(filepath.Join(dir, "admin.json"), "admin", "password")}
	token := h.createSession()
	call := func(path string, enabled, authenticated bool) *httptest.ResponseRecorder {
		data := url.Values{"host": {"example.test"}}
		if enabled {
			data.Set("maintenance", "on")
		}
		r := httptest.NewRequest("POST", path, strings.NewReader(data.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Set("Accept", "application/json")
		if authenticated {
			r.AddCookie(&http.Cookie{Name: "router_session", Value: token})
		}
		w := httptest.NewRecorder()
		if path == "/" {
			h.Index(w, r)
		} else {
			h.RuleMaintenance(w, r)
		}
		return w
	}
	for _, path := range []string{"/", "/rule/maintenance"} {
		if w := call(path, true, false); w.Code != 401 {
			t.Fatal("unauthenticated mutation", w.Code)
		}
		for _, enabled := range []bool{true, false, true, false} {
			w := call(path, enabled, true)
			if w.Code != 200 || !strings.Contains(w.Header().Get("Content-Type"), "application/json") {
				t.Fatal(w.Code, w.Body.String())
			}
			rule, _ := store.GetRule("example.test")
			if path == "/" && store.IsMaintenanceMode() != enabled || path != "/" && rule.Maintenance != enabled {
				t.Fatal("wrong state")
			}
		}
	}
}
