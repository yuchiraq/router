package storage

import (
	"context"
	"fmt"
	"net"
	"router/internal/clog"
	"strings"
	"sync"
	"time"
)

// Rule represents a routing rule with its status and last access time
type Rule struct {
	Host        string    `json:"-"` // Host is the map key, not stored in the struct's JSON
	Target      string    `json:"target"`
	Maintenance bool      `json:"maintenance"`
	LastAccess  time.Time `json:"-"`
	ServiceDown bool      `json:"-"`
}

// RuleStore manages the routing rules
type RuleStore struct {
	mu              sync.RWMutex
	rules           map[string]*Rule
	storage         *Storage
	MaintenanceMode bool `json:"maintenanceMode"`
}

// NewRuleStore creates a new RuleStore
func NewRuleStore(storage *Storage) *RuleStore {
	rs := &RuleStore{
		rules:   make(map[string]*Rule), // Always initialize to a non-nil map
		storage: storage,
	}

	loadedRules, maintenanceMode, err := storage.Load()
	if err != nil {
		clog.Warnf("Error loading rules: %v. Starting with a fresh rule set.", err)
	} else {
		// Only assign if the loaded map is not nil
		if loadedRules != nil {
			rs.rules = loadedRules
		}
		rs.MaintenanceMode = maintenanceMode
	}

	go rs.startHealthCheck()
	return rs
}

// Add adds a new rule or updates an existing one
func (s *RuleStore) Add(host, target string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// The Host field is primarily for template display and is populated by the All() method.
	s.rules[host] = &Rule{Target: target}
	s.storage.Save(s.rules, s.MaintenanceMode)
}

// Remove removes a rule
func (s *RuleStore) Remove(host string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.rules, host)
	s.storage.Save(s.rules, s.MaintenanceMode)
}

// Get retrieves a rule
func (s *RuleStore) Get(host string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rule, ok := s.rules[host]
	if ok {
		rule.LastAccess = time.Now()
		return rule.Target, true
	}
	return "", false
}

// GetRule retrieves a rule with full metadata.
func (s *RuleStore) GetRule(host string) (*Rule, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rule, ok := s.rules[host]
	if ok {
		rule.LastAccess = time.Now()
		return rule, true
	}
	return nil, false
}

// All returns all rules as a slice, with the Host field populated for template use.
func (s *RuleStore) All() []*Rule {
	s.mu.RLock()
	defer s.mu.RUnlock()

	allRules := make([]*Rule, 0, len(s.rules))
	for host, rule := range s.rules {
		rule.Host = host // Populate the Host field from the map key
		allRules = append(allRules, rule)
	}
	return allRules
}

// HostPolicy is used by autocert to determine which domains to request certificates for.
func (s *RuleStore) HostPolicy(ctx context.Context, host string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, ok := s.rules[host]; ok {
		return nil
	}
	return fmt.Errorf("host %q not allowed", host)
}

// SetMaintenanceMode sets the maintenance mode status
func (s *RuleStore) SetMaintenanceMode(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.MaintenanceMode = enabled
	s.storage.Save(s.rules, s.MaintenanceMode) // Save the updated state
}

// SetRuleMaintenance updates maintenance mode for a specific rule.
func (s *RuleStore) SetRuleMaintenance(host string, enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rule, ok := s.rules[host]
	if !ok {
		return
	}
	rule.Maintenance = enabled
	s.storage.Save(s.rules, s.MaintenanceMode)
}

// startHealthCheck periodically checks the health of the services
func (s *RuleStore) startHealthCheck() {
	for {
		time.Sleep(1 * time.Minute) // Check every minute
		s.checkServices()
	}
}

// checkServices attempts to connect to each service to check its status
func (s *RuleStore) checkServices() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, rule := range s.rules {
		// Clean up the target address for dialing
		targetAddr := rule.Target
		if strings.HasPrefix(targetAddr, "https://") {
			targetAddr = strings.TrimPrefix(targetAddr, "https://")
		} else if strings.HasPrefix(targetAddr, "http://") {
			targetAddr = strings.TrimPrefix(targetAddr, "http://")
		}

		// If the address has no port, Dial will fail. We need to split and check.
		// This is a simplified health check.
		_, _, err := net.SplitHostPort(targetAddr)
		if err != nil {
			// If splitting fails, it might be because there's no port.
			// For a simple health check, we can just skip or assume a default port.
			// For now, we'll log it and mark it as potentially down.
			// A robust solution would be more complex.
			clog.Warnf("Could not parse target for health check: %s. Assuming down.", rule.Target)
			rule.ServiceDown = true
			continue
		}

		conn, err := net.DialTimeout("tcp", targetAddr, 5*time.Second)
		if err != nil {
			rule.ServiceDown = true
		} else {
			rule.ServiceDown = false
			conn.Close()
		}
	}
}
