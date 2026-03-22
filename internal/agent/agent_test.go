package agent

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/crussella0129/Animus_Prion/internal/core"
	"github.com/crussella0129/Animus_Prion/internal/llm"
	"github.com/crussella0129/Animus_Prion/internal/tools"
)

// --- Mock Provider ---

type mockProvider struct {
	responses []string // scripted responses, returned in order
	errors    []error  // if non-nil at index, return error instead
	callCount int
	caps      llm.ModelCapabilities
}

func newMockProvider(responses ...string) *mockProvider {
	return &mockProvider{
		responses: responses,
		errors:    make([]error, len(responses)),
		caps: llm.ModelCapabilities{
			ContextLength: 8192,
			SizeTier:      "medium",
		},
	}
}

func (m *mockProvider) Generate(_ context.Context, _ []llm.Message, _ llm.GenerateOptions) (string, error) {
	if m.callCount >= len(m.responses) {
		return "Done.", nil
	}
	idx := m.callCount
	m.callCount++
	if m.errors[idx] != nil {
		return "", m.errors[idx]
	}
	return m.responses[idx], nil
}

func (m *mockProvider) Available() bool                     { return true }
func (m *mockProvider) Capabilities() llm.ModelCapabilities { return m.caps }

// --- Mock Tool ---

type mockTool struct {
	name   string
	result string
	err    error
	calls  int
}

func (t *mockTool) Name() string        { return t.name }
func (t *mockTool) Description() string { return "mock tool for testing" }
func (t *mockTool) Parameters() tools.ParameterSchema {
	return tools.ParameterSchema{
		Type:       "object",
		Properties: map[string]tools.ParameterSchema{"input": {Type: "string"}},
	}
}
func (t *mockTool) Execute(_ map[string]interface{}) (string, error) {
	t.calls++
	return t.result, t.err
}

// --- Helpers ---

func toolCallJSON(name string, args string) string {
	return fmt.Sprintf(`{"name": "%s", "arguments": {%s}}`, name, args)
}

func newTestAgent(provider llm.Provider, registry *tools.Registry) *Agent {
	return New(Config{
		Provider:      provider,
		Registry:      registry,
		Workspace:     nil, // not needed for these tests
		SystemPrompt:  "You are a test agent.",
		MaxTurns:      10,
		SizeTier:      "medium",
		ContextLength: 8192,
	})
}

// --- Tests ---

