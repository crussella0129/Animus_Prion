package tools

import (
	"testing"
)

// mockTool implements Tool for testing.
type mockTool struct {
	name string
}

func (m *mockTool) Name() string        { return m.name }
func (m *mockTool) Description() string  { return "A mock tool" }
func (m *mockTool) Parameters() ParameterSchema {
	return ParameterSchema{
		Type: "object",
		Properties: map[string]ParameterSchema{
			"input": {Type: "string", Description: "Input value"},
		},
		Required: []string{"input"},
	}
}
func (m *mockTool) Execute(args map[string]interface{}) (string, error) {
	input, _ := args["input"].(string)
	return "result: " + input, nil
}

func TestRegistryRegisterAndGet(t *testing.T) {
	r := NewRegistry()
	tool := &mockTool{name: "test_tool"}

	if err := r.Register(tool); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	got, ok := r.Get("test_tool")
	if !ok {
		t.Fatal("Get returned false for registered tool")
	}
	if got.Name() != "test_tool" {
		t.Errorf("got name '%s', want 'test_tool'", got.Name())
	}
}

func TestRegistryDuplicateRegister(t *testing.T) {
	r := NewRegistry()
	tool := &mockTool{name: "test_tool"}

	r.Register(tool)
	err := r.Register(tool)
	if err == nil {
		t.Error("expected error for duplicate registration")
	}
}

func TestRegistryExecute(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockTool{name: "test_tool"})

	result, err := r.Execute("test_tool", map[string]interface{}{"input": "hello"})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result != "result: hello" {
		t.Errorf("got '%s', want 'result: hello'", result)
	}
}

func TestRegistryMissingRequired(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockTool{name: "test_tool"})

	_, err := r.Execute("test_tool", map[string]interface{}{})
	if err == nil {
		t.Error("expected error for missing required arg")
	}
}

func TestRegistryUnknownTool(t *testing.T) {
	r := NewRegistry()
	_, err := r.Execute("nonexistent", map[string]interface{}{})
	if err == nil {
		t.Error("expected error for unknown tool")
	}
}

func TestCoerceArgs(t *testing.T) {
	schema := ParameterSchema{
		Properties: map[string]ParameterSchema{
			"count":   {Type: "integer"},
			"ratio":   {Type: "number"},
			"enabled": {Type: "boolean"},
		},
	}

	args := map[string]interface{}{
		"count":   "42",
		"ratio":   "3.14",
		"enabled": "true",
	}

	coerced := coerceArgs(args, schema)

	if v, ok := coerced["count"].(int); !ok || v != 42 {
		t.Errorf("expected count=42 (int), got %v (%T)", coerced["count"], coerced["count"])
	}
	if v, ok := coerced["ratio"].(float64); !ok || v != 3.14 {
		t.Errorf("expected ratio=3.14 (float64), got %v", coerced["ratio"])
	}
	if v, ok := coerced["enabled"].(bool); !ok || v != true {
		t.Errorf("expected enabled=true (bool), got %v", coerced["enabled"])
	}
}

func TestOpenAISchema(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockTool{name: "test_tool"})

	schemas := r.ToOpenAISchemas()
	if len(schemas) != 1 {
		t.Fatalf("expected 1 schema, got %d", len(schemas))
	}
	if schemas[0].Function.Name != "test_tool" {
		t.Errorf("schema name = '%s', want 'test_tool'", schemas[0].Function.Name)
	}
	if schemas[0].Type != "function" {
		t.Errorf("schema type = '%s', want 'function'", schemas[0].Type)
	}
}
