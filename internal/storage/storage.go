package storage

import (
	"encoding/json"
	"os"
)

// Rule defines the structure for a routing rule
type Rule struct {
	Host   string `json:"host"`
	Target string `json:"target"`
}

// storageData is the format of the data stored in the JSON file.
type storageData struct {
	Rules           map[string]*Rule `json:"rules"`
	MaintenanceMode bool             `json:"maintenanceMode"`
}

// NewStorage creates a new Storage instance.
func NewStorage(filePath string) *Storage {
	return &Storage{filePath: filePath}
}

// Load reads the routing rules and maintenance mode from the storage file.
func (s *Storage) Load() (map[string]*Rule, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]*Rule), false, nil
		}
		return nil, false, err
	}

	if len(data) == 0 {
		return make(map[string]*Rule), false, nil
	}

	var storedData storageData
	if err := json.Unmarshal(data, &storedData); err != nil {
		return nil, false, err
	}
	return storedData.Rules, storedData.MaintenanceMode, nil
}

// Save writes the routing rules and maintenance mode to the storage file.
func (s *Storage) Save(rules map[string]*Rule, maintenanceMode bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	storedData := storageData{
		Rules:           rules,
		MaintenanceMode: maintenanceMode,
	}

	data, err := json.MarshalIndent(storedData, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.filePath, data, 0644)
}
