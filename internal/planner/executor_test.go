package planner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectVerifyCommand(t *testing.T) {
	tests := []struct {
		name     string
		steps    []Step
		expected string
	}{
		{
			name: "rust project from steps",
			steps: []Step{
				{Description: "Create Cargo.toml and src/lib.rs", Type: StepWrite},
			},
			expected: "cargo build",
		},
		{
			name: "python project from steps",
			steps: []Step{
				{Description: "Write main.py with the solution", Type: StepWrite},
			},
			expected: "python -m py_compile main.py",
		},
		{
			name: "go project from steps",
			steps: []Step{
				{Description: "Create main.go with the handler", Type: StepWrite},
			},
			expected: "go build ./...",
		},
		{
			name: "rust and python from steps",
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

func TestDetectVerifyCommandFilesystem(t *testing.T) {
	// Test filesystem probing — no step descriptions, just files on disk
	dir := t.TempDir()

	// Empty dir → no verify command
	got := detectVerifyCommand(nil, dir)
	if got != "" {
		t.Errorf("empty dir: got %q, want empty", got)
	}

	// Create Cargo.toml → should detect rust
	os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte("[package]\nname = \"test\""), 0644)
	got = detectVerifyCommand(nil, dir)
	if got != "cargo build" {
		t.Errorf("with Cargo.toml: got %q, want 'cargo build'", got)
	}

	// Also add a .py file → still cargo build (rust takes priority)
	os.WriteFile(filepath.Join(dir, "main.py"), []byte("print('hi')"), 0644)
	got = detectVerifyCommand(nil, dir)
	if got != "cargo build" {
		t.Errorf("with Cargo.toml + main.py: got %q, want 'cargo build'", got)
	}

	// Python only — should find the actual .py file
	dir2 := t.TempDir()
	os.WriteFile(filepath.Join(dir2, "app.py"), []byte("pass"), 0644)
	got = detectVerifyCommand(nil, dir2)
	if got != "python -m py_compile app.py" {
		t.Errorf("with app.py: got %q, want 'python -m py_compile app.py'", got)
	}

	// Go only
	dir3 := t.TempDir()
	os.WriteFile(filepath.Join(dir3, "go.mod"), []byte("module test"), 0644)
	got = detectVerifyCommand(nil, dir3)
	if got != "go build ./..." {
		t.Errorf("with go.mod: got %q, want 'go build ./...'", got)
	}
}

func TestContainsError(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected bool
	}{
		{"clean compile", "Compiling foo v0.1.0\nFinished dev target", false},
		{"rust error code", "error[E0433]: failed to resolve", true},
		{"python traceback", "Traceback (most recent call last):", true},
		{"syntax error", "SyntaxError: invalid syntax", true},
		{"compile failure", "could not compile `foo`", true},
		{"success", "Build successful", false},
		{"empty", "", false},
		{"warning only", "warning: unused variable", false},
		{"zero errors", "Build complete. 0 errors, 2 warnings.", false},
		{"one error", "Build complete. 1 error generated.", true},
		{"code reference", "error_handler.go:15: func handleError()", false}, // false positive fix
		{"variable name", "var errorCount = 0", false},                       // false positive fix
		{"error line start", "error: cannot find module", true},
		{"no such file", "no such file or directory: main.py", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containsError(tt.output)
			if got != tt.expected {
				t.Errorf("containsError(%q) = %v, want %v", tt.output[:min(len(tt.output), 50)], got, tt.expected)
			}
		})
	}
}

func TestExtractExpectedFiles(t *testing.T) {
	tests := []struct {
		task     string
		expected []string
	}{
		{
			task:     "Create Cargo.toml with cdylib crate-type and src/lib.rs that exposes functions. Also create main.py.",
			expected: []string{"Cargo.toml", "src/lib.rs", "main.py"},
		},
		{
			task:     "Write a Python script called app.py",
			expected: []string{"app.py"},
		},
		{
			task:     "Build a Rust library with cdylib",
			expected: []string{"Cargo.toml"},
		},
		{
			task:     "Explain what fibonacci numbers are",
			expected: nil, // no file expectations
		},
		{
			task:     "Create handler.go and handler_test.go",
			expected: []string{"handler.go", "handler_test.go"},
		},
	}

	for _, tt := range tests {
		got := extractExpectedFiles(tt.task)
		if len(got) != len(tt.expected) {
			t.Errorf("extractExpectedFiles(%q) = %v (len %d), want %v (len %d)",
				tt.task[:min(len(tt.task), 50)], got, len(got), tt.expected, len(tt.expected))
			continue
		}
		for i := range got {
			if got[i] != tt.expected[i] {
				t.Errorf("extractExpectedFiles()[%d] = %q, want %q", i, got[i], tt.expected[i])
			}
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
