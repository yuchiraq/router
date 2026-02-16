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
	if s.sshSessions == nil {
		t.Fatalf("expected sshSessions map to be initialized")
	}
	if s.listConnections == nil {
		t.Fatalf("expected listConnections to be initialized")
	}
}

func TestRecordSSHConnectionsBuildsCurrentSessions(t *testing.T) {
	s := New()
	s.listConnections = func(string) ([]netutil.ConnectionStat, error) {
		return []netutil.ConnectionStat{
			{Laddr: netutil.Addr{Port: 22}, Raddr: netutil.Addr{IP: "10.0.0.1", Port: 50000}, Status: "ESTABLISHED"},
			{Laddr: netutil.Addr{Port: 22}, Raddr: netutil.Addr{IP: "10.0.0.2", Port: 50001}, Status: "ESTABLISHED"},
			{Laddr: netutil.Addr{Port: 22}, Raddr: netutil.Addr{IP: "10.0.0.2", Port: 50002}, Status: "ESTABLISHED"},
			{Laddr: netutil.Addr{Port: 22}, Raddr: netutil.Addr{IP: "10.0.0.3", Port: 50003}, Status: "SYN_RECV"},
			{Laddr: netutil.Addr{Port: 443}, Raddr: netutil.Addr{IP: "10.0.0.4", Port: 50004}, Status: "ESTABLISHED"},
		}, nil
	}

	s.RecordSSHConnections()
	data := s.GetSSHData()

	current, ok := data["current"].(int)
	if !ok || current != 3 {
		t.Fatalf("expected current=3, got %#v", data["current"])
	}

	sessions, ok := data["sessions"].([]map[string]interface{})
	if !ok {
		t.Fatalf("sessions type mismatch: %T", data["sessions"])
	}
	if len(sessions) != 3 {
		t.Fatalf("expected 3 sessions, got %d", len(sessions))
	}

	if sessions[0]["ip"] == "" || sessions[0]["date"] == "" || sessions[0]["time"] == "" {
		t.Fatalf("expected non-empty session metadata: %#v", sessions[0])
	}
}

func TestRecordSSHConnectionsOnErrorClearsCurrentSessions(t *testing.T) {
	s := New()
	s.sshSessions["10.0.0.1:50000"] = sshSessionState{RemoteIP: "10.0.0.1", RemotePort: 50000}
	s.listConnections = func(string) ([]netutil.ConnectionStat, error) {
		return nil, errors.New("permission denied")
	}

	s.RecordSSHConnections()
	data := s.GetSSHData()
	current, ok := data["current"].(int)
	if !ok || current != 0 {
		t.Fatalf("expected current=0 after error, got %#v", data["current"])
	}
}
