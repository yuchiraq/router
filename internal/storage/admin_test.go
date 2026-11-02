package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAdminStoreVerifyAndUpdate(t *testing.T) {
	s := NewAdminStore(filepath.Join(t.TempDir(), "admin.json"), "testuser", "test-password-123")
	if !s.Verify("testuser", "test-password-123") {
		t.Fatalf("expected default credentials to verify")
	}
	if !s.Update("newuser", "new-password-123") {
		t.Fatalf("update failed")
	}
	if !s.Verify("newuser", "new-password-123") {
		t.Fatalf("updated credentials should verify")
	}
}

func TestAdminFailClosedAndSaveFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "admin.json")
	if NewAdminStore(path, "admin", "").Ready() {
		t.Fatal("empty password accepted")
	}
	os.WriteFile(path, []byte("broken-json"), 0600)
	if NewAdminStore(path, "admin", "new-default-password").Ready() {
		t.Fatal("damaged account reset to defaults")
	}
	s := NewAdminStore(filepath.Join(t.TempDir(), "admin.json"), "admin", "old-strong-password")
	s.path = filepath.Join(t.TempDir(), "missing", "admin.json")
	if s.Update("admin", "new-strong-password") || !s.Verify("admin", "old-strong-password") {
		t.Fatal("failed save changed credentials")
	}
}
