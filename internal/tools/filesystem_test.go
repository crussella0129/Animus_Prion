package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/crussella0129/Animus_Prion/internal/core"
	"github.com/crussella0129/Animus_Prion/internal/permission"
)

func setupTestWorkspace(t *testing.T) (*core.Workspace, *permission.Checker) {
	t.Helper()
	dir := t.TempDir()
	ws, err := core.NewWorkspace(dir)
	if err != nil {
		t.Fatalf("NewWorkspace: %v", err)
	}
	return ws, permission.NewChecker(false)
}

func TestReadFileTool(t *testing.T) {
	ws, checker := setupTestWorkspace(t)

	// Create a test file
	testFile := filepath.Join(ws.Root(), "test.txt")
	os.WriteFile(testFile, []byte("line1\nline2\nline3\nline4\nline5"), 0644)

	tool := NewReadFileTool(ws, checker)

	// Read entire file
	result, err := tool.Execute(map[string]interface{}{"path": "test.txt"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(result, "line1") || !strings.Contains(result, "line5") {
		t.Error("expected all lines in result")
	}

	// Read with max_lines
	result, err = tool.Execute(map[string]interface{}{"path": "test.txt", "max_lines": 2})
	if err != nil {
		t.Fatalf("Execute with max_lines: %v", err)
	}
	lines := strings.Split(result, "\n")
	if len(lines) > 2 {
		t.Errorf("expected at most 2 lines, got %d", len(lines))
	}
}

func TestReadFileToolBoundary(t *testing.T) {
	ws, checker := setupTestWorkspace(t)
	tool := NewReadFileTool(ws, checker)

	_, err := tool.Execute(map[string]interface{}{"path": "../../etc/passwd"})
	if err == nil {
		t.Error("expected boundary error")
	}
}

func TestReadFileToolNonexistent(t *testing.T) {
	ws, checker := setupTestWorkspace(t)
	tool := NewReadFileTool(ws, checker)

	_, err := tool.Execute(map[string]interface{}{"path": "nonexistent.txt"})
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestWriteFileTool(t *testing.T) {
	ws, checker := setupTestWorkspace(t)
	tool := NewWriteFileTool(ws, checker)

	result, err := tool.Execute(map[string]interface{}{
		"path":    "output.txt",
		"content": "hello world",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(result, "11 bytes") {
		t.Errorf("unexpected result: %s", result)
	}

	// Verify file was written
	data, err := os.ReadFile(filepath.Join(ws.Root(), "output.txt"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "hello world" {
		t.Errorf("file content = %q, want %q", string(data), "hello world")
	}

	// Verify audit log
	entries := GetWriteLog()
	found := false
	for _, e := range entries {
		if strings.HasSuffix(e.Path, "output.txt") {
			found = true
			if e.Size != 11 {
				t.Errorf("audit log size = %d, want 11", e.Size)
			}
		}
	}
	if !found {
		t.Error("expected audit log entry for output.txt")
	}
}

func TestWriteFileToolCreatesDirectories(t *testing.T) {
	ws, checker := setupTestWorkspace(t)
	tool := NewWriteFileTool(ws, checker)

	_, err := tool.Execute(map[string]interface{}{
		"path":    "sub/dir/file.txt",
		"content": "nested",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(ws.Root(), "sub", "dir", "file.txt"))
	if string(data) != "nested" {
		t.Error("file not created in nested directory")
	}
}

func TestWriteFileToolJSONUnescape(t *testing.T) {
	ws, checker := setupTestWorkspace(t)
	tool := NewWriteFileTool(ws, checker)

	_, err := tool.Execute(map[string]interface{}{
		"path":    "escaped.txt",
		"content": "line1\\nline2\\ttab",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(ws.Root(), "escaped.txt"))
	if !strings.Contains(string(data), "\n") {
		t.Error("expected \\n to be unescaped")
	}
}

func TestWriteFileToolBoundary(t *testing.T) {
	ws, checker := setupTestWorkspace(t)
	tool := NewWriteFileTool(ws, checker)

	_, err := tool.Execute(map[string]interface{}{
		"path":    "../../escape.txt",
		"content": "should fail",
	})
	if err == nil {
		t.Error("expected boundary error")
	}
}

func TestListFilesTool(t *testing.T) {
	ws, chk := setupTestWorkspace(t)
	checker := chk

	// Create some files
	os.WriteFile(filepath.Join(ws.Root(), "a.txt"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(ws.Root(), "b.txt"), []byte("bb"), 0644)
	os.Mkdir(filepath.Join(ws.Root(), "subdir"), 0755)

	tool := NewListFilesTool(ws, checker)
	result, err := tool.Execute(map[string]interface{}{})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(result, "a.txt") || !strings.Contains(result, "b.txt") || !strings.Contains(result, "subdir") {
		t.Errorf("missing files in listing: %s", result)
	}
	if !strings.Contains(result, "d ") {
		t.Error("directory should be prefixed with 'd '")
	}
}

func TestListFilesToolEmpty(t *testing.T) {
	ws, chk := setupTestWorkspace(t)
	tool := NewListFilesTool(ws, chk)

	result, err := tool.Execute(map[string]interface{}{})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result != "(empty directory)" {
		t.Errorf("expected empty directory message, got: %s", result)
	}
}
