package storage

import (
	"net"
	"strings"
	"sync"
	"time"
)

// Rule represents a routing rule with its status and last access time

type Rule struct {
	Target      string
	LastAccess  time.Time
	ServiceDown bool
}

// RuleStore manages the routing rules

type RuleStore struct {
	mu    sync.RWMutex
	rules map[string]*Rule
}

// NewRuleStore creates a new RuleStore

func NewRuleStore() *RuleStore {
	rs := &RuleStore{
		rules: make(map[string]*Rule),
	}
	go rs.startHealthCheck()
	return rs
}

// Add adds a new rule or updates an existing one

func (s *RuleStore) Add(host, target string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rules[host] = &Rule{Target: target}
}

// Remove removes a rule

func (s *RuleStore) Remove(host string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.rules, host)
}

// Get retrieves a rule

func (s *RuleStore) Get(host string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rule, ok := s.rules[host]
	if ok {
		rule.LastAccess = time.Now() // Update LastAccess on rule retrieval
		return rule.Target, true
	}
	return "", false
}

// All returns all rules

func (s *RuleStore) All() map[string]*Rule {
	s.mu.RLock()
	defer s.mu.RUnlock()
	// Create a copy to avoid race conditions when the map is read in the template
	newMap := make(map[string]*Rule, len(s.rules))
	for k, v := range s.rules {
		newMap[k] = v
	}
	return newMap
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
		// Extract host and port from target
		targetHost, targetPort, err := net.SplitHostPort(rule.Target)
		if err != nil {
			// If the target is not in host:port format, assume it's a domain and default to port 80 or 443
			if strings.HasSuffix(rule.Target, ":443") {
				targetPort = "443"
			} else {
				targetPort = "80"
			}
			targetHost = rule.Target
		}

		conn, err := net.DialTimeout("tcp", net.JoinHostPort(targetHost, targetPort), 5*time.Second)
		if err != nil {
			rule.ServiceDown = true
		} else {
			rule.ServiceDown = false
			conn.Close()
		}
	}
}
