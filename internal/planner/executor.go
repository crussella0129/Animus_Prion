package planner

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/crussella0129/Animus_Prion/internal/core"
	"github.com/crussella0129/Animus_Prion/internal/llm"
	"github.com/crussella0129/Animus_Prion/internal/tools"
)

// ChunkedExecutor executes individual plan steps with fresh context.
type ChunkedExecutor struct {
	provider   llm.Provider
	fullRegistry *tools.Registry
	workspace  *core.Workspace
	maxStepTurns int
	maxToolCalls int
}

// NewChunkedExecutor creates a step executor.
func NewChunkedExecutor(provider llm.Provider, registry *tools.Registry, ws *core.Workspace) *ChunkedExecutor {
	// Scale limits with model tier
	maxTurns := 6
	maxCalls := 6
	caps := provider.Capabilities()
	switch caps.SizeTier {
	case "medium":
		maxTurns = 8
		maxCalls = 8
	case "large":
		maxTurns = 12
		maxCalls = 12
	}

	return &ChunkedExecutor{
		provider:     provider,
		fullRegistry: registry,
		workspace:    ws,
		maxStepTurns: maxTurns,
		maxToolCalls: maxCalls,
	}
}

// ExecuteStep runs a single plan step with filtered tools and scope enforcement.
func (e *ChunkedExecutor) ExecuteStep(step *Step, learnedContext string) StepResult {
	step.Status = StatusRunning

	// Filter tools to only those allowed for this step type
	filtered := tools.NewRegistry()
	allowed := step.Type.AllowedTools()
	for _, name := range allowed {
		if t, ok := e.fullRegistry.Get(name); ok {
			filtered.Register(t)
		}
	}

	// Build per-step prompt with explicit tool restriction and format example
	toolNames := filtered.List()
	prompt := fmt.Sprintf(
		"Execute this step by calling a tool: %s\n\n"+
			"Available tools: %s\n"+
			"Working directory: %s\n\n"+
			"You MUST respond with a JSON tool call. Example:\n"+
			"{\"name\": \"write_file\", \"arguments\": {\"path\": \"example.py\", \"content\": \"print('hello')\"}}\n\n"+
			"Do it now. Respond with ONLY the JSON tool call, no explanation.\n",
		step.Description,
		strings.Join(toolNames, ", "),
		e.workspace.CWD(),
	)

	if learnedContext != "" {
		prompt += "\nContext from previous steps:\n" + learnedContext + "\n"
	}

	// Fresh context per step — no history carryover
	messages := []llm.Message{
		{Role: "system", Content: buildStepSystemPrompt(filtered)},
		{Role: "user", Content: prompt},
	}

	var lastOutput string
	toolCallCount := 0
	outOfScopeCount := 0

	// Agentic loop per step
	for turn := 0; turn < e.maxStepTurns; turn++ {
		schemas := filtered.ToOpenAISchemas()
		toolsAny := make([]any, len(schemas))
		for i, s := range schemas {
			toolsAny[i] = s
		}
		response, err := e.provider.Generate(messages, llm.GenerateOptions{
			Temperature: 0.5,
			MaxTokens:   2048,
			Tools:       toolsAny,
		})
		if err != nil {
			step.Status = StatusFailed
			return StepResult{Step: step, Error: fmt.Errorf("generation failed: %w", err)}
		}

		messages = append(messages, llm.Message{Role: "assistant", Content: response})

		// Parse tool calls
		toolCalls := core.ParseToolCalls(response)
		if len(toolCalls) == 0 {
			// No tool calls → step is complete
			step.Status = StatusCompleted
			lastOutput = response
			break
		}

		// Execute tool calls with scope enforcement
		for _, tc := range toolCalls {
			// Scope check: is this tool in our filtered set?
			if _, ok := filtered.Get(tc.Name); !ok {
				outOfScopeCount++
				if outOfScopeCount >= 2 {
					// Hard block: terminate step after 2nd violation
					step.Status = StatusFailed
					return StepResult{
						Step:   step,
						Output: lastOutput,
						Error:  fmt.Errorf("step terminated: repeated out-of-scope tool use (%s)", tc.Name),
					}
				}
				messages = append(messages, llm.Message{
					Role:    "user",
					Content: fmt.Sprintf("Tool '%s' is NOT available for this step. Use only: %s", tc.Name, strings.Join(toolNames, ", ")),
				})
				continue
			}

			toolCallCount++
			if toolCallCount > e.maxToolCalls {
				step.Status = StatusCompleted
				return StepResult{Step: step, Output: lastOutput}
			}

			result, err := e.fullRegistry.Execute(tc.Name, tc.Arguments)
			if err != nil {
				errMsg := fmt.Sprintf("Error: %s — Do NOT retry this exact call.", err.Error())
				messages = append(messages, llm.Message{Role: "tool", Content: errMsg, Name: tc.Name})
			} else {
				messages = append(messages, llm.Message{Role: "tool", Content: result, Name: tc.Name})
				lastOutput = result
			}
		}
	}

	if step.Status != StatusCompleted {
		step.Status = StatusCompleted // reached max turns — treat as done
	}

	return StepResult{Step: step, Output: lastOutput}
}

