package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/crussella0129/Animus_Prion/internal/core"
	"github.com/crussella0129/Animus_Prion/internal/llm"
	"github.com/crussella0129/Animus_Prion/internal/tools"
)

// SkeletonPlanner implements recursive skeleton tree decomposition.
type SkeletonPlanner struct {
	provider   llm.Provider
	executor   *ChunkedExecutor
	registry   *tools.Registry
	workspace  *core.Workspace
	maxDepth   int
	onProgress ProgressFunc
}

// NewSkeletonPlanner creates a skeleton tree planner.
func NewSkeletonPlanner(provider llm.Provider, registry *tools.Registry, ws *core.Workspace) *SkeletonPlanner {
	return &SkeletonPlanner{
		provider:  provider,
		executor:  NewChunkedExecutor(provider, registry, ws),
		registry:  registry,
		workspace: ws,
		maxDepth:  5,
	}
}

// SetProgress sets a callback for progress updates.
func (sp *SkeletonPlanner) SetProgress(fn ProgressFunc) {
	sp.onProgress = fn
}

func (sp *SkeletonPlanner) progress(format string, args ...interface{}) {
	if sp.onProgress != nil {
		sp.onProgress(fmt.Sprintf(format, args...))
	}
}

// Plan sends the user's original task to the model and parses the response
// into a tree of TaskNodes. The model responds naturally — usually a dashed list.
// Then recursively expands non-leaf nodes.
func (sp *SkeletonPlanner) Plan(ctx context.Context, task string) (*TaskNode, error) {
	sp.progress("Thinking...")

	// Pass 1: Send the original task. The model responds however it wants.
	// Small models naturally produce dashed lists.
	response, err := sp.provider.Generate(ctx, []llm.Message{
		{Role: "user", Content: task},
	}, llm.GenerateOptions{Temperature: 0.3, MaxTokens: 512})
	if err != nil {
		return nil, fmt.Errorf("initial decomposition failed: %w", err)
	}

	// Parse the response into top-level nodes
	children := parseSkeletonLevel(response, 1)

	// If the model didn't produce a list, try the structured decompose prompt
	if len(children) == 0 {
		response, err = sp.provider.Generate(ctx, []llm.Message{
			{Role: "user", Content: fmt.Sprintf(
				"Break this task into steps. Use dashes to mark each step.\n\nTask: %s\n\nRespond with a dashed list only:\n- ",
				task,
			)},
		}, llm.GenerateOptions{Temperature: 0.3, MaxTokens: 512})
		if err != nil {
			return nil, fmt.Errorf("structured decomposition failed: %w", err)
		}
		children = parseSkeletonLevel(response, 1)
	}

	// If still no list, treat the whole task as a single leaf
	if len(children) == 0 {
		return &TaskNode{
			Depth:       1,
			Description: task,
			Type:        inferStepType(task),
			Status:      StatusPending,
		}, nil
	}

	// Build root node
	root := &TaskNode{
		Depth:       0,
		Description: task,
		Type:        StepAnalyze, // root is never executed directly
		Status:      StatusPending,
		Children:    children,
	}

	sp.progress("Plan: %d top-level steps", len(children))

	// Recursively expand non-leaf children
	for _, child := range root.Children {
		if !child.IsLeaf() {
			sp.expandNode(ctx, child)
		}
	}

	return root, nil
}

// expandNode recursively decomposes a non-leaf node by asking the model
// to break it into sub-steps. Each round, the model gets the same simple prompt
// and produces a dashed list. It never knows it's recursing.
func (sp *SkeletonPlanner) expandNode(ctx context.Context, node *TaskNode) {
	if node.IsLeaf() || node.Depth >= sp.maxDepth {
		return
	}

	// Check for cancellation
	if ctx.Err() != nil {
		return
	}

	sp.progress("  Expanding: %s", truncate(node.Description, 60))

	prompt := fmt.Sprintf(
		"Break this task into sub-steps. Use dashes to mark each step.\n\nTask: %s\n\nRespond with a dashed list only:\n- ",
		node.Description,
	)

	response, err := sp.provider.Generate(ctx, []llm.Message{
		{Role: "user", Content: prompt},
	}, llm.GenerateOptions{Temperature: 0.3, MaxTokens: 256})
	if err != nil {
		// Non-fatal: treat as leaf if expansion fails
		return
	}

	children := parseSkeletonLevel(response, node.Depth+1)

	// --- "Can't go any farther" detection ---
	// Model bottomed out if it produced nothing
	if len(children) == 0 {
		return
	}

	// Model produced a single item — check if it's a restatement
	if len(children) == 1 {
		if similar(children[0].Description, node.Description) {
			return // restatement, treat parent as leaf
		}
		if len(children[0].Description) >= len(node.Description)*8/10 {
			return // barely shorter, not a real decomposition
		}
		// Single child that's genuinely simpler — keep it but don't recurse further
		node.Children = children
		return
	}

	node.Children = children

	// Recursively expand children that aren't leaves
	for _, child := range node.Children {
		if !child.IsLeaf() {
			sp.expandNode(ctx, child)
		}
	}
}

