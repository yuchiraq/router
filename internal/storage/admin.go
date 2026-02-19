package storage

import (
	"crypto/subtle"
	"encoding/json"
	"os"
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
	if !s.load() {
		hash, _ := bcrypt.GenerateFromPassword([]byte(defaultPass), bcrypt.DefaultCost)
		s.cfg = AdminConfig{Username: defaultUser, PasswordHash: string(hash)}
		s.saveLocked()
	}
	if s.cfg.Username == "" || s.cfg.PasswordHash == "" {
		hash, _ := bcrypt.GenerateFromPassword([]byte(defaultPass), bcrypt.DefaultCost)
		s.cfg = AdminConfig{Username: defaultUser, PasswordHash: string(hash)}
		s.saveLocked()
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

func (s *AdminStore) saveLocked() {
	b, err := json.MarshalIndent(s.cfg, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(s.path, b, 0644)
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
	if subtle.ConstantTimeCompare([]byte(cfg.Username), []byte(username)) != 1 {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(cfg.PasswordHash), []byte(password)) == nil
}

func (s *AdminStore) Update(username, password string) bool {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return false
	}
	s.mu.Lock()
	s.cfg.Username = username
	s.cfg.PasswordHash = string(hash)
	s.saveLocked()
	s.mu.Unlock()
	return true
}
