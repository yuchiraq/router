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
