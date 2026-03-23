// Package memory provides vector storage and embedding-based search.
// Uses SQLite BLOBs for embedding storage with brute-force KNN search.
package memory

import (
	"database/sql"
	"encoding/binary"
	"fmt"
	"math"
	"sort"

	_ "modernc.org/sqlite"
)

// Embedder generates vector embeddings for text.
type Embedder interface {
	Embed(text string) ([]float32, error)
	Dimensions() int
}

// Chunk represents a text chunk stored with its embedding.
type Chunk struct {
	ID        int64     `json:"id"`
	Text      string    `json:"text"`
	Source    string    `json:"source"` // file path
	StartLine int       `json:"start_line"`
	EndLine   int       `json:"end_line"`
	Embedding []float32 `json:"-"` // not serialized to JSON
}

// SearchHit is a chunk with its similarity score.
type SearchHit struct {
	Chunk      Chunk   `json:"chunk"`
	Similarity float64 `json:"similarity"`
}

// hnswThreshold is the chunk count above which HNSW index is used for search.
// Below this, brute-force KNN is fast enough and more accurate.
const hnswThreshold = 10000

// VectorStore manages embedding storage and KNN search in SQLite.
// Automatically uses HNSW index when chunk count exceeds hnswThreshold.
type VectorStore struct {
	db         *sql.DB
	dimensions int
	index      *HNSWIndex // lazily built when chunk count exceeds threshold
}

// NewVectorStore opens or creates a vector store at the given path.
func NewVectorStore(path string, dimensions int) (*VectorStore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening vector store: %w", err)
	}

	vs := &VectorStore{db: db, dimensions: dimensions}
	if err := vs.init(); err != nil {
		db.Close()
		return nil, err
	}

	return vs, nil
}

func (vs *VectorStore) init() error {
	schema := `
	CREATE TABLE IF NOT EXISTS chunks (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		text       TEXT NOT NULL,
		source     TEXT NOT NULL DEFAULT '',
		start_line INTEGER NOT NULL DEFAULT 0,
		end_line   INTEGER NOT NULL DEFAULT 0,
		embedding  BLOB NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_chunks_source ON chunks(source);
	`
	_, err := vs.db.Exec(schema)
	return err
}

// Close closes the database.
func (vs *VectorStore) Close() error {
	return vs.db.Close()
}

