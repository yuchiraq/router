package storage

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type BackupJob struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Sources         []string  `json:"sources"`
	DestinationDir  string    `json:"destinationDir"`
	IntervalMinutes int       `json:"intervalMinutes"`
	KeepCopies      int       `json:"keepCopies"`
	Enabled         bool      `json:"enabled"`
	LastRunAt       time.Time `json:"lastRunAt,omitempty"`
}

type BackupEntry struct {
	JobID     string    `json:"jobId"`
	JobName   string    `json:"jobName"`
	Path      string    `json:"path"`
	CreatedAt time.Time `json:"createdAt"`
	SizeBytes int64     `json:"sizeBytes"`
}

type backupState struct {
	Jobs      []BackupJob   `json:"jobs"`
	Entries   []BackupEntry `json:"entries"`
	LastError string        `json:"lastError,omitempty"`
}

type BackupStore struct {
	runMu     sync.Mutex
	mu        sync.RWMutex
	path      string
	jobs      []BackupJob
	entries   []BackupEntry
	lastError string
	OnResult  func(err error, archivePath string)
}

func NewBackupStore(path string) *BackupStore {
	s := &BackupStore{path: path, jobs: []BackupJob{}}
	s.load()
	return s
}

func (s *BackupStore) load() {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path)
	if err != nil || len(data) == 0 {
		return
	}
	var st backupState
	if err := json.Unmarshal(data, &st); err != nil {
		return
	}
	for i := range st.Jobs {
		st.Jobs[i] = normalizeJob(st.Jobs[i])
	}
	s.jobs = st.Jobs
	s.entries = st.Entries
	s.lastError = st.LastError
}

func normalizeJob(job BackupJob) BackupJob {
	if job.IntervalMinutes <= 0 {
		job.IntervalMinutes = 60
	}
	if job.KeepCopies <= 0 {
		job.KeepCopies = 10
	}
	job.Sources = normalizeSources(job.Sources)
	job.Name = strings.TrimSpace(job.Name)
	if job.Name == "" {
		job.Name = "Backup job"
	}
	return job
}

func (s *BackupStore) saveLocked() {
	data, err := json.MarshalIndent(backupState{Jobs: s.jobs, Entries: s.entries, LastError: s.lastError}, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(s.path, data, 0644)
}

func (s *BackupStore) Get() ([]BackupJob, []BackupEntry, string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	jobs := make([]BackupJob, len(s.jobs))
	copy(jobs, s.jobs)
	entries := make([]BackupEntry, len(s.entries))
	copy(entries, s.entries)
	return jobs, entries, s.lastError
}

func (s *BackupStore) UpsertJob(job BackupJob) BackupJob {
	s.mu.Lock()
	defer s.mu.Unlock()
	job = normalizeJob(job)
	if strings.TrimSpace(job.ID) == "" {
		job.ID = fmt.Sprintf("job-%d", time.Now().UnixNano())
		s.jobs = append(s.jobs, job)
		s.saveLocked()
		return job
	}
	for i := range s.jobs {
		if s.jobs[i].ID == job.ID {
			job.LastRunAt = s.jobs[i].LastRunAt
			s.jobs[i] = job
			s.saveLocked()
			return job
		}
	}
	s.jobs = append(s.jobs, job)
	s.saveLocked()
	return job
}

func (s *BackupStore) DeleteJob(jobID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := -1
	for i := range s.jobs {
		if s.jobs[i].ID == jobID {
			idx = i
			break
		}
	}
	if idx == -1 {
		return false
	}
	s.jobs = append(s.jobs[:idx], s.jobs[idx+1:]...)
	filtered := s.entries[:0]
	for _, e := range s.entries {
		if e.JobID != jobID {
			filtered = append(filtered, e)
			continue
		}
		_ = os.Remove(e.Path)
	}
	s.entries = filtered
	s.saveLocked()
	return true
}

func normalizeSources(sources []string) []string {
	out := make([]string, 0, len(sources))
	seen := map[string]struct{}{}
	for _, src := range sources {
		src = strings.TrimSpace(src)
		if src == "" {
			continue
		}
		if _, ok := seen[src]; ok {
			continue
		}
		seen[src] = struct{}{}
		out = append(out, src)
	}
	return out
}

func (s *BackupStore) Start() {
	for {
		time.Sleep(1 * time.Minute)
		_ = s.runDueJobs()
	}
}

func (s *BackupStore) runDueJobs() error {
	now := time.Now()
	jobs, _, _ := s.Get()
	for _, job := range jobs {
		if !job.Enabled {
			continue
		}
		if job.DestinationDir == "" || len(job.Sources) == 0 {
			continue
		}
		if !job.LastRunAt.IsZero() && now.Sub(job.LastRunAt) < time.Duration(job.IntervalMinutes)*time.Minute {
			continue
		}
		if err := s.RunJobNow(job.ID); err != nil {
			return err
		}
	}
	return nil
}

func (s *BackupStore) RunJobNow(jobID string) error {
	if !s.runMu.TryLock() {
		return fmt.Errorf("Копирование уже выполняется")
	}
	defer s.runMu.Unlock()
	s.mu.Lock()
	var job *BackupJob
	for i := range s.jobs {
		if s.jobs[i].ID == jobID {
			job = &s.jobs[i]
			break
		}
	}
	if job == nil {
		s.mu.Unlock()
		return s.setError(fmt.Errorf("backup job not found"))
	}
	cfg := *job
	s.mu.Unlock()

	if cfg.DestinationDir == "" {
		return s.setError(fmt.Errorf("destinationDir is required"))
	}
	if len(cfg.Sources) == 0 {
		return s.setError(fmt.Errorf("at least one source is required"))
	}
	if err := os.MkdirAll(cfg.DestinationDir, 0700); err != nil {
		return s.setError(err)
	}
	destination, err := filepath.EvalSymlinks(cfg.DestinationDir)
	if err != nil {
		return s.setError(err)
	}
	destination, err = filepath.Abs(destination)
	if err != nil {
		return s.setError(err)
	}
	for _, source := range cfg.Sources {
		resolved, err := filepath.EvalSymlinks(source)
		if err != nil {
			return s.setError(err)
		}
		resolved, err = filepath.Abs(resolved)
		if err != nil {
			return s.setError(err)
		}
		info, err := os.Stat(resolved)
		if err != nil {
			return s.setError(err)
		}
		if info.IsDir() {
			rel, err := filepath.Rel(resolved, destination)
			if err == nil && (rel == "." || rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
				return s.setError(fmt.Errorf("Папка архивов не должна находиться внутри копируемой папки"))
			}
		}
	}

	archivePath := filepath.Join(cfg.DestinationDir, fmt.Sprintf("%s-%s.zip", sanitizeName(cfg.Name), time.Now().Format("20060102-150405.000000000")))
	file, err := os.OpenFile(archivePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return s.setError(err)
	}
	zw := zip.NewWriter(file)

	for _, src := range cfg.Sources {
		if err := addSourceToZip(zw, src); err != nil {
			zw.Close()
			file.Close()
			os.Remove(archivePath)
			return s.setError(err)
		}
	}
	if err := zw.Close(); err != nil {
		_ = file.Close()
		_ = os.Remove(archivePath)
		return s.setError(err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(archivePath)
		return s.setError(err)
	}

	st, err := os.Stat(archivePath)
	if err != nil {
		return s.setError(err)
	}

	s.mu.Lock()
	for i := range s.jobs {
		if s.jobs[i].ID == cfg.ID {
			s.jobs[i].LastRunAt = time.Now()
			cfg = s.jobs[i]
			break
		}
	}
	s.entries = append(s.entries, BackupEntry{JobID: cfg.ID, JobName: cfg.Name, Path: archivePath, CreatedAt: time.Now(), SizeBytes: st.Size()})
	s.lastError = ""
	s.enforceRetentionLocked(cfg.ID, cfg.KeepCopies)
	s.saveLocked()
	s.mu.Unlock()

	if s.OnResult != nil {
		s.OnResult(nil, archivePath)
	}
	return nil
}

func sanitizeName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "backup"
	}
	name = strings.Map(func(r rune) rune {
		if r < 32 || strings.ContainsRune(" /\\<>:\"|?*", r) {
			return '-'
		}
		return r
	}, name)
	return name
}

