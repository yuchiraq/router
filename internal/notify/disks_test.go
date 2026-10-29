package notify

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"router/internal/stats"
	"router/internal/storage"
)

type diskTransport struct {
	calls map[string]int
	fail  string
	texts []string
}

func (t *diskTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	b, _ := io.ReadAll(r.Body)
	values, _ := url.ParseQuery(string(b))
	chat := values.Get("chat_id")
	t.calls[chat]++
	t.texts = append(t.texts, values.Get("text"))
	body := `{"ok":true}`
	if chat == t.fail {
		body = `{"ok":false,"error_code":429}`
	}
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
}

func TestDiskAlerts(t *testing.T) {
	store := storage.NewNotificationStore(filepath.Join(t.TempDir(), "n.json"))
	cfg := storage.NotificationConfig{Enabled: true, Token: "private-token", ChatIDs: []int64{1, 2}, Events: map[string]bool{"disk_usage": true}}
	if err := store.Update(cfg); err != nil {
		t.Fatal(err)
	}
	n := NewTelegramNotifier(store)
	transport := &diskTransport{calls: map[string]int{}}
	n.client = &http.Client{Transport: transport}
	m := NewDiskMonitor(n)
	now := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	check := func(percent float64) {
		m.Check(now, []stats.DiskUsage{{Time: now, Mountpoint: "/", Device: "/dev/test", UsedPercent: percent, Total: 100 << 30, Free: uint64(100-percent) << 30}})
	}
	assertCalls := func(a, b int) {
		t.Helper()
		if transport.calls["1"] != a || transport.calls["2"] != b {
			t.Fatalf("calls: %v, want %d %d", transport.calls, a, b)
		}
	}
	check(84)
	assertCalls(0, 0)
	transport.fail = "2"
	check(85)
	assertCalls(1, 1)
	transport.fail = ""
	now = now.Add(time.Minute)
	check(86)
	assertCalls(1, 2)
	now = now.Add(time.Minute)
	check(84)
	assertCalls(1, 2)
	check(95)
	assertCalls(2, 3)
	check(94)
	assertCalls(2, 3)
	now = now.Add(6 * time.Hour)
	check(96)
	assertCalls(3, 4)
	check(79)
	assertCalls(4, 5)
	if !strings.Contains(transport.texts[len(transport.texts)-1], "освобождено") {
		t.Fatal("missing recovery message")
	}
	check(79)
	assertCalls(4, 5)
	cfg = store.Get()
	cfg.QuietHoursOn = true
	cfg.QuietHoursStart = 0
	cfg.QuietHoursEnd = 0
	store.Update(cfg)
	check(99)
	assertCalls(4, 5)
	cfg.QuietHoursOn = false
	store.Update(cfg)
	check(99)
	assertCalls(5, 6)
	cfg.Events["disk_usage"] = false
	store.Update(cfg)
	now = now.Add(8 * time.Hour)
	check(100)
	assertCalls(5, 6)
}

func TestDiskAlertsIgnoreStaleAndDisabled(t *testing.T) {
	store := storage.NewNotificationStore(filepath.Join(t.TempDir(), "n.json"))
	store.Update(storage.NotificationConfig{Enabled: true, Token: "t", ChatIDs: []int64{1}, Events: map[string]bool{"disk_usage": true}})
	n := NewTelegramNotifier(store)
	transport := &diskTransport{calls: map[string]int{}}
	n.client = &http.Client{Transport: transport}
	m := NewDiskMonitor(n)
	now := time.Now()
	m.Check(now, []stats.DiskUsage{{Time: now.Add(-3 * time.Minute), Mountpoint: "/", Total: 100, UsedPercent: 99}})
	if len(transport.calls) != 0 {
		t.Fatal("stale sample alerted")
	}
	for _, percent := range []float64{-1, 101} {
		m.Check(now, []stats.DiskUsage{{Time: now, Mountpoint: "/", Total: 100, UsedPercent: percent}})
	}
	if len(transport.calls) != 0 {
		t.Fatal("invalid sample alerted")
	}
}

type errorTransport struct{}

func (errorTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, fmt.Errorf("network down")
}
func TestTelegramErrorsDoNotLeakToken(t *testing.T) {
	n := NewTelegramNotifier(storage.NewNotificationStore(filepath.Join(t.TempDir(), "n.json")))
	n.client = &http.Client{Transport: errorTransport{}}
	err := n.callBot("SUPER-SECRET", "sendMessage", url.Values{})
	if err == nil || strings.Contains(err.Error(), "SUPER-SECRET") {
		t.Fatal("unsafe network error")
	}
}

func TestDiskMonitorDiagnosticsAndExplicitTest(t *testing.T) {
	store := storage.NewNotificationStore(filepath.Join(t.TempDir(), "n.json"))
	n := NewTelegramNotifier(store)
	transport := &diskTransport{calls: map[string]int{}}
	n.client = &http.Client{Transport: transport}
	m := NewDiskMonitor(n)
	now := time.Now()
	if !strings.Contains(m.Status(now).Reason, "выключена") {
		t.Fatal("disabled state missing")
	}
	cfg := storage.NotificationConfig{Enabled: true, Token: "token", ChatIDs: []int64{1}, Events: map[string]bool{"disk_usage": true}}
	store.Update(cfg)
	sample := stats.DiskUsage{Time: now, Mountpoint: "/", Total: 100 << 30, Free: 1 << 30, UsedPercent: 99}
	transport.fail = "1"
	m.Check(now, []stats.DiskUsage{sample})
	status := m.Status(now)
	if status.LastError == "" || status.LastCheck.IsZero() || !status.LastSent.IsZero() {
		t.Fatal("failed delivery looks successful", status)
	}
	transport.fail = ""
	m.Check(now, []stats.DiskUsage{sample})
	status = m.Status(now)
	if status.LastError != "" || status.LastSent.IsZero() {
		t.Fatal("recovery not reflected", status)
	}
	cfg.Enabled = false
	cfg.QuietHoursOn = true
	cfg.QuietHoursStart = 0
	cfg.QuietHoursEnd = 0
	store.Update(cfg)
	if err := n.TestDiskMessage([]stats.DiskUsage{sample}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(transport.texts[len(transport.texts)-1], "не авария") {
		t.Fatal("test looks like real alert")
	}
	before := transport.calls["1"]
	if err := n.TestDiskMessage(nil); err == nil || transport.calls["1"] != before {
		t.Fatal("test sent without samples")
	}
}
