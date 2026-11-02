package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type GPTConfig struct {
	Enabled      bool    `json:"enabled"`
	APIKey       string  `json:"apiKey"`
	Model        string  `json:"model"`
	SystemPrompt string  `json:"systemPrompt"`
	MaxLogLines  int     `json:"maxLogLines"`
	OnlyChatIDs  []int64 `json:"onlyChatIds"`
}

type GPTStore struct {
	mu     sync.RWMutex
	path   string
	config GPTConfig
}

func NewGPTStore(path string) *GPTStore {
	s := &GPTStore{path: path, config: GPTConfig{Model: "gpt-4o-mini", MaxLogLines: 20, OnlyChatIDs: []int64{}}}
	s.load()
	return s
}

func (s *GPTStore) load() {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(s.path)
	if err != nil || len(data) == 0 {
		return
	}
	var cfg GPTConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return
	}
	if cfg.Model == "" {
		cfg.Model = "gpt-4o-mini"
	}
	if cfg.MaxLogLines <= 0 {
		cfg.MaxLogLines = 20
	}
	s.config = cfg
}

func (s *GPTStore) saveConfig(cfg GPTConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(s.path), ".gpt-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err != nil {
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

func (s *GPTStore) Get() GPTConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cfg := s.config
	cfg.OnlyChatIDs = append([]int64{}, s.config.OnlyChatIDs...)
	return cfg
}

func (s *GPTStore) Update(cfg GPTConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if cfg.Model == "" {
		cfg.Model = "gpt-4o-mini"
	}
	if cfg.MaxLogLines <= 0 {
		cfg.MaxLogLines = 20
	}
	if err := s.saveConfig(cfg); err != nil {
		return err
	}
	s.config = cfg
	return nil
}