func buildStepSystemPrompt(registry *tools.Registry) string {
	var sb strings.Builder
	sb.WriteString("You are executing a single step of a larger plan. Focus only on this step.\n\n")

	// Platform awareness (mirrors agent's defaultSystemPrompt)
	sb.WriteString(fmt.Sprintf("Platform: %s/%s\n", runtime.GOOS, runtime.GOARCH))
	switch runtime.GOOS {
	case "windows":
		sb.WriteString("Shared libraries use .dll extension (e.g. mylib.dll). Executables use .exe.\n")
	case "darwin":
		sb.WriteString("Shared libraries use .dylib extension (e.g. libmylib.dylib).\n")
	default:
		sb.WriteString("Shared libraries use .so extension (e.g. libmylib.so).\n")
	}
	sb.WriteString("\n")

	sb.WriteString("Available tools:\n")
	for _, name := range registry.List() {
		t, _ := registry.Get(name)
		sb.WriteString(fmt.Sprintf("- %s: %s\n", name, t.Description()))
	}
	sb.WriteString("\nTo create files, use write_file — it creates parent directories automatically.\n")
	sb.WriteString("Do NOT use run_shell for mkdir or file creation. Use write_file directly.\n")
	sb.WriteString("Respond with a JSON tool call or plain text when done.\n")
	return sb.String()
}

// ProgressFunc is called with status updates during plan execution.
// If nil, progress is silent.
type ProgressFunc func(msg string)

// PlanExecutor orchestrates the full plan-then-execute pipeline.
type PlanExecutor struct {
	decomposer *Decomposer
	executor   *ChunkedExecutor
	workspace  *core.Workspace
	onProgress ProgressFunc
}

// NewPlanExecutor creates a plan executor.
func NewPlanExecutor(provider llm.Provider, registry *tools.Registry, ws *core.Workspace) *PlanExecutor {
	return &PlanExecutor{
		decomposer: NewDecomposer(provider, registry),
		executor:   NewChunkedExecutor(provider, registry, ws),
		workspace:  ws,
	}
}

// SetProgress sets a callback for progress updates (displayed to the user).
func (pe *PlanExecutor) SetProgress(fn ProgressFunc) {
	pe.onProgress = fn
}

func (pe *PlanExecutor) progress(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	if pe.onProgress != nil {
		pe.onProgress(msg)
	}
}

