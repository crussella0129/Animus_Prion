// Package knowledge provides code intelligence via AST parsing and graph storage.
// Currently supports Go source files via the standard library's go/ast package.
// Other languages will be added via tree-sitter bindings.
package knowledge

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
)

// NodeKind classifies a code entity in the knowledge graph.
type NodeKind string

const (
	KindFunction  NodeKind = "function"
	KindMethod    NodeKind = "method"
	KindStruct    NodeKind = "struct"
	KindInterface NodeKind = "interface"
	KindPackage   NodeKind = "package"
	KindImport    NodeKind = "import"
	KindVariable  NodeKind = "variable"
	KindConstant  NodeKind = "constant"
	KindType      NodeKind = "type" // type aliases
)

// EdgeKind classifies the relationship between two code entities.
type EdgeKind string

const (
	EdgeCalls      EdgeKind = "calls"
	EdgeImports    EdgeKind = "imports"
	EdgeImplements EdgeKind = "implements"
	EdgeContains   EdgeKind = "contains" // package contains function
	EdgeReceiver   EdgeKind = "receiver" // method has receiver type
	EdgeReturns    EdgeKind = "returns"  // function returns type
	EdgeReferences EdgeKind = "references"
)

// Node represents a code entity extracted from source.
type Node struct {
	ID        string   `json:"id"`   // unique: "pkg.Name" or "pkg.Type.Method"
	Name      string   `json:"name"` // short name
	Kind      NodeKind `json:"kind"`
	File      string   `json:"file"`       // source file path
	Line      int      `json:"line"`       // line number
	Signature string   `json:"signature"`  // function signature or type definition
	DocString string   `json:"doc_string"` // doc comment
	Package   string   `json:"package"`    // package name
}

// Edge represents a relationship between two nodes.
type Edge struct {
	Source string   `json:"source"` // source node ID
	Target string   `json:"target"` // target node ID
	Kind   EdgeKind `json:"kind"`
	File   string   `json:"file"` // where the relationship was found
	Line   int      `json:"line"`
}

// ParseResult holds all nodes and edges extracted from a single file.
type ParseResult struct {
	Nodes []Node
	Edges []Edge
	File  string
}

// ParseGoFile extracts nodes and edges from a Go source file.
func ParseGoFile(filePath string) (*ParseResult, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", filePath, err)
	}

	result := &ParseResult{File: filePath}
	pkgName := node.Name.Name

	// Package node
	result.Nodes = append(result.Nodes, Node{
		ID:      pkgName,
		Name:    pkgName,
		Kind:    KindPackage,
		File:    filePath,
		Line:    fset.Position(node.Pos()).Line,
		Package: pkgName,
	})

	// Walk the AST
	ast.Inspect(node, func(n ast.Node) bool {
		switch decl := n.(type) {
		case *ast.FuncDecl:
			result.extractFunction(decl, fset, pkgName, filePath)

		case *ast.GenDecl:
			result.extractGenDecl(decl, fset, pkgName, filePath)

		case *ast.ImportSpec:
			result.extractImport(decl, fset, pkgName, filePath)
		}
		return true
	})

	// Second pass: extract call edges from function bodies
	ast.Inspect(node, func(n ast.Node) bool {
		if fn, ok := n.(*ast.FuncDecl); ok {
			result.extractCalls(fn, fset, pkgName, filePath)
		}
		return true
	})

	return result, nil
}

// extractFunction extracts a function or method declaration.
func (r *ParseResult) extractFunction(fn *ast.FuncDecl, fset *token.FileSet, pkg, file string) {
	pos := fset.Position(fn.Pos())
	name := fn.Name.Name

	kind := KindFunction
	id := pkg + "." + name
	var receiverType string

	// Check if it's a method (has receiver)
	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		kind = KindMethod
		receiverType = exprToString(fn.Recv.List[0].Type)
		id = pkg + "." + receiverType + "." + name
	}

	// Build signature
	sig := formatFuncSignature(fn)

	// Extract doc comment
	doc := ""
	if fn.Doc != nil {
		doc = strings.TrimSpace(fn.Doc.Text())
	}

	r.Nodes = append(r.Nodes, Node{
		ID:        id,
		Name:      name,
		Kind:      kind,
		File:      file,
		Line:      pos.Line,
		Signature: sig,
		DocString: doc,
		Package:   pkg,
	})

	// Package contains function
	r.Edges = append(r.Edges, Edge{
		Source: pkg,
		Target: id,
		Kind:   EdgeContains,
		File:   file,
		Line:   pos.Line,
	})

	// Method has receiver type
	if receiverType != "" {
		r.Edges = append(r.Edges, Edge{
			Source: id,
			Target: pkg + "." + receiverType,
			Kind:   EdgeReceiver,
			File:   file,
			Line:   pos.Line,
		})
	}
}

