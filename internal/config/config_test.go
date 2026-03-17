package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Model.Provider != "openai" {
		t.Errorf("expected default provider 'openai', got '%s'", cfg.Model.Provider)
	}
	if cfg.Agent.MaxTurns != 20 {
		t.Errorf("expected max_turns 20, got %d", cfg.Agent.MaxTurns)
	}
	if cfg.RAG.TopK != 5 {
		t.Errorf("expected top_k 5, got %d", cfg.RAG.TopK)
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cfg := DefaultConfig()
	cfg.Model.ModelName = "test-model"
	cfg.Agent.MaxTurns = 42

	if err := cfg.Save(path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.Model.ModelName != "test-model" {
		t.Errorf("expected model_name 'test-model', got '%s'", loaded.Model.ModelName)
	}
	if loaded.Agent.MaxTurns != 42 {
		t.Errorf("expected max_turns 42, got %d", loaded.Agent.MaxTurns)
	}
}

func TestLoadNonexistent(t *testing.T) {
	cfg, err := Load("/nonexistent/path/config.yaml")
	if err != nil {
		t.Fatalf("Load should not error on missing file: %v", err)
	}
	if cfg.Model.Provider != "openai" {
		t.Errorf("expected default provider on missing file")
	}
}

func TestConfigDir(t *testing.T) {
	// Test default
	dir, err := ConfigDir()
	if err != nil {
		t.Fatalf("ConfigDir failed: %v", err)
	}
	if dir == "" {
		t.Error("ConfigDir returned empty string")
	}

	// Test env override
	os.Setenv("ANIMUS_CONFIG_DIR", "/tmp/test-config")
	defer os.Unsetenv("ANIMUS_CONFIG_DIR")

	dir, err = ConfigDir()
	if err != nil {
		t.Fatalf("ConfigDir with env failed: %v", err)
	}
	if dir != "/tmp/test-config" {
		t.Errorf("expected env override, got '%s'", dir)
	}
}
