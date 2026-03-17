package permission

import (
	"testing"
)

func TestInjectionDetection(t *testing.T) {
	c := NewChecker(false)

	tests := []struct {
		command  string
		expected bool
	}{
		{"ls -la", false},
		{"echo hello", false},
		{"cat file.txt", false},
		{"echo $(whoami)", true},      // $() injection
		{"echo `whoami`", true},       // backtick injection
		{"ls; rm -rf /", true},        // semicolon chaining
		{"ls && rm -rf /", true},      // && chaining
		{"ls || rm -rf /", true},      // || chaining
		{"cat file > /etc/passwd", true}, // redirect
		{"cat file | grep x", true},   // pipe
	}

	for _, tt := range tests {
		got := c.HasInjectionPattern(tt.command)
		if got != tt.expected {
			t.Errorf("HasInjectionPattern(%q) = %v, want %v", tt.command, got, tt.expected)
		}
	}
}

func TestCommandBlocking(t *testing.T) {
	c := NewChecker(false)

	tests := []struct {
		command string
		allowed bool
	}{
		{"ls -la", true},
		{"python script.py", true},
		{"rm -rf /", false},         // blocked
		{"curl https://example.com", false}, // network blocked
		{"wget file", false},        // network blocked
		{"git push origin main", false}, // network blocked
	}

	for _, tt := range tests {
		result := c.CheckCommand(tt.command)
		if result.Allowed != tt.allowed {
			t.Errorf("CheckCommand(%q) = %v, want %v (reason: %s)", tt.command, result.Allowed, tt.allowed, result.Reason)
		}
	}
}

func TestNetworkAllowed(t *testing.T) {
	c := NewChecker(true) // allow network

	result := c.CheckCommand("curl https://example.com")
	if !result.Allowed {
		t.Error("curl should be allowed when network is enabled")
	}
}

func TestDangerousCommand(t *testing.T) {
	c := NewChecker(false)

	if !c.IsDangerous("rm file.txt") {
		t.Error("rm should be flagged as dangerous")
	}
	if !c.IsDangerous("sudo apt install") {
		t.Error("sudo should be flagged as dangerous")
	}
	if c.IsDangerous("ls -la") {
		t.Error("ls should not be flagged as dangerous")
	}
}

func TestPathSafety(t *testing.T) {
	c := NewChecker(false)

	tests := []struct {
		path    string
		allowed bool
	}{
		{"./src/main.go", true},
		{"project/README.md", true},
		{"/etc/passwd", false},
		{"/etc/shadow", false},
	}

	for _, tt := range tests {
		result := c.IsPathSafe(tt.path)
		if result.Allowed != tt.allowed {
			t.Errorf("IsPathSafe(%q) = %v, want %v (reason: %s)", tt.path, result.Allowed, tt.allowed, result.Reason)
		}
	}
}
