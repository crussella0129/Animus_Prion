package retrieval

import (
	"testing"
)

func TestRouteSemanticQueries(t *testing.T) {
	queries := []string{
		"how does authentication work?",
		"explain the error handling approach",
		"why is this function slow?",
		"describe the data flow",
	}

	for _, q := range queries {
		d := Route(q)
		if d.Strategy != StrategySemantic {
			t.Errorf("Route(%q) = %v, want SEMANTIC", q, d.Strategy)
		}
	}
}

func TestRouteStructuralQueries(t *testing.T) {
	tests := []struct {
		query    string
		strategy Strategy
		op       StructuralOp
	}{
		{"who calls authenticate()?", StrategyStructural, OpCallers},
		{"what calls the validate function", StrategyStructural, OpCallers},
		{"callers of handleRequest", StrategyStructural, OpCallers},
		{"blast radius of ConfigManager", StrategyStructural, OpBlastRadius},
		{"downstream impact of changing parse()", StrategyStructural, OpBlastRadius},
		{"subclasses of BaseProvider", StrategyStructural, OpInheritance},
		{"what inherits from Tool", StrategyStructural, OpInheritance},
	}

	for _, tt := range tests {
		d := Route(tt.query)
		if d.Strategy != tt.strategy {
			t.Errorf("Route(%q) strategy = %v, want %v", tt.query, d.Strategy, tt.strategy)
		}
		if d.StructuralOp != tt.op {
			t.Errorf("Route(%q) op = %v, want %v", tt.query, d.StructuralOp, tt.op)
		}
	}
}

func TestRouteKeywordQueries(t *testing.T) {
	tests := []struct {
		query    string
		strategy Strategy
	}{
		{"find TODO comments", StrategyKeyword},
		{"FIXME annotations", StrategyKeyword},
		{`search for "connection_pool"`, StrategyKeyword},
	}

	for _, tt := range tests {
		d := Route(tt.query)
		if d.Strategy != tt.strategy {
			t.Errorf("Route(%q) = %v, want %v", tt.query, d.Strategy, tt.strategy)
		}
	}
}

func TestRouteHybridQueries(t *testing.T) {
	queries := []string{
		"who calls authenticate() and how is it used",
		"blast radius of parse() and what does it affect",
	}

	for _, q := range queries {
		d := Route(q)
		if d.Strategy != StrategyHybrid {
			t.Errorf("Route(%q) = %v, want HYBRID", q, d.Strategy)
		}
	}
}

func TestRouteSymbolDetection(t *testing.T) {
	tests := []struct {
		query    string
		symbol   string
	}{
		{"who calls `authenticate`?", "authenticate"},
		{"callers of handleRequest()", "handleRequest"},
		{"blast radius of ConfigManager", "ConfigManager"},
	}

	for _, tt := range tests {
		d := Route(tt.query)
		if d.StructuralQuery != tt.symbol {
			t.Errorf("Route(%q) symbol = %q, want %q", tt.query, d.StructuralQuery, tt.symbol)
		}
	}
}

func TestRouteEmptyQuery(t *testing.T) {
	d := Route("")
	if d.Confidence != 0.0 {
		t.Errorf("empty query confidence = %f, want 0.0", d.Confidence)
	}
}

func TestRouteConfidence(t *testing.T) {
	// Structural queries should have high confidence
	d := Route("who calls authenticate()?")
	if d.Confidence < 0.8 {
		t.Errorf("structural query confidence = %f, want >= 0.8", d.Confidence)
	}

	// Unknown queries should have low confidence
	d = Route("something vague about code")
	if d.Confidence > 0.6 {
		t.Errorf("vague query confidence = %f, want <= 0.6", d.Confidence)
	}
}

func TestRRFusion(t *testing.T) {
	semantic := []Result{
		{Text: "semantic result 1", Source: "a.go", Score: 0.9},
		{Text: "shared result", Source: "b.go", Score: 0.8},
	}
	structural := []Result{
		{Text: "shared result", Source: "b.go", Score: 0.9},
		{Text: "structural result 1", Source: "c.go", Score: 0.7},
	}

	fused := reciprocalRankFusion(semantic, structural, 0.9)
	if len(fused) != 3 {
		t.Fatalf("expected 3 fused results, got %d", len(fused))
	}

	// The shared result should rank highest (boosted by both strategies)
	if fused[0].Source != "b.go" {
		t.Errorf("expected shared result (b.go) to rank first, got %s", fused[0].Source)
	}
}

func TestDeduplicateResults(t *testing.T) {
	results := []Result{
		{Text: "same text", Source: "a.go"},
		{Text: "same text", Source: "a.go"},
		{Text: "different", Source: "b.go"},
	}

	unique := deduplicateResults(results)
	if len(unique) != 2 {
		t.Errorf("expected 2 unique results, got %d", len(unique))
	}
}
