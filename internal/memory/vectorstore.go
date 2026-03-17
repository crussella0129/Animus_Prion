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
	Source    string    `json:"source"`    // file path
	StartLine int       `json:"start_line"`
	EndLine   int       `json:"end_line"`
	Embedding []float32 `json:"-"`         // not serialized to JSON
}

// SearchHit is a chunk with its similarity score.
type SearchHit struct {
	Chunk      Chunk   `json:"chunk"`
	Similarity float64 `json:"similarity"`
}

// VectorStore manages embedding storage and KNN search in SQLite.
type VectorStore struct {
	db         *sql.DB
	dimensions int
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
	return result.LastInsertId()
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

// Search performs brute-force KNN search using cosine similarity.
// Returns the top-K most similar chunks to the query embedding.
func (vs *VectorStore) Search(queryEmbedding []float32, topK int) ([]SearchHit, error) {
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
