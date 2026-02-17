package storage

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestBackupStoreRunNowAndRetentionPerJob(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "src")
	dst1 := filepath.Join(dir, "dst1")
	dst2 := filepath.Join(dir, "dst2")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatalf("mkdir src: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "a.txt"), []byte("hello"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	store := NewBackupStore(filepath.Join(dir, "backup_config.json"))
	job1 := store.UpsertJob(BackupJob{Name: "job1", Sources: []string{srcDir}, DestinationDir: dst1, KeepCopies: 1, IntervalMinutes: 60, Enabled: true})
	job2 := store.UpsertJob(BackupJob{Name: "job2", Sources: []string{srcDir}, DestinationDir: dst2, KeepCopies: 2, IntervalMinutes: 60, Enabled: true})

	if err := store.RunJobNow(job1.ID); err != nil {
		t.Fatalf("run job1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "b.txt"), []byte("world"), 0644); err != nil {
		t.Fatalf("write second: %v", err)
	}
	if err := store.RunJobNow(job1.ID); err != nil {
		t.Fatalf("run job1 second: %v", err)
	}
	if err := store.RunJobNow(job2.ID); err != nil {
		t.Fatalf("run job2: %v", err)
	}

	jobs, entries, lastErr := store.Get()
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}
	if lastErr != "" {
		t.Fatalf("unexpected last error: %s", lastErr)
	}
	job1Count, job2Count := 0, 0
	for _, e := range entries {
		if e.JobID == job1.ID {
			job1Count++
		}
		if e.JobID == job2.ID {
			job2Count++
		}
	}
	if job1Count != 1 {
		t.Fatalf("expected job1 retained 1 entry, got %d", job1Count)
	}
	if job2Count != 1 {
		t.Fatalf("expected job2 retained 1 entry, got %d", job2Count)
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
