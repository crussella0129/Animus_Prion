package knowledge

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// FileState tracks the hash and mod time of an indexed file.
type FileState struct {
	Path    string
	Hash    string
	ModTime time.Time
}

// Indexer incrementally indexes a project directory into the knowledge graph.
type Indexer struct {
	db    *GraphDB
	root  string
	mu    sync.Mutex
	state map[string]FileState // path → state
}

// NewIndexer creates an indexer for the given project root.
func NewIndexer(db *GraphDB, root string) *Indexer {
	return &Indexer{
		db:    db,
		root:  root,
		state: make(map[string]FileState),
	}
}

// IndexAll performs a full index of all supported files in the project.
func (idx *Indexer) IndexAll() (int, error) {
	files, err := idx.discoverFiles()
	if err != nil {
		return 0, err
	}

	indexed := 0
	for _, path := range files {
		if err := idx.indexFile(path); err != nil {
			log.Printf("Warning: failed to index %s: %v", path, err)
			continue
		}
		indexed++
	}

	return indexed, nil
}

// IndexChanged performs an incremental index — only re-parses changed files.
func (idx *Indexer) IndexChanged() (int, error) {
	files, err := idx.discoverFiles()
	if err != nil {
		return 0, err
	}

	reindexed := 0
	currentFiles := make(map[string]bool)

	for _, path := range files {
		currentFiles[path] = true

		// Check if file has changed
		hash, err := fileHash(path)
		if err != nil {
			continue
		}

		idx.mu.Lock()
		prev, exists := idx.state[path]
		idx.mu.Unlock()

		if exists && prev.Hash == hash {
			continue // unchanged
		}

		// File is new or changed — re-index
		if err := idx.indexFile(path); err != nil {
			log.Printf("Warning: failed to re-index %s: %v", path, err)
			continue
		}

		idx.mu.Lock()
		idx.state[path] = FileState{Path: path, Hash: hash, ModTime: time.Now()}
		idx.mu.Unlock()
		reindexed++
	}

	// Remove deleted files from the graph
	idx.mu.Lock()
	for path := range idx.state {
		if !currentFiles[path] {
			idx.db.DeleteByFile(path)
			delete(idx.state, path)
		}
	}
	idx.mu.Unlock()

	return reindexed, nil
}

// indexFile parses a single file and ingests it into the graph.
func (idx *Indexer) indexFile(path string) error {
	ext := strings.ToLower(filepath.Ext(path))

	var result *ParseResult
	var err error

	switch ext {
	case ".go":
		result, err = ParseGoFile(path)
	default:
		return fmt.Errorf("unsupported file type: %s", ext)
	}

	if err != nil {
		return err
	}

	// Delete old data for this file, then insert new
	if err := idx.db.DeleteByFile(path); err != nil {
		return fmt.Errorf("clearing old data for %s: %w", path, err)
	}

	return idx.db.IngestResult(result)
}

// discoverFiles walks the project root and returns all supported source files.
func (idx *Indexer) discoverFiles() ([]string, error) {
	var files []string

	err := filepath.WalkDir(idx.root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip errors
		}

		// Skip hidden directories and common non-source directories
		if d.IsDir() {
			name := d.Name()
			if strings.HasPrefix(name, ".") || name == "vendor" || name == "node_modules" || name == "testdata" || name == "__pycache__" || name == "bin" {
				return filepath.SkipDir
			}
			return nil
		}

		// Only index supported file types
		ext := strings.ToLower(filepath.Ext(path))
		if isSupportedExtension(ext) {
			files = append(files, path)
		}

		return nil
	})

	return files, err
}

// isSupportedExtension checks if a file extension is indexable.
func isSupportedExtension(ext string) bool {
	supported := map[string]bool{
		".go": true,
		// Future: ".py", ".js", ".ts", ".rs", ".java", ".c", ".cpp"
	}
	return supported[ext]
}

// fileHash computes a SHA-256 hash of a file's contents.
func fileHash(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:]), nil
}
