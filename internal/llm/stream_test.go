package llm

import (
	"strings"
	"testing"
)

func TestParseOpenAISSE(t *testing.T) {
	// Simulate an OpenAI-compatible SSE stream
	stream := strings.NewReader(
		"data: {\"choices\":[{\"delta\":{\"content\":\"Hello\"}}]}\n" +
			"\n" +
			"data: {\"choices\":[{\"delta\":{\"content\":\" world\"}}]}\n" +
			"\n" +
			"data: {\"choices\":[{\"delta\":{\"content\":\"!\"}}]}\n" +
			"\n" +
			"data: [DONE]\n",
	)

	var chunks []string
	result, err := parseOpenAISSE(stream, func(chunk string) {
		chunks = append(chunks, chunk)
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Hello world!" {
		t.Errorf("result = %q, want %q", result, "Hello world!")
	}
	if len(chunks) != 3 {
		t.Errorf("got %d chunks, want 3", len(chunks))
	}
}

func TestParseOpenAISSEEmptyDeltas(t *testing.T) {
	stream := strings.NewReader(
		"data: {\"choices\":[{\"delta\":{}}]}\n" +
			"\n" +
			"data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n" +
			"\n" +
			"data: [DONE]\n",
	)

	result, err := parseOpenAISSE(stream, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "ok" {
		t.Errorf("result = %q, want %q", result, "ok")
	}
}

func TestParseOpenAISSENilCallback(t *testing.T) {
	stream := strings.NewReader(
		"data: {\"choices\":[{\"delta\":{\"content\":\"test\"}}]}\n" +
			"data: [DONE]\n",
	)

	result, err := parseOpenAISSE(stream, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "test" {
		t.Errorf("result = %q, want %q", result, "test")
	}
}

func TestParseAnthropicSSE(t *testing.T) {
	stream := strings.NewReader(
		"event: content_block_delta\n" +
			"data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Hello\"}}\n" +
			"\n" +
			"event: content_block_delta\n" +
			"data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\" there\"}}\n" +
			"\n" +
			"event: message_stop\n" +
			"data: {}\n",
	)

	var chunks []string
	result, err := parseAnthropicSSE(stream, func(chunk string) {
		chunks = append(chunks, chunk)
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Hello there" {
		t.Errorf("result = %q, want %q", result, "Hello there")
	}
	if len(chunks) != 2 {
		t.Errorf("got %d chunks, want 2", len(chunks))
	}
}

func TestParseAnthropicSSEToolUse(t *testing.T) {
	// Small input: arrives fully in content_block_start, finalized at content_block_stop
	stream := strings.NewReader(
		"event: content_block_start\n" +
			"data: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"tool_use\",\"id\":\"toolu_1\",\"name\":\"read_file\",\"input\":{\"path\":\"main.go\"}}}\n" +
			"\n" +
			"event: content_block_stop\n" +
			"data: {\"type\":\"content_block_stop\",\"index\":0}\n" +
			"\n" +
			"event: message_stop\n" +
			"data: {}\n",
	)

	result, err := parseAnthropicSSE(stream, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "read_file") {
		t.Errorf("result should contain tool name, got %q", result)
	}
	if !strings.Contains(result, "main.go") {
		t.Errorf("result should contain tool arg, got %q", result)
	}
}

func TestParseAnthropicSSEToolUseInputDelta(t *testing.T) {
	// Large input: arrives incrementally via input_json_delta events
	stream := strings.NewReader(
		"event: content_block_start\n" +
			"data: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"tool_use\",\"id\":\"toolu_2\",\"name\":\"write_file\",\"input\":{}}}\n" +
			"\n" +
			"event: content_block_delta\n" +
			"data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"{\\\"path\\\": \\\"test.py\\\"\"}}\n" +
			"\n" +
			"event: content_block_delta\n" +
			"data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\", \\\"content\\\": \\\"print('hello')\\\"}\"}}\n" +
			"\n" +
			"event: content_block_stop\n" +
			"data: {\"type\":\"content_block_stop\",\"index\":0}\n" +
			"\n" +
			"event: message_stop\n" +
			"data: {}\n",
	)

	result, err := parseAnthropicSSE(stream, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "write_file") {
		t.Errorf("result should contain tool name, got %q", result)
	}
	if !strings.Contains(result, "test.py") {
		t.Errorf("result should contain file path, got %q", result)
	}
	if !strings.Contains(result, "print") {
		t.Errorf("result should contain file content, got %q", result)
	}
}

func TestMergeConsecutiveMessages(t *testing.T) {
	messages := []anthropicMessage{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi"},
		{Role: "user", Content: "tool result 1"},
		{Role: "user", Content: "eval message"},
		{Role: "user", Content: "tool result 2"},
		{Role: "assistant", Content: "done"},
	}

	merged := mergeConsecutiveMessages(messages)

	if len(merged) != 4 {
		t.Fatalf("got %d messages, want 4 (3 user messages merged into 1)", len(merged))
	}
	if merged[0].Role != "user" || merged[0].Content != "hello" {
		t.Errorf("message 0: %+v", merged[0])
	}
	if merged[1].Role != "assistant" || merged[1].Content != "hi" {
		t.Errorf("message 1: %+v", merged[1])
	}
	// The 3 consecutive user messages should be merged
	if merged[2].Role != "user" {
		t.Errorf("message 2 role: %q, want 'user'", merged[2].Role)
	}
	if !strings.Contains(merged[2].Content, "tool result 1") ||
		!strings.Contains(merged[2].Content, "eval message") ||
		!strings.Contains(merged[2].Content, "tool result 2") {
		t.Errorf("merged message should contain all 3 parts, got: %q", merged[2].Content)
	}
	if merged[3].Role != "assistant" || merged[3].Content != "done" {
		t.Errorf("message 3: %+v", merged[3])
	}
}

func TestMergeConsecutiveMessagesSingle(t *testing.T) {
	messages := []anthropicMessage{{Role: "user", Content: "hi"}}
	merged := mergeConsecutiveMessages(messages)
	if len(merged) != 1 {
		t.Fatalf("got %d, want 1", len(merged))
	}
}

func TestParseOpenAISSEMalformedData(t *testing.T) {
	stream := strings.NewReader(
		"data: not json\n" +
			"data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n" +
			"data: [DONE]\n",
	)

	result, err := parseOpenAISSE(stream, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should skip malformed data and get the valid chunk
	if result != "ok" {
		t.Errorf("result = %q, want %q", result, "ok")
	}
}
