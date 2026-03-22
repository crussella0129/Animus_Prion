package planner

import (
	"testing"
)

func TestParseSkeletonLevel(t *testing.T) {
	response := `Ok here's what I need to do:
- create the src directory
- write main.py with the entry point
- install dependencies with pip
`
	nodes := parseSkeletonLevel(response, 1)
	if len(nodes) != 3 {
		t.Fatalf("got %d nodes, want 3", len(nodes))
	}
	if nodes[0].Description != "create the src directory" {
		t.Errorf("node 0: got %q", nodes[0].Description)
	}
	if nodes[0].Depth != 1 {
		t.Errorf("node 0 depth: got %d, want 1", nodes[0].Depth)
	}
	if nodes[1].Type != StepWrite {
		t.Errorf("node 1 type: got %v, want StepWrite", nodes[1].Type)
	}
	if nodes[2].Type != StepShell {
		t.Errorf("node 2 type: got %v, want StepShell", nodes[2].Type)
	}
}

func TestParseSkeletonLevelAsteriskBullets(t *testing.T) {
	response := `* read the config file
* update the timeout
* run tests`
	nodes := parseSkeletonLevel(response, 3)
	if len(nodes) != 3 {
		t.Fatalf("got %d nodes, want 3", len(nodes))
	}
	if nodes[0].Depth != 3 {
		t.Errorf("depth: got %d, want 3", nodes[0].Depth)
	}
}

func TestParseSkeletonLevelSkipsNonListContent(t *testing.T) {
	response := `Sure, here's the breakdown:

- first step
- second step

Let me know if you need more detail.`
	nodes := parseSkeletonLevel(response, 1)
	if len(nodes) != 2 {
		t.Fatalf("got %d nodes, want 2", len(nodes))
	}
}

func TestIsLeafShortAndTyped(t *testing.T) {
	node := &TaskNode{Depth: 2, Description: "create src/main.py with entry point", Type: StepWrite}
	if !node.IsLeaf() {
		t.Error("short, typed, single-file node should be a leaf")
	}
}

func TestIsLeafLongDescriptionNotLeaf(t *testing.T) {
	node := &TaskNode{
		Depth:       1,
		Description: "write the entire data processing pipeline including the scraper the cache layer the CLI parser and the test suite",
		Type:        StepWrite,
	}
	if node.IsLeaf() {
		t.Error("13+ word description should not be a leaf")
	}
}

func TestIsLeafSingleFileOverridesLength(t *testing.T) {
	// Long description but references exactly one file
	node := &TaskNode{
		Depth:       2,
		Description: "write a comprehensive retry wrapper with exponential backoff in src/retry.py",
		Type:        StepWrite,
	}
	if !node.IsLeaf() {
		t.Error("single-file reference should force leaf regardless of length")
	}
}

func TestIsLeafDepthLimit(t *testing.T) {
	node := &TaskNode{Depth: 5, Description: "something vague and long and complicated and multi-file"}
	if !node.IsLeaf() {
		t.Error("depth 5 should force leaf regardless of content")
	}
}

func TestIsLeafWithChildren(t *testing.T) {
	node := &TaskNode{
		Depth:       1,
		Description: "short task",
		Type:        StepWrite,
		Children:    []*TaskNode{{Description: "child"}},
	}
	if node.IsLeaf() {
		t.Error("node with children should not be a leaf")
	}
}

func TestSimilar(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"write the scraper module", "write the scraper module for weather data", true},
		{"create src directory", "install pip dependencies", false},
		{"", "something", false},
		{"write main.py", "write main.py", true},
	}
	for _, tt := range tests {
		got := similar(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("similar(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestSummarizeBranch(t *testing.T) {
	node := &TaskNode{
		Children: []*TaskNode{
			{Status: StatusCompleted, Result: "Wrote 100 bytes to main.py"},
			{Status: StatusFailed, Result: "error happened"},
			{Status: StatusCompleted, Result: "Wrote 200 bytes to lib.py"},
		},
	}
	summary := summarizeBranch(node)
	if summary == "" {
		t.Error("summary should not be empty")
	}
	// Should include completed results, not failed
	if !containsStr(summary, "main.py") || !containsStr(summary, "lib.py") {
		t.Errorf("summary missing completed results: %q", summary)
	}
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
