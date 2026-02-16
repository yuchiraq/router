package stats

import (
	"errors"
	"testing"

	netutil "github.com/shirou/gopsutil/net"
)

func TestNewInitializesSSHStorage(t *testing.T) {
	s := New()
	if s.ssh == nil {
		t.Fatalf("expected ssh slice to be initialized")
	}
	if len(s.ssh) != 0 {
		t.Fatalf("expected empty ssh slice, got %d", len(s.ssh))
	}
	if s.listConnections == nil {
		t.Fatalf("expected listConnections to be initialized")
	}
}

func TestRecordSSHConnectionsCountsEstablishedOnPort22(t *testing.T) {
	s := New()
	s.listConnections = func(string) ([]netutil.ConnectionStat, error) {
		return []netutil.ConnectionStat{
			{Laddr: netutil.Addr{Port: 22}, Raddr: netutil.Addr{IP: "10.0.0.1"}, Status: "ESTABLISHED"},
			{Laddr: netutil.Addr{Port: 22}, Raddr: netutil.Addr{IP: "10.0.0.1"}, Status: "ESTABLISHED"},
			{Laddr: netutil.Addr{Port: 22}, Raddr: netutil.Addr{IP: "2001:db8::1"}, Status: "ESTABLISHED"},
			{Laddr: netutil.Addr{Port: 22}, Raddr: netutil.Addr{IP: "10.0.0.2"}, Status: "SYN_RECV"},
			{Laddr: netutil.Addr{Port: 443}, Raddr: netutil.Addr{IP: "10.0.0.3"}, Status: "ESTABLISHED"},
		}, nil
	}

	s.RecordSSHConnections()

	data := s.GetSSHData()
	current, ok := data["current"].(int)
	if !ok {
		t.Fatalf("current type mismatch: %T", data["current"])
	}
	if current != 3 {
		t.Fatalf("expected current=3, got %d", current)
	}

	clients, ok := data["clients"].([]map[string]interface{})
	if !ok {
		t.Fatalf("clients type mismatch: %T", data["clients"])
	}
	if len(clients) != 2 {
		t.Fatalf("expected 2 clients, got %d", len(clients))
	}
	if clients[0]["ip"] != "10.0.0.1" || clients[0]["count"] != 2 {
		t.Fatalf("unexpected top client row: %#v", clients[0])
	}
}

func TestRecordSSHConnectionsOnErrorAppendsZeroSample(t *testing.T) {
	s := New()
	s.listConnections = func(string) ([]netutil.ConnectionStat, error) {
		return nil, errors.New("permission denied")
	}

	s.RecordSSHConnections()

	if got := len(s.ssh); got != 1 {
		t.Fatalf("expected one SSH sample after error, got %d", got)
	}
	if s.ssh[0].Established != 0 {
		t.Fatalf("expected zero established after error, got %d", s.ssh[0].Established)
	}
	if len(s.ssh[0].ByRemoteIP) != 0 {
		t.Fatalf("expected no clients after error, got %#v", s.ssh[0].ByRemoteIP)
	}
}

func TestGetSSHDataFromManualSamples(t *testing.T) {
	s := New()
	s.ssh = append(s.ssh,
		SSHConnections{Established: 1, ByRemoteIP: map[string]int{"10.0.0.1": 1}},
		SSHConnections{Established: 3, ByRemoteIP: map[string]int{"192.168.1.5": 2, "10.0.0.1": 1}},
	)

	data := s.GetSSHData()

	labels, ok := data["labels"].([]string)
	if !ok {
		t.Fatalf("labels type mismatch: %T", data["labels"])
	}
	if len(labels) != 2 {
		t.Fatalf("expected 2 labels, got %d", len(labels))
	}

	values, ok := data["values"].([]int)
	if !ok {
		t.Fatalf("values type mismatch: %T", data["values"])
	}
	if len(values) != 2 || values[1] != 3 {
		t.Fatalf("unexpected values: %#v", values)
	}

	current, ok := data["current"].(int)
	if !ok {
		t.Fatalf("current type mismatch: %T", data["current"])
	}
	if current != 3 {
		t.Fatalf("expected current=3, got %d", current)
	}

	clients, ok := data["clients"].([]map[string]interface{})
	if !ok {
		t.Fatalf("clients type mismatch: %T", data["clients"])
	}
	if len(clients) != 2 {
		t.Fatalf("expected 2 clients, got %d", len(clients))
	}
	if clients[0]["ip"] != "192.168.1.5" || clients[0]["count"] != 2 {
		t.Fatalf("unexpected top client row: %#v", clients[0])
	}
}