// extractGenDecl extracts type, var, and const declarations.
func (r *ParseResult) extractGenDecl(decl *ast.GenDecl, fset *token.FileSet, pkg, file string) {
	for _, spec := range decl.Specs {
		switch s := spec.(type) {
		case *ast.TypeSpec:
			pos := fset.Position(s.Pos())
			kind := KindType
			switch s.Type.(type) {
			case *ast.StructType:
				kind = KindStruct
			case *ast.InterfaceType:
				kind = KindInterface
			}

			id := pkg + "." + s.Name.Name
			doc := ""
			if decl.Doc != nil {
				doc = strings.TrimSpace(decl.Doc.Text())
			}

			r.Nodes = append(r.Nodes, Node{
				ID:        id,
				Name:      s.Name.Name,
				Kind:      kind,
				File:      file,
				Line:      pos.Line,
				Signature: exprToString(s.Type),
				DocString: doc,
				Package:   pkg,
			})

			r.Edges = append(r.Edges, Edge{
				Source: pkg,
				Target: id,
				Kind:   EdgeContains,
				File:   file,
				Line:   pos.Line,
			})

		case *ast.ValueSpec:
			pos := fset.Position(s.Pos())
			kind := KindVariable
			if decl.Tok == token.CONST {
				kind = KindConstant
			}
			for _, name := range s.Names {
				if name.Name == "_" {
					continue
				}
				id := pkg + "." + name.Name
				r.Nodes = append(r.Nodes, Node{
					ID:      id,
					Name:    name.Name,
					Kind:    kind,
					File:    file,
					Line:    pos.Line,
					Package: pkg,
				})
			}
		}
	}
}

// extractImport extracts an import declaration.
func (r *ParseResult) extractImport(imp *ast.ImportSpec, fset *token.FileSet, pkg, file string) {
	pos := fset.Position(imp.Pos())
	path := strings.Trim(imp.Path.Value, `"`)
	importName := filepath.Base(path)
	if imp.Name != nil {
		importName = imp.Name.Name
	}

	id := pkg + ".import." + importName
	r.Nodes = append(r.Nodes, Node{
		ID:      id,
		Name:    importName,
		Kind:    KindImport,
		File:    file,
		Line:    pos.Line,
		Package: pkg,
	})

	r.Edges = append(r.Edges, Edge{
		Source: pkg,
		Target: path,
		Kind:   EdgeImports,
		File:   file,
		Line:   pos.Line,
	})
}

// extractCalls walks a function body to find call expressions.
func (r *ParseResult) extractCalls(fn *ast.FuncDecl, fset *token.FileSet, pkg, file string) {
	if fn.Body == nil {
		return
	}

	callerName := fn.Name.Name
	callerID := pkg + "." + callerName
	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		receiverType := exprToString(fn.Recv.List[0].Type)
		callerID = pkg + "." + receiverType + "." + callerName
	}

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		pos := fset.Position(call.Pos())
		calleeName := callExprName(call)
		if calleeName == "" {
			return true
		}

		r.Edges = append(r.Edges, Edge{
			Source: callerID,
			Target: calleeName,
			Kind:   EdgeCalls,
			File:   file,
			Line:   pos.Line,
		})

		return true
	})
}

// callExprName extracts a human-readable name from a CallExpr.
func callExprName(call *ast.CallExpr) string {
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		return fn.Name
	case *ast.SelectorExpr:
		if x, ok := fn.X.(*ast.Ident); ok {
			return x.Name + "." + fn.Sel.Name
		}
		return fn.Sel.Name
	}
	return ""
}

// exprToString converts an AST expression to a string representation.
func exprToString(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.StarExpr:
		return "*" + exprToString(e.X)
	case *ast.SelectorExpr:
		return exprToString(e.X) + "." + e.Sel.Name
	case *ast.ArrayType:
		return "[]" + exprToString(e.Elt)
	case *ast.MapType:
		return "map[" + exprToString(e.Key) + "]" + exprToString(e.Value)
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.StructType:
		return "struct{}"
	default:
		return fmt.Sprintf("%T", expr)
	}
}

// formatFuncSignature builds a readable function signature.
func formatFuncSignature(fn *ast.FuncDecl) string {
	var sb strings.Builder
	sb.WriteString("func ")
	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		sb.WriteString("(")
		sb.WriteString(exprToString(fn.Recv.List[0].Type))
		sb.WriteString(") ")
	}
	sb.WriteString(fn.Name.Name)
	sb.WriteString("(")

	if fn.Type.Params != nil {
		params := make([]string, 0, len(fn.Type.Params.List))
		for _, p := range fn.Type.Params.List {
			typeName := exprToString(p.Type)
			for _, name := range p.Names {
				params = append(params, name.Name+" "+typeName)
			}
			if len(p.Names) == 0 {
				params = append(params, typeName)
			}
		}
		sb.WriteString(strings.Join(params, ", "))
	}
	sb.WriteString(")")

	if fn.Type.Results != nil && len(fn.Type.Results.List) > 0 {
		results := make([]string, 0, len(fn.Type.Results.List))
		for _, r := range fn.Type.Results.List {
			results = append(results, exprToString(r.Type))
		}
		if len(results) == 1 {
			sb.WriteString(" " + results[0])
		} else {
			sb.WriteString(" (" + strings.Join(results, ", ") + ")")
		}
	}

	return sb.String()
}
