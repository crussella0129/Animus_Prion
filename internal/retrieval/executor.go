package retrieval

import (
	"sort"
	"strings"
)

// Result represents a single retrieval result.
type Result struct {
	Text     string            `json:"text"`
	Score    float64           `json:"score"`
	Source   string            `json:"source"` // file path
	Strategy Strategy          `json:"strategy"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// SearchResult holds the results from a retrieval operation.
type SearchResult struct {
	Results  []Result
	Strategy Strategy
	Query    string
}

// VectorSearcher is the interface for semantic/vector search backends.
type VectorSearcher interface {
	Search(query string, topK int) ([]Result, error)
}

// GraphSearcher is the interface for structural/graph search backends.
type GraphSearcher interface {
	Search(query string) ([]Result, error)
	GetCallers(symbol string) ([]Result, error)
	GetCallees(symbol string) ([]Result, error)
	GetBlastRadius(symbol string) ([]Result, error)
	GetInheritance(className string) ([]Result, error)
}

// KeywordSearcher is the interface for grep-style keyword search backends.
type KeywordSearcher interface {
	Search(query string, topK int) ([]Result, error)
}

// Executor dispatches retrieval queries to the appropriate backends.
type Executor struct {
	vectorSearch  VectorSearcher
	graphSearch   GraphSearcher
	keywordSearch KeywordSearcher
	topK          int
}

// NewExecutor creates a retrieval executor with the given backends.
// Any backend can be nil (will be skipped).
func NewExecutor(vector VectorSearcher, graph GraphSearcher, keyword KeywordSearcher, topK int) *Executor {
	if topK == 0 {
		topK = 5
	}
	return &Executor{
		vectorSearch:  vector,
		graphSearch:   graph,
		keywordSearch: keyword,
		topK:          topK,
	}
}

// Execute runs the retrieval pipeline based on the routing decision.
func (e *Executor) Execute(decision RoutingDecision) (SearchResult, error) {
	switch decision.Strategy {
	case StrategySemantic:
		return e.executeSemantic(decision)
	case StrategyStructural:
		return e.executeStructural(decision)
	case StrategyHybrid:
		return e.executeHybrid(decision)
	case StrategyKeyword:
		return e.executeKeyword(decision)
	default:
		return e.executeSemantic(decision)
	}
}

func (e *Executor) executeSemantic(d RoutingDecision) (SearchResult, error) {
	if e.vectorSearch == nil {
		return SearchResult{Strategy: StrategySemantic, Query: d.SemanticQuery}, nil
	}

	results, err := e.vectorSearch.Search(d.SemanticQuery, e.topK)
	if err != nil {
		return SearchResult{}, err
	}

	for i := range results {
		results[i].Strategy = StrategySemantic
	}

	return SearchResult{
		Results:  results,
		Strategy: StrategySemantic,
		Query:    d.SemanticQuery,
	}, nil
}

func (e *Executor) executeStructural(d RoutingDecision) (SearchResult, error) {
	if e.graphSearch == nil {
		return SearchResult{Strategy: StrategyStructural, Query: d.StructuralQuery}, nil
	}

	var results []Result
	var err error

	switch d.StructuralOp {
	case OpCallers:
		results, err = e.graphSearch.GetCallers(d.StructuralQuery)
	case OpCallees:
		results, err = e.graphSearch.GetCallees(d.StructuralQuery)
	case OpBlastRadius:
		results, err = e.graphSearch.GetBlastRadius(d.StructuralQuery)
	case OpInheritance:
		results, err = e.graphSearch.GetInheritance(d.StructuralQuery)
	default:
		results, err = e.graphSearch.Search(d.StructuralQuery)
	}

	if err != nil {
		return SearchResult{}, err
	}

	for i := range results {
		results[i].Strategy = StrategyStructural
	}

	return SearchResult{
		Results:  results,
		Strategy: StrategyStructural,
		Query:    d.StructuralQuery,
	}, nil
}

func (e *Executor) executeKeyword(d RoutingDecision) (SearchResult, error) {
	if e.keywordSearch == nil {
		return SearchResult{Strategy: StrategyKeyword, Query: d.KeywordQuery}, nil
	}

	results, err := e.keywordSearch.Search(d.KeywordQuery, e.topK)
	if err != nil {
		return SearchResult{}, err
	}

	for i := range results {
		results[i].Strategy = StrategyKeyword
	}

	return SearchResult{
		Results:  results,
		Strategy: StrategyKeyword,
		Query:    d.KeywordQuery,
	}, nil
}

func (e *Executor) executeHybrid(d RoutingDecision) (SearchResult, error) {
	var semanticResults, structuralResults []Result

	// Run both strategies
	if e.vectorSearch != nil {
		sr, err := e.vectorSearch.Search(d.SemanticQuery, e.topK)
		if err == nil {
			for i := range sr {
				sr[i].Strategy = StrategySemantic
			}
			semanticResults = sr
		}
	}

	if e.graphSearch != nil {
		var gr []Result
		var err error
		switch d.StructuralOp {
		case OpCallers:
			gr, err = e.graphSearch.GetCallers(d.StructuralQuery)
		case OpCallees:
			gr, err = e.graphSearch.GetCallees(d.StructuralQuery)
		case OpBlastRadius:
			gr, err = e.graphSearch.GetBlastRadius(d.StructuralQuery)
		case OpInheritance:
			gr, err = e.graphSearch.GetInheritance(d.StructuralQuery)
		default:
			gr, err = e.graphSearch.Search(d.StructuralQuery)
		}
		if err == nil {
			for i := range gr {
				gr[i].Strategy = StrategyStructural
			}
			structuralResults = gr
		}
	}

	// Fuse results using Reciprocal Rank Fusion (RRF)
	fused := reciprocalRankFusion(semanticResults, structuralResults, d.Confidence)

	// Deduplicate
	fused = deduplicateResults(fused)

	// Limit to topK
	if len(fused) > e.topK {
		fused = fused[:e.topK]
	}

	return SearchResult{
		Results:  fused,
		Strategy: StrategyHybrid,
		Query:    d.SemanticQuery,
	}, nil
}

// reciprocalRankFusion combines results from multiple strategies using RRF.
// Each result's RRF score is 1/(k+rank) where k=60 (standard constant).
// confidence weights the structural results.
func reciprocalRankFusion(semantic, structural []Result, confidence float64) []Result {
	const k = 60.0

	type scored struct {
		result Result
		score  float64
	}

	scoreMap := make(map[string]*scored)

	// Score semantic results
	for i, r := range semantic {
		key := resultKey(r)
		rank := float64(i + 1)
		score := 1.0 / (k + rank)
		if s, ok := scoreMap[key]; ok {
			s.score += score
		} else {
			scoreMap[key] = &scored{result: r, score: score}
		}
	}

	// Score structural results (weighted by confidence)
	for i, r := range structural {
		key := resultKey(r)
		rank := float64(i + 1)
		score := confidence * (1.0 / (k + rank))
		if s, ok := scoreMap[key]; ok {
			s.score += score
		} else {
			scoreMap[key] = &scored{result: r, score: score}
		}
	}

	// Convert to slice and sort by score descending
	results := make([]Result, 0, len(scoreMap))
	for _, s := range scoreMap {
		s.result.Score = s.score
		results = append(results, s.result)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return results
}

// resultKey creates a deduplication key from source + text preview.
func resultKey(r Result) string {
	preview := r.Text
	if len(preview) > 100 {
		preview = preview[:100]
	}
	return r.Source + ":" + preview
}

// deduplicateResults removes duplicate results by source+text.
func deduplicateResults(results []Result) []Result {
	seen := make(map[string]bool)
	var unique []Result

	for _, r := range results {
		key := resultKey(r)
		if !seen[key] {
			seen[key] = true
			unique = append(unique, r)
		}
	}

	return unique
}

// FormatResults formats retrieval results as a string for LLM context.
func FormatResults(sr SearchResult) string {
	if len(sr.Results) == 0 {
		return "No results found."
	}

	var sb strings.Builder
	for i, r := range sr.Results {
		sb.WriteString(strings.Repeat("-", 40))
		sb.WriteByte('\n')
		if r.Source != "" {
			sb.WriteString("Source: ")
			sb.WriteString(r.Source)
			sb.WriteByte('\n')
		}
		sb.WriteString(r.Text)
		if i < len(sr.Results)-1 {
			sb.WriteByte('\n')
		}
	}

	return sb.String()
}
