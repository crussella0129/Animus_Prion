package tools

import (
	"testing"

	"github.com/crussella0129/Animus_Prion/internal/core"
)

func TestValidateGitArgsBlocked(t *testing.T) {
	blocked := [][]string{
		{"push", "origin", "main"},
		{"pull"},
		{"clone", "https://example.com/repo"},
		{"fetch", "--all"},
		{"remote", "add", "origin", "url"},
		{"clean", "-fd"},
	}

	for _, args := range blocked {
		err := validateGitArgs(args)
		if err == nil {
			t.Errorf("expected git %v to be blocked", args)
		}
	}
}

func TestValidateGitArgsAllowed(t *testing.T) {
	allowed := [][]string{
		{"init"},
		{"status", "--porcelain"},
		{"diff"},
		{"diff", "--cached"},
		{"log", "--oneline", "-n10"},
		{"add", "file.go"},
		{"add", "src/main.go", "src/lib.go"},
		{"commit", "-m", "feat: add feature"},
		{"branch", "new-feature"},
		{"branch", "--list"},
		{"checkout", "main"},
	}

	for _, args := range allowed {
		err := validateGitArgs(args)
		if err != nil {
			t.Errorf("expected git %v to be allowed, got: %v", args, err)
		}
	}
}

func TestValidateGitArgsDangerous(t *testing.T) {
	dangerous := []struct {
		args []string
		desc string
	}{
		{[]string{"add", "-A"}, "git add -A too broad"},
		{[]string{"add", "--all"}, "git add --all too broad"},
		{[]string{"add", "."}, "git add . too broad"},
		{[]string{"reset", "--hard"}, "git reset --hard destructive"},
		{[]string{"checkout", "--", "."}, "git checkout -- . discards changes"},
		{[]string{"commit", "--no-verify", "-m", "skip hooks"}, "no-verify blocked"},
	}

	for _, tt := range dangerous {
		err := validateGitArgs(tt.args)
		if err == nil {
			t.Errorf("expected git %v to be blocked (%s)", tt.args, tt.desc)
		}
	}
}

func TestValidateGitArgsEmpty(t *testing.T) {
	err := validateGitArgs(nil)
	if err == nil {
		t.Error("expected error for empty git args")
	}
}

func TestGitAddToolPathTraversal(t *testing.T) {
	dir := t.TempDir()
	ws, err := core.NewWorkspace(dir)
	if err != nil {
		t.Fatalf("NewWorkspace: %v", err)
	}

	tool := NewGitAddTool(ws)

	// Path traversal should be blocked by workspace boundary
	_, err = tool.Execute(map[string]interface{}{
		"files": "../../etc/shadow",
	})
	if err == nil {
		t.Fatal("SECURITY: git add with path traversal should be blocked")
	}

	// Absolute path outside workspace should be blocked
	_, err = tool.Execute(map[string]interface{}{
		"files": "/etc/passwd",
	})
	if err == nil {
		t.Fatal("SECURITY: git add with absolute path outside workspace should be blocked")
	}
}
