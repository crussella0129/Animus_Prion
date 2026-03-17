package planner

import (
	"fmt"
	"strings"

	"github.com/crussella0129/Animus_Prion/internal/llm"
	"github.com/crussella0129/Animus_Prion/internal/tools"
)

// Decomposer uses an LLM to break tasks into numbered steps.
type Decomposer struct {
	provider llm.Provider
	registry *tools.Registry
}

// NewDecomposer creates a task decomposer.
func NewDecomposer(provider llm.Provider, registry *tools.Registry) *Decomposer {
	return &Decomposer{
		provider: provider,
		registry: registry,
	}
}

// Decompose breaks a user task into a plan of numbered steps.
func (d *Decomposer) Decompose(task string, cwd string) ([]Step, error) {
	// Build minimal prompt — no tools, no history
	toolNames := d.registry.List()

	prompt := fmt.Sprintf(
		`Break the following task into numbered steps (max %d steps).
Each step should be a single, concrete action.

Available tools: %s
Current directory: %s

Task: %s

Respond with numbered steps only. Example format:
1. Read the configuration file
2. Modify the timeout setting
3. Run the tests`,
		MaxSteps,
		strings.Join(toolNames, ", "),
		cwd,
		task,
	)

	messages := []llm.Message{
		{Role: "user", Content: prompt},
	}

	response, err := d.provider.Generate(messages, llm.GenerateOptions{
		Temperature: 0.3, // Low temperature for structured output
		MaxTokens:   512, // Plans should be concise
	})
	if err != nil {
		return nil, fmt.Errorf("decomposition failed: %w", err)
	}

	steps := ParsePlan(response)
	if len(steps) == 0 {
		return nil, fmt.Errorf("no steps extracted from plan")
	}

	return steps, nil
}

// SplitConjunctions splits multi-part tasks into sub-tasks.
// Handles temporal ("then"), verb-based (", and <verb>"), and sentence boundaries.
func SplitConjunctions(task string) []string {
	// Temporal conjunctions
	temporals := []string{" then ", " and then ", " after that ", " next ", " finally "}
	for _, t := range temporals {
		if strings.Contains(strings.ToLower(task), t) {
			parts := strings.SplitN(strings.ToLower(task), t, 2)
			if len(parts) == 2 {
				return []string{strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])}
			}
		}
	}

	// Sentence boundaries
	sentences := strings.Split(task, ". ")
	if len(sentences) > 1 {
		var result []string
		for _, s := range sentences {
			s = strings.TrimSpace(s)
			if s != "" {
				result = append(result, s)
			}
		}
		if len(result) > 1 {
			return result
		}
	}

	return []string{task}
}
