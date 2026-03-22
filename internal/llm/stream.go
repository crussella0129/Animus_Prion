package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// --- LocalProvider streaming (OpenAI-compatible SSE) ---

// streamChatRequest extends chatRequest with stream flag.
type streamChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Tools       []any     `json:"tools,omitempty"`
	Stop        []string  `json:"stop,omitempty"`
	Stream      bool      `json:"stream"`
}

// streamChunk is the SSE data payload for OpenAI-compatible streaming.
type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
}

// GenerateStream implements StreamProvider for LocalProvider.
// Sends a streaming request and calls onChunk for each content delta.
// Returns the complete assembled response.
func (p *LocalProvider) GenerateStream(ctx context.Context, messages []Message, opts GenerateOptions, onChunk func(string)) (string, error) {
	reqBody := streamChatRequest{
		Model:       p.model,
		Messages:    messages,
		Temperature: opts.Temperature,
		MaxTokens:   opts.MaxTokens,
		Stop:        opts.StopTokens,
		Stream:      true,
	}

	if len(opts.Tools) > 0 && p.caps.SupportsTools {
		reqBody.Tools = opts.Tools
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" && p.apiKey != "not-needed" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("streaming request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("streaming error (HTTP %d): %s", resp.StatusCode, string(errBody))
	}

	return parseOpenAISSE(resp.Body, onChunk)
}

// parseOpenAISSE reads an OpenAI-compatible SSE stream.
// Each line is "data: {json}" or "data: [DONE]".
func parseOpenAISSE(reader io.Reader, onChunk func(string)) (string, error) {
	scanner := bufio.NewScanner(reader)
	var full strings.Builder

	for scanner.Scan() {
		line := scanner.Text()

		// Skip empty lines and non-data lines
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")

		// Stream terminator
		if data == "[DONE]" {
			break
		}

		var chunk streamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue // skip malformed chunks
		}

		if len(chunk.Choices) > 0 {
			content := chunk.Choices[0].Delta.Content
			if content != "" {
				full.WriteString(content)
				if onChunk != nil {
					onChunk(content)
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return full.String(), fmt.Errorf("reading stream: %w", err)
	}

	return full.String(), nil
}

// --- AnthropicProvider streaming (Anthropic SSE) ---

// anthropicStreamRequest extends anthropicRequest with stream flag.
type anthropicStreamRequest struct {
	Model       string             `json:"model"`
	MaxTokens   int                `json:"max_tokens"`
	System      string             `json:"system,omitempty"`
	Messages    []anthropicMessage `json:"messages"`
	Tools       []anthropicTool    `json:"tools,omitempty"`
	Temperature float64            `json:"temperature,omitempty"`
	Stream      bool               `json:"stream"`
}

// anthropicStreamEvent is a parsed SSE event from Anthropic's stream.
type anthropicStreamEvent struct {
	Type  string `json:"type"`
	Index int    `json:"index"`
	Delta struct {
		Type string `json:"type"` // "text_delta"
		Text string `json:"text"`
	} `json:"delta,omitempty"`
	ContentBlock *struct {
		Type  string                 `json:"type"` // "tool_use"
		ID    string                 `json:"id"`
		Name  string                 `json:"name"`
		Input map[string]interface{} `json:"input"`
	} `json:"content_block,omitempty"`
}

// GenerateStream implements StreamProvider for AnthropicProvider.
func (p *AnthropicProvider) GenerateStream(ctx context.Context, messages []Message, opts GenerateOptions, onChunk func(string)) (string, error) {
	var system string
	var apiMessages []anthropicMessage

	for _, m := range messages {
		if m.Role == "system" {
			system = m.Content
			continue
		}
		role := m.Role
		if role == "tool" {
			role = "user"
		}
		apiMessages = append(apiMessages, anthropicMessage{
			Role:    role,
			Content: m.Content,
		})
	}

	maxTokens := opts.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	reqBody := anthropicStreamRequest{
		Model:       p.model,
		MaxTokens:   maxTokens,
		System:      system,
		Messages:    apiMessages,
		Temperature: opts.Temperature,
		Stream:      true,
	}

	if len(opts.Tools) > 0 {
		reqBody.Tools = convertToAnthropicTools(opts.Tools)
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("streaming request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("streaming error (HTTP %d): %s", resp.StatusCode, string(errBody))
	}

	return parseAnthropicSSE(resp.Body, onChunk)
}

// parseAnthropicSSE reads an Anthropic SSE stream.
// Events use "event: <type>" and "data: {json}" lines.
// Text arrives as content_block_delta events with text_delta type.
// Tool use arrives as content_block_start with tool_use type, followed by input_json_delta.
func parseAnthropicSSE(reader io.Reader, onChunk func(string)) (string, error) {
	scanner := bufio.NewScanner(reader)
	var full strings.Builder
	var currentEvent string
	var toolBlocks []anthropicContentBlock

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "event: ") {
			currentEvent = strings.TrimPrefix(line, "event: ")
			continue
		}

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")

		switch currentEvent {
		case "content_block_delta":
			var event anthropicStreamEvent
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue
			}
			if event.Delta.Type == "text_delta" && event.Delta.Text != "" {
				full.WriteString(event.Delta.Text)
				if onChunk != nil {
					onChunk(event.Delta.Text)
				}
			}

		case "content_block_start":
			// Check if this is a tool_use block
			var event anthropicStreamEvent
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue
			}
			if event.ContentBlock != nil && event.ContentBlock.Type == "tool_use" {
				toolBlocks = append(toolBlocks, anthropicContentBlock{
					Type:  "tool_use",
					ID:    event.ContentBlock.ID,
					Name:  event.ContentBlock.Name,
					Input: event.ContentBlock.Input,
				})
			}

		case "message_stop":
			// Stream complete
		}
	}

	if err := scanner.Err(); err != nil {
		return full.String(), fmt.Errorf("reading stream: %w", err)
	}

	// If there were tool_use blocks, append them as JSON (same as non-streaming)
	if len(toolBlocks) > 0 {
		toolText := formatAnthropicResponse(toolBlocks)
		if full.Len() > 0 {
			full.WriteString("\n")
		}
		full.WriteString(toolText)
	}

	return full.String(), nil
}
