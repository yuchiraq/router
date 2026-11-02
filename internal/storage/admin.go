package storage

import (
	"crypto/subtle"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/crypto/bcrypt"
)

type AdminConfig struct {
	Username     string `json:"username"`
	PasswordHash string `json:"passwordHash"`
}

type AdminStore struct {
	mu   sync.RWMutex
	path string
	cfg  AdminConfig
}

func NewAdminStore(path, defaultUser, defaultPass string) *AdminStore {
	s := &AdminStore{path: path}
	if s.load() {
		return s
	}
	// A damaged or unreadable account file must never reset credentials to defaults.
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		return s
	}
	if defaultUser != "" && len(defaultPass) >= 12 && len(defaultPass) <= 72 {
		hash, err := bcrypt.GenerateFromPassword([]byte(defaultPass), bcrypt.DefaultCost)
		if err == nil {
			cfg := AdminConfig{Username: defaultUser, PasswordHash: string(hash)}
			if s.saveConfig(cfg) == nil {
				s.cfg = cfg
			}
		}
	}
	return s
}

func (s *AdminStore) load() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, err := os.ReadFile(s.path)
	if err != nil || len(b) == 0 {
		return false
	}
	var cfg AdminConfig
	if err := json.Unmarshal(b, &cfg); err != nil {
		return false
	}
	s.cfg = cfg
	return true
}

func (s *AdminStore) saveConfig(cfg AdminConfig) error {
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(s.path), ".admin-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(0600); err != nil {
		f.Close()
		return err
	}
	if _, err = f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), s.path)
}

func (s *AdminStore) Ready() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, err := bcrypt.Cost([]byte(s.cfg.PasswordHash))
	return s.cfg.Username != "" && err == nil
}

func (s *AdminStore) Username() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg.Username
}

func (s *AdminStore) Verify(username, password string) bool {
	s.mu.RLock()
	cfg := s.cfg
	s.mu.RUnlock()
	validPassword := bcrypt.CompareHashAndPassword([]byte(cfg.PasswordHash), []byte(password)) == nil
	return subtle.ConstantTimeCompare([]byte(cfg.Username), []byte(username)) == 1 && validPassword
}

func (s *AdminStore) Update(username, password string) bool {
	if username == "" || len(password) < 12 || len(password) > 72 {
		return false
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cfg := AdminConfig{Username: username, PasswordHash: string(hash)}
	if s.saveConfig(cfg) != nil {
		return false
	}
	s.cfg = cfg
	return true
}
