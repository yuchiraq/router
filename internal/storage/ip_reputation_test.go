package storage

import (
	"path/filepath"
	"testing"
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