// Insert adds a chunk with its embedding to the store.
func (vs *VectorStore) Insert(chunk Chunk) (int64, error) {
	blob := float32ToBytes(chunk.Embedding)
	result, err := vs.db.Exec(
		"INSERT INTO chunks (text, source, start_line, end_line, embedding) VALUES (?, ?, ?, ?, ?)",
		chunk.Text, chunk.Source, chunk.StartLine, chunk.EndLine, blob,
	)
	if err != nil {
		return 0, fmt.Errorf("inserting chunk: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	// Add to HNSW index if active
	if vs.index != nil {
		vs.index.Insert(id, chunk.Embedding)
	}

	return id, nil
}

// InsertBatch inserts multiple chunks in a single transaction.
func (vs *VectorStore) InsertBatch(chunks []Chunk) error {
	tx, err := vs.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(
		"INSERT INTO chunks (text, source, start_line, end_line, embedding) VALUES (?, ?, ?, ?, ?)",
	)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, c := range chunks {
		blob := float32ToBytes(c.Embedding)
		if _, err := stmt.Exec(c.Text, c.Source, c.StartLine, c.EndLine, blob); err != nil {
			return fmt.Errorf("inserting chunk: %w", err)
		}
	}

	return tx.Commit()
}

// Search performs KNN search. Uses HNSW for large datasets (>10k chunks),
// brute-force cosine similarity for small ones.
func (vs *VectorStore) Search(queryEmbedding []float32, topK int) ([]SearchHit, error) {
	// Use HNSW index if available
	if vs.index != nil && vs.index.Len() > 0 {
		return vs.searchHNSW(queryEmbedding, topK)
	}
	return vs.searchBruteForce(queryEmbedding, topK)
}

// searchHNSW uses the HNSW index for fast approximate search.
func (vs *VectorStore) searchHNSW(queryEmbedding []float32, topK int) ([]SearchHit, error) {
	ids, dists := vs.index.Search(queryEmbedding, topK)

	var hits []SearchHit
	for i, id := range ids {
		// Fetch chunk metadata from SQLite (embedding not needed — we have the distance)
		var c Chunk
		err := vs.db.QueryRow(
			"SELECT id, text, source, start_line, end_line FROM chunks WHERE id = ?", id,
		).Scan(&c.ID, &c.Text, &c.Source, &c.StartLine, &c.EndLine)
		if err != nil {
			continue // chunk may have been deleted
		}
		// Convert cosine distance back to similarity
		hits = append(hits, SearchHit{Chunk: c, Similarity: float64(1.0 - dists[i])})
	}
	return hits, nil
}

// searchBruteForce performs brute-force KNN search using cosine similarity.
// Returns the top-K most similar chunks to the query embedding.
func (vs *VectorStore) searchBruteForce(queryEmbedding []float32, topK int) ([]SearchHit, error) {
	rows, err := vs.db.Query("SELECT id, text, source, start_line, end_line, embedding FROM chunks")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hits []SearchHit

	for rows.Next() {
		var c Chunk
		var blob []byte
		if err := rows.Scan(&c.ID, &c.Text, &c.Source, &c.StartLine, &c.EndLine, &blob); err != nil {
			return nil, err
		}
		c.Embedding = bytesToFloat32(blob)

		sim := cosineSimilarity(queryEmbedding, c.Embedding)
		hits = append(hits, SearchHit{Chunk: c, Similarity: sim})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Sort by similarity descending
	sort.Slice(hits, func(i, j int) bool {
		return hits[i].Similarity > hits[j].Similarity
	})

	// Return top-K
	if len(hits) > topK {
		hits = hits[:topK]
	}

	return hits, nil
}

// DeleteBySource removes all chunks from a specific source file.
func (vs *VectorStore) DeleteBySource(source string) error {
	_, err := vs.db.Exec("DELETE FROM chunks WHERE source = ?", source)
	return err
}

// BuildIndex constructs the HNSW index from all chunks in the store.
// Call this after bulk loading or when chunk count exceeds hnswThreshold.
func (vs *VectorStore) BuildIndex() error {
	count, err := vs.Count()
	if err != nil {
		return err
	}
	if count < hnswThreshold {
		return nil // not enough chunks to warrant an index
	}

	vs.index = NewHNSWIndex()

	rows, err := vs.db.Query("SELECT id, embedding FROM chunks")
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var id int64
		var blob []byte
		if err := rows.Scan(&id, &blob); err != nil {
			return err
		}
		vs.index.Insert(id, bytesToFloat32(blob))
	}
	return rows.Err()
}

// EnsureIndex builds the HNSW index if chunk count exceeds the threshold
// and the index hasn't been built yet.
func (vs *VectorStore) EnsureIndex() error {
	if vs.index != nil {
		return nil // already built
	}
	count, err := vs.Count()
	if err != nil {
		return err
	}
	if count >= hnswThreshold {
		return vs.BuildIndex()
	}
	return nil
}

// Count returns the total number of chunks in the store.
func (vs *VectorStore) Count() (int, error) {
	var count int
	err := vs.db.QueryRow("SELECT COUNT(*) FROM chunks").Scan(&count)
	return count, err
}

// --- Math utilities ---

// cosineSimilarity computes cosine similarity between two vectors.
func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0.0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	denominator := math.Sqrt(normA) * math.Sqrt(normB)
	if denominator == 0 {
		return 0.0
	}

	return dotProduct / denominator
}

// --- Serialization ---

// float32ToBytes converts a float32 slice to a byte slice (little-endian).
func float32ToBytes(floats []float32) []byte {
	buf := make([]byte, len(floats)*4)
	for i, f := range floats {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(f))
	}
	return buf
}

// bytesToFloat32 converts a byte slice back to float32 (little-endian).
func bytesToFloat32(buf []byte) []float32 {
	floats := make([]float32, len(buf)/4)
	for i := range floats {
		floats[i] = math.Float32frombits(binary.LittleEndian.Uint32(buf[i*4:]))
	}
	return floats
}
