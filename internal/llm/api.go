package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// LocalProvider communicates with local LLM servers via OpenAI-compatible API.
// Works with llama-server, vLLM, LM Studio, Ollama, and similar endpoints.
type LocalProvider struct {
	baseURL string
	apiKey  string
	model   string
	client  *http.Client
	caps    ModelCapabilities
}

// LocalProviderConfig holds configuration for the local provider.
type LocalProviderConfig struct {
	BaseURL       string
	APIKey        string
	Model         string
	ContextLength int
	SizeTier      string
}

// NewLocalProvider creates a local LLM provider using the OpenAI-compatible protocol.
func NewLocalProvider(cfg LocalProviderConfig) *LocalProvider {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "http://127.0.0.1:8080/v1"
	}
	if cfg.APIKey == "" {
		cfg.APIKey = "not-needed" // local servers don't require keys
	}
	if cfg.ContextLength == 0 {
		cfg.ContextLength = 4096
	}
	if cfg.SizeTier == "" {
		cfg.SizeTier = "medium"
	}

	return &LocalProvider{
		baseURL: cfg.BaseURL,
		apiKey:  cfg.APIKey,
		model:   cfg.Model,
		client:  &http.Client{Timeout: 300 * time.Second}, // 5 min — 14B models need longer
		caps: ModelCapabilities{
			ContextLength: cfg.ContextLength,
			SizeTier:      cfg.SizeTier,
			SupportsTools: false,
			SupportsJSON:  false,
		},
	}
}

func (p *LocalProvider) Available() bool {
	healthURL := strings.TrimSuffix(p.baseURL, "/v1") + "/health"
	// Use a short timeout for health checks — don't inherit the 300s client timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", healthURL, nil)
	if err != nil {
		return false
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return false // server is not reachable
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}

func (p *LocalProvider) Capabilities() ModelCapabilities {
	return p.caps
}

// chatRequest is the request body for the chat completions endpoint.
type chatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Tools       []any     `json:"tools,omitempty"`
	Stop        []string  `json:"stop,omitempty"`
	Grammar     string    `json:"grammar,omitempty"` // GBNF grammar (llama-server)
}

