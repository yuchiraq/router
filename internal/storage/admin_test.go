package storage

import (
	"path/filepath"
	"testing"
)

func TestAdminStoreVerifyAndUpdate(t *testing.T) {
	s := NewAdminStore(filepath.Join(t.TempDir(), "admin.json"), "testuser", "testpass")
	if !s.Verify("testuser", "testpass") {
		t.Fatalf("expected default credentials to verify")
	}
	if !s.Update("newuser", "newpass123") {
		t.Fatalf("update failed")
	}
	if !s.Verify("newuser", "newpass123") {
		t.Fatalf("updated credentials should verify")
	}
}
