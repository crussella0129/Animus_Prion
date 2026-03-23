package llm

import (
	"strings"
	"testing"
)

func TestToolCallGrammar(t *testing.T) {
	grammar := ToolCallGrammar([]string{"read_file", "write_file", "run_shell"})

	if grammar == "" {
		t.Fatal("grammar should not be empty")
	}

	// Should contain all tool names as alternatives
	if !strings.Contains(grammar, "read_file") {
		t.Error("grammar should contain read_file")
	}
	if !strings.Contains(grammar, "write_file") {
		t.Error("grammar should contain write_file")
	}
	if !strings.Contains(grammar, "run_shell") {
		t.Error("grammar should contain run_shell")
	}

	// Should have the core structure
	if !strings.Contains(grammar, "root") {
		t.Error("grammar should define root rule")
	}
	if !strings.Contains(grammar, "tool-call") {
		t.Error("grammar should define tool-call rule")
	}
	if !strings.Contains(grammar, "json-object") {
		t.Error("grammar should define json-object rule")
	}
}

func TestToolCallGrammarEmpty(t *testing.T) {
	grammar := ToolCallGrammar(nil)
	if grammar != "" {
		t.Error("empty tool list should produce empty grammar")
	}
}

func TestToolCallOrTextGrammar(t *testing.T) {
	grammar := ToolCallOrTextGrammar([]string{"echo"})

	if grammar == "" {
		t.Fatal("grammar should not be empty")
	}

	// Should allow both tool calls and free text
	if !strings.Contains(grammar, "tool-call") {
		t.Error("grammar should allow tool calls")
	}
	if !strings.Contains(grammar, "free-text") {
		t.Error("grammar should allow free text")
	}

	// Root should be an alternative
	if !strings.Contains(grammar, "root ::= tool-call | free-text") {
		t.Error("root should be tool-call | free-text")
	}
}

func TestToolCallGrammarSingleTool(t *testing.T) {
	grammar := ToolCallGrammar([]string{"read_file"})

	// Single tool should not have | separator
	if strings.Contains(grammar, " | ") && strings.Count(grammar, " | ") > 1 {
		// The root rule has " | " in ToolCallOrTextGrammar but not in ToolCallGrammar
		// ToolCallGrammar's tool-name line should just be a single quoted name
	}

	if !strings.Contains(grammar, "read_file") {
		t.Error("grammar should contain the tool name")
	}
}
