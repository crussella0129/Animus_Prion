package retrieval

import (
	"testing"
)

type mockVectorSearcher struct {
	results []Result
}

func (m *mockVectorSearcher) Search(query string, topK int) ([]Result, error) {
	return m.results, nil
}

type mockKeywordSearcher struct {
	results []Result
}

func (m *mockKeywordSearcher) Search(query string, topK int) ([]Result, error) {
	return m.results, nil
}

func TestExecutorSemantic(t *testing.T) {
	mock := &mockVectorSearcher{
		results: []Result{
			{Text: "auth handler", Source: "auth.go", Score: 0.9},
		},
	}
	exec := NewExecutor(mock, nil, nil, 5)
	decision := RoutingDecision{
		Strategy:      StrategySemantic,
		SemanticQuery: "how does auth work",
	}

	result, err := exec.Execute(decision)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(result.Results) != 1 {
		t.Errorf("expected 1 result, got %d", len(result.Results))
	}
	if result.Strategy != StrategySemantic {
		t.Errorf("strategy = %v, want SEMANTIC", result.Strategy)
	}
}

func TestExecutorKeyword(t *testing.T) {
	mock := &mockKeywordSearcher{
		results: []Result{
			{Text: "// TODO: fix this", Source: "main.go", Score: 1.0},
		},
	}
	exec := NewExecutor(nil, nil, mock, 5)
	decision := RoutingDecision{
		Strategy:     StrategyKeyword,
		KeywordQuery: "TODO",
	}

	result, err := exec.Execute(decision)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(result.Results) != 1 {
		t.Errorf("expected 1 result, got %d", len(result.Results))
	}
}

func TestExecutorNilBackends(t *testing.T) {
	exec := NewExecutor(nil, nil, nil, 5)
	decision := RoutingDecision{Strategy: StrategySemantic, SemanticQuery: "test"}

	result, err := exec.Execute(decision)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(result.Results) != 0 {
		t.Errorf("expected 0 results with nil backend, got %d", len(result.Results))
	}
}

func TestFormatResults(t *testing.T) {
	sr := SearchResult{
		Results: []Result{
			{Text: "func Foo()", Source: "foo.go"},
			{Text: "func Bar()", Source: "bar.go"},
		},
	}
	formatted := FormatResults(sr)
	if formatted == "" {
		t.Error("expected non-empty formatted output")
	}
}

func TestFormatResultsEmpty(t *testing.T) {
	sr := SearchResult{}
	formatted := FormatResults(sr)
	if formatted != "No results found." {
		t.Errorf("expected 'No results found.', got %q", formatted)
	}
}