// Execute runs the full plan-then-execute pipeline.
func (pe *PlanExecutor) Execute(task string) (PlanResult, error) {
	// Skip planning for simple tasks
	if IsSimpleTask(task) {
		pe.progress("Executing directly...")
		step := Step{Number: 1, Description: task, Type: inferStepType(task), Status: StatusPending}
		result := pe.executor.ExecuteStep(&step, "")
		return PlanResult{
			Steps:   []StepResult{result},
			Success: result.Error == nil,
			Summary: result.Output,
		}, nil
	}

	// Decompose task into steps
	pe.progress("Planning...")
	steps, err := pe.decomposer.Decompose(task, pe.workspace.CWD())
	if err != nil {
		return PlanResult{}, fmt.Errorf("planning failed: %w", err)
	}
	pe.progress("Plan: %d steps", len(steps))

	// Heuristic fallback: if LLM produces ≤1 step for multi-conjunction task
	parts := SplitConjunctions(task)
	if len(steps) <= 1 && len(parts) > 1 {
		steps = nil
		for i, part := range parts {
			steps = append(steps, Step{
				Number:      i + 1,
				Description: part,
				Type:        inferStepType(part),
				Status:      StatusPending,
			})
		}
	}

	// Execute steps sequentially with inter-step context
	var results []PlanResult
	var learnedContext strings.Builder
	allSuccess := true

	for i := range steps {
		pe.progress("  [%d/%d] %s", steps[i].Number, len(steps), steps[i].Description)

		result := pe.executor.ExecuteStep(&steps[i], learnedContext.String())
		results = append(results, PlanResult{
			Steps:   []StepResult{result},
			Success: result.Error == nil,
		})

		if result.Error != nil {
			allSuccess = false
			learnedContext.WriteString(fmt.Sprintf("Step %d FAILED: %s\n", steps[i].Number, result.Error))
		} else if result.Output != "" {
			learnedContext.WriteString(fmt.Sprintf("Step %d result: %s\n", steps[i].Number, truncate(result.Output, 500)))
		}
	}

	// --- Requirements Completeness Check ---
	// Extract expected files from the original task and verify each exists.
	// If any are missing, inject steps to create them before verification.
	missingResults := pe.checkCompleteness(task, steps, &learnedContext)
	results = append(results, missingResults...)

	// --- Verify-and-Repair Loop ---
	// After all steps (including any completeness fills), verify compilation.
	allSteps := make([]Step, len(steps))
	copy(allSteps, steps)
	// Include any steps added by completeness check
	for _, mr := range missingResults {
		for _, sr := range mr.Steps {
			allSteps = append(allSteps, *sr.Step)
		}
	}
	verifyResults := pe.verifyAndRepair(allSteps, &learnedContext)
	results = append(results, verifyResults...)

	// Build combined result
	var allResults []StepResult
	for _, r := range results {
		allResults = append(allResults, r.Steps...)
	}

	// Summary from last successful step
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

// checkCompleteness extracts expected deliverables from the task prompt,
// checks which ones exist in the workspace, and creates steps for any missing ones.
func (pe *PlanExecutor) checkCompleteness(task string, steps []Step, learnedContext *strings.Builder) []PlanResult {
	expected := extractExpectedFiles(task)
	if len(expected) == 0 {
		return nil
	}

	var missing []string
	for _, f := range expected {
		path := filepath.Join(pe.workspace.CWD(), f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			missing = append(missing, f)
		}
	}

	if len(missing) == 0 {
		pe.progress("  All %d expected files present", len(expected))
		return nil
	}

	pe.progress("  Missing %d files: %v", len(missing), missing)
	learnedContext.WriteString(fmt.Sprintf("\nMissing files that were requested: %s\n", strings.Join(missing, ", ")))

	var results []PlanResult
	// Create a single step to write all missing files
	missingDesc := fmt.Sprintf("Create the missing files: %s", strings.Join(missing, ", "))
	fillStep := Step{
		Number:      len(steps) + 1,
		Description: missingDesc,
		Type:        StepWrite,
		Status:      StatusPending,
	}

	fillResult := pe.executor.ExecuteStep(&fillStep, learnedContext.String()+
		fmt.Sprintf("\nThe following files were requested but not yet created: %s\n"+
			"Original task: %s\n"+
			"Create these missing files now, using the context from previous steps.\n",
			strings.Join(missing, ", "), truncate(task, 500)))

	results = append(results, PlanResult{
		Steps:   []StepResult{fillResult},
		Success: fillResult.Error == nil,
	})

	if fillResult.Output != "" {
		learnedContext.WriteString(fmt.Sprintf("Fill step result: %s\n", truncate(fillResult.Output, 500)))
	}

	return results
}

// extractExpectedFiles pulls file names and paths from a task description.
// Uses regex patterns to find explicit file references in natural language.
// This is deterministic (no LLM call), fast, and catches the common patterns.
func extractExpectedFiles(task string) []string {
	var files []string
	seen := make(map[string]bool)

	// Pattern 1: Explicit file paths with extensions
	// Matches: main.py, src/lib.rs, Cargo.toml, handler.go, etc.
	filePattern := regexp.MustCompile(`(?i)\b((?:[\w./\\-]+/)?[\w-]+\.(?:py|rs|go|js|ts|toml|yaml|yml|json|c|cpp|h|java|rb|sh))\b`)
	for _, m := range filePattern.FindAllString(task, -1) {
		m = strings.TrimSpace(m)
		if !seen[m] {
			seen[m] = true
			files = append(files, m)
		}
	}

	// Pattern 2: Cargo.toml implied by "Rust library" or "cdylib"
	if !seen["Cargo.toml"] {
		if regexp.MustCompile(`(?i)cargo\.toml|cargo|cdylib|rust\s+library`).MatchString(task) {
			files = append(files, "Cargo.toml")
			seen["Cargo.toml"] = true
		}
	}

	return files
}

// verifyAndRepair checks if code was written and runs verification commands.
// If verification fails, it creates a repair step with the error output.
// Returns additional PlanResults from verify/repair steps.
func (pe *PlanExecutor) verifyAndRepair(steps []Step, learnedContext *strings.Builder) []PlanResult {
	var results []PlanResult

	// Detect what was written by looking at step types and descriptions
	verifyCmd := detectVerifyCommand(steps, pe.workspace.CWD())
	if verifyCmd == "" {
		return nil // nothing to verify
	}

	pe.progress("  Verifying: %s", verifyCmd)

	// Run the verification command directly via the tool registry
	verifyStep := Step{
		Number:      len(steps) + 1,
		Description: fmt.Sprintf("Verify: %s", verifyCmd),
		Type:        StepShell,
		Status:      StatusPending,
	}

	verifyResult := pe.executor.ExecuteStep(&verifyStep, learnedContext.String()+
		fmt.Sprintf("\nRun this command to verify the code: %s\nIf it fails, do NOT fix anything — just report the error.\n", verifyCmd))

	results = append(results, PlanResult{
		Steps:   []StepResult{verifyResult},
		Success: verifyResult.Error == nil,
	})

	// Check if verification failed
	if verifyResult.Error != nil || containsError(verifyResult.Output) {
		errorOutput := verifyResult.Output
		if verifyResult.Error != nil {
			errorOutput = verifyResult.Error.Error()
		}

		pe.progress("  Verification failed — repairing...")
		learnedContext.WriteString(fmt.Sprintf("\nVerification FAILED:\n%s\n", truncate(errorOutput, 1000)))

		// Repair step: give the model the error and ask it to fix
		repairStep := Step{
			Number:      len(steps) + 2,
			Description: "Fix the errors found during verification",
			Type:        StepWrite,
			Status:      StatusPending,
		}

		repairResult := pe.executor.ExecuteStep(&repairStep, learnedContext.String()+
			"\nThe code failed verification. Read the error above and fix the source files. "+
			"Then re-run the verification command to confirm the fix.\n")

		results = append(results, PlanResult{
			Steps:   []StepResult{repairResult},
			Success: repairResult.Error == nil,
		})
	} else {
		pe.progress("  Verification passed")
	}

	return results
}

// detectVerifyCommand determines what verification to run.
// Uses TWO strategies: step description keywords AND filesystem probing.
// Filesystem probing catches cases where the agent loop (not planner) wrote files.
func detectVerifyCommand(steps []Step, cwd string) string {
	wroteRust := false
	wrotePython := false
	wroteGo := false

	// Strategy 1: Step description keywords
	for _, s := range steps {
		lower := strings.ToLower(s.Description)
		if s.Type == StepWrite || strings.Contains(lower, "write") || strings.Contains(lower, "create") {
			if strings.Contains(lower, ".rs") || strings.Contains(lower, "cargo") || strings.Contains(lower, "rust") {
				wroteRust = true
			}
			if strings.Contains(lower, ".py") || strings.Contains(lower, "python") {
				wrotePython = true
			}
			if strings.Contains(lower, ".go") || strings.Contains(lower, "golang") {
				wroteGo = true
			}
		}
	}

	// Strategy 2: Filesystem probing — check what actually exists
	if !wroteRust {
		if _, err := os.Stat(filepath.Join(cwd, "Cargo.toml")); err == nil {
			wroteRust = true
		}
	}
	if !wroteGo {
		if _, err := os.Stat(filepath.Join(cwd, "go.mod")); err == nil {
			wroteGo = true
		}
	}
	if !wrotePython {
		// Check for any .py file
		matches, _ := filepath.Glob(filepath.Join(cwd, "*.py"))
		if len(matches) > 0 {
			wrotePython = true
		}
	}

	// Build verification command based on what was detected
	switch {
	case wroteRust && wrotePython:
		return "cargo build"
	case wroteRust:
		return "cargo build"
	case wroteGo:
		return "go build ./..."
	case wrotePython:
		return "python -m py_compile main.py"
	default:
		return ""
	}
}

// errorIndicators are regex patterns that detect actual build/runtime failures
// while avoiding false positives on code references like error_handler or ErrorType.
var errorIndicators = []*regexp.Regexp{
	regexp.MustCompile(`(?im)^error\b`),                 // line starts with "error"
	regexp.MustCompile(`(?i)\berror\[E\d+\]`),           // Rust error codes: error[E0433]
	regexp.MustCompile(`(?i)error:\s+`),                  // "error: message"
	regexp.MustCompile(`(?i)\bfailed\b.*\bcompil`),       // "failed to compile"
	regexp.MustCompile(`(?im)^Traceback`),                // Python traceback
	regexp.MustCompile(`(?i)SyntaxError|CompileError`),   // specific error types
	regexp.MustCompile(`(?i)could not compile`),          // Rust catch-all
	regexp.MustCompile(`(?i)\b[1-9]\d*\s+errors?\b`),    // "1 error generated" (but not "0 errors")
	regexp.MustCompile(`(?i)cannot find module`),         // Go module errors
	regexp.MustCompile(`(?i)no such file or directory`),  // filesystem errors
	regexp.MustCompile(`(?i)unresolved import`),          // Rust unresolved
	regexp.MustCompile(`(?i)undefined reference`),        // linker errors
}

// zeroErrorsPattern matches "0 errors" which is a success indicator, not a failure.
var zeroErrorsPattern = regexp.MustCompile(`(?i)\b0\s+errors?\b`)

// containsError checks if command output indicates an actual build or runtime failure.
// Uses regex patterns to avoid false positives on variable names and code references.
func containsError(output string) bool {
	if output == "" {
		return false
	}
	// "0 errors" is success
	if zeroErrorsPattern.MatchString(output) {
		return false
	}
	for _, pat := range errorIndicators {
		if pat.MatchString(output) {
			return true
		}
	}
	return false
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
