package proxy

import (
	"net/http"
	"testing"
)

func TestClientIPPrefersForwardHeaders(t *testing.T) {
	tests := []struct {
		name     string
		headers  map[string]string
		remote   string
		expected string
	}{
		{
			name:     "cf connecting ip has highest priority",
			headers:  map[string]string{"CF-Connecting-IP": "198.51.100.7", "X-Real-IP": "198.51.100.8"},
			remote:   "127.0.0.1:12345",
			expected: "198.51.100.7",
		},
		{
			name:     "x real ip used when cf missing",
			headers:  map[string]string{"X-Real-IP": "203.0.113.9"},
			remote:   "127.0.0.1:12345",
			expected: "203.0.113.9",
		},
		{
			name:     "first x forwarded for ip is used",
			headers:  map[string]string{"X-Forwarded-For": "198.51.100.11, 10.0.0.2"},
			remote:   "127.0.0.1:12345",
			expected: "198.51.100.11",
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
