package panel

import (
	"github.com/gorilla/websocket"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"router/internal/logstream"
	"router/internal/notify"
	"router/internal/storage"
	"strings"
	"testing"
	"time"
)

func TestWebsocketOriginAndRevokedSession(t *testing.T) {
	r := httptest.NewRequest("GET", "https://panel.test/ws/logs", nil)
	for origin, allowed := range map[string]bool{"https://panel.test": true, "http://panel.test": false, "https://other.test": false, "null": false} {
		r.Header.Set("Origin", origin)
		if upgrader.CheckOrigin(r) != allowed {
			t.Fatalf("unexpected origin decision: %s", origin)
		}
	}
	h := secureTestHandler(t)
	h.broadcaster = logstream.New()
	s := httptest.NewServer(http.HandlerFunc(h.Logs))
	defer s.Close()
	token := h.createSession()
	headers := http.Header{"Cookie": []string{"router_session=" + token}, "Origin": []string{s.URL}}
	c, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(s.URL, "http")+"/ws/logs", headers)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	h.broadcaster.Write([]byte("before revocation"))
	_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, _, err = c.ReadMessage(); err != nil {
		t.Fatal(err)
	}
	h.invalidateSession(token)
	h.broadcaster.Write([]byte("must not be sent"))
	if _, _, err = c.ReadMessage(); err == nil {
		t.Fatal("revoked session received logs")
	}
}

func secureTestHandler(t *testing.T) *Handler {
	t.Helper()
	return &Handler{auth: newAuthState(), adminStore: storage.NewAdminStore(filepath.Join(t.TempDir(), "admin.json"), "admin", "test-password-123")}
}

func TestAllAdminEndpointsRequireSession(t *testing.T) {
	h := secureTestHandler(t)
	endpoints := map[string]http.HandlerFunc{"/": h.Index, "/stats": h.Stats, "/stats/data": h.StatsData, "/backups": h.Backups, "/backups/data": h.BackupsData, "/backups/config": h.SaveBackupsConfig, "/backups/run": h.RunBackupNow, "/backups/delete": h.DeleteBackupJob, "/add": h.AddRule, "/remove": h.RemoveRule, "/rule/maintenance": h.RuleMaintenance, "/routes/status": h.RouteStatuses, "/maintenance-preview": h.MaintenancePreview, "/notifications/data": h.NotificationsData, "/notifications/config": h.SaveNotificationsConfig, "/settings/data": h.SettingsData, "/settings/config": h.SaveSettingsConfig, "/account/data": h.AccountData, "/account/config": h.SaveAccountConfig, "/logout": h.Logout, "/ws/logs": h.Logs}
	for path, handler := range endpoints {
		for _, method := range []string{"GET", "POST", "PUT", "DELETE"} {
			for _, token := range []string{"", "forged-session"} {
				r := httptest.NewRequest(method, "https://panel.test"+path, nil)
				r.Header.Set("Accept", "application/json")
				r.Header.Set("Origin", "https://panel.test")
				r.SetBasicAuth("admin", "test-password-123")
				if token != "" {
					r.AddCookie(&http.Cookie{Name: "router_session", Value: token})
				}
				w := httptest.NewRecorder()
				handler(w, r)
				if w.Code != 401 {
					t.Fatalf("%s %s allowed without session: %d", method, path, w.Code)
				}
			}
		}
	}
	h.adminStore = nil
	w := httptest.NewRecorder()
	h.AccountData(w, httptest.NewRequest("GET", "/account/data", nil))
	if w.Code != 503 {
		t.Fatal("missing admin store fails open")
	}
}

func TestMutationOriginAndLimits(t *testing.T) {
	h := secureTestHandler(t)
	token := h.createSession()
	called := false
	handler := h.basicAuth(func(w http.ResponseWriter, r *http.Request) { called = true; w.WriteHeader(204) })
	for _, origin := range []string{"https://evil.test", "https://sibling.panel.test", "http://panel.test", "null", ""} {
		r := httptest.NewRequest("POST", "https://panel.test/account/config", nil)
		r.AddCookie(&http.Cookie{Name: "router_session", Value: token})
		r.Header.Set("Origin", origin)
		w := httptest.NewRecorder()
		handler(w, r)
		if w.Code != 403 || called {
			t.Fatal("cross-origin mutation accepted", origin, w.Code)
		}
	}
	for _, method := range []string{"POST", "PUT", "DELETE"} {
		r := httptest.NewRequest(method, "https://panel.test/account/config", nil)
		r.AddCookie(&http.Cookie{Name: "router_session", Value: token})
		r.Header.Set("Origin", "https://panel.test")
		w := httptest.NewRecorder()
		handler(w, r)
		if w.Code != 204 {
			t.Fatal(w.Code)
		}
	}
	r := httptest.NewRequest("POST", "https://panel.test/", strings.NewReader("a="+strings.Repeat("x", 2<<20)))
	r.Header.Set("Origin", "https://panel.test")
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.AddCookie(&http.Cookie{Name: "router_session", Value: token})
	w := httptest.NewRecorder()
	called = false
	handler(w, r)
	if called || w.Code != 400 {
		t.Fatal("oversized mutation accepted")
	}
}

