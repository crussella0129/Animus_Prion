package planner

import (
	"testing"
)

func TestDetectVerifyCommand(t *testing.T) {
	tests := []struct {
		name     string
		steps    []Step
		expected string
	}{
		{
			name: "rust project",
			steps: []Step{
				{Description: "Create Cargo.toml and src/lib.rs", Type: StepWrite},
			},
			expected: "cargo build",
		},
		{
			name: "python project",
			steps: []Step{
				{Description: "Write main.py with the solution", Type: StepWrite},
			},
			expected: "python -m py_compile main.py",
		},
		{
			name: "go project",
			steps: []Step{
				{Description: "Create main.go with the handler", Type: StepWrite},
			},
			expected: "go build ./...",
		},
		{
			name: "rust and python",
			steps: []Step{
				{Description: "Create Cargo.toml and src/lib.rs", Type: StepWrite},
				{Description: "Write main.py to call the Rust library", Type: StepWrite},
			},
			expected: "cargo build",
		},
		{
			name: "no code written",
			steps: []Step{
				{Description: "Read the config file", Type: StepRead},
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectVerifyCommand(tt.steps, ".")
			if got != tt.expected {
				t.Errorf("detectVerifyCommand() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestContainsError(t *testing.T) {
	tests := []struct {
		output   string
		expected bool
	}{
		{"Compiling foo v0.1.0\nFinished dev target", false},
		{"error[E0433]: failed to resolve", true},
		{"Traceback (most recent call last):", true},
		{"SyntaxError: invalid syntax", true},
		{"could not compile `foo`", true},
		{"Build successful", false},
		{"", false},
		{"warning: unused variable", false},
	}

	for _, tt := range tests {
		got := containsError(tt.output)
		if got != tt.expected {
			t.Errorf("containsError(%q) = %v, want %v", tt.output[:min(len(tt.output), 40)], got, tt.expected)
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