func TestRunSimpleResponse(t *testing.T) {
	provider := newMockProvider("Hello, world!")
	registry := tools.NewRegistry()
	ag := newTestAgent(provider, registry)

	result, err := ag.Run(context.Background(), "say hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Hello, world!" {
		t.Errorf("got %q, want %q", result, "Hello, world!")
	}
}

func TestRunToolCallAndResponse(t *testing.T) {
	mt := &mockTool{name: "echo", result: "echoed: test"}
	registry := tools.NewRegistry()
	registry.Register(mt)

	provider := newMockProvider(
		toolCallJSON("echo", `"input": "test"`),
		"The echo tool returned: echoed: test",
	)
	ag := newTestAgent(provider, registry)

	result, err := ag.Run(context.Background(), "echo test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mt.calls != 1 {
		t.Errorf("tool called %d times, want 1", mt.calls)
	}
	if !strings.Contains(result, "echoed: test") {
		t.Errorf("expected result to contain tool output, got %q", result)
	}
}

func TestRepeatDetectionSingleTool(t *testing.T) {
	mt := &mockTool{name: "echo", result: "ok"}
	registry := tools.NewRegistry()
	registry.Register(mt)

	// Same tool call 5 times — should break after 3 successful identical calls
	call := toolCallJSON("echo", `"input": "same"`)
	provider := newMockProvider(call, call, call, call, call)
	ag := newTestAgent(provider, registry)

	_, err := ag.Run(context.Background(), "repeat test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Repeat detection allows 3 identical successful calls (repeatCount hits 2 on the 3rd)
	if mt.calls > 3 {
		t.Errorf("tool called %d times, repeat detection should have stopped at 3", mt.calls)
	}
}

func TestRepeatDetectionMultiTool(t *testing.T) {
	mt1 := &mockTool{name: "read_file", result: "file content"}
	mt2 := &mockTool{name: "write_file", result: "wrote 10 bytes"}
	registry := tools.NewRegistry()
	registry.Register(mt1)
	registry.Register(mt2)

	// Two tool calls per turn, identical across turns
	multiCall := toolCallJSON("read_file", `"input": "a.txt"`) + "\n" +
		toolCallJSON("write_file", `"input": "b.txt"`)
	provider := newMockProvider(multiCall, multiCall, multiCall, multiCall, multiCall)
	ag := newTestAgent(provider, registry)

	_, err := ag.Run(context.Background(), "multi-tool repeat")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should detect multi-tool repeat pattern
	totalCalls := mt1.calls + mt2.calls
	if totalCalls > 6 { // 3 turns × 2 tools
		t.Errorf("total tool calls %d, multi-tool repeat detection should have limited this", totalCalls)
	}
}

func TestRepeatDetectionResetOnDifferentCall(t *testing.T) {
	mt := &mockTool{name: "echo", result: "ok"}
	registry := tools.NewRegistry()
	registry.Register(mt)

	// Different calls should not trigger repeat detection
	provider := newMockProvider(
		toolCallJSON("echo", `"input": "a"`),
		toolCallJSON("echo", `"input": "b"`),
		toolCallJSON("echo", `"input": "c"`),
		toolCallJSON("echo", `"input": "d"`),
		"All done.",
	)
	ag := newTestAgent(provider, registry)

	result, err := ag.Run(context.Background(), "different calls")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mt.calls != 4 {
		t.Errorf("tool called %d times, want 4 (all different, no repeat detection)", mt.calls)
	}
	if result != "All done." {
		t.Errorf("got %q, want %q", result, "All done.")
	}
}

func TestContextCancellation(t *testing.T) {
	provider := newMockProvider("should not be returned")
	registry := tools.NewRegistry()
	ag := newTestAgent(provider, registry)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := ag.Run(ctx, "cancelled task")
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
	if err != context.Canceled {
		t.Errorf("got error %v, want context.Canceled", err)
	}
}

func TestErrorRecoveryRetryable(t *testing.T) {
	provider := newMockProvider("", "Success after retry")
	provider.errors[0] = fmt.Errorf("connection refused")
	registry := tools.NewRegistry()
	ag := newTestAgent(provider, registry)

	result, err := ag.Run(context.Background(), "retry test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Success after retry" {
		t.Errorf("got %q, want %q", result, "Success after retry")
	}
	if provider.callCount != 2 {
		t.Errorf("provider called %d times, want 2 (1 failure + 1 retry)", provider.callCount)
	}
}

func TestErrorRecoveryNonRetryable(t *testing.T) {
	provider := newMockProvider("")
	provider.errors[0] = fmt.Errorf("401 unauthorized invalid api key")
	registry := tools.NewRegistry()
	ag := newTestAgent(provider, registry)

	_, err := ag.Run(context.Background(), "auth failure")
	if err == nil {
		t.Fatal("expected error for non-retryable failure")
	}
	if provider.callCount != 1 {
		t.Errorf("provider called %d times, want 1 (no retry for auth error)", provider.callCount)
	}
}

func TestToolExecutionError(t *testing.T) {
	mt := &mockTool{name: "failing_tool", err: fmt.Errorf("tool broke")}
	registry := tools.NewRegistry()
	registry.Register(mt)

	provider := newMockProvider(
		toolCallJSON("failing_tool", `"input": "x"`),
		"The tool failed, here is my response instead.",
	)
	ag := newTestAgent(provider, registry)

	result, err := ag.Run(context.Background(), "fail test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "response instead") {
		t.Errorf("expected agent to recover with text response, got %q", result)
	}
}

func TestMaxTurnsLimit(t *testing.T) {
	// Agent with only 3 turns, provider always returns tool calls
	mt := &mockTool{name: "echo", result: "ok"}
	registry := tools.NewRegistry()
	registry.Register(mt)

	provider := newMockProvider(
		toolCallJSON("echo", `"input": "1"`),
		toolCallJSON("echo", `"input": "2"`),
		toolCallJSON("echo", `"input": "3"`),
		toolCallJSON("echo", `"input": "4"`),
	)

	ag := New(Config{
		Provider:      provider,
		Registry:      registry,
		SystemPrompt:  "test",
		MaxTurns:      3,
		SizeTier:      "medium",
		ContextLength: 8192,
	})

	_, err := ag.Run(context.Background(), "max turns test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should stop at maxTurns=3, not continue indefinitely
	if provider.callCount > 3 {
		t.Errorf("provider called %d times, should be capped at maxTurns=3", provider.callCount)
	}
}

func TestEvaluateToolResult(t *testing.T) {
	tests := []struct {
		name     string
		toolName string
		result   string
		err      error
		want     string // substring to check
	}{
		{
			name:     "error result",
			toolName: "read_file",
			err:      fmt.Errorf("file not found"),
			want:     "Do NOT retry",
		},
		{
			name:     "empty result",
			toolName: "write_file",
			result:   "",
			want:     "returned empty output",
		},
		{
			name:     "large result",
			toolName: "read_file",
			result:   strings.Repeat("x", 2001),
			want:     "large result",
		},
		{
			name:     "normal result",
			toolName: "echo",
			result:   "hello",
			want:     "", // empty string expected
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evaluateToolResult(tt.toolName, tt.result, tt.err)
			if tt.want == "" {
				if got != "" {
					t.Errorf("evaluateToolResult() = %q, want empty", got)
				}
			} else {
				if !strings.Contains(got, tt.want) {
					t.Errorf("evaluateToolResult() = %q, want substring %q", got, tt.want)
				}
			}
		})
	}
}

func TestContextTrimming(t *testing.T) {
	// Use a very small context window to force trimming
	mt := &mockTool{name: "echo", result: "ok"}
	registry := tools.NewRegistry()
	registry.Register(mt)

	provider := newMockProvider(
		toolCallJSON("echo", `"input": "1"`),
		toolCallJSON("echo", `"input": "2"`),
		toolCallJSON("echo", `"input": "3"`),
		"Final answer.",
	)

	ag := New(Config{
		Provider:      provider,
		Registry:      registry,
		SystemPrompt:  "You are a test agent.",
		MaxTurns:      10,
		SizeTier:      "small", // 30% history ratio = very tight
		ContextLength: 200,     // tiny window to force trimming
	})

	result, err := ag.Run(context.Background(), "trim test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should complete without error despite aggressive trimming
	if result != "Final answer." {
		t.Errorf("got %q, want %q", result, "Final answer.")
	}
	// History should have been trimmed — system prompt is always kept
	if ag.history[0].Role != "system" {
		t.Error("system message should always be first in history")
	}
}

func TestHistoryGrowth(t *testing.T) {
	mt := &mockTool{name: "echo", result: "ok"}
	registry := tools.NewRegistry()
	registry.Register(mt)

	provider := newMockProvider(
		toolCallJSON("echo", `"input": "x"`),
		"Done.",
	)
	ag := newTestAgent(provider, registry)

	_, err := ag.Run(context.Background(), "history test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	history := ag.History()
	// Should contain: system, user, assistant(tool call), tool result, eval msg, assistant(done)
	if len(history) < 5 {
		t.Errorf("history has %d messages, expected at least 5", len(history))
	}
	if history[0].Role != "system" {
		t.Error("first message should be system")
	}
	if history[1].Role != "user" {
		t.Error("second message should be user")
	}
}

func TestReset(t *testing.T) {
	provider := newMockProvider("Hello!")
	registry := tools.NewRegistry()
	ag := newTestAgent(provider, registry)

	ag.Run(context.Background(), "first input")
	if len(ag.History()) <= 1 {
		t.Fatal("history should have grown after Run")
	}

	ag.Reset()
	history := ag.History()
	if len(history) != 1 {
		t.Errorf("after Reset, history has %d messages, want 1 (system only)", len(history))
	}
	if history[0].Role != "system" {
		t.Error("after Reset, first message should be system")
	}
}

func TestTrimMessagesNegativeBudget(t *testing.T) {
	// System prompt that exceeds maxTokens — the bug from review item 2.2
	longSystem := core.NewSystemMessage(strings.Repeat("x", 10000))
	user := core.NewUserMessage("hello")
	assistant := core.NewAssistantMessage("world")

	messages := []core.Message{longSystem, user, assistant}
	// maxTokens = 100, but system prompt alone is ~2500 tokens
	result := core.TrimMessages(messages, 100)

	// Should return system + last message, not panic or return everything
	if len(result) != 2 {
		t.Errorf("TrimMessages with negative budget: got %d messages, want 2 (system + last)", len(result))
	}
	if result[0].Role != "system" {
		t.Error("first message should be system")
	}
	if result[len(result)-1].Role != "assistant" {
		t.Errorf("last message should be assistant, got %q", result[len(result)-1].Role)
	}
}
