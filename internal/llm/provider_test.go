package llm

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSizeTierFromParams(t *testing.T) {
	tests := []struct {
		billions float64
		expected string
	}{
		{0.5, "small"},
		{1.0, "small"},
		{3.9, "small"},
		{4.0, "medium"},
		{7.0, "medium"},
		{12.9, "medium"},
		{13.0, "large"},
		{70.0, "large"},
	}

	for _, tt := range tests {
		got := SizeTierFromParams(tt.billions)
		if got != tt.expected {
			t.Errorf("SizeTierFromParams(%v) = %q, want %q", tt.billions, got, tt.expected)
		}
	}
}

func TestLocalProviderAvailableNoServer(t *testing.T) {
	p := NewLocalProvider(LocalProviderConfig{
		BaseURL: "http://127.0.0.1:19999/v1", // nothing running here
	})
	if p.Available() {
		t.Error("should not be available when server is unreachable")
	}
}

func TestLocalProviderCapabilities(t *testing.T) {
	p := NewLocalProvider(LocalProviderConfig{
		ContextLength: 8192,
		SizeTier:      "large",
	})
	caps := p.Capabilities()
	if caps.ContextLength != 8192 {
		t.Errorf("context_length = %d, want 8192", caps.ContextLength)
	}
	if caps.SizeTier != "large" {
		t.Errorf("size_tier = %q, want 'large'", caps.SizeTier)
	}
}

func TestFindModelNotFound(t *testing.T) {
	_, err := FindModel("nonexistent-model-xyz.gguf")
	if err == nil {
		t.Error("expected error for nonexistent model")
	}
}

func TestConvertToAnthropicTools(t *testing.T) {
	// Simulate OpenAI-format tool schemas (as []any)
	openaiTools := []any{
		map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "read_file",
				"description": "Read a file",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]interface{}{"type": "string"},
					},
					"required": []string{"path"},
				},
			},
		},
	}

	result := convertToAnthropicTools(openaiTools)
	if len(result) != 1 {
		t.Fatalf("got %d tools, want 1", len(result))
	}
	if result[0].Name != "read_file" {
		t.Errorf("name = %q, want %q", result[0].Name, "read_file")
	}
	if result[0].Description != "Read a file" {
		t.Errorf("description = %q, want %q", result[0].Description, "Read a file")
	}
	// InputSchema should contain the parameters
	raw, _ := json.Marshal(result[0].InputSchema)
	if !strings.Contains(string(raw), `"path"`) {
		t.Errorf("input_schema missing 'path' property: %s", raw)
	}
}

func TestFormatAnthropicResponseTextOnly(t *testing.T) {
	blocks := []anthropicContentBlock{
		{Type: "text", Text: "Hello, world!"},
	}
	got := formatAnthropicResponse(blocks)
	if got != "Hello, world!" {
		t.Errorf("got %q, want %q", got, "Hello, world!")
	}
}

func TestFormatAnthropicResponseToolUse(t *testing.T) {
	blocks := []anthropicContentBlock{
		{Type: "text", Text: "Let me read that file."},
		{Type: "tool_use", ID: "toolu_123", Name: "read_file", Input: map[string]interface{}{"path": "main.go"}},
	}
	got := formatAnthropicResponse(blocks)
	if !strings.Contains(got, "Let me read that file.") {
		t.Error("missing text block in output")
	}
	if !strings.Contains(got, `"name":"read_file"`) {
		t.Error("missing tool name in output")
	}
	if !strings.Contains(got, `"path":"main.go"`) {
		t.Error("missing tool arguments in output")
	}
}

func TestFormatAnthropicResponseToolUseOnly(t *testing.T) {
	blocks := []anthropicContentBlock{
		{Type: "tool_use", ID: "toolu_456", Name: "write_file", Input: map[string]interface{}{
			"path":    "test.py",
			"content": "print('hello')",
		}},
	}
	got := formatAnthropicResponse(blocks)
	// Should be parseable as a tool call by core.ParseToolCalls
	if !strings.Contains(got, `"name":"write_file"`) {
		t.Error("missing tool name in output")
	}
	if !strings.Contains(got, `"arguments"`) {
		t.Error("missing arguments key in output")
	}
}

func TestAnthropicProviderCapabilities(t *testing.T) {
	p := NewAnthropicProvider(AnthropicConfig{
		APIKey: "test-key",
	})
	caps := p.Capabilities()
	if !caps.SupportsTools {
		t.Error("Anthropic should support tools")
	}
	if caps.SizeTier != "large" {
		t.Errorf("size_tier = %q, want 'large'", caps.SizeTier)
	}
	if !p.Available() {
		t.Error("should be available with API key set")
	}
}
