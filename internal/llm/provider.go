// Package llm provides LLM provider abstractions and implementations.
package llm

// ModelCapabilities describes what a model can do.
type ModelCapabilities struct {
	ContextLength  int    `json:"context_length"`
	ParameterCount float64 `json:"parameter_count_b"` // billions
	SizeTier       string `json:"size_tier"`          // "small", "medium", "large"
	SupportsTools  bool   `json:"supports_tools"`
	SupportsJSON   bool   `json:"supports_json_mode"`
}

// SizeTierFromParams determines tier from parameter count.
func SizeTierFromParams(billions float64) string {
	switch {
	case billions < 4:
		return "small"
	case billions < 13:
		return "medium"
	default:
		return "large"
	}
}

// GenerateOptions configures a single generation call.
type GenerateOptions struct {
	Temperature float64
	MaxTokens   int
	Tools       []any // OpenAI-format tool schemas (avoids import cycle with tools package)
	StopTokens  []string
	JSONMode    bool
}

// Provider is the interface for LLM backends.
type Provider interface {
	// Generate produces a response from messages.
	Generate(messages []Message, opts GenerateOptions) (string, error)

	// Available returns true if the provider is ready.
	Available() bool

	// Capabilities returns the model's capabilities.
	Capabilities() ModelCapabilities
}

// StreamProvider extends Provider with streaming support.
type StreamProvider interface {
	Provider

	// GenerateStream produces a response token-by-token.
	GenerateStream(messages []Message, opts GenerateOptions, onChunk func(string)) (string, error)
}

// Message is the LLM message format (compatible with OpenAI API).
type Message struct {
	Role       string      `json:"role"`
	Content    string      `json:"content"`
	Name       string      `json:"name,omitempty"`
	ToolCalls  []ToolCall  `json:"tool_calls,omitempty"`
	ToolCallID string      `json:"tool_call_id,omitempty"`
}

// ToolCall represents a function call in the OpenAI format.
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"` // "function"
	Function FunctionCall `json:"function"`
}

// FunctionCall holds the function name and arguments.
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}
