package proxy

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"testing"
)

func TestWorkServiceForwarding(t *testing.T) {
	for _, secure := range []bool{false, true} {
		r := httptest.NewRequest("PROPFIND", "https://appworks.avayusstroi.by/dav/", nil)
		if secure {
			r.TLS = &tls.ConnectionState{}
		} else {
			r.TLS = nil
		}
		r.Host = "localhost:8099"
		r.Header.Set("X-Forwarded-Proto", "https")
		r.Header.Set("X-Forwarded-Host", "evil.example")
		r.Header.Set("X-Forwarded-For", "fake")
		r.Header.Set("CF-Connecting-IP", "fake")
		r.Header.Set("Forwarded", "proto=https;host=evil.example")
		r.Header.Set("Authorization", "Basic test")
		r.Header.Set("Destination", "https://appworks.avayusstroi.by/dav/new")
		setWorkServiceHeaders(r, "appworks.avayusstroi.by", "203.0.113.7")
		wantProto := "http"
		if secure {
			wantProto = "https"
		}
		if r.Host != "appworks.avayusstroi.by" || r.Header.Get("X-Forwarded-Proto") != wantProto || r.Header.Get("X-Forwarded-Host") != r.Host || r.Header.Get("X-Forwarded-For") != "203.0.113.7" || r.Header.Get("CF-Connecting-IP") != "" || r.Header.Get("Forwarded") != "" {
			t.Fatal("forwarding not sanitized")
		}
		if r.Header.Get("Authorization") != "Basic test" || r.Header.Get("Destination") != "https://appworks.avayusstroi.by/dav/new" {
			t.Fatal("DAV headers lost")
		}
	}
	r := httptest.NewRequest("GET", "https://another.example/", nil)
	r.Host = "localhost:8080"
	r.Header.Set("X-Forwarded-Proto", "legacy")
	setWorkServiceHeaders(r, "another.example", "203.0.113.7")
	if r.Host != "localhost:8080" || r.Header.Get("X-Forwarded-Proto") != "legacy" {
		t.Fatal("unrelated route modified")
	}
}

func TestWorkServiceDAVThroughReverseProxy(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PROPFIND" || r.URL.Path != "/dav/Folder/" || r.Host != "appworks.avayusstroi.by" || r.Header.Get("X-Forwarded-Proto") != "https" || r.Header.Get("Depth") != "1" || r.Header.Get("Authorization") != "Basic test" {
			t.Error("DAV request changed by proxy")
		}
		w.WriteHeader(207)
	}))
	defer upstream.Close()
	u, _ := url.Parse(upstream.URL)
	r := httptest.NewRequest("PROPFIND", "https://appworks.avayusstroi.by/dav/Folder/", nil)
	r.TLS = &tls.ConnectionState{}
	r.Header.Set("Depth", "1")
	r.Header.Set("Authorization", "Basic test")
	setWorkServiceHeaders(r, r.Host, "203.0.113.7")
	w := httptest.NewRecorder()
	httputil.NewSingleHostReverseProxy(u).ServeHTTP(w, r)
	if w.Code != 207 {
		t.Fatal(w.Code)
	}
}
