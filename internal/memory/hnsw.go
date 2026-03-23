package memory

import (
	"math"
	"math/rand"
	"sort"
	"sync"
)

// HNSWIndex implements Hierarchical Navigable Small World for approximate
// nearest neighbor search. Pure Go, no external dependencies.
//
// Key parameters:
//   - M: max connections per node per layer (default 16)
//   - efConstruction: beam width during insertion (default 200)
//   - efSearch: beam width during query (default 100)
//
// For Prion's scale (up to ~100k chunks), these defaults give >95% recall
// with sub-millisecond query times.
type HNSWIndex struct {
	mu             sync.RWMutex
	nodes          []hnswNode
	entryPoint     int // index of the entry point node
	maxLevel       int
	M              int     // max connections per layer
	mMax0          int     // max connections at layer 0 (typically 2*M)
	efConstruction int     // beam width during build
	efSearch       int     // beam width during query
	levelMult      float64 // level generation multiplier: 1/ln(M)
}

// hnswNode is a single node in the HNSW graph.
type hnswNode struct {
	ID        int64 // maps to chunk ID in the vector store
	Vector    []float32
	Neighbors [][]int // neighbors[level] = list of node indices at that level
}

// candidate is a search result during traversal.
type candidate struct {
	index    int
	distance float32
}

// NewHNSWIndex creates a new HNSW index with default parameters.
func NewHNSWIndex() *HNSWIndex {
	m := 16
	return &HNSWIndex{
		entryPoint:     -1,
		maxLevel:       0,
		M:              m,
		mMax0:          2 * m,
		efConstruction: 200,
		efSearch:       100,
		levelMult:      1.0 / math.Log(float64(m)),
	}
}

// SetEfSearch sets the query-time beam width. Higher = better recall, slower.
func (h *HNSWIndex) SetEfSearch(ef int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.efSearch = ef
}

// Len returns the number of vectors in the index.
func (h *HNSWIndex) Len() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.nodes)
}

// Insert adds a vector to the index.
func (h *HNSWIndex) Insert(id int64, vector []float32) {
	h.mu.Lock()
	defer h.mu.Unlock()

	newLevel := h.randomLevel()
	newIdx := len(h.nodes)

	// Create node with empty neighbor lists for each level
	node := hnswNode{
		ID:        id,
		Vector:    vector,
		Neighbors: make([][]int, newLevel+1),
	}
	h.nodes = append(h.nodes, node)

	// First node — set as entry point
	if h.entryPoint < 0 {
		h.entryPoint = newIdx
		h.maxLevel = newLevel
		return
	}

	ep := h.entryPoint

	// Phase 1: Traverse from top to the node's level (greedy, single nearest)
	for level := h.maxLevel; level > newLevel; level-- {
		ep = h.greedyClosest(vector, ep, level)
	}

	// Phase 2: Insert at each level from newLevel down to 0
	for level := min(newLevel, h.maxLevel); level >= 0; level-- {
		// Find ef nearest neighbors at this level
		ef := h.efConstruction
		neighbors := h.searchLevel(vector, ep, ef, level)

		// Select M best neighbors
		maxConn := h.M
		if level == 0 {
			maxConn = h.mMax0
		}
		selected := selectNeighbors(neighbors, maxConn)

		// Connect new node to selected neighbors
		h.nodes[newIdx].Neighbors[level] = make([]int, len(selected))
		for i, s := range selected {
			h.nodes[newIdx].Neighbors[level][i] = s.index
		}

		// Add back-connections from neighbors to new node
		for _, s := range selected {
			h.addConnection(s.index, newIdx, level, maxConn)
		}

		// Use the closest neighbor as entry point for the next level
		if len(selected) > 0 {
			ep = selected[0].index
		}
	}

	// Update entry point if new node has higher level
	if newLevel > h.maxLevel {
		h.maxLevel = newLevel
		h.entryPoint = newIdx
	}
}

// Search finds the top-K nearest neighbors to the query vector.
// Returns (IDs, distances) sorted by distance ascending (most similar first).
func (h *HNSWIndex) Search(query []float32, topK int) ([]int64, []float32) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if len(h.nodes) == 0 || h.entryPoint < 0 {
		return nil, nil
	}

	ep := h.entryPoint

	// Traverse from top to layer 1
	for level := h.maxLevel; level > 0; level-- {
		ep = h.greedyClosest(query, ep, level)
	}

	// Search at layer 0 with efSearch beam width
	ef := h.efSearch
	if ef < topK {
		ef = topK
	}
	candidates := h.searchLevel(query, ep, ef, 0)

	// Sort by distance and take top-K
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].distance < candidates[j].distance
	})
	if len(candidates) > topK {
		candidates = candidates[:topK]
	}

	ids := make([]int64, len(candidates))
	dists := make([]float32, len(candidates))
	for i, c := range candidates {
		ids[i] = h.nodes[c.index].ID
		dists[i] = c.distance
	}
	return ids, dists
}

