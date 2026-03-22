// Package agent provides the core agentic loop that orchestrates LLM generation and tool execution.
package agent

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"strings"
	"time"

	"github.com/crussella0129/Animus_Prion/internal/core"
	"github.com/crussella0129/Animus_Prion/internal/llm"
	"github.com/crussella0129/Animus_Prion/internal/tools"
)

// Agent is the core agentic loop that orchestrates LLM generation and tool execution.
// Agent is NOT safe for concurrent use — Run must not be called from multiple goroutines.
// The REPL is single-threaded, so this is fine. If concurrent agents are needed in the
// future, protect the history slice with a sync.Mutex.
type Agent struct {
	provider      llm.Provider
	registry      *tools.Registry
	workspace     *core.Workspace
	systemPrompt  string
	maxTurns      int
	history       []core.Message
	contextWindow *core.ContextWindow
}

// Config holds agent initialization parameters.
type Config struct {
	Provider      llm.Provider
	Registry      *tools.Registry
	Workspace     *core.Workspace
	SystemPrompt  string
	MaxTurns      int
	SizeTier      string
	ContextLength int
}

// New creates a new agent with the given configuration.
func New(cfg Config) *Agent {
	if cfg.MaxTurns == 0 {
		cfg.MaxTurns = 20
	}
	if cfg.SizeTier == "" {
		cfg.SizeTier = "large"
	}
	if cfg.ContextLength == 0 {
		cfg.ContextLength = 8192
	}
	if cfg.SystemPrompt == "" {
		cfg.SystemPrompt = defaultSystemPrompt(cfg.Registry)
	}

	return &Agent{
		provider:     cfg.Provider,
		registry:     cfg.Registry,
		workspace:    cfg.Workspace,
		systemPrompt: cfg.SystemPrompt,
		maxTurns:     cfg.MaxTurns,
		history:      []core.Message{core.NewSystemMessage(cfg.SystemPrompt)},
		contextWindow: &core.ContextWindow{
			ContextLength: cfg.ContextLength,
			SizeTier:      cfg.SizeTier,
		},
	}
}

// Run executes the agentic loop for the given user input.
// Returns the final assistant response. Cancellable via context.
func (a *Agent) Run(ctx context.Context, input string) (string, error) {
	a.history = append(a.history, core.NewUserMessage(input))

	var lastResponse string
	var lastToolResult string
	var prevCallKey string
	prevCallSucceeded := true
	repeatCount := 0

	for turn := 0; turn < a.maxTurns; turn++ {
		// Check for cancellation
		if ctx.Err() != nil {
			return lastResponse, ctx.Err()
		}

		// Trim history to fit context window
		budget := a.contextWindow.ComputeBudget(core.EstimateTokens(a.systemPrompt, false))
		a.history = core.TrimMessages(a.history, budget.HistoryTokens)

		// Rate limiting: progressive sleep
		if turn > 0 {
			time.Sleep(time.Duration(turn*100) * time.Millisecond)
		}

		// Generate response
		response, err := a.step(ctx)
		if err != nil {
			classified := core.ClassifyError(err)
			if classified.Retryable && turn < a.maxTurns-1 {
				// Exponential backoff: 1s, 2s, 4s, 8s, capped at 30s
				backoff := time.Duration(1<<uint(min(turn, 4))) * time.Second
				if backoff > 30*time.Second {
					backoff = 30 * time.Second
				}
				slog.Warn("retryable error, backing off", "turn", turn, "backoff", backoff, "error", err)
				time.Sleep(backoff)
				continue
			}
			if lastToolResult != "" {
				return lastToolResult, nil
			}
			return "", fmt.Errorf("agent error on turn %d: %w", turn, err)
		}

		a.history = append(a.history, core.NewAssistantMessage(response))
		lastResponse = response

		// Parse tool calls from response
		toolCalls := core.ParseToolCalls(response)
		toolCalls = core.DeduplicateToolCalls(toolCalls)

		// No tool calls → agent is done
		if len(toolCalls) == 0 {
			return response, nil
		}

		// Repeat detection — only count as repeat if previous SUCCEEDED.
		// A retry after failure is a correction attempt, not a loop.
		// Hash ALL tool calls, not just the first — prevents multi-tool loops.
		var keyParts []string
		for _, tc := range toolCalls {
			keyParts = append(keyParts, fmt.Sprintf("%s:%v", tc.Name, tc.Arguments))
		}
		callKey := strings.Join(keyParts, "|")
		if callKey == prevCallKey && prevCallSucceeded {
			repeatCount++
			if repeatCount >= 2 { // allow up to 3 identical successful calls before breaking
				slog.Debug("repeat detected, breaking loop", "turn", turn, "repeats", repeatCount)
				if lastToolResult != "" {
					return lastToolResult, nil
				}
				return response, nil
			}
		} else {
			repeatCount = 0
		}
		prevCallKey = callKey

		// Execute tool calls
		turnSucceeded := true
		for _, tc := range toolCalls {
			result, err := a.registry.Execute(tc.Name, tc.Arguments)
			if err != nil {
				turnSucceeded = false
				errorMsg := fmt.Sprintf("Error executing %s: %s", tc.Name, err.Error())
				a.history = append(a.history, core.NewToolMessage(tc.Name, errorMsg, ""))
				lastToolResult = errorMsg

				evalMsg := evaluateToolResult(tc.Name, "", err)
				a.history = append(a.history, core.NewUserMessage(evalMsg))
			} else {
				a.history = append(a.history, core.NewToolMessage(tc.Name, result, ""))
				lastToolResult = result

				evalMsg := evaluateToolResult(tc.Name, result, nil)
				if evalMsg != "" {
					a.history = append(a.history, core.NewUserMessage(evalMsg))
				}
			}
		}
		prevCallSucceeded = turnSucceeded
	}

	if lastToolResult != "" {
		return lastToolResult, nil
	}
	return lastResponse, nil
}

