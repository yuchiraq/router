package storage

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestHealthLifecycle(t *testing.T) {
	var code atomic.Int32
	code.Store(200)
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != "app.test" {
			t.Errorf("health Host: %s", r.Host)
		}
		w.WriteHeader(int(code.Load()))
	}))
	defer backend.Close()
	s := NewRuleStore(NewStorage(filepath.Join(t.TempDir(), "rules.json")))
	s.Add("app.test", backend.URL)
	assertDown := func(want bool) {
		t.Helper()
		rule, _ := s.GetRule("app.test")
		if rule.ServiceDown != want {
			t.Fatalf("down=%v want %v", rule.ServiceDown, want)
		}
	}
	s.CheckServices(context.Background())
	assertDown(false)
	code.Store(503)
	s.CheckServices(context.Background())
	assertDown(false)
	s.CheckServices(context.Background())
	assertDown(true)
	s.SetRuleMaintenance("app.test", true)
	code.Store(401)
	s.CheckServices(context.Background())
	assertDown(true)
	s.CheckServices(context.Background())
	assertDown(false)
	rule, _ := s.GetRule("app.test")
	if !rule.Maintenance || rule.HealthCheckedAt.IsZero() {
		t.Fatal("health cleared manual maintenance")
	}
	persisted := NewRuleStore(NewStorage(s.storage.filePath))
	saved, _ := persisted.GetRule("app.test")
	if saved.ServiceDown || !saved.Maintenance {
		t.Fatal("automatic health persisted or manual state lost")
	}
}

func TestProxyFailureAndStaleProbe(t *testing.T) {
	s := NewRuleStore(NewStorage(filepath.Join(t.TempDir(), "rules.json")))
	s.Add("a.test", "localhost:1234")
	old, _ := s.GetRule("a.test")
	s.ReportServiceFailure("a.test", old.Target)
	s.applyHealth(old, old.healthRevision, nil, time.Now())
	current, _ := s.GetRule("a.test")
	if !current.ServiceDown {
		t.Fatal("stale probe cleared outage")
	}
	s.Add("a.test", "localhost:5678")
	s.applyHealth(old, old.healthRevision, fmt.Errorf("stale"), time.Now())
	s.ReportServiceFailure("a.test", old.Target)
	current, _ = s.GetRule("a.test")
	if current.ServiceDown {
		t.Fatal("old target failure changed new rule")
	}
}

func TestHealthNoRedirectFollowingAndCancellation(t *testing.T) {
	var redirected atomic.Int32
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { redirected.Add(1) }))
	defer destination.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, destination.URL, 302) }))
	defer server.Close()
	if err := probeService(context.Background(), &Rule{Target: server.URL, Host: "login.test"}); err != nil {
		t.Fatal(err)
	}
	if redirected.Load() != 0 {
		t.Fatal("followed redirect")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s := NewRuleStore(NewStorage(filepath.Join(t.TempDir(), "rules.json")))
	s.Add("login.test", server.URL)
	s.CheckServices(ctx)
	rule, _ := s.GetRule("login.test")
	if !rule.HealthCheckedAt.IsZero() {
		t.Fatal("cancelled check changed health")
	}
}

func TestParseHealthTarget(t *testing.T) {
	for _, target := range []string{"localhost:8099", "http://localhost:8099/app", "https://example.test", "http://[::1]:8080"} {
		if _, err := ParseTarget(target); err != nil {
			t.Fatal(target, err)
		}
	}
	for _, target := range []string{"", "file:///etc/passwd", "ftp://host/path", "http://", "http://user:secret@host", "http://host/#fragment"} {
		if _, err := ParseTarget(target); err == nil {
			t.Fatal("invalid target", target)
		}
	}
}
