package planner

import (
	"regexp"
	"strings"
)

// TaskNode is a single node in the decomposition tree.
// Branch nodes have Children. Leaf nodes have nil Children and get executed.
type TaskNode struct {
	Depth       int         `json:"depth"`       // 1 = top-level, 2 = first expansion, etc.
	Description string      `json:"description"` // plain English task description
	Type        StepType    `json:"type"`        // inferred by inferStepType
	Children    []*TaskNode `json:"children,omitempty"`
	Status      StepStatus  `json:"status"`
	Result      string      `json:"result,omitempty"` // execution output (leaf) or summary (branch)
}

// IsLeaf returns true if this node should be executed directly without further decomposition.
// The model never decides this — the heuristic does.
func (n *TaskNode) IsLeaf() bool {
	// Already has children from a previous expansion
	if len(n.Children) > 0 {
		return false
	}
	return isLeafHeuristic(n)
}

// dashItemPattern matches lines starting with - or * followed by text.
// This is the ONLY format the model needs to produce. No depth markers, no numbering.
var dashItemPattern = regexp.MustCompile(`(?m)^[-*]\s+(.+)$`)

// singleFilePattern matches a single file reference in a description.
var singleFilePattern = regexp.MustCompile(`\b[\w/.-]+\.\w{1,4}\b`)

// parseSkeletonLevel parses the model's dashed-list output into TaskNodes at the given depth.
// The model always produces the same format: "- do this thing". The system assigns depth.
func parseSkeletonLevel(response string, depth int) []*TaskNode {
	matches := dashItemPattern.FindAllStringSubmatch(response, -1)
	var nodes []*TaskNode
	for _, m := range matches {
		desc := strings.TrimSpace(m[1])
		if desc == "" {
			continue
		}
		nodes = append(nodes, &TaskNode{
			Depth:       depth,
			Description: desc,
			Type:        inferStepType(desc),
			Status:      StatusPending,
		})
	}
	return nodes
}

// isLeafHeuristic determines if a node is atomic enough to execute directly.
// Rules are ordered by confidence. The model never makes this decision.
func isLeafHeuristic(node *TaskNode) bool {
	desc := node.Description
	words := strings.Fields(desc)

	// Rule 1: Short description that maps to a known step type
	if len(words) <= 12 {
		stepType := inferStepType(desc)
		if stepType != StepAnalyze { // StepAnalyze is the "I don't know" default
			return true
		}
	}

	// Rule 2: References exactly one file — single-file operations are atomic
	files := singleFilePattern.FindAllString(desc, -1)
	if len(files) == 1 {
		return true
	}

	// Rule 3: Hard depth limit — never recurse past depth 5
	if node.Depth >= 5 {
		return true
	}

	return false
}

// similar checks if two descriptions are close enough to be a restatement.
// Uses word-overlap ratio. No embeddings, no LLM call.
func similar(a, b string) bool {
	wordsA := strings.Fields(strings.ToLower(a))
	wordsB := strings.Fields(strings.ToLower(b))
	if len(wordsA) == 0 || len(wordsB) == 0 {
		return false
	}
	setB := make(map[string]bool)
	for _, w := range wordsB {
		setB[w] = true
	}
	overlap := 0
	for _, w := range wordsA {
		if setB[w] {
			overlap++
		}
	}
	// Use the shorter list as denominator — detects when one description
	// is a subset/elaboration of the other (the restatement we want to catch).
	minLen := len(wordsA)
	if len(wordsB) < minLen {
		minLen = len(wordsB)
	}
	ratio := float64(overlap) / float64(minLen)
	return ratio > 0.7
}

// summarizeBranch concatenates child results into a short branch summary.
// Option A: deterministic, no LLM call. Good enough for small models.
func summarizeBranch(node *TaskNode) string {
	var parts []string
	for _, child := range node.Children {
		if child.Status == StatusCompleted && child.Result != "" {
			r := child.Result
			if len(r) > 200 {
				r = r[:200] + "..."
			}
			parts = append(parts, r)
		}
	}
	return strings.Join(parts, "\n")
}
