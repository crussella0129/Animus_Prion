// Package retrieval implements the Manifold multi-strategy retrieval system.
// Query classification is 100% hardcoded (no LLM) for determinism and speed.
package retrieval

import (
	"regexp"
	"strings"
)

// Strategy identifies the retrieval approach.
type Strategy int

const (
	StrategySemantic   Strategy = iota // "how does authentication work?"
	StrategyStructural                 // "what calls authenticate()?"
	StrategyHybrid                     // "find the auth code and what depends on it"
	StrategyKeyword                    // "find TODO comments", quoted strings
)

func (s Strategy) String() string {
	switch s {
	case StrategySemantic:
		return "SEMANTIC"
	case StrategyStructural:
		return "STRUCTURAL"
	case StrategyHybrid:
		return "HYBRID"
	case StrategyKeyword:
		return "KEYWORD"
	default:
		return "UNKNOWN"
	}
}

// StructuralOp identifies the specific graph operation for structural queries.
type StructuralOp int

const (
	OpSearch      StructuralOp = iota // general search
	OpCallers                         // "who calls X"
	OpCallees                         // "what does X call"
	OpBlastRadius                     // "blast radius of X"
	OpInheritance                     // "subclasses of X"
)

// RoutingDecision holds the classification result.
type RoutingDecision struct {
	Strategy        Strategy
	Confidence      float64
	Reasoning       string
	SemanticQuery   string // cleaned query for vector search
	StructuralQuery string // symbol/function name for graph query
	KeywordQuery    string // exact string for grep-style search
	StructuralOp    StructuralOp
}

// Pattern sets for classification
var (
	// Symbol patterns: backticks, function(), CamelCase, snake_case_multi
	symbolPattern = regexp.MustCompile(
		"`[^`]+`" + // backtick-wrapped
			`|[a-zA-Z_]\w*\(` + // function_name(
			`|[A-Z][a-z]+[A-Z]\w*` + // CamelCase
			`|[a-z]+_[a-z]+_[a-z]+`, // snake_case_multi (3+ parts)
	)

	// Relationship patterns
	callerPattern     = regexp.MustCompile(`(?i)who\s+calls|what\s+calls|callers?\s+of|called\s+by`)
	calleePattern     = regexp.MustCompile(`(?i)what\s+does\s+\w+\s+call|callees?\s+of|calls\s+to`)
	blastRadiusPattern = regexp.MustCompile(`(?i)blast\s+radius|downstream|affected\s+by|impact\s+of`)
	inheritancePattern = regexp.MustCompile(`(?i)subclass|inherits?\s+from|extends|implements|children\s+of|parent\s+of`)

	// Semantic patterns
	semanticPatterns = regexp.MustCompile(`(?i)\bhow\b|\bwhy\b|\bfind\b|\bimplement|\berror\b|\bbug\b|\bexplain\b|\bdescribe\b|\bunderstand\b`)

	// Keyword patterns
	todoPattern  = regexp.MustCompile(`(?i)\bTODO\b|\bFIXME\b|\bHACK\b|\bDEPRECATED\b|\bXXX\b`)
	quotedPattern = regexp.MustCompile(`"[^"]+"`)

	// Hybrid indicators
	hybridPattern = regexp.MustCompile(`(?i)\band\s+(what|how|where|who)\b|\balso\s+show\b|\bfull\s+picture\b|\btogether\s+with\b`)
)

