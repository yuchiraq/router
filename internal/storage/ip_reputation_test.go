package storage

import (
	"path/filepath"
	"testing"
	"time"
)

func TestIPReputationStoreMarkAndBan(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ip_reputation.json")

	store := NewIPReputationStore(path)
	store.MarkSuspicious("1.2.3.4", "unknown host")
	store.MarkSuspicious("1.2.3.4", "suspicious path probe")

	items := store.List()
	if len(items) != 1 {
		t.Fatalf("expected 1 suspicious item, got %d", len(items))
	}
	if items[0].Count != 2 {
		t.Fatalf("expected count 2, got %d", items[0].Count)
	}
	if items[0].Reason != "suspicious path probe" {
		t.Fatalf("unexpected reason: %s", items[0].Reason)
	}

	if !store.Ban("1.2.3.4") {
		t.Fatalf("expected ban to succeed")
	}
	if !store.IsBanned("1.2.3.4") {
		t.Fatalf("expected ip to be banned")
	}

	reloaded := NewIPReputationStore(path)
	if !reloaded.IsBanned("1.2.3.4") {
		t.Fatalf("expected persisted banned state")
	}
}

func TestIPReputationStoreUnban(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ip_reputation.json")

	store := NewIPReputationStore(path)
	store.Ban("5.6.7.8")
	if !store.IsBanned("5.6.7.8") {
		t.Fatalf("expected ip to be banned")
	}

	if !store.Unban("5.6.7.8") {
		t.Fatalf("expected unban to succeed")
	}
	if store.IsBanned("5.6.7.8") {
		t.Fatalf("expected ip to be unbanned")
	}

	reloaded := NewIPReputationStore(path)
	if reloaded.IsBanned("5.6.7.8") {
		t.Fatalf("expected persisted unbanned state")
	}
}

func TestIPReputationStoreRemove(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ip_reputation.json")

	store := NewIPReputationStore(path)
	store.MarkSuspicious("9.8.7.6", "unknown host")
	if len(store.List()) != 1 {
		t.Fatalf("expected one entry before remove")
	}

	if !store.Remove("9.8.7.6") {
		t.Fatalf("expected remove to succeed")
	}
	if len(store.List()) != 0 {
		t.Fatalf("expected no entries after remove")
	}

	reloaded := NewIPReputationStore(path)
	if len(reloaded.List()) != 0 {
		t.Fatalf("expected remove to persist")
	}
}

func TestIPReputationStoreAutoBanAndExpire(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ip_reputation.json")

	now := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	store := NewIPReputationStore(path)
	store.nowFn = func() time.Time { return now }

	var autoBanned bool
	for i := 0; i < 10; i++ {
		autoBanned, _ = store.MarkSuspicious("10.20.30.40", "suspicious path probe")
	}
	if !autoBanned {
		t.Fatalf("expected auto-ban on rapid suspicious hits")
	}
	if !store.IsBanned("10.20.30.40") {
		t.Fatalf("expected ip to be banned")
	}
	if len(store.AutoBannedList()) != 1 {
		t.Fatalf("expected one auto-banned ip")
	}

	now = now.Add(25 * time.Hour)
	if store.IsBanned("10.20.30.40") {
		t.Fatalf("expected auto-ban to expire after 24h")
	}
}
