package tools

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/crussella0129/Animus_Prion/internal/core"
	"github.com/crussella0129/Animus_Prion/internal/permission"
)

// ExecutionBudget tracks cumulative execution time for safety.
type ExecutionBudget struct {
	mu       sync.Mutex
	limit    time.Duration
	consumed time.Duration
}

// NewExecutionBudget creates a budget with the given limit.
func NewExecutionBudget(limit time.Duration) *ExecutionBudget {
	return &ExecutionBudget{limit: limit}
}

// Remaining returns how much budget is left.
func (b *ExecutionBudget) Remaining() time.Duration {
	b.mu.Lock()
	defer b.mu.Unlock()
	r := b.limit - b.consumed
	if r < 0 {
		return 0
	}
	return r
}

// Consume records elapsed execution time. Returns error if budget exceeded.
func (b *ExecutionBudget) Consume(d time.Duration) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.consumed += d
	if b.consumed > b.limit {
		return fmt.Errorf("execution budget exceeded: %v of %v consumed", b.consumed, b.limit)
	}
	return nil
}

// ShellTool executes shell commands via list-based subprocess (no shell=true).
type ShellTool struct {
	workspace *core.Workspace
	checker   *permission.Checker
	budget    *ExecutionBudget
}

// NewShellTool creates a shell tool with workspace boundary and permission enforcement.
func NewShellTool(ws *core.Workspace, checker *permission.Checker, budget *ExecutionBudget) *ShellTool {
	return &ShellTool{
		workspace: ws,
		checker:   checker,
		budget:    budget,
	}
}

func (t *ShellTool) Name() string { return "run_shell" }

func (t *ShellTool) Description() string {
	return "Execute a shell command. Commands are run as list-based subprocesses (no shell interpretation). Metacharacters (|, &&, ;, redirects) are rejected."
}

func (t *ShellTool) Parameters() ParameterSchema {
	return ParameterSchema{
		Type: "object",
		Properties: map[string]ParameterSchema{
			"command": {Type: "string", Description: "The command to execute (e.g. 'python script.py')"},
			"timeout": {Type: "integer", Description: "Timeout in seconds (default: 30)"},
		},
		Required: []string{"command"},
	}
}

func (t *ShellTool) Execute(args map[string]interface{}) (string, error) {
	// Robust command extraction — LLMs sometimes pass non-string types
	var command string
	switch v := args["command"].(type) {
	case string:
		command = v
	case []interface{}:
		// Model passed an array like ["cargo", "build"] — join it
		parts := make([]string, len(v))
		for i, p := range v {
			parts[i] = fmt.Sprintf("%v", p)
		}
		command = strings.Join(parts, " ")
	case map[string]interface{}:
		// Model passed an object — try to extract a "command" or "cmd" field
		if cmd, ok := v["command"].(string); ok {
			command = cmd
		} else if cmd, ok := v["cmd"].(string); ok {
			command = cmd
		} else {
			return "", fmt.Errorf("command must be a string, got nested object: %v", v)
		}
	default:
		if args["command"] != nil {
			command = fmt.Sprintf("%v", args["command"])
		} else {
			return "", fmt.Errorf("command must be a string, got %T", args["command"])
		}
	}

	timeout := 30
	if v, ok := args["timeout"]; ok {
		switch tv := v.(type) {
		case int:
			timeout = tv
		case float64:
			timeout = int(tv)
		}
	}

	// Security: check for metacharacters
	if t.checker.HasMetachars(command) {
		return "", fmt.Errorf("command contains shell metacharacters — use separate commands instead")
	}

	// Security: check command against deny lists
	result := t.checker.CheckCommand(command)
	if !result.Allowed {
		return "", fmt.Errorf("command blocked: %s", result.Reason)
	}

	// Check execution budget
	if t.budget.Remaining() <= 0 {
		return "", fmt.Errorf("execution budget exhausted")
	}

	// Parse command into args list (no shell interpretation)
	cmdArgs := splitCommand(command)
	if len(cmdArgs) == 0 {
		return "", fmt.Errorf("empty command")
	}

	// Windows: fix quoting (single → double quotes for paths)
	if runtime.GOOS == "windows" {
		for i, arg := range cmdArgs {
			cmdArgs[i] = strings.ReplaceAll(arg, "'", "\"")
		}
	}

	// Execute with context timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, cmdArgs[0], cmdArgs[1:]...)
	cmd.Dir = t.workspace.CWD()

	start := time.Now()
	output, err := cmd.CombinedOutput()
	elapsed := time.Since(start)

	// Consume budget
	if budgetErr := t.budget.Consume(elapsed); budgetErr != nil {
		return string(output), budgetErr
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return string(output), fmt.Errorf("command timed out after %ds", timeout)
		}
		return string(output), fmt.Errorf("command failed: %w\nOutput: %s", err, string(output))
	}

	return string(output), nil
}

// splitCommand splits a command string into arguments, respecting quoted strings.
// This is a simple parser — not a full shell parser (by design, for security).
func splitCommand(command string) []string {
	var args []string
	var current strings.Builder
	inQuote := false
	quoteChar := byte(0)

	for i := 0; i < len(command); i++ {
		ch := command[i]
		switch {
		case inQuote:
			if ch == quoteChar {
				inQuote = false
			} else {
				current.WriteByte(ch)
			}
		case ch == '"' || ch == '\'':
			inQuote = true
			quoteChar = ch
		case ch == ' ' || ch == '\t':
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
		default:
			current.WriteByte(ch)
		}
	}
	if current.Len() > 0 {
		args = append(args, current.String())
	}
	return args
}