// Execute walks the tree depth-first and executes each leaf via ChunkedExecutor.ExecuteStep.
// Branch nodes get summaries from their children's results.
// Returns a PlanResult compatible with the existing pipeline.
func (sp *SkeletonPlanner) Execute(ctx context.Context, root *TaskNode) (PlanResult, error) {
	var allResults []StepResult
	allSuccess := true

	sp.executeNode(ctx, root, "", &allResults, &allSuccess)

	// Summary from last successful result
	summary := ""
	for i := len(allResults) - 1; i >= 0; i-- {
		if allResults[i].Error == nil && allResults[i].Output != "" {
			summary = allResults[i].Output
			break
		}
	}

	return PlanResult{
		Steps:   allResults,
		Success: allSuccess,
		Summary: summary,
	}, nil
}

// executeNode recursively executes the tree. Leaves run via ExecuteStep.
// Branch nodes collect child summaries. siblingContext carries sibling branch summaries.
func (sp *SkeletonPlanner) executeNode(ctx context.Context, node *TaskNode, siblingContext string, results *[]StepResult, allSuccess *bool) {
	if ctx.Err() != nil {
		return
	}

	// Leaf node — execute it
	if node.IsLeaf() || len(node.Children) == 0 {
		node.Status = StatusRunning
		sp.progress("  [exec] %s", truncate(node.Description, 60))

		step := &Step{
			Number:      node.Depth,
			Description: node.Description,
			Type:        node.Type,
			Status:      StatusPending,
		}

		result := sp.executor.ExecuteStep(ctx, step, siblingContext)
		*results = append(*results, result)

		if result.Error != nil {
			node.Status = StatusFailed
			node.Result = fmt.Sprintf("FAILED: %s", result.Error)
			*allSuccess = false
		} else {
			node.Status = StatusCompleted
			node.Result = result.Output
		}

		// Checkpoint after each leaf
		sp.checkpoint(node)
		return
	}

	// Branch node — execute children depth-first
	node.Status = StatusRunning
	var childContext strings.Builder
	if siblingContext != "" {
		childContext.WriteString(siblingContext)
		childContext.WriteString("\n")
	}

	for _, child := range node.Children {
		sp.executeNode(ctx, child, childContext.String(), results, allSuccess)

		// Feed completed child's result into context for next sibling
		if child.Status == StatusCompleted && child.Result != "" {
			childContext.WriteString(truncate(child.Result, 200))
			childContext.WriteString("\n")
		} else if child.Status == StatusFailed {
			childContext.WriteString(fmt.Sprintf("(previous step failed: %s)\n", truncate(child.Result, 100)))
		}
	}

	// Summarize branch
	node.Result = summarizeBranch(node)
	node.Status = StatusCompleted
}

// checkpoint saves the tree state after each leaf execution.
// Writes to .prion_session.json in the workspace root.
func (sp *SkeletonPlanner) checkpoint(node *TaskNode) {
	// Walk up to find root — for now, just log. Full implementation in Phase 3.
	// The tree is serializable via json.Marshal since all fields have json tags.
	_ = node // placeholder for session persistence
}

// --- Session State (Phase 2/3 — structure now, full implementation later) ---

// SessionState holds the full tree for persistence and resume.
type SessionState struct {
	ID           string    `json:"id"`
	Task         string    `json:"task"`
	Root         *TaskNode `json:"root"`
	CreatedFiles []string  `json:"created_files,omitempty"`
	Timestamp    time.Time `json:"timestamp"`
}

// SaveSession writes the session state to a JSON file.
func SaveSession(state *SessionState, dir string) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling session: %w", err)
	}
	path := filepath.Join(dir, fmt.Sprintf(".prion_session_%s.json", state.ID))
	return os.WriteFile(path, data, 0644)
}

// LoadSession reads a session state from a JSON file.
func LoadSession(path string) (*SessionState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading session: %w", err)
	}
	var state SessionState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parsing session: %w", err)
	}
	return &state, nil
}

// FindNextPending walks the tree and returns the first pending leaf node.
// Used for session resume.
func FindNextPending(node *TaskNode) *TaskNode {
	if node == nil {
		return nil
	}
	if (node.IsLeaf() || len(node.Children) == 0) && node.Status == StatusPending {
		return node
	}
	for _, child := range node.Children {
		if found := FindNextPending(child); found != nil {
			return found
		}
	}
	return nil
}

// PlanAndExecute is the top-level entry point: plan the tree, then execute it.
func (sp *SkeletonPlanner) PlanAndExecute(ctx context.Context, task string) (PlanResult, error) {
	root, err := sp.Plan(ctx, task)
	if err != nil {
		return PlanResult{}, err
	}
	return sp.Execute(ctx, root)
}
