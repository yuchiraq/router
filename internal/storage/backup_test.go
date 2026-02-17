package storage

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestBackupStoreRunNowAndRetention(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "src")
	dstDir := filepath.Join(dir, "dst")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatalf("mkdir src: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "a.txt"), []byte("hello"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	store := NewBackupStore(filepath.Join(dir, "backup_config.json"))
	store.UpdateConfig(BackupConfig{
		Sources:         []string{srcDir},
		DestinationDir:  dstDir,
		IntervalMinutes: 60,
		KeepCopies:      1,
		Enabled:         true,
	})

	if err := store.RunNow(); err != nil {
		t.Fatalf("run backup: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "b.txt"), []byte("world"), 0644); err != nil {
		t.Fatalf("write second: %v", err)
	}
	if err := store.RunNow(); err != nil {
		t.Fatalf("run backup 2: %v", err)
	}

	cfg, entries, lastErr := store.Get()
	if cfg.KeepCopies != 1 {
		t.Fatalf("unexpected keep copies: %d", cfg.KeepCopies)
	}
	if lastErr != "" {
		t.Fatalf("unexpected last error: %s", lastErr)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 retained entry, got %d", len(entries))
	}

	r, err := zip.OpenReader(entries[0].Path)
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	defer r.Close()
	if len(r.File) == 0 {
		t.Fatalf("expected files in archive")
	}
}
