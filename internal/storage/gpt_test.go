package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGPTStoreFailedSaveKeepsPreviousConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gpt.json")
	s := NewGPTStore(path)
	if err := s.Update(GPTConfig{APIKey: "original"}); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	if err := s.Update(GPTConfig{APIKey: "changed"}); err == nil {
		t.Fatal("save must fail")
	}
	if s.Get().APIKey != "original" {
		t.Fatal("failed write changed live configuration")
	}
}

func TestGPTStoreDefaultsAndPersist(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gpt.json")

	store := NewGPTStore(path)
	defaults := store.Get()
	if defaults.Model != "gpt-4o-mini" || defaults.MaxLogLines != 20 || defaults.OnlyChatIDs == nil {
		t.Fatalf("unexpected defaults: %+v", defaults)
	}

	store.Update(GPTConfig{
		Enabled:      true,
		APIKey:       "key",
		Model:        "gpt-4.1-mini",
		SystemPrompt: "prompt",
		MaxLogLines:  42,
		OnlyChatIDs:  []int64{-1001, 222},
	})

	reloaded := NewGPTStore(path)
	cfg := reloaded.Get()
	if !cfg.Enabled || cfg.APIKey != "key" || cfg.Model != "gpt-4.1-mini" || cfg.SystemPrompt != "prompt" || cfg.MaxLogLines != 42 {
		t.Fatalf("unexpected config after reload: %+v", cfg)
	}
	if len(cfg.OnlyChatIDs) != 2 || cfg.OnlyChatIDs[0] != -1001 || cfg.OnlyChatIDs[1] != 222 {
		t.Fatalf("unexpected only chat ids: %+v", cfg.OnlyChatIDs)
	}
}

func TestGPTStoreGetReturnsCopy(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gpt.json")

	store := NewGPTStore(path)
	store.Update(GPTConfig{OnlyChatIDs: []int64{1, 2, 3}})

	cfg := store.Get()
	cfg.OnlyChatIDs[0] = 999

	reloaded := store.Get()
	if reloaded.OnlyChatIDs[0] != 1 {
		t.Fatalf("expected internal slice to remain unchanged, got: %+v", reloaded.OnlyChatIDs)
	}
}
