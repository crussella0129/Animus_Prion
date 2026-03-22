package planner

import (
	"testing"
)

func TestParsePlan(t *testing.T) {
	plan := `1. Read the configuration file
2. Modify the timeout setting in config.yaml
3. Run the tests with go test
4. Commit the changes with git`

	steps := ParsePlan(plan)
	if len(steps) != 4 {
		t.Fatalf("expected 4 steps, got %d", len(steps))
	}

	// Step 1 should be READ
	if steps[0].Type != StepRead {
		t.Errorf("step 1 type = %v, want READ", steps[0].Type)
	}

	// Step 2 should be WRITE (has .yaml extension)
	if steps[1].Type != StepWrite {
		t.Errorf("step 2 type = %v, want WRITE", steps[1].Type)
	}

	// Step 3 should be SHELL
	if steps[2].Type != StepShell {
		t.Errorf("step 3 type = %v, want SHELL", steps[2].Type)
	}

	// Step 4 should be GIT
	if steps[3].Type != StepGit {
		t.Errorf("step 4 type = %v, want GIT", steps[3].Type)
	}
}

func TestParsePlanMaxSteps(t *testing.T) {
	plan := `1. Step one
2. Step two
3. Step three
4. Step four
5. Step five
6. Step six
7. Step seven
8. Step eight (should be dropped)
9. Step nine (should be dropped)`

	steps := ParsePlan(plan)
	if len(steps) != MaxSteps {
		t.Errorf("expected %d steps (max), got %d", MaxSteps, len(steps))
	}
}

func TestStepTypeInference(t *testing.T) {
	tests := []struct {
		desc     string
		expected StepType
	}{
		{"Read the main.py file", StepWrite},   // .py extension takes priority
		{"Examine the error logs", StepRead},   // keyword "examine"
		{"Run go test ./...", StepShell},       // keyword "run"
		{"Create a new handler.go", StepWrite}, // .go extension
		{"Commit the changes", StepGit},        // keyword "commit"
		{"Search for the bug", StepAnalyze},    // keyword "search"
		{"Generate the report", StepGenerate},  // keyword "generate"
	}

	for _, tt := range tests {
		got := inferStepType(tt.desc)
		if got != tt.expected {
			t.Errorf("inferStepType(%q) = %v, want %v", tt.desc, got, tt.expected)
		}
	}
}

func TestIsSimpleTask(t *testing.T) {
	if !IsSimpleTask("Fix the bug") {
		t.Error("'Fix the bug' should be simple")
	}
	if IsSimpleTask("Read the config, then update the setting, and finally run the tests") {
		t.Error("multi-step task with 'then' should not be simple")
	}
	if !IsSimpleTask("Hello") {
		t.Error("single word should be simple")
	}
}

func TestAllowedTools(t *testing.T) {
	tools := StepRead.AllowedTools()
	if len(tools) == 0 {
		t.Error("READ step should have allowed tools")
	}

	found := false
	for _, tool := range tools {
		if tool == "read_file" {
			found = true
			break
		}
	}
	if !found {
		t.Error("READ step should include read_file tool")
	}
}

func TestSplitConjunctions(t *testing.T) {
	parts := SplitConjunctions("read the file then write the output")
	if len(parts) != 2 {
		t.Errorf("expected 2 parts, got %d", len(parts))
	}
}
