package panel

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"net/netip"
	"router/internal/clog"
	"strings"
	"sync"
	"time"
)

type loginAttempt struct {
	Count       int
	BlockedTill time.Time
	UpdatedAt   time.Time
}

type authState struct {
	sessions      map[string]time.Time
	sessionsMu    sync.RWMutex
	loginFails    map[string]loginAttempt
	loginFailsMu  sync.Mutex
	credentialsMu sync.Mutex
}

func newAuthState() *authState {
	return &authState{
		sessions:   map[string]time.Time{},
		loginFails: map[string]loginAttempt{},
	}
}

func (h *Handler) isAuthenticated(r *http.Request) bool {
	cookie, err := r.Cookie("router_session")
	if err != nil || cookie.Value == "" {
		return false
	}
	h.auth.sessionsMu.RLock()
	expiresAt, ok := h.auth.sessions[cookie.Value]
	h.auth.sessionsMu.RUnlock()
	if !ok || time.Now().After(expiresAt) {
		return false
	}
	return true
}

func (h *Handler) createSession() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	token := hex.EncodeToString(b)
	h.auth.sessionsMu.Lock()
	for key, expiry := range h.auth.sessions {
		if time.Now().After(expiry) {
			delete(h.auth.sessions, key)
		}
	}
	if len(h.auth.sessions) >= 128 {
		h.auth.sessionsMu.Unlock()
		return ""
	}
	h.auth.sessions[token] = time.Now().Add(24 * time.Hour)
	h.auth.sessionsMu.Unlock()
	return token
}

func (h *Handler) invalidateSession(token string) {
	if token == "" {
		return
	}
	h.auth.sessionsMu.Lock()
	delete(h.auth.sessions, token)
	h.auth.sessionsMu.Unlock()
}

func clientIPFromRequest(r *http.Request) string {
	xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
	if localProxy(r) && xff != "" {
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
	h.auth.loginFailsMu.Lock()
	defer h.auth.loginFailsMu.Unlock()
	entry, ok := h.auth.loginFails[ip]
	if !ok && len(h.auth.loginFails) >= 4096 {
		for key, value := range h.auth.loginFails {
			if time.Since(value.UpdatedAt) > time.Hour && time.Now().After(value.BlockedTill) {
				delete(h.auth.loginFails, key)
			}
		}
		if len(h.auth.loginFails) >= 4096 {
			return time.Minute, true
		}
	}
	if !ok || entry.BlockedTill.IsZero() {
		return 0, false
	}
	now := time.Now()
	if now.After(entry.BlockedTill) {
		delete(h.auth.loginFails, ip)
		return 0, false
	}
	return time.Until(entry.BlockedTill), true
}

func (h *Handler) registerLoginFailure(ip string) {
	h.auth.loginFailsMu.Lock()
	defer h.auth.loginFailsMu.Unlock()
	now := time.Now()
	entry := h.auth.loginFails[ip]
	if !entry.BlockedTill.IsZero() && now.After(entry.BlockedTill) {
		entry = loginAttempt{}
	}
	entry.Count++
	entry.UpdatedAt = now
	if entry.Count >= 5 {
		entry.BlockedTill = now.Add(1 * time.Hour)
		entry.Count = 0
		clog.Warnf("Login brute force protection: ip=%s blocked for 1 hour", ip)
	}
	h.auth.loginFails[ip] = entry
}

func (h *Handler) clearLoginFailures(ip string) {
	h.auth.loginFailsMu.Lock()
	delete(h.auth.loginFails, ip)
	h.auth.loginFailsMu.Unlock()
}
