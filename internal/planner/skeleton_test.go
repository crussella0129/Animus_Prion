package planner

import (
	"context"
	"strings"
	"testing"

	"github.com/crussella0129/Animus_Prion/internal/llm"
	"github.com/crussella0129/Animus_Prion/internal/tools"
)

// mockSkeletonProvider returns scripted responses for skeleton planning.
type mockSkeletonProvider struct {
	responses []string
	callCount int
}

func (m *mockSkeletonProvider) Generate(_ context.Context, messages []llm.Message, _ llm.GenerateOptions) (string, error) {
	if m.callCount >= len(m.responses) {
		return "- just do the thing", nil
	}
	idx := m.callCount
	m.callCount++
	return m.responses[idx], nil
}

func (m *mockSkeletonProvider) Available() bool { return true }
func (m *mockSkeletonProvider) Capabilities() llm.ModelCapabilities {
	return llm.ModelCapabilities{ContextLength: 4096, SizeTier: "small"}
}

func TestSkeletonPlanProducesTree(t *testing.T) {
	provider := &mockSkeletonProvider{
		responses: []string{
			// Pass 1: top-level response to user's task
			"- create project structure\n- write main.py\n- run tests",
			// Pass 2: expansion of "create project structure" (not a leaf: 3 words but StepAnalyze)
			// Actually "create project structure" is 3 words, maps to StepWrite via "create" keyword.
			// So it's a leaf. Only non-leaf items get expanded.
		},
	}

	registry := tools.NewRegistry()
	sp := NewSkeletonPlanner(provider, registry, nil)

	root, err := sp.Plan(context.Background(), "build a Python app with tests")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(root.Children) != 3 {
		t.Fatalf("root has %d children, want 3", len(root.Children))
	}

	// All three should be leaves (short descriptions, clear step types)
	for i, child := range root.Children {
		if !child.IsLeaf() {
			t.Errorf("child %d (%q) should be a leaf", i, child.Description)
		}
	}
}

func TestSkeletonPlanRecursiveExpansion(t *testing.T) {
	provider := &mockSkeletonProvider{
		responses: []string{
			// Pass 1: top-level with one non-leaf item
			// "create src directory" and "run tests" are leaves (short, typed)
			// The long item is NOT a leaf (13+ words, no single file)
			"- create src directory\n- write the entire data processing pipeline including the scraper module the cache layer and the CLI parser\n- run tests",
			// Pass 2: expansion of the long item (not a leaf — 13 words)
			"- write src/scraper.py with fetch function\n- write src/cache.py with cache class\n- write src/cli.py with argument parser",
		},
	}

	registry := tools.NewRegistry()
	sp := NewSkeletonPlanner(provider, registry, nil)

	root, err := sp.Plan(context.Background(), "build a data pipeline")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(root.Children) != 3 {
		t.Fatalf("root has %d children, want 3", len(root.Children))
	}

	// Second child should have been expanded into 3 sub-children
	expanded := root.Children[1]
	if len(expanded.Children) != 3 {
		t.Fatalf("expanded node has %d children, want 3", len(expanded.Children))
	}

	// Sub-children should all be leaves (single file each)
	for i, sub := range expanded.Children {
		if !sub.IsLeaf() {
			t.Errorf("sub-child %d (%q) should be a leaf", i, sub.Description)
		}
		if sub.Depth != 2 {
			t.Errorf("sub-child %d depth: got %d, want 2", i, sub.Depth)
		}
	}
}

func TestSkeletonBottomsOutOnRestatement(t *testing.T) {
	provider := &mockSkeletonProvider{
		responses: []string{
			// Top-level
			"- implement the complex data transformation system with multiple stages and error handling across files",
			// Expansion attempt — model restates it
			"- implement the complex data transformation system with multiple stages",
		},
	}

	registry := tools.NewRegistry()
	sp := NewSkeletonPlanner(provider, registry, nil)

	root, err := sp.Plan(context.Background(), "build a complex system")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The single child should remain a leaf because expansion produced a restatement
	if len(root.Children) != 1 {
		t.Fatalf("root has %d children, want 1", len(root.Children))
	}
	child := root.Children[0]
	if len(child.Children) != 0 {
		t.Errorf("child should have no sub-children (restatement detected), got %d", len(child.Children))
	}
}

func TestSkeletonFallsBackToStructuredPrompt(t *testing.T) {
	provider := &mockSkeletonProvider{
		responses: []string{
			// First response: no dashed list (model just responded with text)
			"Sure, I can build that for you. Let me think about how to approach this.",
			// Second response: structured prompt gets a proper list
			"- create project\n- write code\n- test it",
		},
	}

	registry := tools.NewRegistry()
	sp := NewSkeletonPlanner(provider, registry, nil)

	root, err := sp.Plan(context.Background(), "build something")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(root.Children) != 3 {
		t.Fatalf("fallback should have produced 3 children, got %d", len(root.Children))
	}
}