// --- Internal algorithms ---

// greedyClosest finds the single closest node to query at the given level,
// starting from ep. Used for traversing upper layers.
func (h *HNSWIndex) greedyClosest(query []float32, ep int, level int) int {
	bestDist := cosineDistance(query, h.nodes[ep].Vector)
	changed := true

	for changed {
		changed = false
		if level < len(h.nodes[ep].Neighbors) {
			for _, neighbor := range h.nodes[ep].Neighbors[level] {
				dist := cosineDistance(query, h.nodes[neighbor].Vector)
				if dist < bestDist {
					bestDist = dist
					ep = neighbor
					changed = true
				}
			}
		}
	}
	return ep
}

// searchLevel performs a beam search at the given level, returning up to ef candidates.
func (h *HNSWIndex) searchLevel(query []float32, ep int, ef int, level int) []candidate {
	visited := make(map[int]bool)
	visited[ep] = true

	dist := cosineDistance(query, h.nodes[ep].Vector)
	candidates := []candidate{{index: ep, distance: dist}}
	results := []candidate{{index: ep, distance: dist}}

	for len(candidates) > 0 {
		// Pop the closest candidate
		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].distance < candidates[j].distance
		})
		current := candidates[0]
		candidates = candidates[1:]

		// Check if current is farther than the worst result
		worstResult := results[len(results)-1].distance
		if current.distance > worstResult && len(results) >= ef {
			break
		}

		// Expand neighbors
		if level < len(h.nodes[current.index].Neighbors) {
			for _, neighbor := range h.nodes[current.index].Neighbors[level] {
				if visited[neighbor] {
					continue
				}
				visited[neighbor] = true

				nDist := cosineDistance(query, h.nodes[neighbor].Vector)

				if len(results) < ef || nDist < results[len(results)-1].distance {
					candidates = append(candidates, candidate{index: neighbor, distance: nDist})
					results = append(results, candidate{index: neighbor, distance: nDist})

					// Keep results sorted and bounded
					sort.Slice(results, func(i, j int) bool {
						return results[i].distance < results[j].distance
					})
					if len(results) > ef {
						results = results[:ef]
					}
				}
			}
		}
	}

	return results
}

// selectNeighbors picks the best maxConn neighbors from candidates.
// Simple selection: take the closest ones.
func selectNeighbors(candidates []candidate, maxConn int) []candidate {
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].distance < candidates[j].distance
	})
	if len(candidates) > maxConn {
		candidates = candidates[:maxConn]
	}
	return candidates
}

// addConnection adds a bidirectional connection, pruning if over capacity.
func (h *HNSWIndex) addConnection(from, to, level, maxConn int) {
	// Ensure the node has enough neighbor layers
	for len(h.nodes[from].Neighbors) <= level {
		h.nodes[from].Neighbors = append(h.nodes[from].Neighbors, nil)
	}

	neighbors := h.nodes[from].Neighbors[level]
	neighbors = append(neighbors, to)

	// Prune if over capacity
	if len(neighbors) > maxConn {
		// Keep the closest maxConn neighbors
		type neighborDist struct {
			idx  int
			dist float32
		}
		nds := make([]neighborDist, len(neighbors))
		for i, n := range neighbors {
			nds[i] = neighborDist{idx: n, dist: cosineDistance(h.nodes[from].Vector, h.nodes[n].Vector)}
		}
		sort.Slice(nds, func(i, j int) bool {
			return nds[i].dist < nds[j].dist
		})
		pruned := make([]int, maxConn)
		for i := 0; i < maxConn; i++ {
			pruned[i] = nds[i].idx
		}
		neighbors = pruned
	}

	h.nodes[from].Neighbors[level] = neighbors
}

// randomLevel generates a random level for a new node.
// Distribution: P(level=l) = (1/M)^l, giving a skip-list-like structure.
func (h *HNSWIndex) randomLevel() int {
	level := 0
	for rand.Float64() < h.levelMult/(1.0+h.levelMult) && level < 16 {
		level++
	}
	return level
}

// cosineDistance returns 1 - cosineSimilarity. Lower = more similar.
func cosineDistance(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 1.0
	}

	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	denom := math.Sqrt(normA) * math.Sqrt(normB)
	if denom == 0 {
		return 1.0
	}

	return float32(1.0 - dot/denom)
}

// min is a builtin in Go 1.21+ — no local definition needed.
