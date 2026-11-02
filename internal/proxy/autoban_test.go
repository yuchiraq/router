package proxy

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"router/internal/stats"
	"router/internal/storage"
	"sync/atomic"
	"testing"
)

func TestAutoBanBlocksTriggeringRequestAndIgnoresSpoofedIP(t *testing.T) {
	var hits atomic.Int32
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1); w.WriteHeader(204) }))
	defer backend.Close()
	rules := storage.NewRuleStore(storage.NewStorage(filepath.Join(t.TempDir(), "rules.json")))
	rules.Add("site.test", backend.URL)
	ips := storage.NewIPReputationStore(filepath.Join(t.TempDir(), "ips.json"))
	p := &Proxy{store: rules, stats: stats.New(), reputation: ips}
	for i := 0; i < 4; i++ {
		r := httptest.NewRequest("GET", "http://site.test/.env", nil)
		r.RemoteAddr = "198.51.100.3:4000"
		r.Header.Set("X-Forwarded-For", "198.51.100.99")
		w := httptest.NewRecorder()
		p.ServeHTTP(w, r)
		if i >= 2 && w.Code != 403 {
			t.Fatal("banned request reached upstream", i, w.Code)
		}
	}
	if hits.Load() != 2 || ips.IsBanned("198.51.100.99") || !ips.IsBanned("198.51.100.3") {
		t.Fatal("ban/identity mismatch")
	}
	for _, path := range []string{"/articles/wp-admin-guide", "/photos/adminer-photo.jpg", "/foo.gitignore", "/environment"} {
		if suspiciousPath(path) {
			t.Fatal("false positive", path)
		}
	}
}
