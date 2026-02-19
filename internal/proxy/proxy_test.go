package proxy

import (
	"net/http"
	"testing"
)

func TestClientIPSelection(t *testing.T) {
	tests := []struct {
		name     string
		headers  map[string]string
		remote   string
		expected string
	}{
		{
			name:     "cf connecting ip has highest priority when public",
			headers:  map[string]string{"CF-Connecting-IP": "198.51.100.7", "X-Real-IP": "198.51.100.8"},
			remote:   "127.0.0.1:12345",
			expected: "198.51.100.7",
		},
		{
			name:     "public xff beats localhost x-real",
			headers:  map[string]string{"X-Real-IP": "127.0.0.1", "X-Forwarded-For": "198.51.100.11, 127.0.0.1"},
			remote:   "185.177.72.13:23088",
			expected: "198.51.100.11",
		},
		{
			name:     "remote public beats localhost headers",
			headers:  map[string]string{"X-Real-IP": "127.0.0.1", "X-Forwarded-For": "127.0.0.1, 127.0.0.1"},
			remote:   "185.177.72.13:23088",
			expected: "185.177.72.13",
		},
		{
			name:     "remote addr fallback",
			headers:  map[string]string{},
			remote:   "203.0.113.20:443",
			expected: "203.0.113.20",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, "http://example.com/", nil)
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}
			req.RemoteAddr = tt.remote
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			if got := clientIP(req); got != tt.expected {
				t.Fatalf("clientIP() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestAppendForwardedFor(t *testing.T) {
	if got := appendForwardedFor("", "198.51.100.1"); got != "198.51.100.1" {
		t.Fatalf("appendForwardedFor empty = %q", got)
	}
	if got := appendForwardedFor("198.51.100.1", "203.0.113.2"); got != "198.51.100.1, 203.0.113.2" {
		t.Fatalf("appendForwardedFor append = %q", got)
	}
}
