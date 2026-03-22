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
	stream := strings.NewReader(
		"event: content_block_start\n" +
			"data: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"tool_use\",\"id\":\"toolu_1\",\"name\":\"read_file\",\"input\":{\"path\":\"main.go\"}}}\n" +
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
