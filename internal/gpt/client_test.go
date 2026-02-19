package gpt

import (
	"path/filepath"
	"testing"

	"router/internal/storage"
)

func TestIsAllowedChat(t *testing.T) {
	store := storage.NewGPTStore(filepath.Join(t.TempDir(), "gpt.json"))
	store.Update(storage.GPTConfig{Enabled: true, OnlyChatIDs: []int64{10, 20}})
	client := NewClient(store)
	if !client.IsAllowedChat(10) {
		t.Fatalf("chat 10 should be allowed")
	}
	if client.IsAllowedChat(30) {
		t.Fatalf("chat 30 should not be allowed")
	}
}

func TestReplyWhenDisabled(t *testing.T) {
	store := storage.NewGPTStore(filepath.Join(t.TempDir(), "gpt.json"))
	store.Update(storage.GPTConfig{Enabled: false})
	client := NewClient(store)
	resp, err := client.Reply(1, "hello")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if resp == "" {
		t.Fatalf("expected explanatory response")
	}
}