// Route classifies a query and determines the retrieval strategy.
// This is 100% hardcoded — no LLM involved, <1ms latency.
func Route(query string) RoutingDecision {
	query = strings.TrimSpace(query)
	if query == "" {
		return RoutingDecision{Strategy: StrategySemantic, Confidence: 0.0, Reasoning: "empty query"}
	}

	decision := RoutingDecision{
		SemanticQuery: query,
	}

	// 1. Check for hybrid indicators first (they modify the strategy)
	hasHybrid := hybridPattern.MatchString(query)

	// 2. Check for structural patterns (highest specificity)
	if callerPattern.MatchString(query) {
		decision.Strategy = StrategyStructural
		decision.StructuralOp = OpCallers
		decision.Confidence = 0.9
		decision.Reasoning = "caller relationship query detected"
		decision.StructuralQuery = extractSymbol(query)
		if hasHybrid {
			decision.Strategy = StrategyHybrid
		}
		return decision
	}

	if calleePattern.MatchString(query) {
		decision.Strategy = StrategyStructural
		decision.StructuralOp = OpCallees
		decision.Confidence = 0.9
		decision.Reasoning = "callee relationship query detected"
		decision.StructuralQuery = extractSymbol(query)
		if hasHybrid {
			decision.Strategy = StrategyHybrid
		}
		return decision
	}

	if blastRadiusPattern.MatchString(query) {
		decision.Strategy = StrategyStructural
		decision.StructuralOp = OpBlastRadius
		decision.Confidence = 0.85
		decision.Reasoning = "blast radius / impact query detected"
		decision.StructuralQuery = extractSymbol(query)
		if hasHybrid {
			decision.Strategy = StrategyHybrid
		}
		return decision
	}

	if inheritancePattern.MatchString(query) {
		decision.Strategy = StrategyStructural
		decision.StructuralOp = OpInheritance
		decision.Confidence = 0.85
		decision.Reasoning = "inheritance/subclass query detected"
		decision.StructuralQuery = extractSymbol(query)
		if hasHybrid {
			decision.Strategy = StrategyHybrid
		}
		return decision
	}

	// 3. Check for keyword patterns
	if todoPattern.MatchString(query) {
		decision.Strategy = StrategyKeyword
		decision.Confidence = 0.9
		decision.Reasoning = "TODO/FIXME keyword detected"
		decision.KeywordQuery = query
		return decision
	}

	if quotes := quotedPattern.FindString(query); quotes != "" {
		decision.Strategy = StrategyKeyword
		decision.Confidence = 0.85
		decision.Reasoning = "quoted string — exact match search"
		decision.KeywordQuery = strings.Trim(quotes, `"`)
		return decision
	}

	// 4. Check for symbol references without relationship context
	if symbolPattern.MatchString(query) {
		decision.Strategy = StrategyStructural
		decision.StructuralOp = OpSearch
		decision.Confidence = 0.7
		decision.Reasoning = "symbol reference detected"
		decision.StructuralQuery = extractSymbol(query)
		if hasHybrid || semanticPatterns.MatchString(query) {
			decision.Strategy = StrategyHybrid
			decision.Reasoning = "symbol + semantic context — hybrid search"
		}
		return decision
	}

	// 5. Default: semantic search
	if semanticPatterns.MatchString(query) {
		decision.Strategy = StrategySemantic
		decision.Confidence = 0.7
		decision.Reasoning = "semantic question pattern detected"
		return decision
	}

	// 6. Truly unknown — still default to semantic
	decision.Strategy = StrategySemantic
	decision.Confidence = 0.5
	decision.Reasoning = "no specific pattern matched — defaulting to semantic"
	return decision
}

// extractSymbol pulls the most likely symbol name from a query.
func extractSymbol(query string) string {
	// Try backtick-wrapped first
	if m := regexp.MustCompile("`([^`]+)`").FindStringSubmatch(query); len(m) > 1 {
		return m[1]
	}

	// Try function_name( pattern
	if m := regexp.MustCompile(`([a-zA-Z_]\w*)\(`).FindStringSubmatch(query); len(m) > 1 {
		return m[1]
	}

	// Try CamelCase
	if m := regexp.MustCompile(`([A-Z][a-z]+[A-Z]\w*)`).FindString(query); m != "" {
		return m
	}

	// Try snake_case_multi
	if m := regexp.MustCompile(`([a-z]+_[a-z]+_[a-z]+)`).FindString(query); m != "" {
		return m
	}

	return query
}
