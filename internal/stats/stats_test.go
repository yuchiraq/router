package stats

import "testing"

func TestNewInitializesSSHStorage(t *testing.T) {
	s := New()
	if s.ssh == nil {
		t.Fatalf("expected ssh slice to be initialized")
	}
	if len(s.ssh) != 0 {
		t.Fatalf("expected empty ssh slice, got %d", len(s.ssh))
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