// step performs a single LLM generation call.
func (a *Agent) step(ctx context.Context) (string, error) {
	caps := a.provider.Capabilities()
	schemas := a.registry.ToOpenAISchemas()
	toolsAny := make([]any, len(schemas))
	for i, s := range schemas {
		toolsAny[i] = s
	}

	opts := llm.GenerateOptions{
		Temperature: 0.7,
		MaxTokens:   caps.ContextLength / 4,
		Tools:       toolsAny,
	}

	return a.provider.Generate(ctx, a.history, opts)
}

// evaluateToolResult generates observation guidance for the model.
func evaluateToolResult(toolName, result string, err error) string {
	if err != nil {
		return fmt.Sprintf(
			"The tool '%s' failed with error: %s. Do NOT retry the same call. "+
				"Consider an alternative approach or explain the issue to the user.",
			toolName, err.Error(),
		)
	}
	if result == "" {
		return fmt.Sprintf("The tool '%s' returned empty output. This may be normal (e.g., successful write). Proceed.", toolName)
	}
	if len(result) > 2000 {
		return "The tool returned a large result. Focus on the relevant parts for the user's task."
	}
	return ""
}

// defaultSystemPrompt generates the system prompt with available tool descriptions.
func defaultSystemPrompt(registry *tools.Registry) string {
	var sb strings.Builder
	sb.WriteString("You are Prion, a local-first AI agent. You help users by executing tasks using available tools.\n\n")

	// Platform awareness — prevents .so/.dll/.dylib mistakes
	sb.WriteString(fmt.Sprintf("Platform: %s/%s\n", runtime.GOOS, runtime.GOARCH))
	switch runtime.GOOS {
	case "windows":
		sb.WriteString("Shared libraries use .dll extension. Executables use .exe.\n")
	case "darwin":
		sb.WriteString("Shared libraries use .dylib extension.\n")
	default:
		sb.WriteString("Shared libraries use .so extension.\n")
	}
	sb.WriteString("\n")

	sb.WriteString("Available tools:\n")
	for _, name := range registry.List() {
		t, _ := registry.Get(name)
		sb.WriteString(fmt.Sprintf("- %s: %s\n", name, t.Description()))
	}

	sb.WriteString("\nTo use a tool, respond with a JSON object: {\"name\": \"tool_name\", \"arguments\": {...}}\n")
	sb.WriteString("You may ONLY use the tools listed above. Do not invent tools.\n")
	sb.WriteString("When you have completed the task, respond with plain text (no JSON).\n")
	sb.WriteString("\nRULES:\n")
	sb.WriteString("- To create files, use write_file with the full path and content. Do NOT use run_shell for file creation.\n")
	sb.WriteString("- write_file automatically creates parent directories — no need for mkdir.\n")
	sb.WriteString("- After writing code, verify with run_shell (e.g. 'cargo build' or 'python -c \"print(1)\"').\n")
	sb.WriteString("- Write one file at a time. After each write_file, proceed to the next file.\n")

	return sb.String()
}

// History returns the conversation history.
func (a *Agent) History() []core.Message {
	return a.history
}

// Reset clears the conversation history, keeping the system prompt.
func (a *Agent) Reset() {
	a.history = []core.Message{core.NewSystemMessage(a.systemPrompt)}
}
