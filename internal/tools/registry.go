// Package tools provides the tool framework — interfaces, registry, and built-in tools.
package tools

import (
	"encoding/json"
	"fmt"
	"sync"
)

// ParameterSchema defines a JSON Schema for a tool parameter.
type ParameterSchema struct {
	Type        string                     `json:"type"`
	Description string                     `json:"description,omitempty"`
	Properties  map[string]ParameterSchema `json:"properties,omitempty"`
	Required    []string                   `json:"required,omitempty"`
	Enum        []string                   `json:"enum,omitempty"`
}

// Tool is the interface that all tools must implement.
type Tool interface {
	Name() string
	Description() string
	Parameters() ParameterSchema
	Execute(args map[string]interface{}) (string, error)
}

// OpenAISchema represents the function-calling schema format used by OpenAI-compatible APIs.
type OpenAISchema struct {
	Type     string `json:"type"`
	Function struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		Parameters  ParameterSchema `json:"parameters"`
	} `json:"function"`
}

// ToOpenAISchema converts a Tool to the OpenAI function-calling format.
func ToOpenAISchema(t Tool) OpenAISchema {
	s := OpenAISchema{Type: "function"}
	s.Function.Name = t.Name()
	s.Function.Description = t.Description()
	s.Function.Parameters = t.Parameters()
	return s
}

// Registry manages tool registration, lookup, and execution.
type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

// NewRegistry creates an empty tool registry.
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
	}
}

// Register adds a tool to the registry.
func (r *Registry) Register(t Tool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := t.Name()
	if _, exists := r.tools[name]; exists {
		return fmt.Errorf("tool already registered: %s", name)
	}
	r.tools[name] = t
	return nil
}

// Get retrieves a tool by name.
func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

// List returns all registered tool names.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}

// Execute runs a tool by name with the given arguments.
// Performs argument validation before execution.
func (r *Registry) Execute(name string, args map[string]interface{}) (string, error) {
	t, ok := r.Get(name)
	if !ok {
		return "", fmt.Errorf("unknown tool: %s", name)
	}

	// Validate required arguments
	params := t.Parameters()
	for _, req := range params.Required {
		if _, ok := args[req]; !ok {
			return "", fmt.Errorf("tool %s: missing required argument: %s", name, req)
		}
	}

	// Coerce string arguments to expected types
	args = coerceArgs(args, params)

	return t.Execute(args)
}

// ToOpenAISchemas returns all tools in OpenAI function-calling format.
func (r *Registry) ToOpenAISchemas() []OpenAISchema {
	r.mu.RLock()
	defer r.mu.RUnlock()
	schemas := make([]OpenAISchema, 0, len(r.tools))
	for _, t := range r.tools {
		schemas = append(schemas, ToOpenAISchema(t))
	}
	return schemas
}

// ToJSON returns the OpenAI schemas as a JSON byte slice.
func (r *Registry) ToJSON() ([]byte, error) {
	return json.Marshal(r.ToOpenAISchemas())
}

// coerceArgs fixes common type mismatches from LLM output.
// E.g., "30" (string) → 30 (int) when schema expects integer.
func coerceArgs(args map[string]interface{}, schema ParameterSchema) map[string]interface{} {
	if schema.Properties == nil {
		return args
	}
	for key, propSchema := range schema.Properties {
		val, ok := args[key]
		if !ok {
			continue
		}
		switch propSchema.Type {
		case "integer":
			if s, ok := val.(string); ok {
				var n int
				if _, err := fmt.Sscanf(s, "%d", &n); err == nil {
					args[key] = n
				}
			}
			// JSON numbers are float64 by default
			if f, ok := val.(float64); ok {
				args[key] = int(f)
			}
		case "number":
			if s, ok := val.(string); ok {
				var f float64
				if _, err := fmt.Sscanf(s, "%f", &f); err == nil {
					args[key] = f
				}
			}
		case "boolean":
			if s, ok := val.(string); ok {
				args[key] = s == "true" || s == "1"
			}
		}
	}
	return args
}
