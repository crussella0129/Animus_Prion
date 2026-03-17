package knowledge

import (
	"database/sql"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

// GraphDB stores the code knowledge graph in SQLite.
type GraphDB struct {
	db *sql.DB
}

// NewGraphDB opens or creates a SQLite graph database at the given path.
func NewGraphDB(path string) (*GraphDB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening graph db: %w", err)
	}

	g := &GraphDB{db: db}
	if err := g.init(); err != nil {
		db.Close()
		return nil, err
	}

	return g, nil
}

// init creates the schema if it doesn't exist.
func (g *GraphDB) init() error {
	schema := `
	CREATE TABLE IF NOT EXISTS nodes (
		id        TEXT PRIMARY KEY,
		name      TEXT NOT NULL,
		kind      TEXT NOT NULL,
		file      TEXT NOT NULL,
		line      INTEGER NOT NULL DEFAULT 0,
		signature TEXT NOT NULL DEFAULT '',
		doc       TEXT NOT NULL DEFAULT '',
		package   TEXT NOT NULL DEFAULT ''
	);

	CREATE TABLE IF NOT EXISTS edges (
		source TEXT NOT NULL,
		target TEXT NOT NULL,
		kind   TEXT NOT NULL,
		file   TEXT NOT NULL DEFAULT '',
		line   INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (source, target, kind)
	);

	CREATE INDEX IF NOT EXISTS idx_nodes_kind ON nodes(kind);
	CREATE INDEX IF NOT EXISTS idx_nodes_file ON nodes(file);
	CREATE INDEX IF NOT EXISTS idx_nodes_package ON nodes(package);
	CREATE INDEX IF NOT EXISTS idx_edges_source ON edges(source);
	CREATE INDEX IF NOT EXISTS idx_edges_target ON edges(target);
	CREATE INDEX IF NOT EXISTS idx_edges_kind ON edges(kind);
	`

	_, err := g.db.Exec(schema)
	if err != nil {
		return fmt.Errorf("creating schema: %w", err)
	}
	return nil
}

// Close closes the database.
func (g *GraphDB) Close() error {
	return g.db.Close()
}

// UpsertNode inserts or updates a node.
func (g *GraphDB) UpsertNode(n Node) error {
	_, err := g.db.Exec(`
		INSERT INTO nodes (id, name, kind, file, line, signature, doc, package)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name=excluded.name, kind=excluded.kind, file=excluded.file,
			line=excluded.line, signature=excluded.signature, doc=excluded.doc,
			package=excluded.package
	`, n.ID, n.Name, string(n.Kind), n.File, n.Line, n.Signature, n.DocString, n.Package)
	return err
}

// UpsertEdge inserts or updates an edge.
func (g *GraphDB) UpsertEdge(e Edge) error {
	_, err := g.db.Exec(`
		INSERT INTO edges (source, target, kind, file, line)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(source, target, kind) DO UPDATE SET
			file=excluded.file, line=excluded.line
	`, e.Source, e.Target, string(e.Kind), e.File, e.Line)
	return err
}

// IngestResult ingests all nodes and edges from a ParseResult.
// Uses a transaction for atomicity and performance.
func (g *GraphDB) IngestResult(result *ParseResult) error {
	tx, err := g.db.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	nodeStmt, err := tx.Prepare(`
		INSERT INTO nodes (id, name, kind, file, line, signature, doc, package)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name=excluded.name, kind=excluded.kind, file=excluded.file,
			line=excluded.line, signature=excluded.signature, doc=excluded.doc,
			package=excluded.package
	`)
	if err != nil {
		return err
	}
	defer nodeStmt.Close()

	edgeStmt, err := tx.Prepare(`
		INSERT INTO edges (source, target, kind, file, line)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(source, target, kind) DO UPDATE SET
			file=excluded.file, line=excluded.line
	`)
	if err != nil {
		return err
	}
	defer edgeStmt.Close()

	for _, n := range result.Nodes {
		if _, err := nodeStmt.Exec(n.ID, n.Name, string(n.Kind), n.File, n.Line, n.Signature, n.DocString, n.Package); err != nil {
			return fmt.Errorf("upserting node %s: %w", n.ID, err)
		}
	}

	for _, e := range result.Edges {
		if _, err := edgeStmt.Exec(e.Source, e.Target, string(e.Kind), e.File, e.Line); err != nil {
			return fmt.Errorf("upserting edge %s->%s: %w", e.Source, e.Target, err)
		}
	}

	return tx.Commit()
}