func addSourceToZip(zw *zip.Writer, source string) error {
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	baseName := filepath.Base(source)
	if !info.IsDir() {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("Источник должен быть обычным файлом или папкой: %s", source)
		}
		return addFileToZip(zw, source, baseName)
	}
	root, err := os.OpenRoot(source)
	if err != nil {
		return err
	}
	defer root.Close()

	return filepath.WalkDir(source, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if !d.Type().IsRegular() {
			return fmt.Errorf("Специальный файл нельзя архивировать: %s", path)
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(filepath.Join(baseName, rel))
		f, err := root.Open(rel)
		if err != nil {
			return err
		}
		defer f.Close()
		st, err := f.Stat()
		if err != nil {
			return err
		}
		if !st.Mode().IsRegular() {
			return fmt.Errorf("Источник изменился: %s", path)
		}
		w, err := zw.Create(name)
		if err != nil {
			return err
		}
		_, err = io.Copy(w, f)
		return err
	})
}

func addFileToZip(zw *zip.Writer, diskPath, archivePath string) error {
	root, err := os.OpenRoot(filepath.Dir(diskPath))
	if err != nil {
		return err
	}
	defer root.Close()
	f, err := root.Open(filepath.Base(diskPath))
	if err != nil {
		return err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return err
	}
	if !st.Mode().IsRegular() {
		return fmt.Errorf("Источник изменился: %s", diskPath)
	}

	w, err := zw.Create(archivePath)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, f)
	return err
}

func (s *BackupStore) enforceRetentionLocked(jobID string, keep int) {
	if keep <= 0 {
		keep = 1
	}
	jobEntries := make([]BackupEntry, 0)
	other := make([]BackupEntry, 0)
	for _, e := range s.entries {
		if e.JobID == jobID {
			jobEntries = append(jobEntries, e)
		} else {
			other = append(other, e)
		}
	}
	sort.Slice(jobEntries, func(i, j int) bool { return jobEntries[i].CreatedAt.After(jobEntries[j].CreatedAt) })
	if len(jobEntries) > keep {
		for _, old := range jobEntries[keep:] {
			_ = os.Remove(old.Path)
		}
		jobEntries = jobEntries[:keep]
	}
	s.entries = append(other, jobEntries...)
	sort.Slice(s.entries, func(i, j int) bool { return s.entries[i].CreatedAt.After(s.entries[j].CreatedAt) })
}

func (s *BackupStore) setError(err error) error {
	s.mu.Lock()
	s.lastError = err.Error()
	s.saveLocked()
	s.mu.Unlock()
	if s.OnResult != nil {
		s.OnResult(err, "")
	}
	return err
}
