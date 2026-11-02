package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReputationLifecycle(t *testing.T) {
	s := NewIPReputationStore(filepath.Join(t.TempDir(), "ips.json"))
	now := time.Now()
	s.nowFn = func() time.Time { return now }
	s.autoBanHits = 1
	if banned, _ := s.MarkSuspicious("::ffff:198.51.100.1", "probe"); !banned {
		t.Fatal("threshold 1 ignored")
	}
	if !s.IsBanned("198.51.100.1") {
		t.Fatal("mapped address bypass")
	}
	now = now.Add(24 * time.Hour)
	if len(s.AutoBannedList()) != 0 || s.IsBanned("198.51.100.1") {
		t.Fatal("exact expiry not applied without another request")
	}
	s.autoBanHits = 3
	for i := 0; i < 3; i++ {
		s.MarkSuspicious("198.51.100.1", "probe")
	}
	s.Unban("198.51.100.1")
	if banned, _ := s.MarkSuspicious("198.51.100.1", "probe"); banned {
		t.Fatal("old strike window survived unban")
	}
	s.Ban("198.51.100.2")
	now = now.Add(8 * 24 * time.Hour)
	s.Cleanup()
	items := s.List()
	if len(items) != 1 || items[0].IP != "198.51.100.2" || !items[0].Banned {
		t.Fatal("retention removed manual ban or retained history", items)
	}
	if !NewIPReputationStore(s.path).IsBanned("198.51.100.2") {
		t.Fatal("manual ban lost on reload")
	}
}

func TestReputationLimitAndPersistence(t *testing.T) {
	s := NewIPReputationStore(filepath.Join(t.TempDir(), "ips.json"))
	now := time.Now()
	for i := 0; i < reputationLimit; i++ {
		ip := fmt.Sprintf("10.%d.%d.%d", i/65536, (i/256)%256, i%256)
		s.entries[ip] = &SuspiciousIP{IP: ip, LastSeen: now}
	}
	s.Ban("198.51.100.1")
	s.MarkSuspicious("198.51.100.2", "probe")
	if len(s.entries) > reputationLimit || !s.IsBanned("198.51.100.1") {
		t.Fatal("limit/active ban invariant")
	}
	before, err := os.ReadFile(s.path)
	if err != nil {
		t.Fatal(err)
	}
	s.MarkSuspicious("198.51.100.3", "probe")
	after, _ := os.ReadFile(s.path)
	if string(before) != string(after) {
		t.Fatal("ordinary probe synchronously rewrote JSON")
	}
	s.Cleanup()
	if s.LastError() != "" {
		t.Fatal(s.LastError())
	}
	s.path = filepath.Join(t.TempDir(), "missing", "ips.json")
	s.MarkSuspicious("198.51.100.4", "probe")
	s.Cleanup()
	if s.LastError() == "" || !s.dirty {
		t.Fatal("write failure silently discarded")
	}
}

func TestReputationRejectsInvalidAndProtectsManualBan(t *testing.T) {
	s := NewIPReputationStore(filepath.Join(t.TempDir(), "ips.json"))
	s.autoBanHits = 1
	for _, ip := range []string{"bad", "127.0.0.1", "::1", "0.0.0.0", "224.0.0.1"} {
		s.MarkSuspicious(ip, "probe")
		if s.Ban(ip) {
			t.Fatal("accepted invalid/local IP", ip)
		}
	}
	if len(s.List()) != 0 {
		t.Fatal("invalid entries saved")
	}
	s.MarkSuspicious("198.51.100.8", "probe")
	s.Ban("198.51.100.8")
	if s.List()[0].AutoBanned || !s.List()[0].BanUntil.IsZero() {
		t.Fatal("manual ban did not replace temporary ban")
	}
	if s.Remove("198.51.100.8") {
		t.Fatal("removed active ban")
	}
}
