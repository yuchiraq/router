
package storage

import (
	"sync"
)

// RuleStore holds the routing rules
// RuleStore holds the routing rules and maintenance mode status
type RuleStore struct {
	mu             sync.RWMutex
	rules          map[string]string
	maintenanceMode bool
}

// NewRuleStore creates a new RuleStore
func NewRuleStore(initialRules map[string]string) *RuleStore {
	if initialRules == nil {
		initialRules = make(map[string]string)
	}
	return &RuleStore{
		rules: initialRules,
	}
}

// Add adds a new rule to the store
func (s *RuleStore) Add(host, target string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rules[host] = target
}

// Remove removes a rule from the store
func (s *RuleStore) Remove(host string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.rules, host)
}

// Get returns the target for a given host
func (s *RuleStore) Get(host string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	target, ok := s.rules[host]
	return target, ok
}

// All returns all rules in the store
func (s *RuleStore) All() map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	// Return a copy to prevent race conditions
	newRules := make(map[string]string)
	for k, v := range s.rules {
		newRules[k] = v
	}
	return newRules
}

// SetMaintenanceMode sets the maintenance mode status
func (s *RuleStore) SetMaintenanceMode(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.maintenanceMode = enabled
}

// IsMaintenanceMode returns the current maintenance mode status
func (s *RuleStore) IsMaintenanceMode() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.maintenanceMode
}
