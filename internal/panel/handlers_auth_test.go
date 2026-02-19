package panel

import (
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
