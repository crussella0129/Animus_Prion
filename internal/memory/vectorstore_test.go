package memory

import (
	"math"
	"path/filepath"
	"testing"
)

func setupTestVectorStore(t *testing.T) *VectorStore {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "vectors.db")
	vs, err := NewVectorStore(path, 4)
	if err != nil {
		t.Fatalf("NewVectorStore: %v", err)
	}
	t.Cleanup(func() { vs.Close() })
	return vs
}

func TestVectorStoreInsertAndSearch(t *testing.T) {
	vs := setupTestVectorStore(t)

	// Insert chunks with embeddings
	chunks := []Chunk{
		{Text: "authentication handler", Source: "auth.go", Embedding: []float32{1, 0, 0, 0}},
		{Text: "database connection", Source: "db.go", Embedding: []float32{0, 1, 0, 0}},
		{Text: "auth middleware", Source: "middleware.go", Embedding: []float32{0.9, 0.1, 0, 0}},
	}

	for _, c := range chunks {
		if _, err := vs.Insert(c); err != nil {
			t.Fatalf("Insert: %v", err)
		}
	}

	// Search for auth-like embeddings
	hits, err := vs.Search([]float32{1, 0, 0, 0}, 2)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	if len(hits) != 2 {
		t.Fatalf("expected 2 hits, got %d", len(hits))
	}

	// First hit should be exact match (auth.go)
	if hits[0].Chunk.Source != "auth.go" {
		t.Errorf("first hit source = %q, want 'auth.go'", hits[0].Chunk.Source)
	}
	if hits[0].Similarity < 0.99 {
		t.Errorf("first hit similarity = %f, want ~1.0", hits[0].Similarity)
	}

	// Second should be the partial match (middleware.go)
	if hits[1].Chunk.Source != "middleware.go" {
		t.Errorf("second hit source = %q, want 'middleware.go'", hits[1].Chunk.Source)
	}
}

func TestVectorStoreInsertBatch(t *testing.T) {
	vs := setupTestVectorStore(t)

	chunks := []Chunk{
		{Text: "chunk 1", Source: "a.go", Embedding: []float32{1, 0, 0, 0}},
		{Text: "chunk 2", Source: "b.go", Embedding: []float32{0, 1, 0, 0}},
		{Text: "chunk 3", Source: "c.go", Embedding: []float32{0, 0, 1, 0}},
	}

	if err := vs.InsertBatch(chunks); err != nil {
		t.Fatalf("InsertBatch: %v", err)
	}

	count, err := vs.Count()
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 3 {
		t.Errorf("count = %d, want 3", count)
	}
}

func TestVectorStoreDeleteBySource(t *testing.T) {
	vs := setupTestVectorStore(t)

	vs.Insert(Chunk{Text: "a", Source: "keep.go", Embedding: []float32{1, 0, 0, 0}})
	vs.Insert(Chunk{Text: "b", Source: "delete.go", Embedding: []float32{0, 1, 0, 0}})

	if err := vs.DeleteBySource("delete.go"); err != nil {
		t.Fatalf("DeleteBySource: %v", err)
	}

	count, _ := vs.Count()
	if count != 1 {
		t.Errorf("count = %d, want 1", count)
	}
}

func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		a, b     []float32
		expected float64
	}{
		{[]float32{1, 0, 0}, []float32{1, 0, 0}, 1.0},    // identical
		{[]float32{1, 0, 0}, []float32{0, 1, 0}, 0.0},    // orthogonal
		{[]float32{1, 0, 0}, []float32{-1, 0, 0}, -1.0},  // opposite
		{[]float32{1, 1, 0}, []float32{1, 0, 0}, 0.7071}, // 45 degrees
	}

	for _, tt := range tests {
		got := cosineSimilarity(tt.a, tt.b)
		if math.Abs(got-tt.expected) > 0.01 {
			t.Errorf("cosineSimilarity(%v, %v) = %f, want %f", tt.a, tt.b, got, tt.expected)
		}
	}
}

func TestFloat32Serialization(t *testing.T) {
	original := []float32{1.5, -2.7, 0.0, 3.14159}
	bytes := float32ToBytes(original)
	recovered := bytesToFloat32(bytes)

	if len(recovered) != len(original) {
		t.Fatalf("len = %d, want %d", len(recovered), len(original))
	}

	for i := range original {
		if math.Abs(float64(recovered[i]-original[i])) > 1e-6 {
			t.Errorf("[%d] = %f, want %f", i, recovered[i], original[i])
		}
	}
}

func TestChunkText(t *testing.T) {
	text := "line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10"

	// Small chunk size to force multiple chunks
	chunks := ChunkText(text, "test.go", 10, 0) // ~35 chars max per chunk
	if len(chunks) < 2 {
		t.Errorf("expected multiple chunks, got %d", len(chunks))
	}

	// All chunks should have source and line info
	for _, c := range chunks {
		if c.Source != "test.go" {
			t.Errorf("chunk source = %q, want 'test.go'", c.Source)
		}
		if c.StartLine == 0 {
			t.Error("chunk start_line should not be 0")
		}
	}
}

func TestChunkTextEmpty(t *testing.T) {
	chunks := ChunkText("", "test.go", 512, 0)
	if len(chunks) != 0 {
		t.Errorf("expected 0 chunks for empty text, got %d", len(chunks))
	}
}
