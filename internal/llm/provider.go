// Package llm provides LLM provider abstractions and implementations.
package llm

import (
	"context"

	"github.com/crussella0129/Animus_Prion/internal/core"
)

// Message is an alias for core.Message — single message type throughout the codebase.
type Message = core.Message

// ToolCall is an alias for core.ToolCall.
type ToolCall = core.ToolCall

// FunctionCall is an alias for core.FunctionCall.
type FunctionCall = core.FunctionCall

// ModelCapabilities describes what a model can do.
type ModelCapabilities struct {
	ContextLength  int     `json:"context_length"`
	ParameterCount float64 `json:"parameter_count_b"` // billions
	SizeTier       string  `json:"size_tier"`         // "small", "medium", "large"
	SupportsTools  bool    `json:"supports_tools"`
	SupportsJSON   bool    `json:"supports_json_mode"`
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
	Tools       []any // OpenAI-format tool schemas
	StopTokens  []string
	JSONMode    bool
	Grammar     string // GBNF grammar string (llama-server only)
}

// Provider is the interface for LLM backends.
type Provider interface {
	Generate(ctx context.Context, messages []Message, opts GenerateOptions) (string, error)
	Available() bool
	Capabilities() ModelCapabilities
}

// StreamProvider extends Provider with streaming support.
type StreamProvider interface {
	Provider
	GenerateStream(ctx context.Context, messages []Message, opts GenerateOptions, onChunk func(string)) (string, error)
}
