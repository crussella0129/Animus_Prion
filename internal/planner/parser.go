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
	Step   *Step
	Output string
	Error  error
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

// stepPattern entry pairs a keyword/extension with a step type.
type stepPatternEntry struct {
	keyword  string
	stepType StepType
}

// File extension patterns for step type inference (prioritized over keywords).
// Ordered slice for deterministic matching (Go map iteration is randomized).
var fileExtPatterns = []stepPatternEntry{
	{".c", StepWrite},
	{".cpp", StepWrite},
	{".go", StepWrite},
	{".h", StepWrite},
	{".java", StepWrite},
	{".js", StepWrite},
	{".json", StepWrite},
	{".md", StepWrite},
	{".py", StepWrite},
	{".rs", StepWrite},
	{".toml", StepWrite},
	{".ts", StepWrite},
	{".yaml", StepWrite},
	{".yml", StepWrite},
}

// Keyword patterns for step type inference.
// Ordered by specificity: multi-word first, then alphabetical within each step type.
var keywordPatterns = []stepPatternEntry{
	// Read (check these first — "look at" is multi-word)
	{"look at", StepRead},
	{"check", StepRead},
	{"examine", StepRead},
	{"inspect", StepRead},
	{"read", StepRead},
	{"review", StepRead},
	// Write
	{"add", StepWrite},
	{"create", StepWrite},
	{"edit", StepWrite},
	{"implement", StepWrite},
	{"modify", StepWrite},
	{"update", StepWrite},
	{"write", StepWrite},
	// Shell
	{"build", StepShell},
	{"compile", StepShell},
	{"execute", StepShell},
	{"install", StepShell},
	{"run", StepShell},
	{"test", StepShell},
	// Git
	{"branch", StepGit},
	{"commit", StepGit},
	{"git", StepGit},
	{"merge", StepGit},
	// Analyze
	{"analyze", StepAnalyze},
	{"find", StepAnalyze},
	{"identify", StepAnalyze},
	{"search", StepAnalyze},
	// Generate
	{"generate", StepGenerate},
	{"output", StepGenerate},
	{"produce", StepGenerate},
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
// Both pattern lists are ordered slices for deterministic matching.
func inferStepType(description string) StepType {
	lower := strings.ToLower(description)

	// Priority 1: File extension patterns
	for _, p := range fileExtPatterns {
		if strings.Contains(lower, p.keyword) {
			return p.stepType
		}
	}

	// Priority 2: Keyword patterns
	for _, p := range keywordPatterns {
		if strings.Contains(lower, p.keyword) {
			return p.stepType
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
