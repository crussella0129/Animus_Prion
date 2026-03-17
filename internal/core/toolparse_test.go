package core

import (
	"testing"
)

func TestParseRawJSON(t *testing.T) {
	input := `{"name": "read_file", "arguments": {"path": "main.go"}}`
	calls := ParseToolCalls(input)
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Name != "read_file" {
		t.Errorf("expected name 'read_file', got '%s'", calls[0].Name)
	}
	if calls[0].Arguments["path"] != "main.go" {
		t.Errorf("expected path 'main.go', got '%v'", calls[0].Arguments["path"])
	}
}

func TestParseJSONCodeBlock(t *testing.T) {
	input := "Let me read the file.\n```json\n{\"name\": \"read_file\", \"arguments\": {\"path\": \"test.py\"}}\n```"
	calls := ParseToolCalls(input)
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Name != "read_file" {
		t.Errorf("expected name 'read_file', got '%s'", calls[0].Name)
	}
}

func TestParseInlineJSON(t *testing.T) {
	input := `I'll run the command now: {"name": "run_shell", "arguments": {"command": "go test"}}`
	calls := ParseToolCalls(input)
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Name != "run_shell" {
		t.Errorf("expected name 'run_shell', got '%s'", calls[0].Name)
	}
}

func TestParseNoToolCalls(t *testing.T) {
	input := "Here is the answer: the sky is blue."
	calls := ParseToolCalls(input)
	if len(calls) != 0 {
		t.Errorf("expected 0 calls, got %d", len(calls))
	}
}

func TestParseEmptyInput(t *testing.T) {
	calls := ParseToolCalls("")
	if calls != nil {
		t.Errorf("expected nil for empty input, got %v", calls)
	}
}

func TestDeduplicateToolCalls(t *testing.T) {
	calls := []ToolCall{
		{Name: "read_file", Arguments: map[string]interface{}{"path": "a.txt"}},
		{Name: "read_file", Arguments: map[string]interface{}{"path": "a.txt"}},
		{Name: "read_file", Arguments: map[string]interface{}{"path": "b.txt"}},
	}
	unique := DeduplicateToolCalls(calls)
	if len(unique) != 2 {
		t.Errorf("expected 2 unique calls, got %d", len(unique))
	}
}
