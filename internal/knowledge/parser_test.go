package knowledge

import (
	"os"
	"path/filepath"
	"testing"
)

const testGoSource = `package example

import (
	"fmt"
	"strings"
)

// Greeter produces greetings.
type Greeter struct {
	Name string
}

// Greet returns a greeting message.
func (g *Greeter) Greet() string {
	return fmt.Sprintf("Hello, %s!", g.Name)
}

// FormatName cleans and formats a name.
func FormatName(name string) string {
	return strings.TrimSpace(name)
}

// BuildGreeting creates a full greeting for a name.
func BuildGreeting(name string) string {
	clean := FormatName(name)
	g := &Greeter{Name: clean}
	return g.Greet()
}

// Runner is an interface for runnable things.
type Runner interface {
	Run() error
}
`

func writeTestFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "example.go")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}
	return path
}

func TestParseGoFile(t *testing.T) {
	path := writeTestFile(t, testGoSource)
	result, err := ParseGoFile(path)
	if err != nil {
		t.Fatalf("ParseGoFile: %v", err)
	}

	if result.File != path {
		t.Errorf("file = %q, want %q", result.File, path)
	}

	// Should have nodes for: package, struct, interface, 3 functions, 2 imports
	if len(result.Nodes) < 7 {
		t.Errorf("expected at least 7 nodes, got %d", len(result.Nodes))
		for _, n := range result.Nodes {
			t.Logf("  node: %s (%s)", n.ID, n.Kind)
		}
	}
}

func TestExtractFunctions(t *testing.T) {
	path := writeTestFile(t, testGoSource)
	result, err := ParseGoFile(path)
	if err != nil {
		t.Fatalf("ParseGoFile: %v", err)
	}

	// Check FormatName exists as a function
	found := false
	for _, n := range result.Nodes {
		if n.Name == "FormatName" && n.Kind == KindFunction {
			found = true
			if n.Signature == "" {
				t.Error("FormatName should have a signature")
			}
			break
		}
	}
	if !found {
		t.Error("FormatName function not found")
	}
}

func TestExtractMethods(t *testing.T) {
	path := writeTestFile(t, testGoSource)
	result, err := ParseGoFile(path)
	if err != nil {
		t.Fatalf("ParseGoFile: %v", err)
	}

	found := false
	for _, n := range result.Nodes {
		if n.Name == "Greet" && n.Kind == KindMethod {
			found = true
			if n.DocString == "" {
				t.Error("Greet should have a doc comment")
			}
			break
		}
	}
	if !found {
		t.Error("Greet method not found")
	}
}

func TestExtractTypes(t *testing.T) {
	path := writeTestFile(t, testGoSource)
	result, err := ParseGoFile(path)
	if err != nil {
		t.Fatalf("ParseGoFile: %v", err)
	}

	var hasStruct, hasInterface bool
	for _, n := range result.Nodes {
		if n.Name == "Greeter" && n.Kind == KindStruct {
			hasStruct = true
		}
		if n.Name == "Runner" && n.Kind == KindInterface {
			hasInterface = true
		}
	}

	if !hasStruct {
		t.Error("Greeter struct not found")
	}
	if !hasInterface {
		t.Error("Runner interface not found")
	}
}

func TestExtractCallEdges(t *testing.T) {
	path := writeTestFile(t, testGoSource)
	result, err := ParseGoFile(path)
	if err != nil {
		t.Fatalf("ParseGoFile: %v", err)
	}

	// BuildGreeting should call FormatName
	found := false
	for _, e := range result.Edges {
		if e.Kind == EdgeCalls && e.Source == "example.BuildGreeting" && e.Target == "FormatName" {
			found = true
			break
		}
	}
	if !found {
		t.Error("BuildGreeting -> FormatName call edge not found")
		for _, e := range result.Edges {
			if e.Kind == EdgeCalls {
				t.Logf("  call: %s -> %s", e.Source, e.Target)
			}
		}
	}
}

func TestExtractImports(t *testing.T) {
	path := writeTestFile(t, testGoSource)
	result, err := ParseGoFile(path)
	if err != nil {
		t.Fatalf("ParseGoFile: %v", err)
	}

	hasImportEdge := false
	for _, e := range result.Edges {
		if e.Kind == EdgeImports {
			hasImportEdge = true
			break
		}
	}
	if !hasImportEdge {
		t.Error("no import edges found")
	}
}

func TestParseInvalidFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.go")
	os.WriteFile(path, []byte("not valid go code !!!"), 0644)

	_, err := ParseGoFile(path)
	if err == nil {
		t.Error("expected error for invalid Go file")
	}
}
