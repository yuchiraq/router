package storage

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"sync"
)

// Storage handles saving and loading routing rules to a file.
type Storage struct {
	filePath string
	mu       sync.Mutex
}

// NewStorage creates a new Storage instance.
func NewStorage(filePath string) *Storage {
	return &Storage{
		filePath: filePath,
	}
}

// Save writes the rules to the specified file.
func (s *Storage) Save(rules map[string]*Rule) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.MarshalIndent(rules, "", "  ")
	if err != nil {
		return err
	}

	return ioutil.WriteFile(s.filePath, data, 0644)
}

// Load reads the rules from the specified file.
func (s *Storage) Load() (map[string]*Rule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := os.Stat(s.filePath); os.IsNotExist(err) {
		return make(map[string]*Rule), nil // Return empty map if file doesn't exist
	}

	data, err := ioutil.ReadFile(s.filePath)
	if err != nil {
		return nil, err
	}

	var rules map[string]*Rule
	if err := json.Unmarshal(data, &rules); err != nil {
		return nil, err
	}

	return rules, nil
}