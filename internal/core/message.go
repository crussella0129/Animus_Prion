package core

// Message represents a chat message in the conversation history.
type Message struct {
	Role    string `json:"role"`    // "system", "user", "assistant", "tool"
	Content string `json:"content"`
	Name    string `json:"name,omitempty"`    // tool name (for tool role)
	ToolID  string `json:"tool_id,omitempty"` // tool call ID for correlation
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
func NewToolMessage(name, content, toolID string) Message {
	return Message{Role: "tool", Content: content, Name: name, ToolID: toolID}
}
