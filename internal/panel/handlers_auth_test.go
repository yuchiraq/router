package panel

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestBruteforceBlockAfterFiveFailures(t *testing.T) {
	h := &Handler{loginFails: map[string]loginAttempt{}}
	ip := "1.2.3.4"
	for i := 0; i < 5; i++ {
		h.registerLoginFailure(ip)
	}
	if _, blocked := h.checkLoginBlocked(ip); !blocked {
		t.Fatalf("ip should be blocked after 5 failed attempts")
	}
}

func TestClientIPFromRequest(t *testing.T) {
	r := httptest.NewRequest("POST", "/login", nil)
	r.Header.Set("X-Forwarded-For", "9.8.7.6, 1.1.1.1")
	if got := clientIPFromRequest(r); got != "9.8.7.6" {
		t.Fatalf("unexpected client ip: %s", got)
	}
}

func TestBlockExpires(t *testing.T) {
	h := &Handler{loginFails: map[string]loginAttempt{"1.1.1.1": {BlockedTill: time.Now().Add(-time.Minute)}}}
	if _, blocked := h.checkLoginBlocked("1.1.1.1"); blocked {
		t.Fatalf("block should expire")
	}
}

func TestSessionLifecycle(t *testing.T) {
	h := &Handler{sessions: map[string]time.Time{}, loginFails: map[string]loginAttempt{}}
	token := h.createSession()
	if token == "" {
		t.Fatalf("expected non-empty token")
	}
	r := httptest.NewRequest("GET", "/", nil)
	r.AddCookie(&http.Cookie{Name: "router_session", Value: token})
	if !h.isAuthenticated(r) {
		t.Fatalf("session should be authenticated")
	}
	h.invalidateSession(token)
	if h.isAuthenticated(r) {
		t.Fatalf("session should be invalidated")
	}
}
