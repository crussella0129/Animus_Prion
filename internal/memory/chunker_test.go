package memory

import (
	"strings"
	"testing"
)

func TestChunkByFunction(t *testing.T) {
	source := `package example

import "fmt"

// Greet says hello.
func Greet(name string) {
	fmt.Println("Hello, " + name)
}

// Add returns the sum.
func Add(a, b int) int {
	return a + b
}
`
	chunks := ChunkByFunction(source, "example.go")
	if len(chunks) < 2 {
		t.Errorf("expected at least 2 function chunks, got %d", len(chunks))
		for i, c := range chunks {
			t.Logf("  chunk %d: lines %d-%d: %s", i, c.StartLine, c.EndLine, c.Text[:min(len(c.Text), 40)])
		}
	}

	// All chunks should have source set
	for _, c := range chunks {
		if c.Source != "example.go" {
			t.Errorf("chunk source = %q, want 'example.go'", c.Source)
		}
	}
}

func TestChunkByFunctionEmpty(t *testing.T) {
	chunks := ChunkByFunction("", "empty.go")
	// Should produce 0 or 1 chunks (empty remainder)
	for _, c := range chunks {
		if strings.TrimSpace(c.Text) != "" {
			t.Errorf("expected empty chunks, got: %q", c.Text)
		}
	}
}

func TestChunkTextWithOverlap(t *testing.T) {
	lines := make([]string, 100)
	for i := range lines {
		lines[i] = "This is a line of text for testing chunking behavior."
	}
	text := strings.Join(lines, "\n")

	chunks := ChunkText(text, "test.txt", 50, 10)
	if len(chunks) < 2 {
		t.Errorf("expected multiple chunks with overlap, got %d", len(chunks))
	}

	// Verify all chunks have source
	for _, c := range chunks {
		if c.Source != "test.txt" {
			t.Errorf("source = %q, want 'test.txt'", c.Source)
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
