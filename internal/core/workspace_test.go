package core

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestWorkspaceResolve(t *testing.T) {
	dir := t.TempDir()
	ws, err := NewWorkspace(dir)
	if err != nil {
		t.Fatalf("NewWorkspace failed: %v", err)
	}

	// Resolve a relative path within workspace
	resolved, err := ws.Resolve("subdir/file.txt")
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	expected := filepath.Join(dir, "subdir", "file.txt")
	if resolved != expected {
		t.Errorf("expected %s, got %s", expected, resolved)
	}
}

func TestWorkspaceBoundaryEscape(t *testing.T) {
	dir := t.TempDir()
	ws, err := NewWorkspace(dir)
	if err != nil {
		t.Fatalf("NewWorkspace failed: %v", err)
	}

	// Try to escape via ..
	_, err = ws.Resolve("../../etc/passwd")
	if err == nil {
		t.Fatal("expected boundary error, got nil")
	}
	if !errors.Is(err, ErrWorkspaceBoundary) {
		t.Errorf("expected ErrWorkspaceBoundary, got: %v", err)
	}
}

func TestWorkspaceSetCWD(t *testing.T) {
	dir := t.TempDir()
	subdir := filepath.Join(dir, "sub")
	os.Mkdir(subdir, 0755)

	ws, err := NewWorkspace(dir)
	if err != nil {
		t.Fatalf("NewWorkspace failed: %v", err)
	}

	// Set CWD to subdirectory
	err = ws.SetCWD(subdir)
	if err != nil {
		t.Fatalf("SetCWD failed: %v", err)
	}

	if ws.CWD() != subdir {
		t.Errorf("CWD should be %s, got %s", subdir, ws.CWD())
	}

	// Try to set CWD outside workspace — use a definitely-outside path
	outsidePath := filepath.Join(os.TempDir(), "definitely_outside_workspace_test")
	os.MkdirAll(outsidePath, 0755)
	defer os.RemoveAll(outsidePath)
	err = ws.SetCWD(outsidePath)
	if err == nil {
		t.Fatalf("expected boundary error for %s", outsidePath)
	}
}

func TestWorkspaceContains(t *testing.T) {
	dir := t.TempDir()
	ws, err := NewWorkspace(dir)
	if err != nil {
		t.Fatalf("NewWorkspace failed: %v", err)
	}

	if !ws.Contains(filepath.Join(dir, "file.txt")) {
		t.Error("should contain file within workspace")
	}

	if ws.Contains("/etc/passwd") {
		t.Error("should not contain /etc/passwd")
	}
}

func TestWorkspacePrefixCollision(t *testing.T) {
	// CRITICAL security test: "project-evil" must NOT pass boundary check for "project"
	dir := t.TempDir()
	ws, err := NewWorkspace(dir)
	if err != nil {
		t.Fatalf("NewWorkspace failed: %v", err)
	}

	// Create a sibling directory with the workspace name as prefix
	evilDir := dir + "-evil"
	os.MkdirAll(evilDir, 0755)
	defer os.RemoveAll(evilDir)

	evilFile := filepath.Join(evilDir, "steal.txt")
	os.WriteFile(evilFile, []byte("secrets"), 0644)

	// Resolve must reject the evil path
	_, err = ws.Resolve(evilFile)
	if err == nil {
		t.Fatalf("SECURITY: Resolve accepted %s which is outside workspace %s", evilFile, dir)
	}
	if !errors.Is(err, ErrWorkspaceBoundary) {
		t.Errorf("expected ErrWorkspaceBoundary, got: %v", err)
	}

	// Contains must also reject it
	if ws.Contains(evilFile) {
		t.Fatalf("SECURITY: Contains returned true for %s which is outside workspace %s", evilFile, dir)
	}

	// But the root itself should still work
	if !ws.Contains(dir) {
		t.Error("root directory itself should be contained")
	}
	_, err = ws.Resolve(dir)
	if err != nil {
		t.Errorf("root directory should resolve: %v", err)
	}
}
