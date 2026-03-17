// Package config provides YAML-based configuration for Animus Prion.
// Mirrors Animus's AnimusConfig with sub-configs for model, agent, RAG, and paths.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"gopkg.in/yaml.v3"
)

// ModelConfig holds LLM provider settings.
type ModelConfig struct {
	Provider      string  `yaml:"provider"`       // "openai", "anthropic", "native"
	ModelName     string  `yaml:"model_name"`     // e.g. "gpt-4", "claude-sonnet-4-20250514", "qwen2.5-coder-7b"
	Temperature   float64 `yaml:"temperature"`    // 0.0 - 2.0
	ContextLength int     `yaml:"context_length"` // max tokens
	GPULayers     int     `yaml:"gpu_layers"`     // for native provider
	SizeTier      string  `yaml:"size_tier"`      // "small", "medium", "large"
	BaseURL       string  `yaml:"base_url"`       // custom API endpoint
	APIKey        string  `yaml:"api_key"`        // API key (env var preferred)
}

// AgentConfig holds agent behavior settings.
type AgentConfig struct {
	MaxTurns         int    `yaml:"max_turns"`
	SystemPrompt     string `yaml:"system_prompt"`
	ConfirmDangerous bool   `yaml:"confirm_dangerous"`
	WorkspaceRoot    string `yaml:"workspace_root"`
}

// RAGConfig holds retrieval-augmented generation settings.
type RAGConfig struct {
	ChunkSize      int    `yaml:"chunk_size"`
	EmbeddingModel string `yaml:"embedding_model"`
	TopK           int    `yaml:"top_k"`
}

// Config is the top-level configuration structure.
type Config struct {
	Model ModelConfig `yaml:"model"`
	Agent AgentConfig `yaml:"agent"`
	RAG   RAGConfig   `yaml:"rag"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Model: ModelConfig{
			Provider:      "local",
			ModelName:     "qwen2.5-coder-7b",
			Temperature:   0.7,
			ContextLength: 4096,
			GPULayers:     -1,
			SizeTier:      "medium",
		},
		Agent: AgentConfig{
			MaxTurns:         20,
			ConfirmDangerous: true,
		},
		RAG: RAGConfig{
			ChunkSize:      512,
			EmbeddingModel: "all-MiniLM-L6-v2",
			TopK:           5,
		},
	}
}

// ConfigDir returns the configuration directory path.
// Respects ANIMUS_CONFIG_DIR env var, falls back to ~/.animus_prion/
func ConfigDir() (string, error) {
	if dir := os.Getenv("ANIMUS_CONFIG_DIR"); dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".animus_prion"), nil
}

// Load reads configuration from the given YAML file path.
// Returns DefaultConfig if the file does not exist.
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	return cfg, nil
}

// Save writes the configuration to the given YAML file path.
// Sets file permissions to 0600 on Unix systems.
func (c *Config) Save(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	// Enforce restrictive permissions on Unix
	if runtime.GOOS != "windows" {
		if err := os.Chmod(path, 0600); err != nil {
			return fmt.Errorf("setting config permissions: %w", err)
		}
	}

	return nil
}
