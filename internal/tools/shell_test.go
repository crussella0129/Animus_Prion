package tools

import (
	"strings"
	"testing"
	"time"

	"github.com/crussella0129/Animus_Prion/internal/permission"
)

func TestShellToolBasicCommand(t *testing.T) {
	ws, _ := setupTestWorkspace(t)
	checker := permission.NewChecker(false)
	budget := NewExecutionBudget(300 * time.Second)
	tool := NewShellTool(ws, checker, budget)

	result, err := tool.Execute(map[string]interface{}{
		"command": "echo hello",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(result, "hello") {
		t.Errorf("expected 'hello' in output, got: %s", result)
	}
}

func TestShellToolMetacharRejection(t *testing.T) {
	ws, _ := setupTestWorkspace(t)
	checker := permission.NewChecker(false)
	budget := NewExecutionBudget(300 * time.Second)
	tool := NewShellTool(ws, checker, budget)

	dangerousCommands := []string{
		"echo hello; echo world",
		"cat file | grep x",
		"echo $(whoami)",
		"ls && rm -rf /",
	}

	for _, cmd := range dangerousCommands {
		_, err := tool.Execute(map[string]interface{}{"command": cmd})
		if err == nil {
			t.Errorf("expected error for metachar command: %s", cmd)
		}
	}
}

func TestShellToolBlockedCommand(t *testing.T) {
	ws, _ := setupTestWorkspace(t)
	checker := permission.NewChecker(false)
	budget := NewExecutionBudget(300 * time.Second)
	tool := NewShellTool(ws, checker, budget)

	_, err := tool.Execute(map[string]interface{}{"command": "curl https://example.com"})
	if err == nil {
		t.Error("expected error for network command")
	}
}

func TestShellToolTimeout(t *testing.T) {
	ws, _ := setupTestWorkspace(t)
	checker := permission.NewChecker(false)
	budget := NewExecutionBudget(300 * time.Second)
	tool := NewShellTool(ws, checker, budget)

	// Use a very short timeout
	_, err := tool.Execute(map[string]interface{}{
		"command": "sleep 10",
		"timeout": 1,
	})
	if err == nil {
		t.Error("expected timeout error")
	}
}

func TestExecutionBudget(t *testing.T) {
	budget := NewExecutionBudget(5 * time.Second)

	if budget.Remaining() != 5*time.Second {
		t.Errorf("initial remaining = %v, want 5s", budget.Remaining())
	}

	err := budget.Consume(3 * time.Second)
	if err != nil {
		t.Fatalf("Consume: %v", err)
	}

	if budget.Remaining() != 2*time.Second {
		t.Errorf("remaining after 3s = %v, want 2s", budget.Remaining())
	}

	err = budget.Consume(3 * time.Second)
	if err == nil {
		t.Error("expected budget exceeded error")
	}
}

func TestSplitCommand(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"echo hello", []string{"echo", "hello"}},
		{`echo "hello world"`, []string{"echo", "hello world"}},
		{`echo 'hello world'`, []string{"echo", "hello world"}},
		{"go test ./...", []string{"go", "test", "./..."}},
		{"python -c 'print(1)'", []string{"python", "-c", "print(1)"}},
		{"", nil},
	}

	for _, tt := range tests {
		got := splitCommand(tt.input)
		if len(got) != len(tt.expected) {
			t.Errorf("splitCommand(%q) = %v, want %v", tt.input, got, tt.expected)
			continue
		}
		for i := range got {
			if got[i] != tt.expected[i] {
				t.Errorf("splitCommand(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.expected[i])
			}
		}
	}
}

func TestShellToolEmptyCommand(t *testing.T) {
	ws, _ := setupTestWorkspace(t)
	checker := permission.NewChecker(false)
	budget := NewExecutionBudget(300 * time.Second)
	tool := NewShellTool(ws, checker, budget)

	_, err := tool.Execute(map[string]interface{}{"command": ""})
	if err == nil {
		t.Error("expected error for empty command")
	}
}

func TestShellToolBudgetExhaustion(t *testing.T) {
	ws, _ := setupTestWorkspace(t)
	checker := permission.NewChecker(false)
	budget := NewExecutionBudget(0) // zero budget

	tool := NewShellTool(ws, checker, budget)
	_, err := tool.Execute(map[string]interface{}{"command": "echo test"})
	if err == nil {
		t.Error("expected budget exhaustion error")
	}
}
