package knowledge

import (
	"path/filepath"
	"testing"
)

func setupTestDB(t *testing.T) *GraphDB {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := NewGraphDB(dbPath)
	if err != nil {
		t.Fatalf("NewGraphDB: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestGraphDBCreateAndQuery(t *testing.T) {
	db := setupTestDB(t)

	// Insert a node
	err := db.UpsertNode(Node{
		ID: "pkg.Foo", Name: "Foo", Kind: KindFunction,
		File: "foo.go", Line: 10, Signature: "func Foo()", Package: "pkg",
	})
	if err != nil {
		t.Fatalf("UpsertNode: %v", err)
	}

	// Search by name
	results, err := db.SearchNodes("Foo")
	if err != nil {
		t.Fatalf("SearchNodes: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].ID != "pkg.Foo" {
		t.Errorf("ID = %q, want 'pkg.Foo'", results[0].ID)
	}
}

func TestGraphDBCallersCallees(t *testing.T) {
	db := setupTestDB(t)

	// Create two functions
	db.UpsertNode(Node{ID: "pkg.A", Name: "A", Kind: KindFunction, File: "a.go", Package: "pkg"})
	db.UpsertNode(Node{ID: "pkg.B", Name: "B", Kind: KindFunction, File: "b.go", Package: "pkg"})

	// A calls B
	db.UpsertEdge(Edge{Source: "pkg.A", Target: "pkg.B", Kind: EdgeCalls, File: "a.go", Line: 5})

	// Callers of B should be [A]
	callers, err := db.GetCallers("pkg.B")
	if err != nil {
		t.Fatalf("GetCallers: %v", err)
	}
	if len(callers) != 1 || callers[0].ID != "pkg.A" {
		t.Errorf("callers of B = %v, want [A]", callers)
	}

	// Callees of A should be [B]
	callees, err := db.GetCallees("pkg.A")
	if err != nil {
		t.Fatalf("GetCallees: %v", err)
	}
	if len(callees) != 1 || callees[0].ID != "pkg.B" {
		t.Errorf("callees of A = %v, want [B]", callees)
	}
}

func TestGraphDBTransitiveCallers(t *testing.T) {
	db := setupTestDB(t)

	// Chain: C -> B -> A
	db.UpsertNode(Node{ID: "pkg.A", Name: "A", Kind: KindFunction, File: "a.go", Package: "pkg"})
	db.UpsertNode(Node{ID: "pkg.B", Name: "B", Kind: KindFunction, File: "b.go", Package: "pkg"})
	db.UpsertNode(Node{ID: "pkg.C", Name: "C", Kind: KindFunction, File: "c.go", Package: "pkg"})
	db.UpsertEdge(Edge{Source: "pkg.B", Target: "pkg.A", Kind: EdgeCalls})
	db.UpsertEdge(Edge{Source: "pkg.C", Target: "pkg.B", Kind: EdgeCalls})

	// ResolveCallers of A with depth 1 should get [B]
	callers, err := db.ResolveCallers("pkg.A", 1)
	if err != nil {
		t.Fatalf("ResolveCallers: %v", err)
	}
	if len(callers) != 1 {
		t.Errorf("depth-1 callers = %d, want 1", len(callers))
	}

	// ResolveCallers of A with depth 2 should get [B, C]
	callers, err = db.ResolveCallers("pkg.A", 2)
	if err != nil {
		t.Fatalf("ResolveCallers: %v", err)
	}
	if len(callers) != 2 {
		t.Errorf("depth-2 callers = %d, want 2", len(callers))
	}
}

func TestGraphDBBlastRadius(t *testing.T) {
	db := setupTestDB(t)

	// A is called by B and C; B is called by D
	db.UpsertNode(Node{ID: "pkg.A", Name: "A", Kind: KindFunction, File: "a.go", Package: "pkg"})
	db.UpsertNode(Node{ID: "pkg.B", Name: "B", Kind: KindFunction, File: "b.go", Package: "pkg"})
	db.UpsertNode(Node{ID: "pkg.C", Name: "C", Kind: KindFunction, File: "c.go", Package: "pkg"})
	db.UpsertNode(Node{ID: "pkg.D", Name: "D", Kind: KindFunction, File: "d.go", Package: "pkg"})
	db.UpsertEdge(Edge{Source: "pkg.B", Target: "pkg.A", Kind: EdgeCalls})
	db.UpsertEdge(Edge{Source: "pkg.C", Target: "pkg.A", Kind: EdgeCalls})
	db.UpsertEdge(Edge{Source: "pkg.D", Target: "pkg.B", Kind: EdgeCalls})

	// Blast radius of A with depth 2 should include B, C, D
	affected, err := db.GetBlastRadius("pkg.A", 2)
	if err != nil {
		t.Fatalf("GetBlastRadius: %v", err)
	}
	if len(affected) != 3 {
		t.Errorf("blast radius = %d, want 3", len(affected))
		for _, n := range affected {
			t.Logf("  affected: %s", n.ID)
		}
	}
}

func TestGraphDBIngestResult(t *testing.T) {
	db := setupTestDB(t)

	result := &ParseResult{
		File: "test.go",
		Nodes: []Node{
			{ID: "pkg.X", Name: "X", Kind: KindFunction, File: "test.go", Line: 1, Package: "pkg"},
			{ID: "pkg.Y", Name: "Y", Kind: KindFunction, File: "test.go", Line: 5, Package: "pkg"},
		},
		Edges: []Edge{
			{Source: "pkg.X", Target: "pkg.Y", Kind: EdgeCalls, File: "test.go", Line: 2},
		},
	}

	if err := db.IngestResult(result); err != nil {
		t.Fatalf("IngestResult: %v", err)
	}

	nodes, edges, err := db.Stats()
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if nodes != 2 {
		t.Errorf("nodes = %d, want 2", nodes)
	}
	if edges != 1 {
		t.Errorf("edges = %d, want 1", edges)
	}
}

func TestGraphDBDeleteByFile(t *testing.T) {
	db := setupTestDB(t)

	db.UpsertNode(Node{ID: "pkg.A", Name: "A", Kind: KindFunction, File: "a.go", Package: "pkg"})
	db.UpsertNode(Node{ID: "pkg.B", Name: "B", Kind: KindFunction, File: "b.go", Package: "pkg"})
	db.UpsertEdge(Edge{Source: "pkg.A", Target: "pkg.B", Kind: EdgeCalls, File: "a.go"})

	if err := db.DeleteByFile("a.go"); err != nil {
		t.Fatalf("DeleteByFile: %v", err)
	}

	// A should be gone, B should remain
	n, err := db.GetNode("pkg.A")
	if err != nil {
		t.Fatalf("GetNode: %v", err)
	}
	if n != nil {
		t.Error("node A should have been deleted")
	}

	n, err = db.GetNode("pkg.B")
	if err != nil {
		t.Fatalf("GetNode: %v", err)
	}
	if n == nil {
		t.Error("node B should still exist")
	}
}

func TestGraphDBNodesByKind(t *testing.T) {
	db := setupTestDB(t)

	db.UpsertNode(Node{ID: "pkg.Foo", Name: "Foo", Kind: KindFunction, File: "f.go", Package: "pkg"})
	db.UpsertNode(Node{ID: "pkg.Bar", Name: "Bar", Kind: KindStruct, File: "f.go", Package: "pkg"})
	db.UpsertNode(Node{ID: "pkg.Baz", Name: "Baz", Kind: KindFunction, File: "f.go", Package: "pkg"})

	fns, err := db.NodesByKind(KindFunction)
	if err != nil {
		t.Fatalf("NodesByKind: %v", err)
	}
	if len(fns) != 2 {
		t.Errorf("functions = %d, want 2", len(fns))
	}
}
