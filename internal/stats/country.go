package stats

import (
	"encoding/json"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/text/language"
	"golang.org/x/text/language/display"
)

const unknownCountryCode = "UN"

var (
	countryHTTPClient = &http.Client{Timeout: 400 * time.Millisecond}
	countryCache      = &ipCountryCache{items: make(map[string]cachedCountry)}
)

type cachedCountry struct {
	code      string
	expiresAt time.Time
}

type ipCountryCache struct {
	mu    sync.RWMutex
	items map[string]cachedCountry
}

func (c *ipCountryCache) get(ip string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.items[ip]
	if !ok || time.Now().After(entry.expiresAt) {
		return "", false
	}
	return entry.code, true
}

func (c *ipCountryCache) set(ip, code string, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[ip] = cachedCountry{code: code, expiresAt: time.Now().Add(ttl)}
}

// CountryFromRequest resolves request country by headers and client IP.
func CountryFromRequest(r *http.Request) string {
	if r == nil {
		return unknownCountryCode
	}

	if code := countryFromHeaders(r); code != "" {
		return NormalizeCountry(code)
	}

	ip := clientIP(r)
	if ip == nil {
		return unknownCountryCode
	}
	if isLocalIP(ip) {
		return "LOCAL"
	}

	ipText := ip.String()
	if cached, ok := countryCache.get(ipText); ok {
		return cached
	}

	code := lookupCountryByIP(ipText)
	countryCache.set(ipText, code, 24*time.Hour)
	return code
}

func lookupCountryByIP(ip string) string {
	// Simple external lookup. If unavailable, keep Unknown.
	url := "https://ipwho.is/" + ip
	resp, err := countryHTTPClient.Get(url)
	if err != nil {
		return unknownCountryCode
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return unknownCountryCode
	}

	var payload struct {
		Success     bool   `json:"success"`
		CountryCode string `json:"country_code"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return unknownCountryCode
	}
	if !payload.Success {
		return unknownCountryCode
	}
	return NormalizeCountry(payload.CountryCode)
}

func countryFromHeaders(r *http.Request) string {
	for _, header := range []string{"CF-IPCountry", "X-Country-Code", "X-Country"} {
		value := strings.TrimSpace(r.Header.Get(header))
		if value != "" && value != "XX" && value != "T1" {
			return value
		}
	}
	return ""
}

func clientIP(r *http.Request) net.IP {
	xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
	if xff != "" {
		parts := strings.Split(xff, ",")
		for _, part := range parts {
			ip := net.ParseIP(strings.TrimSpace(part))
			if ip != nil {
				return ip
			}
		}
	}

	if xrip := net.ParseIP(strings.TrimSpace(r.Header.Get("X-Real-IP"))); xrip != nil {
		return xrip
	}

	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil {
		if ip := net.ParseIP(host); ip != nil {
			return ip
		}
	}

	return net.ParseIP(strings.TrimSpace(r.RemoteAddr))
}

func isLocalIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalMulticast() || ip.IsLinkLocalUnicast()
}

// NormalizeCountry normalizes country code for storage.
func NormalizeCountry(code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return unknownCountryCode
	}
	return code
}

func countryName(code string) string {
	if code == "LOCAL" {
		return "Local network"
	}
	if code == unknownCountryCode {
		return "Unknown"
	}

	region, err := language.ParseRegion(code)
	if err != nil {
		return code
	}
	name := display.English.Regions().Name(region)
	if strings.TrimSpace(name) == "" {
		return code
	}
	return name
}

// GetCountryData returns request counts grouped by country.
func (s *Stats) GetCountryData() []map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows := make([]map[string]interface{}, 0, len(s.countryStats))
	for code, count := range s.countryStats {
		rows = append(rows, map[string]interface{}{
			"code":  code,
			"name":  countryName(code),
			"count": count,
		})
	}

	sort.Slice(rows, func(i, j int) bool {
		ci := rows[i]["count"].(int)
		cj := rows[j]["count"].(int)
		if ci == cj {
			ni := rows[i]["name"].(string)
			nj := rows[j]["name"].(string)
			return ni < nj
		}
		return ci > cj
	})

	return rows
}
