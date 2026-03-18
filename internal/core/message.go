package core

// Message represents a chat message in the conversation history.
// This is the single message type used throughout the codebase — both
// the agent/planner layer and the LLM provider layer use the same struct.
type Message struct {
	Role       string     `json:"role"`                  // "system", "user", "assistant", "tool"
	Content    string     `json:"content"`
	Name       string     `json:"name,omitempty"`        // tool name (for tool role)
	ToolCallID string     `json:"tool_call_id,omitempty"` // correlates tool results with calls
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`   // function calls from assistant
}

// ToolCall represents a function call in the OpenAI-compatible format.
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

// NewSystemMessage creates a system prompt message.
func NewSystemMessage(content string) Message {
	return Message{Role: "system", Content: content}
}

// NewUserMessage creates a user message.
func NewUserMessage(content string) Message {
	return Message{Role: "user", Content: content}
}

// NewAssistantMessage creates an assistant response message.
func NewAssistantMessage(content string) Message {
	return Message{Role: "assistant", Content: content}
}

// NewToolMessage creates a tool result message.
func NewToolMessage(name, content, toolCallID string) Message {
	return Message{Role: "tool", Content: content, Name: name, ToolCallID: toolCallID}
}
