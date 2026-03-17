package tools

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/crussella0129/Animus_Prion/internal/core"
)

// gitExec runs a git command with list-based args (no shell interpretation).
func gitExec(ws *core.Workspace, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = ws.CWD()
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, string(output))
	}
	return strings.TrimSpace(string(output)), nil
}

// --- GitInitTool ---

type GitInitTool struct{ workspace *core.Workspace }

func NewGitInitTool(ws *core.Workspace) *GitInitTool { return &GitInitTool{workspace: ws} }

func (t *GitInitTool) Name() string        { return "git_init" }
func (t *GitInitTool) Description() string  { return "Initialize a new git repository in the workspace." }
func (t *GitInitTool) Parameters() ParameterSchema {
	return ParameterSchema{Type: "object", Properties: map[string]ParameterSchema{}}
}
func (t *GitInitTool) Execute(args map[string]interface{}) (string, error) {
	return gitExec(t.workspace, "init")
}

// --- GitStatusTool ---

type GitStatusTool struct{ workspace *core.Workspace }

func NewGitStatusTool(ws *core.Workspace) *GitStatusTool { return &GitStatusTool{workspace: ws} }

func (t *GitStatusTool) Name() string        { return "git_status" }
func (t *GitStatusTool) Description() string  { return "Show the working tree status." }
func (t *GitStatusTool) Parameters() ParameterSchema {
	return ParameterSchema{Type: "object", Properties: map[string]ParameterSchema{}}
}
func (t *GitStatusTool) Execute(args map[string]interface{}) (string, error) {
	return gitExec(t.workspace, "status", "--porcelain")
}

// --- GitDiffTool ---

type GitDiffTool struct{ workspace *core.Workspace }

func NewGitDiffTool(ws *core.Workspace) *GitDiffTool { return &GitDiffTool{workspace: ws} }

func (t *GitDiffTool) Name() string        { return "git_diff" }
func (t *GitDiffTool) Description() string  { return "Show changes in the working directory." }
func (t *GitDiffTool) Parameters() ParameterSchema {
	return ParameterSchema{
		Type: "object",
		Properties: map[string]ParameterSchema{
			"staged": {Type: "boolean", Description: "Show staged changes only (default: false)"},
		},
	}
}
func (t *GitDiffTool) Execute(args map[string]interface{}) (string, error) {
	gitArgs := []string{"diff"}
	if staged, ok := args["staged"].(bool); ok && staged {
		gitArgs = append(gitArgs, "--cached")
	}
	return gitExec(t.workspace, gitArgs...)
}

// --- GitLogTool ---

type GitLogTool struct{ workspace *core.Workspace }

func NewGitLogTool(ws *core.Workspace) *GitLogTool { return &GitLogTool{workspace: ws} }

func (t *GitLogTool) Name() string        { return "git_log" }
func (t *GitLogTool) Description() string  { return "Show recent commit history." }
func (t *GitLogTool) Parameters() ParameterSchema {
	return ParameterSchema{
		Type: "object",
		Properties: map[string]ParameterSchema{
			"count": {Type: "integer", Description: "Number of commits to show (default: 10)"},
		},
	}
}
func (t *GitLogTool) Execute(args map[string]interface{}) (string, error) {
	count := 10
	if v, ok := args["count"]; ok {
		switch c := v.(type) {
		case int:
			count = c
		case float64:
			count = int(c)
		}
	}
	return gitExec(t.workspace, "log", "--oneline", fmt.Sprintf("-n%d", count))
}

// --- GitAddTool ---

type GitAddTool struct{ workspace *core.Workspace }

func NewGitAddTool(ws *core.Workspace) *GitAddTool { return &GitAddTool{workspace: ws} }

func (t *GitAddTool) Name() string        { return "git_add" }
func (t *GitAddTool) Description() string  { return "Stage files for commit." }
func (t *GitAddTool) Parameters() ParameterSchema {
	return ParameterSchema{
		Type: "object",
		Properties: map[string]ParameterSchema{
			"files": {Type: "string", Description: "Space-separated list of files to stage"},
		},
		Required: []string{"files"},
	}
}
func (t *GitAddTool) Execute(args map[string]interface{}) (string, error) {
	files, ok := args["files"].(string)
	if !ok {
		return "", fmt.Errorf("files must be a string")
	}
	gitArgs := append([]string{"add"}, strings.Fields(files)...)
	return gitExec(t.workspace, gitArgs...)
}

// --- GitCommitTool ---

type GitCommitTool struct{ workspace *core.Workspace }

func NewGitCommitTool(ws *core.Workspace) *GitCommitTool { return &GitCommitTool{workspace: ws} }

func (t *GitCommitTool) Name() string        { return "git_commit" }
func (t *GitCommitTool) Description() string  { return "Create a git commit with the given message." }
func (t *GitCommitTool) Parameters() ParameterSchema {
	return ParameterSchema{
		Type: "object",
		Properties: map[string]ParameterSchema{
			"message": {Type: "string", Description: "Commit message"},
		},
		Required: []string{"message"},
	}
}
func (t *GitCommitTool) Execute(args map[string]interface{}) (string, error) {
	message, ok := args["message"].(string)
	if !ok {
		return "", fmt.Errorf("message must be a string")
	}
	return gitExec(t.workspace, "commit", "-m", message)
}

// --- GitBranchTool ---

type GitBranchTool struct{ workspace *core.Workspace }

func NewGitBranchTool(ws *core.Workspace) *GitBranchTool { return &GitBranchTool{workspace: ws} }

func (t *GitBranchTool) Name() string        { return "git_branch" }
func (t *GitBranchTool) Description() string  { return "List or create branches." }
func (t *GitBranchTool) Parameters() ParameterSchema {
	return ParameterSchema{
		Type: "object",
		Properties: map[string]ParameterSchema{
			"name": {Type: "string", Description: "Branch name to create (omit to list branches)"},
		},
	}
}
func (t *GitBranchTool) Execute(args map[string]interface{}) (string, error) {
	if name, ok := args["name"].(string); ok && name != "" {
		return gitExec(t.workspace, "branch", name)
	}
	return gitExec(t.workspace, "branch", "--list")
}

// --- GitCheckoutTool ---

type GitCheckoutTool struct{ workspace *core.Workspace }

func NewGitCheckoutTool(ws *core.Workspace) *GitCheckoutTool { return &GitCheckoutTool{workspace: ws} }

func (t *GitCheckoutTool) Name() string        { return "git_checkout" }
func (t *GitCheckoutTool) Description() string  { return "Switch to a branch." }
func (t *GitCheckoutTool) Parameters() ParameterSchema {
	return ParameterSchema{
		Type: "object",
		Properties: map[string]ParameterSchema{
			"branch": {Type: "string", Description: "Branch name to switch to"},
		},
		Required: []string{"branch"},
	}
}
func (t *GitCheckoutTool) Execute(args map[string]interface{}) (string, error) {
	branch, ok := args["branch"].(string)
	if !ok {
		return "", fmt.Errorf("branch must be a string")
	}
	return gitExec(t.workspace, "checkout", branch)
}