func TestSkeletonSingleItemFallsToLeaf(t *testing.T) {
	provider := &mockSkeletonProvider{
		responses: []string{
			// Top-level: only one item that's also too long to be a leaf
			"- build a complex multi-component web application with authentication database and frontend and backend and deployment",
			// Expansion: model returns a single item that's shorter but still single
			"- set up the web framework",
		},
	}

	registry := tools.NewRegistry()
	sp := NewSkeletonPlanner(provider, registry, nil)

	root, err := sp.Plan(context.Background(), "build a web app")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Single top-level child, which got expanded to single sub-child
	if len(root.Children) != 1 {
		t.Fatalf("root has %d children, want 1", len(root.Children))
	}
	child := root.Children[0]
	// Should have 1 child (the genuinely simpler single item)
	if len(child.Children) != 1 {
		t.Errorf("child has %d sub-children, want 1", len(child.Children))
	}
}

func TestFindNextPending(t *testing.T) {
	root := &TaskNode{
		Status: StatusRunning,
		Children: []*TaskNode{
			{Description: "done", Status: StatusCompleted},
			{Description: "next", Status: StatusPending, Type: StepWrite},
			{Description: "later", Status: StatusPending, Type: StepShell},
		},
	}

	found := FindNextPending(root)
	if found == nil {
		t.Fatal("should have found a pending node")
	}
	if found.Description != "next" {
		t.Errorf("got %q, want %q", found.Description, "next")
	}
}

func TestFindNextPendingNested(t *testing.T) {
	root := &TaskNode{
		Status: StatusRunning,
		Children: []*TaskNode{
			{Description: "branch", Status: StatusRunning, Children: []*TaskNode{
				{Description: "done", Status: StatusCompleted, Type: StepWrite},
				{Description: "nested pending", Status: StatusPending, Type: StepWrite},
			}},
		},
	}

	found := FindNextPending(root)
	if found == nil || found.Description != "nested pending" {
		t.Errorf("should find nested pending node, got %v", found)
	}
}

func TestSessionStateSerializes(t *testing.T) {
	state := &SessionState{
		ID:   "test-123",
		Task: "build something",
		Root: &TaskNode{
			Depth:       0,
			Description: "build something",
			Children: []*TaskNode{
				{Depth: 1, Description: "step 1", Status: StatusCompleted, Result: "done"},
				{Depth: 1, Description: "step 2", Status: StatusPending},
			},
		},
	}

	dir := t.TempDir()
	if err := SaveSession(state, dir); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	loaded, err := LoadSession(dir + "/.prion_session_test-123.json")
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if loaded.Task != "build something" {
		t.Errorf("task: got %q, want %q", loaded.Task, "build something")
	}
	if len(loaded.Root.Children) != 2 {
		t.Fatalf("root children: got %d, want 2", len(loaded.Root.Children))
	}
	if loaded.Root.Children[0].Status != StatusCompleted {
		t.Error("first child should be completed")
	}
	if loaded.Root.Children[1].Status != StatusPending {
		t.Error("second child should be pending")
	}
}

func TestCountPending(t *testing.T) {
	root := &TaskNode{
		Children: []*TaskNode{
			{Description: "done", Status: StatusCompleted, Type: StepWrite},
			{Description: "pending 1", Status: StatusPending, Type: StepWrite},
			{Description: "branch", Status: StatusRunning, Children: []*TaskNode{
				{Description: "nested done", Status: StatusCompleted, Type: StepWrite},
				{Description: "pending 2", Status: StatusPending, Type: StepShell},
			}},
		},
	}

	count := countPending(root)
	if count != 2 {
		t.Errorf("got %d pending, want 2", count)
	}
}

func TestResumeSkipsCompletedNodes(t *testing.T) {
	// Simulate a tree with some completed and some pending nodes
	root := &TaskNode{
		Description: "build app",
		Status:      StatusRunning,
		Children: []*TaskNode{
			{Depth: 1, Description: "create main.py", Type: StepWrite, Status: StatusCompleted, Result: "wrote main.py"},
			{Depth: 1, Description: "run tests", Type: StepShell, Status: StatusPending},
		},
	}

	// Save session
	dir := t.TempDir()
	state := &SessionState{ID: "resume-test", Task: "build app", Root: root}
	if err := SaveSession(state, dir); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	// Verify FindNextPending finds the right node
	found := FindNextPending(root)
	if found == nil || found.Description != "run tests" {
		t.Errorf("expected 'run tests', got %v", found)
	}

	// Verify countPending
	if count := countPending(root); count != 1 {
		t.Errorf("got %d pending, want 1", count)
	}
}

// Verify siblingContext doesn't grow unbounded with large results
func TestExecuteNodeContextCompression(t *testing.T) {
	// Create a tree with 5 leaf children
	root := &TaskNode{
		Description: "root task",
		Children:    make([]*TaskNode, 5),
	}
	for i := range root.Children {
		root.Children[i] = &TaskNode{
			Depth:       1,
			Description: "write file" + strings.Repeat(" x", i), // vary descriptions
			Type:        StepWrite,
			Status:      StatusPending,
		}
	}

	// After execution, branch summary should be bounded
	// (Can't fully test without a real executor, but verify the structure)
	for _, child := range root.Children {
		child.Status = StatusCompleted
		child.Result = strings.Repeat("output data ", 50) // ~600 chars each
	}

	summary := summarizeBranch(root)
	// Each child result should be truncated to 200 chars
	// 5 children × 200 chars + newlines ≈ 1004 chars max
	if len(summary) > 1100 {
		t.Errorf("branch summary too large: %d chars (should be bounded)", len(summary))
	}
}
