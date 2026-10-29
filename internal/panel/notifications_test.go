package panel

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"router/internal/storage"
)

func TestNotificationDiskSettings(t *testing.T) {
	dir := t.TempDir()
	s := storage.NewNotificationStore(filepath.Join(dir, "notifications.json"))
	s.RememberKnownChatID(123)
	h := &Handler{notifyStore: s, auth: newAuthState(), adminStore: storage.NewAdminStore(filepath.Join(dir, "admin.json"), "admin", "password")}
	token := h.createSession()
	post := func(values url.Values, authenticated bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "http://localhost/notifications/config", strings.NewReader(values.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Set("Accept", "application/json")
		if authenticated {
			r.AddCookie(&http.Cookie{Name: "router_session", Value: token})
		}
		w := httptest.NewRecorder()
		h.SaveNotificationsConfig(w, r)
		return w
	}
	values := url.Values{"diskWarning": {"80"}, "diskCritical": {"92"}, "diskReminderHours": {"12"}, "diskPaths": {dir}, "event_disk_usage": {"on"}}
	if w := post(values, false); w.Code != 401 {
		t.Fatal("unauthenticated change", w.Code)
	}
	if w := post(values, true); w.Code != 200 {
		t.Fatal("save", w.Code, w.Body.String())
	}
	cfg := storage.NewNotificationStore(filepath.Join(dir, "notifications.json")).Get()
	if cfg.DiskWarning != 80 || cfg.DiskCritical != 92 || cfg.DiskReminderHours != 12 || len(cfg.DiskPaths) != 1 || len(cfg.KnownChatIDs) != 1 || !cfg.Events["disk_usage"] {
		t.Fatal("incorrect saved config")
	}
	for _, pair := range [][2]string{{"diskWarning", "100"}, {"diskCritical", "79"}, {"diskReminderHours", "0"}, {"diskPaths", "relative/path"}, {"diskWarning", "not-a-number"}, {"chatIds", "abc"}, {"chatIds", "0"}, {"enabled", "on"}, {"quietStart", "24"}} {
		previous := values.Get(pair[0])
		values.Set(pair[0], pair[1])
		if w := post(values, true); w.Code != 400 {
			t.Fatal("invalid setting accepted", pair, w.Code)
		}
		if s.Get().DiskWarning != 80 {
			t.Fatal("invalid update applied")
		}
		values.Set(pair[0], previous)
	}
	values.Set("diskPaths", "")
	if w := post(values, true); w.Code != 200 || len(s.Get().DiskPaths) != 0 {
		t.Fatal("cannot clear paths")
	}
}
