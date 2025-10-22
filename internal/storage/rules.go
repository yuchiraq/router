package storage

import "sync"

type RuleStore struct {
	sync.RWMutex
	rules map[string]string // host -> targetAddr
}

func NewRuleStore() *RuleStore {
	return &RuleStore{rules: make(map[string]string)}
}

func (s *RuleStore) Add(host, target string) {
	s.Lock()
	defer s.Unlock()
	s.rules[host] = target
}

func (s *RuleStore) Remove(host string) {
	s.Lock()
	defer s.Unlock()
	delete(s.rules, host)
}

func (s *RuleStore) GetTarget(host string) (string, bool) {
	s.RLock()
	defer s.RUnlock()
	t, ok := s.rules[host]
	return t, ok
}

func (s *RuleStore) All() map[string]string {
	s.RLock()
	defer s.RUnlock()
	copy := make(map[string]string)
	for k, v := range s.rules {
		copy[k] = v
	}
	return copy
}

func (s *RuleStore) Exists(host string) bool {
	s.RLock()
	defer s.RUnlock()
	_, ok := s.rules[host]
	return ok
}
