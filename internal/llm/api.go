package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// OpenAIProvider communicates with OpenAI-compatible APIs.
// Supports OpenAI, vLLM, LM Studio, and any OpenAI-compatible endpoint.
type OpenAIProvider struct {
	baseURL    string
	apiKey     string
	model      string
	client     *http.Client
	caps       ModelCapabilities
}

// OpenAIConfig holds configuration for the OpenAI provider.
type OpenAIConfig struct {
	BaseURL       string
	APIKey        string
	Model         string
	ContextLength int
	SizeTier      string
}

// NewOpenAIProvider creates an OpenAI-compatible API provider.
func NewOpenAIProvider(cfg OpenAIConfig) *OpenAIProvider {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.openai.com/v1"
	}
	if cfg.APIKey == "" {
		cfg.APIKey = os.Getenv("OPENAI_API_KEY")
	}
	if cfg.ContextLength == 0 {
		cfg.ContextLength = 8192
	}
	if cfg.SizeTier == "" {
		cfg.SizeTier = "large"
	}

	return &OpenAIProvider{
		baseURL: cfg.BaseURL,
		apiKey:  cfg.APIKey,
		model:   cfg.Model,
		client:  &http.Client{Timeout: 120 * time.Second},
		caps: ModelCapabilities{
			ContextLength: cfg.ContextLength,
			SizeTier:      cfg.SizeTier,
			SupportsTools: true,
			SupportsJSON:  true,
		},
	}
}

func (p *OpenAIProvider) Available() bool {
	return p.apiKey != ""
}

func (p *OpenAIProvider) Capabilities() ModelCapabilities {
	return p.caps
}

// openAIRequest is the request body for the chat completions endpoint.
type openAIRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Tools       []any     `json:"tools,omitempty"`
	Stop        []string  `json:"stop,omitempty"`
}

// openAIResponse is the response from the chat completions endpoint.
type openAIResponse struct {
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

func (p *OpenAIProvider) Generate(messages []Message, opts GenerateOptions) (string, error) {
	reqBody := openAIRequest{
		Model:       p.model,
		Messages:    messages,
		Temperature: opts.Temperature,
		MaxTokens:   opts.MaxTokens,
		Stop:        opts.StopTokens,
	}

	if len(opts.Tools) > 0 {
		toolsAny := make([]any, len(opts.Tools))
		for i, t := range opts.Tools {
			toolsAny[i] = t
		}
		reqBody.Tools = toolsAny
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequest("POST", p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response: %w", err)
	}

	var result openAIResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parsing response: %w", err)
	}

	if result.Error != nil {
		return "", fmt.Errorf("API error (%s): %s", result.Error.Type, result.Error.Message)
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
	Temperature float64            `json:"temperature,omitempty"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

func (p *AnthropicProvider) Generate(messages []Message, opts GenerateOptions) (string, error) {
	// Extract system message and convert format
	var system string
	var apiMessages []anthropicMessage

	for _, m := range messages {
		if m.Role == "system" {
			system = m.Content
			continue
		}
		role := m.Role
		if role == "tool" {
			role = "user" // Anthropic uses user role for tool results
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

	reqBody := anthropicRequest{
		Model:       p.model,
		MaxTokens:   maxTokens,
		System:      system,
		Messages:    apiMessages,
		Temperature: opts.Temperature,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
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

	respBody, err := io.ReadAll(resp.Body)
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

	return result.Content[0].Text, nil
}
