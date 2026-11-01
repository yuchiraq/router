package storage

import (
	"context"
	"fmt"
	"net"
	"router/internal/clog"
	"sort"
	"strings"
	"sync"
	"time"
)

// Rule represents a routing rule with its status and last access time
type Rule struct {
	Host             string    `json:"-"` // Host is the map key, not stored in the struct's JSON
	Target           string    `json:"target"`
	Maintenance      bool      `json:"maintenance"`
	LastAccess       time.Time `json:"-"`
	ServiceDown      bool      `json:"-"`
	HealthCheckedAt  time.Time `json:"-"`
	HealthError      string    `json:"-"`
	healthFailures   int
	healthSuccesses  int
	healthRevision   uint64
	healthGeneration uint64
}

// RuleStore manages the routing rules
type RuleStore struct {
	mu               sync.RWMutex
	rules            map[string]*Rule
	storage          *Storage
	MaintenanceMode  bool `json:"maintenanceMode"`
	panelAddress     string
	healthMu         sync.Mutex
	healthGeneration uint64
	onServiceDown    func(ServiceOutage)
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

	for _, rule := range rs.rules {
		rs.healthGeneration++
		rule.healthGeneration = rs.healthGeneration
	}
	return rs
}

// Add adds a new rule or updates an existing one
func (s *RuleStore) Add(host, target string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	// The Host field is primarily for template display and is populated by the All() method.
	previous, existed := s.rules[host]
	s.healthGeneration++
	s.rules[host] = &Rule{Target: target, healthGeneration: s.healthGeneration}
	if err := s.storage.Save(s.rules, s.MaintenanceMode); err != nil {
		if existed {
			s.rules[host] = previous
		} else {
			delete(s.rules, host)
		}
		return err
	}
	return nil
}

// Remove removes a rule
func (s *RuleStore) Remove(host string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	previous, existed := s.rules[host]
	delete(s.rules, host)
	if err := s.storage.Save(s.rules, s.MaintenanceMode); err != nil {
		if existed {
			s.rules[host] = previous
		}
		return err
	}
	return nil
}

// Get retrieves a rule
func (s *RuleStore) Get(host string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rule, ok := s.rules[host]
	if ok {
		rule.LastAccess = time.Now()
		return rule.Target, true
	}
	return "", false
}

// GetRule retrieves a rule with full metadata.
func (s *RuleStore) GetRule(host string) (*Rule, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rule, ok := s.rules[host]
	if ok {
		rule.LastAccess = time.Now()
		copy := *rule
		copy.Host = host
		return &copy, true
	}
	return nil, false
}

// All returns all rules as a slice, with the Host field populated for template use.
func (s *RuleStore) All() []*Rule {
	s.mu.RLock()
	defer s.mu.RUnlock()

	allRules := make([]*Rule, 0, len(s.rules))
	for host, rule := range s.rules {
		copy := *rule
		copy.Host = host
		allRules = append(allRules, &copy)
	}
	sort.Slice(allRules, func(i, j int) bool { return allRules[i].Host < allRules[j].Host })
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
func (s *RuleStore) SetMaintenanceMode(enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.storage.Save(s.rules, enabled); err != nil {
		return err
	}
	s.MaintenanceMode = enabled
	return nil
}

func (s *RuleStore) IsMaintenanceMode() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.MaintenanceMode
}

func (s *RuleStore) SetPanelAddress(address string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.panelAddress = address
}

// The management endpoint must remain reachable while the hosted sites are paused.
func (s *RuleStore) IsPanelTarget(target string) bool {
	s.mu.RLock()
	address := s.panelAddress
	s.mu.RUnlock()
	return isPanelTarget(target, address)
}

func isPanelTarget(target, address string) bool {
	if address == "" {
		return false
	}
	if !strings.Contains(target, "://") {
		target = "http://" + target
	}
	u, err := ParseTarget(target)
	if err != nil || u.Scheme != "http" || u.User != nil || (u.Path != "" && u.Path != "/") || u.RawQuery != "" {
		return false
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil || port != u.Port() {
		return false
	}
	ip := net.ParseIP(u.Hostname())
	return u.Hostname() == host || u.Hostname() == "localhost" || ip != nil && ip.IsLoopback()
}

// SetRuleMaintenance updates maintenance mode for a specific rule.
func (s *RuleStore) SetRuleMaintenance(host string, enabled bool) error {
	if rule, ok := s.GetRule(host); ok && enabled && s.IsPanelTarget(rule.Target) {
		return fmt.Errorf("Панель управления должна оставаться доступной")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rule, ok := s.rules[host]
	if !ok {
		return fmt.Errorf("Маршрут не найден")
	}
	previous := rule.Maintenance
	rule.Maintenance = enabled
	if err := s.storage.Save(s.rules, s.MaintenanceMode); err != nil {
		rule.Maintenance = previous
		return err
	}
	return nil
}
