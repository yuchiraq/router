package panel

import (
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
)

const panelCSP = "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'"

func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !localProxy(r) && !secureRequest(r) {
			http.Error(w, "Для панели управления требуется HTTPS", http.StatusUpgradeRequired)
			return
		}
		w.Header().Set("Content-Security-Policy", panelCSP)
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		if secureRequest(r) {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000")
		}
		if !strings.HasPrefix(r.URL.Path, "/static/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}

// Only this machine's reverse proxy may supply forwarding metadata to the panel.
// The public proxy always replaces these headers instead of trusting a client.
func localProxy(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func secureRequest(r *http.Request) bool {
	return r.TLS != nil || localProxy(r) && r.Header.Get("X-Forwarded-Proto") == "https"
}

func sameOrigin(r *http.Request, origin string) bool {
	u, err := url.Parse(origin)
	if err != nil || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return false
	}
	scheme := "http"
	if secureRequest(r) {
		scheme = "https"
	}
	return u.Scheme == scheme && strings.EqualFold(u.Host, r.Host)
}

func protectMutation(w http.ResponseWriter, r *http.Request) bool {
	if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
		return true
	}
	site := r.Header.Get("Sec-Fetch-Site")
	origin := r.Header.Get("Origin")
	// A non-browser API caller must explicitly opt in; a cross-site HTML form cannot.
	if site == "cross-site" || site == "same-site" || (origin != "" && !sameOrigin(r, origin)) || (origin == "" && site != "same-origin" && r.Header.Get("X-Router-Request") != "1") {
		http.Error(w, "Запрос с другого сайта запрещён", http.StatusForbidden)
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Некорректный или слишком большой запрос", http.StatusBadRequest)
			return false
		}
	}
	return true
}

// Public assets cannot expose HTML administration pages, dotfiles or directory listings.
func StaticAssets(directory string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/static/")
		if name == r.URL.Path || name != path.Clean(name) || strings.ContainsAny(name, "\\\x00") {
			http.NotFound(w, r)
			return
		}
		for _, part := range strings.Split(name, "/") {
			if strings.HasPrefix(part, ".") {
				http.NotFound(w, r)
				return
			}
		}
		switch path.Ext(name) {
		case ".css", ".js", ".svg", ".png", ".ico", ".woff2":
		default:
			http.NotFound(w, r)
			return
		}
		root, err := os.OpenRoot(directory)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer root.Close()
		f, err := root.Open(name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		st, err := f.Stat()
		if err != nil || !st.Mode().IsRegular() {
			http.NotFound(w, r)
			return
		}
		http.ServeContent(w, r, name, st.ModTime(), f)
	})
}
