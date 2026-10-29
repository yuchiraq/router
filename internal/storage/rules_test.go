package storage

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestMaintenancePersistenceAndRollback(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rules.json")
	s := NewRuleStore(NewStorage(path))
	s.Add("example.test", "127.0.0.1:8099")
	if err := s.SetMaintenanceMode(true); err != nil {
		t.Fatal(err)
	}
	if err := s.SetRuleMaintenance("example.test", true); err != nil {
		t.Fatal(err)
	}
	rules, mode, err := NewStorage(path).Load()
	if err != nil || !mode || !rules["example.test"].Maintenance {
		t.Fatal("state not persisted", err)
	}
	if err := s.SetRuleMaintenance("missing", true); err == nil {
		t.Fatal("missing rule accepted")
	}
	// A directory at the destination reliably simulates a failed atomic replace.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	if err := s.SetMaintenanceMode(false); err == nil || !s.IsMaintenanceMode() {
		t.Fatal("failed save changed global state")
	}
	if err := s.SetRuleMaintenance("example.test", false); err == nil {
		t.Fatal("failed save not reported")
	}
	rule, _ := s.GetRule("example.test")
	if !rule.Maintenance {
		t.Fatal("failed save changed rule")
	}
	if err := s.Add("new.test", "localhost:9000"); err == nil {
		t.Fatal("add save error ignored")
	}
	if _, exists := s.GetRule("new.test"); exists {
		t.Fatal("failed add changed memory")
	}
	if err := s.Remove("example.test"); err == nil {
		t.Fatal("remove save error ignored")
	}
	if _, exists := s.GetRule("example.test"); !exists {
		t.Fatal("failed removal changed memory")
	}
}

func TestRulesSnapshotsAndPanelProtection(t *testing.T) {
	s := NewRuleStore(NewStorage(filepath.Join(t.TempDir(), "rules.json")))
	s.SetPanelAddress("127.0.0.1:8162")
	for _, target := range []string{"localhost:8162", "http://127.0.0.1:8162/", "[::1]:8162"} {
		if !s.IsPanelTarget(target) {
			t.Fatal("panel target not recognized", target)
		}
	}
	for _, target := range []string{"remote.test:8162", "127.0.0.1:8099", "https://localhost:8162", "http://user@localhost:8162", "http://localhost:8162/api"} {
		if s.IsPanelTarget(target) {
			t.Fatal("unrelated target exempted", target)
		}
	}
	s.Add("panel.test", "localhost:8162")
	if err := s.SetRuleMaintenance("panel.test", true); err == nil {
		t.Fatal("panel can lock itself out")
	}
	s.Add("a.test", "localhost:8099")
	rule, _ := s.GetRule("a.test")
	rule.Maintenance = true
	s.All()[0].Maintenance = true
	actual, _ := s.GetRule("a.test")
	if actual.Maintenance {
		t.Fatal("snapshot aliases live state")
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for n := 0; n < 30; n++ {
				s.Get("a.test")
				s.GetRule("a.test")
				s.All()
				s.IsMaintenanceMode()
			}
		}()
	}
	wg.Wait()
}
