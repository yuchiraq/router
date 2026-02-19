package panel

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"net/netip"
	"router/internal/clog"
	"strings"
	"time"
)

func (h *Handler) isAuthenticated(r *http.Request) bool {
	cookie, err := r.Cookie("router_session")
	if err != nil || cookie.Value == "" {
		return false
	}
	h.sessionsMu.RLock()
	expiresAt, ok := h.sessions[cookie.Value]
	h.sessionsMu.RUnlock()
	if !ok || time.Now().After(expiresAt) {
		return false
	}
	return true
}

func (h *Handler) createSession() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	token := hex.EncodeToString(b)
	h.sessionsMu.Lock()
	h.sessions[token] = time.Now().Add(24 * time.Hour)
	h.sessionsMu.Unlock()
	return token
}

func (h *Handler) invalidateSession(token string) {
	if token == "" {
		return
	}
	h.sessionsMu.Lock()
	delete(h.sessions, token)
	h.sessionsMu.Unlock()
}

func clientIPFromRequest(r *http.Request) string {
	xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
	if xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			ip := strings.TrimSpace(parts[0])
			if addr, err := netip.ParseAddr(ip); err == nil {
				return addr.String()
			}
		}
	}
	hostPort := strings.TrimSpace(r.RemoteAddr)
	if hostPort == "" {
		return "unknown"
	}
	if addr, err := netip.ParseAddrPort(hostPort); err == nil {
		return addr.Addr().String()
	}
	if addr, err := netip.ParseAddr(hostPort); err == nil {
		return addr.String()
	}
	return hostPort
}

func (h *Handler) checkLoginBlocked(ip string) (time.Duration, bool) {
	h.loginFailsMu.Lock()
	defer h.loginFailsMu.Unlock()
	entry, ok := h.loginFails[ip]
	if !ok || entry.BlockedTill.IsZero() {
		return 0, false
	}
	now := time.Now()
	if now.After(entry.BlockedTill) {
		delete(h.loginFails, ip)
		return 0, false
	}
	return time.Until(entry.BlockedTill), true
}

func (h *Handler) registerLoginFailure(ip string) {
	h.loginFailsMu.Lock()
	defer h.loginFailsMu.Unlock()
	now := time.Now()
	entry := h.loginFails[ip]
	if !entry.BlockedTill.IsZero() && now.After(entry.BlockedTill) {
		entry = loginAttempt{}
	}
	entry.Count++
	if entry.Count >= 5 {
		entry.BlockedTill = now.Add(1 * time.Hour)
		entry.Count = 0
		clog.Warnf("Login brute force protection: ip=%s blocked for 1 hour", ip)
	}
	h.loginFails[ip] = entry
}

func (h *Handler) clearLoginFailures(ip string) {
	h.loginFailsMu.Lock()
	delete(h.loginFails, ip)
	h.loginFailsMu.Unlock()
}
