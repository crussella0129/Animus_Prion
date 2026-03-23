package memory

import (
	"math"
	"math/rand"
	"testing"
)

func randomVector(dim int) []float32 {
	v := make([]float32, dim)
	for i := range v {
		v[i] = rand.Float32()*2 - 1 // [-1, 1]
	}
	return v
}

func normalizeVector(v []float32) []float32 {
	var norm float64
	for _, x := range v {
		norm += float64(x) * float64(x)
	}
	norm = math.Sqrt(norm)
	out := make([]float32, len(v))
	for i, x := range v {
		out[i] = float32(float64(x) / norm)
	}
	return out
}

func TestHNSWInsertAndSearch(t *testing.T) {
	idx := NewHNSWIndex()

	// Insert 100 random vectors
	dim := 32
	vectors := make([][]float32, 100)
	for i := 0; i < 100; i++ {
		vectors[i] = normalizeVector(randomVector(dim))
		idx.Insert(int64(i+1), vectors[i])
	}

	if idx.Len() != 100 {
		t.Errorf("Len() = %d, want 100", idx.Len())
	}

	// Search for a vector we inserted — should find itself as nearest
	ids, dists := idx.Search(vectors[0], 1)
	if len(ids) != 1 {
		t.Fatalf("expected 1 result, got %d", len(ids))
	}
	if ids[0] != 1 {
		t.Errorf("nearest to vectors[0] should be ID 1, got %d", ids[0])
	}
	if dists[0] > 0.01 {
		t.Errorf("distance to self should be ~0, got %f", dists[0])
	}
}

func TestHNSWTopK(t *testing.T) {
	idx := NewHNSWIndex()

	dim := 16
	for i := 0; i < 50; i++ {
		idx.Insert(int64(i+1), normalizeVector(randomVector(dim)))
	}

	query := normalizeVector(randomVector(dim))
	ids, dists := idx.Search(query, 5)

	if len(ids) != 5 {
		t.Errorf("expected 5 results, got %d", len(ids))
	}

	// Results should be sorted by distance ascending
	for i := 1; i < len(dists); i++ {
		if dists[i] < dists[i-1] {
			t.Errorf("results not sorted: dist[%d]=%f < dist[%d]=%f", i, dists[i], i-1, dists[i-1])
		}
	}
}

func TestHNSWRecall(t *testing.T) {
	// Insert 1000 vectors, search with HNSW, verify recall against brute-force
	idx := NewHNSWIndex()
	dim := 64
	n := 1000
	topK := 10

	vectors := make([][]float32, n)
	for i := 0; i < n; i++ {
		vectors[i] = normalizeVector(randomVector(dim))
		idx.Insert(int64(i+1), vectors[i])
	}

	query := normalizeVector(randomVector(dim))

	// HNSW search
	hnswIDs, _ := idx.Search(query, topK)

	// Brute-force search for ground truth
	type bf struct {
		id   int64
		dist float32
	}
	var bruteForce []bf
	for i, v := range vectors {
		bruteForce = append(bruteForce, bf{id: int64(i + 1), dist: cosineDistance(query, v)})
	}
	// Sort by distance
	for i := 0; i < len(bruteForce); i++ {
		for j := i + 1; j < len(bruteForce); j++ {
			if bruteForce[j].dist < bruteForce[i].dist {
				bruteForce[i], bruteForce[j] = bruteForce[j], bruteForce[i]
			}
		}
	}

	// Check recall: how many of the true top-K are in HNSW results?
	trueTopK := make(map[int64]bool)
	for i := 0; i < topK && i < len(bruteForce); i++ {
		trueTopK[bruteForce[i].id] = true
	}

	hits := 0
	for _, id := range hnswIDs {
		if trueTopK[id] {
			hits++
		}
	}

	recall := float64(hits) / float64(topK)
	if recall < 0.7 {
		t.Errorf("recall too low: %.1f%% (expected >70%%)", recall*100)
	}
}

func TestHNSWEmpty(t *testing.T) {
	idx := NewHNSWIndex()
	ids, dists := idx.Search([]float32{1, 0, 0}, 5)
	if len(ids) != 0 || len(dists) != 0 {
		t.Error("empty index should return no results")
	}
}

func TestHNSWSingleVector(t *testing.T) {
	idx := NewHNSWIndex()
	v := []float32{1, 0, 0}
	idx.Insert(1, v)

	ids, dists := idx.Search(v, 1)
	if len(ids) != 1 {
		t.Fatalf("expected 1 result, got %d", len(ids))
	}
	if ids[0] != 1 {
		t.Errorf("expected ID 1, got %d", ids[0])
	}
	if dists[0] > 0.001 {
		t.Errorf("distance to self should be ~0, got %f", dists[0])
	}
}

func TestCosineDistance(t *testing.T) {
	// Same vector → distance 0
	v := []float32{1, 2, 3}
	d := cosineDistance(v, v)
	if d > 0.001 {
		t.Errorf("self-distance should be ~0, got %f", d)
	}

	// Orthogonal vectors → distance 1
	a := []float32{1, 0}
	b := []float32{0, 1}
	d = cosineDistance(a, b)
	if math.Abs(float64(d)-1.0) > 0.001 {
		t.Errorf("orthogonal distance should be ~1, got %f", d)
	}

	// Opposite vectors → distance 2
	c := []float32{1, 0}
	e := []float32{-1, 0}
	d = cosineDistance(c, e)
	if math.Abs(float64(d)-2.0) > 0.001 {
		t.Errorf("opposite distance should be ~2, got %f", d)
	}
}
