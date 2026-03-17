package tools

import (
	"fmt"

	"github.com/crussella0129/Animus_Prion/internal/knowledge"
	"github.com/crussella0129/Animus_Prion/internal/retrieval"
)

// ManifoldSearchTool exposes Manifold retrieval as an agent tool.
type ManifoldSearchTool struct {
	executor *retrieval.Executor
	graphDB  *knowledge.GraphDB
}

// NewManifoldSearchTool creates a retrieval tool wired to the graph and vector backends.
// Either backend can be nil (will be gracefully skipped).
func NewManifoldSearchTool(executor *retrieval.Executor, graphDB *knowledge.GraphDB) *ManifoldSearchTool {
	return &ManifoldSearchTool{
		executor: executor,
		graphDB:  graphDB,
	}
}

func (t *ManifoldSearchTool) Name() string { return "manifold_search" }

func (t *ManifoldSearchTool) Description() string {
	return "Search the codebase using Manifold multi-strategy retrieval. Automatically selects between semantic (vector), structural (graph), hybrid, or keyword search based on the query."
}

func (t *ManifoldSearchTool) Parameters() ParameterSchema {
	return ParameterSchema{
		Type: "object",
		Properties: map[string]ParameterSchema{
			"query": {Type: "string", Description: "Natural language query about the codebase"},
		},
		Required: []string{"query"},
	}
}

func (t *ManifoldSearchTool) Execute(args map[string]interface{}) (string, error) {
	query, ok := args["query"].(string)
	if !ok || query == "" {
		return "", fmt.Errorf("query must be a non-empty string")
	}

	// Route the query
	decision := retrieval.Route(query)

	// If structural and we have a graph DB, try graph search directly
	if (decision.Strategy == retrieval.StrategyStructural || decision.Strategy == retrieval.StrategyHybrid) && t.graphDB != nil {
		result, err := t.executeGraphQuery(decision)
		if err == nil && result != "" {
			return fmt.Sprintf("[%s search | confidence: %.0f%%]\n\n%s", decision.Strategy, decision.Confidence*100, result), nil
		}
	}

	// Use the executor for full pipeline
	if t.executor != nil {
		searchResult, err := t.executor.Execute(decision)
		if err != nil {
			return "", fmt.Errorf("search failed: %w", err)
		}
		formatted := retrieval.FormatResults(searchResult)
		return fmt.Sprintf("[%s search | confidence: %.0f%%]\n\n%s", decision.Strategy, decision.Confidence*100, formatted), nil
	}

	return fmt.Sprintf("[%s search | confidence: %.0f%%]\nNo search backends configured.", decision.Strategy, decision.Confidence*100), nil
}

// executeGraphQuery runs a structural query directly against the graph DB.
func (t *ManifoldSearchTool) executeGraphQuery(decision retrieval.RoutingDecision) (string, error) {
	switch decision.StructuralOp {
	case retrieval.OpCallers:
		// Try exact ID first, then search
		nodes, err := t.graphDB.ResolveCallers(decision.StructuralQuery, 3)
		if err != nil || len(nodes) == 0 {
			// Try search to find the symbol
			nodes, err = t.graphDB.SearchNodes(decision.StructuralQuery)
			if err != nil || len(nodes) == 0 {
				return "", fmt.Errorf("symbol not found: %s", decision.StructuralQuery)
			}
			// Use first match as the target
			nodes, err = t.graphDB.ResolveCallers(nodes[0].ID, 3)
			if err != nil {
				return "", err
			}
		}
		return knowledge.FormatNodes(nodes), nil

	case retrieval.OpCallees:
		nodes, err := t.graphDB.GetCallees(decision.StructuralQuery)
		if err != nil || len(nodes) == 0 {
			searchResults, err := t.graphDB.SearchNodes(decision.StructuralQuery)
			if err != nil || len(searchResults) == 0 {
				return "", fmt.Errorf("symbol not found")
			}
			nodes, err = t.graphDB.GetCallees(searchResults[0].ID)
			if err != nil {
				return "", err
			}
		}
		return knowledge.FormatNodes(nodes), nil

	case retrieval.OpBlastRadius:
		nodes, err := t.graphDB.GetBlastRadius(decision.StructuralQuery, 3)
		if err != nil || len(nodes) == 0 {
			searchResults, err := t.graphDB.SearchNodes(decision.StructuralQuery)
			if err != nil || len(searchResults) == 0 {
				return "", fmt.Errorf("symbol not found")
			}
			nodes, err = t.graphDB.GetBlastRadius(searchResults[0].ID, 3)
			if err != nil {
				return "", err
			}
		}
		return knowledge.FormatNodes(nodes), nil

	case retrieval.OpInheritance:
		nodes, err := t.graphDB.GetInheritance(decision.StructuralQuery)
		if err != nil {
			return "", err
		}
		return knowledge.FormatNodes(nodes), nil

	default:
		nodes, err := t.graphDB.SearchNodes(decision.StructuralQuery)
		if err != nil {
			return "", err
		}
		return knowledge.FormatNodes(nodes), nil
	}
}
