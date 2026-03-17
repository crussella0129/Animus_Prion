// Package planner provides the plan-then-execute pipeline.
// Plans are decomposed by LLM, parsed by hardcoded regex, and executed step-by-step.
package planner

import (
	"fmt"
	"regexp"
	"strings"
)

// StepType classifies what kind of operation a step performs.
type StepType int

const (
	StepRead StepType = iota
	StepWrite
	StepShell
	StepGit
	StepAnalyze
	StepGenerate
)

func (s StepType) String() string {
	switch s {
	case StepRead:
		return "READ"
	case StepWrite:
		return "WRITE"
	case StepShell:
		return "SHELL"
	case StepGit:
		return "GIT"
	case StepAnalyze:
		return "ANALYZE"
	case StepGenerate:
		return "GENERATE"
	default:
		return "UNKNOWN"
	}
}

// AllowedTools returns the tool names allowed for this step type.
func (s StepType) AllowedTools() []string {
	switch s {
	case StepRead:
		return []string{"read_file", "list_files", "manifold_search"}
	case StepWrite:
		return []string{"read_file", "write_file", "list_files"}
	case StepShell:
		return []string{"run_shell", "read_file"}
	case StepGit:
		return []string{"git_init", "git_status", "git_diff", "git_log", "git_add", "git_commit", "git_branch", "git_checkout"}
	case StepAnalyze:
		return []string{"read_file", "list_files", "manifold_search"}
	case StepGenerate:
		return []string{"read_file", "write_file", "list_files"}
	default:
		return nil
	}
}

// StepStatus tracks execution progress.
type StepStatus int

const (
	StatusPending StepStatus = iota
	StatusRunning
	StatusCompleted
	StatusFailed
	StatusSkipped
)

func (s StepStatus) String() string {
	switch s {
	case StatusPending:
		return "PENDING"
	case StatusRunning:
		return "RUNNING"
	case StatusCompleted:
		return "COMPLETED"
	case StatusFailed:
		return "FAILED"
	case StatusSkipped:
		return "SKIPPED"
	default:
		return "UNKNOWN"
	}
}

// Step is an atomic unit of work in a plan.
type Step struct {
	Number      int
	Description string
	Type        StepType
	Status      StepStatus
}

// StepResult holds the outcome of executing a single step.
type StepResult struct {
	Step    *Step
	Output  string
	Error   error
}

// PlanResult holds the outcome of executing an entire plan.
type PlanResult struct {
	Steps   []StepResult
	Success bool
	Summary string
}

// MaxSteps is the hard cap on plan steps.
const MaxSteps = 7

// stepPattern matches numbered steps like "1. Do something" or "Step 1: Do something"
var stepPattern = regexp.MustCompile(`(?i)^\s*(?:step\s+)?(\d+)[.):\-]\s*(.+)$`)

// File extension patterns for step type inference (prioritized over keywords).
var fileExtPatterns = map[string]StepType{
	".py":   StepWrite,
	".go":   StepWrite,
	".js":   StepWrite,
	".ts":   StepWrite,
	".rs":   StepWrite,
	".java": StepWrite,
	".c":    StepWrite,
	".cpp":  StepWrite,
	".h":    StepWrite,
	".md":   StepWrite,
	".yaml": StepWrite,
	".yml":  StepWrite,
	".json": StepWrite,
	".toml": StepWrite,
}

// Keyword patterns for step type inference.
var keywordPatterns = map[string]StepType{
	"read":      StepRead,
	"examine":   StepRead,
	"inspect":   StepRead,
	"review":    StepRead,
	"look at":   StepRead,
	"check":     StepRead,
	"write":     StepWrite,
	"create":    StepWrite,
	"modify":    StepWrite,
	"update":    StepWrite,
	"edit":      StepWrite,
	"add":       StepWrite,
	"implement": StepWrite,
	"run":       StepShell,
	"execute":   StepShell,
	"test":      StepShell,
	"install":   StepShell,
	"build":     StepShell,
	"compile":   StepShell,
	"git":       StepGit,
	"commit":    StepGit,
	"branch":    StepGit,
	"merge":     StepGit,
	"analyze":   StepAnalyze,
	"find":      StepAnalyze,
	"search":    StepAnalyze,
	"identify":  StepAnalyze,
	"generate":  StepGenerate,
	"produce":   StepGenerate,
	"output":    StepGenerate,
}

// ParsePlan extracts steps from raw LLM plan text.
// Returns at most MaxSteps steps.
func ParsePlan(text string) []Step {
	lines := strings.Split(text, "\n")
	var steps []Step

	for _, line := range lines {
		if len(steps) >= MaxSteps {
			break
		}

		matches := stepPattern.FindStringSubmatch(line)
		if matches == nil {
			continue
		}

		num := 0
		fmt.Sscanf(matches[1], "%d", &num)
		desc := strings.TrimSpace(matches[2])

		steps = append(steps, Step{
			Number:      num,
			Description: desc,
			Type:        inferStepType(desc),
			Status:      StatusPending,
		})
	}

	return steps
}

// inferStepType determines the step type from its description.
// File extensions are checked first (higher priority), then keywords.
func inferStepType(description string) StepType {
	lower := strings.ToLower(description)

	// Priority 1: File extension patterns
	for ext, st := range fileExtPatterns {
		if strings.Contains(lower, ext) {
			return st
		}
	}

	// Priority 2: Keyword patterns
	for keyword, st := range keywordPatterns {
		if strings.Contains(lower, keyword) {
			return st
		}
	}

	// Default
	return StepAnalyze
}

// IsSimpleTask checks if an input is simple enough to skip planning.
func IsSimpleTask(input string) bool {
	words := strings.Fields(input)
	// Very short tasks don't need plans
	if len(words) <= 5 {
		return true
	}
	// No conjunctions suggesting multi-step work
	conjunctions := []string{" then ", " and then ", " after that ", " next ", " finally "}
	lower := strings.ToLower(input)
	for _, conj := range conjunctions {
		if strings.Contains(lower, conj) {
			return false
		}
	}
	// Count verbs — multiple action verbs suggest multi-step
	verbs := 0
	actionVerbs := []string{"create", "write", "read", "run", "test", "build", "fix", "add", "remove", "update", "modify", "install"}
	for _, v := range actionVerbs {
		if strings.Contains(lower, v) {
			verbs++
		}
	}
	return verbs <= 1
}
