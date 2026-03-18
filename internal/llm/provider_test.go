package llm

import (
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
