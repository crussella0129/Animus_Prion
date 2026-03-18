package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/crussella0129/Animus_Prion/internal/core"
	"github.com/crussella0129/Animus_Prion/internal/permission"
)

// WriteLogEntry records a file write operation for audit purposes.
type WriteLogEntry struct {
	Path      string    `json:"path"`
	Timestamp time.Time `json:"timestamp"`
	Size      int       `json:"size"`
}

// writeLog provides thread-safe audit logging for file writes.
type writeLog struct {
	mu      sync.Mutex
	entries []WriteLogEntry
}

func (l *writeLog) Add(entry WriteLogEntry) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, entry)
}

func (l *writeLog) Entries() []WriteLogEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	result := make([]WriteLogEntry, len(l.entries))
	copy(result, l.entries)
	return result
}

var globalWriteLog = &writeLog{}

// GetWriteLog returns all file write audit entries.
func GetWriteLog() []WriteLogEntry {
	return globalWriteLog.Entries()
}

// --- ReadFileTool ---

// ReadFileTool reads file contents within the workspace boundary.
type ReadFileTool struct {
	workspace *core.Workspace
	checker   *permission.Checker
}

func NewReadFileTool(ws *core.Workspace, checker *permission.Checker) *ReadFileTool {
	return &ReadFileTool{workspace: ws, checker: checker}
}

func (t *ReadFileTool) Name() string { return "read_file" }

func (t *ReadFileTool) Description() string {
	return "Read the contents of a file. Path is resolved relative to the workspace."
}

func (t *ReadFileTool) Parameters() ParameterSchema {
	return ParameterSchema{
		Type: "object",
		Properties: map[string]ParameterSchema{
			"path":      {Type: "string", Description: "File path to read"},
			"max_lines": {Type: "integer", Description: "Maximum lines to read (default: all)"},
		},
		Required: []string{"path"},
	}
}

func (t *ReadFileTool) Execute(args map[string]interface{}) (string, error) {
	path, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("path must be a string")
	}

	// Resolve within workspace boundary
	resolved, err := t.workspace.Resolve(path)
	if err != nil {
		return "", err
	}

	// Permission check
	result := t.checker.IsPathSafe(resolved)
	if !result.Allowed {
		return "", fmt.Errorf("path blocked: %s", result.Reason)
	}

	data, err := os.ReadFile(resolved)
	if err != nil {
		return "", fmt.Errorf("reading file: %w", err)
	}

	content := string(data)

	// Apply max_lines if specified
	if maxLines, ok := args["max_lines"]; ok {
		var limit int
		switch v := maxLines.(type) {
		case int:
			limit = v
		case float64:
			limit = int(v)
		}
		if limit > 0 {
			lines := strings.SplitN(content, "\n", limit+1)
			if len(lines) > limit {
				lines = lines[:limit]
			}
			content = strings.Join(lines, "\n")
		}
	}

	return content, nil
}

// --- WriteFileTool ---

// WriteFileTool writes file contents within the workspace boundary with audit logging.
type WriteFileTool struct {
	workspace *core.Workspace
	checker   *permission.Checker
}

func NewWriteFileTool(ws *core.Workspace, checker *permission.Checker) *WriteFileTool {
	return &WriteFileTool{workspace: ws, checker: checker}
}

func (t *WriteFileTool) Name() string { return "write_file" }

func (t *WriteFileTool) Description() string {
	return "Write content to a file. Creates parent directories if needed. Path is resolved relative to the workspace."
}

func (t *WriteFileTool) Parameters() ParameterSchema {
	return ParameterSchema{
		Type: "object",
		Properties: map[string]ParameterSchema{
			"path":    {Type: "string", Description: "File path to write"},
			"content": {Type: "string", Description: "Content to write to the file"},
		},
		Required: []string{"path", "content"},
	}
}

func (t *WriteFileTool) Execute(args map[string]interface{}) (string, error) {
	path, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("path must be a string")
	}
	content, ok := args["content"].(string)
	if !ok {
		return "", fmt.Errorf("content must be a string")
	}

	// Resolve within workspace boundary
	resolved, err := t.workspace.Resolve(path)
	if err != nil {
		return "", err
	}

	// Permission check
	result := t.checker.IsPathSafe(resolved)
	if !result.Allowed {
		return "", fmt.Errorf("path blocked: %s", result.Reason)
	}

	// Handle JSON-double-encoded content: only unescape \\n → \n if the content
	// has NO real newlines (indicating the LLM double-escaped the entire string).
	// If real newlines exist, the content is already correct — don't corrupt it.
	if strings.Contains(content, "\\n") && !strings.Contains(content, "\n") {
		content = strings.ReplaceAll(content, "\\n", "\n")
		content = strings.ReplaceAll(content, "\\t", "\t")
	}

	// Create parent directories
	dir := filepath.Dir(resolved)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("creating directories: %w", err)
	}

	// Write file
	if err := os.WriteFile(resolved, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("writing file: %w", err)
	}

	// Audit log
	globalWriteLog.Add(WriteLogEntry{
		Path:      resolved,
		Timestamp: time.Now(),
		Size:      len(content),
	})

	return fmt.Sprintf("Wrote %d bytes to %s", len(content), path), nil
}

// --- ListFilesTool ---

// ListFilesTool lists files in a directory within the workspace.
type ListFilesTool struct {
	workspace *core.Workspace
	checker   *permission.Checker
}

func NewListFilesTool(ws *core.Workspace, checker *permission.Checker) *ListFilesTool {
	return &ListFilesTool{workspace: ws, checker: checker}
}

func (t *ListFilesTool) Name() string { return "list_files" }

func (t *ListFilesTool) Description() string {
	return "List files and directories in the given path. Path is resolved relative to the workspace."
}

func (t *ListFilesTool) Parameters() ParameterSchema {
	return ParameterSchema{
		Type: "object",
		Properties: map[string]ParameterSchema{
			"path": {Type: "string", Description: "Directory path to list (default: current directory)"},
		},
	}
}

func (t *ListFilesTool) Execute(args map[string]interface{}) (string, error) {
	path := "."
	if p, ok := args["path"].(string); ok && p != "" {
		path = p
	}

	resolved, err := t.workspace.Resolve(path)
	if err != nil {
		return "", err
	}

	result := t.checker.IsPathSafe(resolved)
	if !result.Allowed {
		return "", fmt.Errorf("path blocked: %s", result.Reason)
	}

	entries, err := os.ReadDir(resolved)
	if err != nil {
		return "", fmt.Errorf("listing directory: %w", err)
	}

	var b strings.Builder
	for _, entry := range entries {
		prefix := "  "
		if entry.IsDir() {
			prefix = "d "
		}
		info, _ := entry.Info()
		size := int64(0)
		if info != nil {
			size = info.Size()
		}
		fmt.Fprintf(&b, "%s%-40s %8d bytes\n", prefix, entry.Name(), size)
	}

	if b.Len() == 0 {
		return "(empty directory)", nil
	}
	return b.String(), nil
}