func TestLoginSpoofingCookiesAndSessionRevocation(t *testing.T) {
	h := secureTestHandler(t)
	login := func(ip, user, password, xff string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "https://panel.test/login", strings.NewReader(url.Values{"username": {user}, "password": {password}}.Encode()))
		r.RemoteAddr = ip + ":1234"
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Set("Origin", "https://panel.test")
		r.Header.Set("X-Forwarded-For", xff)
		w := httptest.NewRecorder()
		h.Login(w, r)
		return w
	}
	for i := 0; i < 5; i++ {
		if w := login("192.0.2.5", "admin", "wrong", time.Now().String()); w.Code != 401 {
			t.Fatal(w.Code)
		}
	}
	if w := login("192.0.2.5", "admin", "test-password-123", "8.8.8.8"); w.Code != 429 {
		t.Fatal("XFF bypassed rate limit", w.Code)
	}
	w := login("192.0.2.6", "admin", "test-password-123", "")
	if w.Code != 204 {
		t.Fatal(w.Code)
	}
	cookie := w.Result().Cookies()[0]
	if !cookie.Secure || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode {
		t.Fatal("unsafe session cookie", cookie)
	}
	other := h.createSession()
	r := httptest.NewRequest("POST", "https://panel.test/account/config", strings.NewReader("username=admin&password=another-strong-password"))
	r.Header.Set("Origin", "https://panel.test")
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.AddCookie(cookie)
	w = httptest.NewRecorder()
	h.SaveAccountConfig(w, r)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	for _, old := range []string{cookie.Value, other} {
		r := httptest.NewRequest("GET", "/", nil)
		r.AddCookie(&http.Cookie{Name: "router_session", Value: old})
		if h.isAuthenticated(r) {
			t.Fatal("old session survived password change")
		}
	}
	r = httptest.NewRequest("GET", "/", nil)
	r.AddCookie(w.Result().Cookies()[0])
	if !h.isAuthenticated(r) {
		t.Fatal("replacement session invalid")
	}
}

func TestStaticAssetsAndSecurityHeaders(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "ok.js"), []byte("/* asset */"), 0600)
	os.WriteFile(filepath.Join(dir, "account.html"), []byte("private"), 0600)
	handler := SecurityHeaders(StaticAssets(dir))
	for _, path := range []string{"/static/", "/static/account.html", "/static/../admin.json", "/static/.env", "/static/x/../../ok.js"} {
		r := httptest.NewRequest("GET", "https://panel.test"+path, nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != 404 {
			t.Fatal(path, w.Code)
		}
	}
	r := httptest.NewRequest("GET", "https://panel.test/static/ok.js", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != 200 || !strings.Contains(w.Header().Get("Content-Security-Policy"), "script-src 'self';") || w.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatal(w.Code, w.Header())
	}
	r = httptest.NewRequest("GET", "http://panel.test/login", nil)
	r.Header.Set("X-Forwarded-Proto", "https")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != 426 {
		t.Fatal("untrusted XFP enabled insecure remote access")
	}
}

func TestWebhookRejectsMissingSecret(t *testing.T) {
	s := storage.NewNotificationStore(filepath.Join(t.TempDir(), "notifications.json"))
	h := &Handler{notifyStore: s, notifier: notify.NewTelegramNotifier(s)}
	for _, secret := range []string{"", "configured-secret"} {
		cfg := s.Get()
		cfg.WebhookSecret = secret
		s.Update(cfg)
		r := httptest.NewRequest("POST", "/telegram/webhook", strings.NewReader(`{"message":{"chat":{"id":123}}}`))
		w := httptest.NewRecorder()
		h.TelegramWebhook(w, r)
		if w.Code != 403 {
			t.Fatal("webhook accepted forged update", w.Code)
		}
		if len(s.Get().KnownChatIDs) != 0 {
			t.Fatal("forged chat stored")
		}
	}
}
