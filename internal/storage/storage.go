package storage

import (
	"encoding/json"
	"os"
	"sync"
)

// Storage handles the reading and writing of routing rules to a file.
type Storage struct {
	filePath string
	mu       sync.Mutex
}

// NewStorage creates a new Storage instance.
func NewStorage(filePath string) *Storage {
	return &Storage{filePath: filePath}
}

// Load reads the routing rules from the storage file.
func (s *Storage) Load() (map[string]*Rule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]*Rule), nil
		}
		return nil, err
	}

	if len(data) == 0 {
		return make(map[string]*Rule), nil
	}

	var rules map[string]*Rule
	if err := json.Unmarshal(data, &rules); err != nil {
		return nil, err
	}
	return rules, nil
}

// Save writes the routing rules to the storage file.
func (s *Storage) Save(rules map[string]*Rule) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.MarshalIndent(rules, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.filePath, data, 0644)
}
