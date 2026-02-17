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

type BackupConfig struct {
	Sources         []string `json:"sources"`
	DestinationDir  string   `json:"destinationDir"`
	IntervalMinutes int      `json:"intervalMinutes"`
	KeepCopies      int      `json:"keepCopies"`
	Enabled         bool     `json:"enabled"`
}

type BackupEntry struct {
	Path      string    `json:"path"`
	CreatedAt time.Time `json:"createdAt"`
	SizeBytes int64     `json:"sizeBytes"`
}

type backupState struct {
	Config    BackupConfig  `json:"config"`
	Entries   []BackupEntry `json:"entries"`
	LastError string        `json:"lastError,omitempty"`
}

type BackupStore struct {
	mu        sync.RWMutex
	path      string
	config    BackupConfig
	entries   []BackupEntry
	lastError string
	OnResult  func(err error, archivePath string)
}

func NewBackupStore(path string) *BackupStore {
	s := &BackupStore{path: path}
	s.config = BackupConfig{IntervalMinutes: 60, KeepCopies: 10, Enabled: false, Sources: []string{}}
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
	if st.Config.IntervalMinutes <= 0 {
		st.Config.IntervalMinutes = 60
	}
	if st.Config.KeepCopies <= 0 {
		st.Config.KeepCopies = 10
	}
	s.config = st.Config
	s.entries = st.Entries
	s.lastError = st.LastError
}

func (s *BackupStore) saveLocked() {
	data, err := json.MarshalIndent(backupState{Config: s.config, Entries: s.entries, LastError: s.lastError}, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(s.path, data, 0644)
}

func (s *BackupStore) Get() (BackupConfig, []BackupEntry, string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cfg := s.config
	entries := make([]BackupEntry, len(s.entries))
	copy(entries, s.entries)
	return cfg, entries, s.lastError
}

func (s *BackupStore) UpdateConfig(cfg BackupConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if cfg.IntervalMinutes <= 0 {
		cfg.IntervalMinutes = 60
	}
	if cfg.KeepCopies <= 0 {
		cfg.KeepCopies = 10
	}
	cfg.Sources = normalizeSources(cfg.Sources)
	s.config = cfg
	s.saveLocked()
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
		cfg, _, _ := s.Get()
		if !cfg.Enabled || cfg.DestinationDir == "" || len(cfg.Sources) == 0 {
			continue
		}
		if time.Now().Unix()%int64(cfg.IntervalMinutes*60) < 60 {
			_ = s.RunNow()
		}
	}
}

func (s *BackupStore) RunNow() error {
	s.mu.Lock()
	cfg := s.config
	s.mu.Unlock()

	if cfg.DestinationDir == "" {
		return s.setError(fmt.Errorf("destinationDir is required"))
	}
	if len(cfg.Sources) == 0 {
		return s.setError(fmt.Errorf("at least one source is required"))
	}

	if err := os.MkdirAll(cfg.DestinationDir, 0755); err != nil {
		return s.setError(err)
	}

	archivePath := filepath.Join(cfg.DestinationDir, "backup-"+time.Now().Format("20060102-150405.000000000")+".zip")
	file, err := os.Create(archivePath)
	if err != nil {
		return s.setError(err)
	}
	zw := zip.NewWriter(file)

	added := 0
	for _, src := range cfg.Sources {
		if err := addSourceToZip(zw, src); err == nil {
			added++
		}
	}
	if err := zw.Close(); err != nil {
		_ = file.Close()
		return s.setError(err)
	}
	if err := file.Close(); err != nil {
		return s.setError(err)
	}
	if added == 0 {
		_ = os.Remove(archivePath)
		return s.setError(fmt.Errorf("no valid sources found"))
	}

	st, err := os.Stat(archivePath)
	if err != nil {
		return s.setError(err)
	}

	s.mu.Lock()
	s.entries = append(s.entries, BackupEntry{Path: archivePath, CreatedAt: time.Now(), SizeBytes: st.Size()})
	s.lastError = ""
	s.enforceRetentionLocked()
	s.saveLocked()
	s.mu.Unlock()
	if s.OnResult != nil {
		s.OnResult(nil, archivePath)
	}
	return nil
}

func addSourceToZip(zw *zip.Writer, source string) error {
	info, err := os.Stat(source)
	if err != nil {
		return err
	}
	baseName := filepath.Base(source)
	if !info.IsDir() {
		return addFileToZip(zw, source, baseName)
	}

	return filepath.WalkDir(source, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(filepath.Join(baseName, rel))
		return addFileToZip(zw, path, name)
	})
}

func addFileToZip(zw *zip.Writer, diskPath, archivePath string) error {
	f, err := os.Open(diskPath)
	if err != nil {
		return err
	}
	defer f.Close()

	w, err := zw.Create(archivePath)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, f)
	return err
}

func (s *BackupStore) enforceRetentionLocked() {
	keep := s.config.KeepCopies
	if keep <= 0 {
		keep = 1
	}
	sort.Slice(s.entries, func(i, j int) bool {
		return s.entries[i].CreatedAt.After(s.entries[j].CreatedAt)
	})
	if len(s.entries) <= keep {
		return
	}
	for _, old := range s.entries[keep:] {
		_ = os.Remove(old.Path)
	}
	s.entries = s.entries[:keep]
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
