// Package core provides the agent loop, workspace, context management, and error handling.
package core

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
)

// ErrWorkspaceBoundary is returned when a path escapes the workspace root.
var ErrWorkspaceBoundary = errors.New("path escapes workspace boundary")

// Workspace enforces an immutable root boundary with a mutable current working directory.
// All path resolution goes through Resolve(), which prevents escape via symlinks or ../ traversal.
type Workspace struct {
	mu   sync.RWMutex
	root string // immutable after construction
	cwd  string // mutable, always within root
}

// NewWorkspace creates a workspace with the given root directory.
// The root is cleaned and converted to an absolute path.
func NewWorkspace(root string) (*Workspace, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolving workspace root: %w", err)
	}
	// Evaluate symlinks to get the real path
	realRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		// If the path doesn't exist yet, use the cleaned absolute path
		realRoot = filepath.Clean(absRoot)
	}

	return &Workspace{
		root: realRoot,
		cwd:  realRoot,
	}, nil
}

// Root returns the immutable workspace root.
func (w *Workspace) Root() string {
	return w.root
}

// CWD returns the current working directory within the workspace.
func (w *Workspace) CWD() string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.cwd
}

// SetCWD changes the current working directory, enforcing the boundary.
func (w *Workspace) SetCWD(dir string) error {
	resolved, err := w.Resolve(dir)
	if err != nil {
		return err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.cwd = resolved
	return nil
}

// Resolve resolves a path relative to CWD and enforces the workspace boundary.
// Absolute paths are checked directly; relative paths are joined with CWD.
// Symlinks are evaluated to prevent escape.
func (w *Workspace) Resolve(path string) (string, error) {
	w.mu.RLock()
	cwd := w.cwd
	w.mu.RUnlock()

	var abs string
	if filepath.IsAbs(path) {
		abs = filepath.Clean(path)
	} else {
		abs = filepath.Clean(filepath.Join(cwd, path))
	}

	// Try to evaluate symlinks for the real path
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		// Path may not exist yet (e.g. writing a new file) — use cleaned abs
		real = abs
	}

	// Normalize for comparison (handles Windows drive letter casing)
	normalRoot := strings.ToLower(filepath.Clean(w.root))
	normalReal := strings.ToLower(filepath.Clean(real))

	// Check: is the resolved path within the root?
	if !strings.HasPrefix(normalReal, normalRoot) {
		// Allow exact match (the root itself)
		if normalReal != normalRoot {
			return "", fmt.Errorf("%w: %s is outside %s", ErrWorkspaceBoundary, path, w.root)
		}
	}

	return real, nil
}

// Contains checks whether a path is within the workspace without resolving symlinks.
// Useful for quick checks where EvalSymlinks would be too slow.
func (w *Workspace) Contains(path string) bool {
	abs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	normalRoot := strings.ToLower(filepath.Clean(w.root))
	normalAbs := strings.ToLower(filepath.Clean(abs))
	return strings.HasPrefix(normalAbs, normalRoot)
}
