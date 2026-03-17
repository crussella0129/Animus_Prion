package core

import (
	"strings"
	"testing"
)

func TestEstimateTokens(t *testing.T) {
	// Code: ~3.1 chars/token
	code := "func main() { fmt.Println(\"hello\") }"
	tokens := EstimateTokens(code, true)
	if tokens == 0 {
		t.Error("expected non-zero tokens for code")
	}

	// Prose: ~4.0 chars/token
	prose := "The quick brown fox jumps over the lazy dog."
	proseTokens := EstimateTokens(prose, false)
	codeTokens := EstimateTokens(prose, true)
	if proseTokens >= codeTokens {
		t.Error("code estimation should produce more tokens than prose for same text")
	}

	// Empty
	if EstimateTokens("", true) != 0 {
		t.Error("empty string should have 0 tokens")
	}
}

func TestComputeBudget(t *testing.T) {
	cw := &ContextWindow{ContextLength: 8192, SizeTier: "medium"}
	budget := cw.ComputeBudget(100)

	if budget.TotalTokens != 8192 {
		t.Errorf("expected 8192 total tokens, got %d", budget.TotalTokens)
	}
	if budget.HistoryTokens != 4096 { // 50% of 8192
		t.Errorf("expected 4096 history tokens, got %d", budget.HistoryTokens)
	}
	if budget.OutputTokens != 2048 { // 25% of 8192
		t.Errorf("expected 2048 output tokens, got %d", budget.OutputTokens)
	}
}

func TestTrimMessages(t *testing.T) {
	// Use long messages to ensure we exceed any reasonable budget
	longContent := strings.Repeat("This is a long message with many words to consume tokens. ", 50)
	messages := []Message{
		NewSystemMessage("System prompt with instructions"),
		NewUserMessage(longContent),
		NewAssistantMessage(longContent),
		NewUserMessage(longContent),
		NewAssistantMessage(longContent),
		NewUserMessage("Final question"),
	}

	// With a budget that can't fit all messages, should trim
	trimmed := TrimMessages(messages, 100)
	if len(trimmed) >= len(messages) {
		t.Errorf("trimmed (%d) should have fewer messages than original (%d)", len(trimmed), len(messages))
	}
	if trimmed[0].Role != "system" {
		t.Error("first message should always be system")
	}
}