// DeleteByFile removes all nodes and edges associated with a file.
// Used for incremental re-indexing: delete old data, then re-parse.
func (g *GraphDB) DeleteByFile(file string) error {
	tx, err := g.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Delete edges where the file matches
	if _, err := tx.Exec("DELETE FROM edges WHERE file = ?", file); err != nil {
		return err
	}

	// Delete nodes where the file matches
	if _, err := tx.Exec("DELETE FROM nodes WHERE file = ?", file); err != nil {
		return err
	}

	return tx.Commit()
}

// SearchNodes finds nodes matching a query string (by name or ID).
func (g *GraphDB) SearchNodes(query string) ([]Node, error) {
	rows, err := g.db.Query(`
		SELECT id, name, kind, file, line, signature, doc, package
		FROM nodes
		WHERE name LIKE ? OR id LIKE ?
		ORDER BY
			CASE WHEN name = ? THEN 0
			     WHEN name LIKE ? THEN 1
			     ELSE 2
			END
		LIMIT 20
	`, "%"+query+"%", "%"+query+"%", query, query+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanNodes(rows)
}

// GetNode retrieves a node by exact ID.
func (g *GraphDB) GetNode(id string) (*Node, error) {
	row := g.db.QueryRow(`
		SELECT id, name, kind, file, line, signature, doc, package
		FROM nodes WHERE id = ?
	`, id)

	var n Node
	var kind string
	err := row.Scan(&n.ID, &n.Name, &kind, &n.File, &n.Line, &n.Signature, &n.DocString, &n.Package)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	n.Kind = NodeKind(kind)
	return &n, nil
}

// GetCallers returns nodes that call the given target.
func (g *GraphDB) GetCallers(targetID string) ([]Node, error) {
	rows, err := g.db.Query(`
		SELECT n.id, n.name, n.kind, n.file, n.line, n.signature, n.doc, n.package
		FROM edges e
		JOIN nodes n ON n.id = e.source
		WHERE e.target = ? AND e.kind = 'calls'
	`, targetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanNodes(rows)
}

// GetCallees returns nodes called by the given source.
func (g *GraphDB) GetCallees(sourceID string) ([]Node, error) {
	rows, err := g.db.Query(`
		SELECT n.id, n.name, n.kind, n.file, n.line, n.signature, n.doc, n.package
		FROM edges e
		JOIN nodes n ON n.id = e.target
		WHERE e.source = ? AND e.kind = 'calls'
	`, sourceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanNodes(rows)
}

// ResolveCallers returns the transitive caller chain up to maxDepth levels.
// depth=1 returns direct callers only; depth=3 gives 3 hops upstream.
// Uses iterative BFS to avoid stack overflow on deep graphs.
//
// TODO(user): This is where YOU decide the traversal behavior.
// See the function body below — the depth limit and cycle detection
// strategy are set, but you might want to add score decay or
// prioritization based on NodeKind.
func (g *GraphDB) ResolveCallers(targetID string, maxDepth int) ([]Node, error) {
	if maxDepth <= 0 {
		maxDepth = 3
	}

	visited := make(map[string]bool)
	visited[targetID] = true
	var allCallers []Node

	frontier := []string{targetID}

	for depth := 0; depth < maxDepth && len(frontier) > 0; depth++ {
		var nextFrontier []string
		for _, id := range frontier {
			callers, err := g.GetCallers(id)
			if err != nil {
				return allCallers, err
			}
			for _, caller := range callers {
				if !visited[caller.ID] {
					visited[caller.ID] = true
					allCallers = append(allCallers, caller)
					nextFrontier = append(nextFrontier, caller.ID)
				}
			}
		}
		frontier = nextFrontier
	}

	return allCallers, nil
}

// GetBlastRadius returns all nodes transitively affected by changes to sourceID.
// This walks the "calls" edges forward (callees), plus "contains" edges.
func (g *GraphDB) GetBlastRadius(sourceID string, maxDepth int) ([]Node, error) {
	if maxDepth <= 0 {
		maxDepth = 3
	}

	visited := make(map[string]bool)
	visited[sourceID] = true
	var affected []Node

	frontier := []string{sourceID}

	for depth := 0; depth < maxDepth && len(frontier) > 0; depth++ {
		var nextFrontier []string
		for _, id := range frontier {
			// Forward: who calls this? (if I change, callers are affected)
			callers, err := g.GetCallers(id)
			if err != nil {
				return affected, err
			}
			for _, n := range callers {
				if !visited[n.ID] {
					visited[n.ID] = true
					affected = append(affected, n)
					nextFrontier = append(nextFrontier, n.ID)
				}
			}
		}
		frontier = nextFrontier
	}

	return affected, nil
}

// GetInheritance returns types that implement or extend the given type.
func (g *GraphDB) GetInheritance(typeID string) ([]Node, error) {
	rows, err := g.db.Query(`
		SELECT n.id, n.name, n.kind, n.file, n.line, n.signature, n.doc, n.package
		FROM edges e
		JOIN nodes n ON n.id = e.source
		WHERE e.target = ? AND e.kind = 'implements'
	`, typeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanNodes(rows)
}

// Stats returns graph statistics.
func (g *GraphDB) Stats() (nodeCount, edgeCount int, err error) {
	err = g.db.QueryRow("SELECT COUNT(*) FROM nodes").Scan(&nodeCount)
	if err != nil {
		return
	}
	err = g.db.QueryRow("SELECT COUNT(*) FROM edges").Scan(&edgeCount)
	return
}

// NodesByKind returns all nodes of a given kind.
func (g *GraphDB) NodesByKind(kind NodeKind) ([]Node, error) {
	rows, err := g.db.Query(`
		SELECT id, name, kind, file, line, signature, doc, package
		FROM nodes WHERE kind = ?
	`, string(kind))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanNodes(rows)
}

// Format formats nodes as a readable string for LLM context injection.
func FormatNodes(nodes []Node) string {
	if len(nodes) == 0 {
		return "No results found."
	}

	var sb strings.Builder
	for _, n := range nodes {
		sb.WriteString(fmt.Sprintf("%-12s %s", n.Kind, n.ID))
		if n.Signature != "" {
			sb.WriteString("\n  " + n.Signature)
		}
		if n.File != "" {
			sb.WriteString(fmt.Sprintf("\n  %s:%d", n.File, n.Line))
		}
		if n.DocString != "" {
			doc := n.DocString
			if len(doc) > 120 {
				doc = doc[:120] + "..."
			}
			sb.WriteString("\n  // " + doc)
		}
		sb.WriteString("\n\n")
	}
	return sb.String()
}

// scanNodes extracts nodes from a sql.Rows result set.
func scanNodes(rows *sql.Rows) ([]Node, error) {
	var nodes []Node
	for rows.Next() {
		var n Node
		var kind string
		if err := rows.Scan(&n.ID, &n.Name, &kind, &n.File, &n.Line, &n.Signature, &n.DocString, &n.Package); err != nil {
			return nil, err
		}
		n.Kind = NodeKind(kind)
		nodes = append(nodes, n)
	}
	return nodes, rows.Err()
}