// chatResponse is the response from the chat completions endpoint.
type chatResponse struct {
	Choices []struct {
		Message struct {
			Content   string     `json:"content"`
			ToolCalls []ToolCall `json:"tool_calls,omitempty"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

func (p *LocalProvider) Generate(ctx context.Context, messages []Message, opts GenerateOptions) (string, error) {
	reqBody := chatRequest{
		Model:       p.model,
		Messages:    messages,
		Temperature: opts.Temperature,
		MaxTokens:   opts.MaxTokens,
		Stop:        opts.StopTokens,
		Grammar:     opts.Grammar,
	}

	// Only send tools if the model supports them
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
		return "", fmt.Errorf("local LLM request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024)) // 10MB cap
	if err != nil {
		return "", fmt.Errorf("reading response: %w", err)
	}

	var result chatResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parsing response: %w", err)
	}

	if result.Error != nil {
		return "", fmt.Errorf("LLM error (%s): %s", result.Error.Type, result.Error.Message)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	return result.Choices[0].Message.Content, nil
}

// --- AnthropicProvider ---

// AnthropicProvider communicates with the Anthropic Messages API.
type AnthropicProvider struct {
	apiKey string
	model  string
	client *http.Client
	caps   ModelCapabilities
}

// AnthropicConfig holds configuration for the Anthropic provider.
type AnthropicConfig struct {
	APIKey        string
	Model         string
	ContextLength int
}

// NewAnthropicProvider creates an Anthropic API provider.
func NewAnthropicProvider(cfg AnthropicConfig) *AnthropicProvider {
	if cfg.APIKey == "" {
		cfg.APIKey = os.Getenv("ANTHROPIC_API_KEY")
	}
	if cfg.Model == "" {
		cfg.Model = "claude-sonnet-4-20250514"
	}
	if cfg.ContextLength == 0 {
		cfg.ContextLength = 200000
	}

	return &AnthropicProvider{
		apiKey: cfg.APIKey,
		model:  cfg.Model,
		client: &http.Client{Timeout: 120 * time.Second},
		caps: ModelCapabilities{
			ContextLength: cfg.ContextLength,
			SizeTier:      "large",
			SupportsTools: true,
			SupportsJSON:  true,
		},
	}
}

func (p *AnthropicProvider) Available() bool {
	return p.apiKey != ""
}

func (p *AnthropicProvider) Capabilities() ModelCapabilities {
	return p.caps
}

// anthropicRequest is the Anthropic Messages API request format.
type anthropicRequest struct {
	Model       string             `json:"model"`
	MaxTokens   int                `json:"max_tokens"`
	System      string             `json:"system,omitempty"`
	Messages    []anthropicMessage `json:"messages"`
	Tools       []anthropicTool    `json:"tools,omitempty"`
	Temperature float64            `json:"temperature,omitempty"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// anthropicTool is the Anthropic Messages API tool definition format.
type anthropicTool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"input_schema"`
}

// anthropicContentBlock represents a content block in the response (text or tool_use).
type anthropicContentBlock struct {
	Type  string                 `json:"type"`            // "text" or "tool_use"
	Text  string                 `json:"text,omitempty"`  // for type "text"
	ID    string                 `json:"id,omitempty"`    // for type "tool_use"
	Name  string                 `json:"name,omitempty"`  // for type "tool_use"
	Input map[string]interface{} `json:"input,omitempty"` // for type "tool_use"
}

type anthropicResponse struct {
	Content []anthropicContentBlock `json:"content"`
	Error   *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
	StopReason string `json:"stop_reason"` // "end_turn", "tool_use", etc.
}

func (p *AnthropicProvider) Generate(ctx context.Context, messages []Message, opts GenerateOptions) (string, error) {
	system, apiMessages := prepareAnthropicMessages(messages)

	maxTokens := opts.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	reqBody := anthropicRequest{
		Model:       p.model,
		MaxTokens:   maxTokens,
		System:      system,
		Messages:    apiMessages,
		Temperature: opts.Temperature,
	}

	// Convert OpenAI-format tool schemas to Anthropic format
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
		return "", fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024)) // 10MB cap
	if err != nil {
		return "", fmt.Errorf("reading response: %w", err)
	}

	var result anthropicResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parsing response: %w", err)
	}

	if result.Error != nil {
		return "", fmt.Errorf("API error (%s): %s", result.Error.Type, result.Error.Message)
	}

	if len(result.Content) == 0 {
		return "", fmt.Errorf("no content in response")
	}

	// Build response text from content blocks.
	// Text blocks are passed through; tool_use blocks are converted to JSON
	// that core.ParseToolCalls can parse (pragmatic integration — no agent loop changes).
	return formatAnthropicResponse(result.Content), nil
}

// prepareAnthropicMessages extracts the system prompt and converts messages to
// Anthropic format. Maps "tool" role to "user" and merges consecutive same-role
// messages to satisfy the Anthropic API's strict alternation requirement.
func prepareAnthropicMessages(messages []Message) (string, []anthropicMessage) {
	var system string
	var raw []anthropicMessage

	for _, m := range messages {
		if m.Role == "system" {
			system = m.Content
			continue
		}
		role := m.Role
		if role == "tool" {
			role = "user"
		}
		raw = append(raw, anthropicMessage{
			Role:    role,
			Content: m.Content,
		})
	}

	return system, mergeConsecutiveMessages(raw)
}

// mergeConsecutiveMessages collapses adjacent messages with the same role.
// Anthropic requires strict user/assistant alternation — consecutive same-role
// messages (e.g., tool result + eval message both mapped to "user") are rejected.
func mergeConsecutiveMessages(messages []anthropicMessage) []anthropicMessage {
	if len(messages) <= 1 {
		return messages
	}

	var merged []anthropicMessage
	current := messages[0]

	for i := 1; i < len(messages); i++ {
		if messages[i].Role == current.Role {
			// Same role — merge content with separator
			current.Content += "\n\n" + messages[i].Content
		} else {
			merged = append(merged, current)
			current = messages[i]
		}
	}
	merged = append(merged, current)

	return merged
}

// convertToAnthropicTools converts OpenAI-format tool schemas to Anthropic format.
// OpenAI: {"type": "function", "function": {"name", "description", "parameters"}}
// Anthropic: {"name", "description", "input_schema"}
func convertToAnthropicTools(openaiTools []any) []anthropicTool {
	var tools []anthropicTool
	for _, t := range openaiTools {
		// The tools are OpenAISchema structs serialized as any
		raw, err := json.Marshal(t)
		if err != nil {
			continue
		}
		var schema struct {
			Type     string `json:"type"`
			Function struct {
				Name        string      `json:"name"`
				Description string      `json:"description"`
				Parameters  interface{} `json:"parameters"`
			} `json:"function"`
		}
		if err := json.Unmarshal(raw, &schema); err != nil {
			continue
		}
		if schema.Function.Name == "" {
			continue
		}
		tools = append(tools, anthropicTool{
			Name:        schema.Function.Name,
			Description: schema.Function.Description,
			InputSchema: schema.Function.Parameters,
		})
	}
	return tools
}

// formatAnthropicResponse converts Anthropic content blocks to text.
// Text blocks are concatenated. Tool_use blocks are serialized as JSON
// in the format that core.ParseToolCalls expects: {"name": "...", "arguments": {...}}
func formatAnthropicResponse(blocks []anthropicContentBlock) string {
	var parts []string
	for _, block := range blocks {
		switch block.Type {
		case "text":
			if block.Text != "" {
				parts = append(parts, block.Text)
			}
		case "tool_use":
			// Convert to the JSON format the agent's ParseToolCalls expects
			call := map[string]interface{}{
				"name":      block.Name,
				"arguments": block.Input,
			}
			if b, err := json.Marshal(call); err == nil {
				parts = append(parts, string(b))
			}
		}
	}
	return strings.Join(parts, "\n")
}
