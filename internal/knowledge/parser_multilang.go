package knowledge

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Multi-language AST extractors using regex patterns.
// These extract functions, classes, imports, and call edges from source files.
// Less precise than tree-sitter but zero-dependency and sufficient for
// knowledge graph population. Same output format (ParseResult) as ParseGoFile.

// --- Python ---

var (
	pyFuncPattern   = regexp.MustCompile(`^(\s*)def\s+(\w+)\s*\(([^)]*)\)`)
	pyClassPattern  = regexp.MustCompile(`^class\s+(\w+)(?:\(([^)]*)\))?`)
	pyImportPattern = regexp.MustCompile(`^(?:import\s+([\w.]+)|from\s+([\w.]+)\s+import\s+(.+))`)
	pyCallPattern   = regexp.MustCompile(`\b(\w+(?:\.\w+)*)\s*\(`)
	pyDecorPattern  = regexp.MustCompile(`^\s*@(\w+)`)
	pyMethodIndent  = regexp.MustCompile(`^\s{4,}def\s+`)
)

// ParsePythonFile extracts nodes and edges from a Python source file.
func ParsePythonFile(filePath string) (*ParseResult, error) {
	lines, err := readLines(filePath)
	if err != nil {
		return nil, err
	}

	result := &ParseResult{File: filePath}
	module := pythonModuleName(filePath)

	result.Nodes = append(result.Nodes, Node{
		ID: module, Name: module, Kind: KindPackage, File: filePath, Line: 1, Package: module,
	})

	var currentClass string

	for i, line := range lines {
		lineNum := i + 1

		// Class detection
		if m := pyClassPattern.FindStringSubmatch(line); m != nil {
			className := m[1]
			currentClass = className
			id := module + "." + className
			result.Nodes = append(result.Nodes, Node{
				ID: id, Name: className, Kind: KindStruct, File: filePath,
				Line: lineNum, Package: module,
			})
			result.Edges = append(result.Edges, Edge{
				Source: module, Target: id, Kind: EdgeContains, File: filePath, Line: lineNum,
			})
			// Inheritance
			if m[2] != "" {
				for _, base := range strings.Split(m[2], ",") {
					base = strings.TrimSpace(base)
					if base != "" {
						result.Edges = append(result.Edges, Edge{
							Source: id, Target: base, Kind: EdgeImplements, File: filePath, Line: lineNum,
						})
					}
				}
			}
			continue
		}

		// Function/method detection
		if m := pyFuncPattern.FindStringSubmatch(line); m != nil {
			indent := m[1]
			funcName := m[2]
			params := m[3]

			kind := KindFunction
			id := module + "." + funcName
			if len(indent) >= 4 && currentClass != "" {
				kind = KindMethod
				id = module + "." + currentClass + "." + funcName
			}

			sig := "def " + funcName + "(" + params + ")"
			result.Nodes = append(result.Nodes, Node{
				ID: id, Name: funcName, Kind: kind, File: filePath,
				Line: lineNum, Signature: sig, Package: module,
			})
			result.Edges = append(result.Edges, Edge{
				Source: module, Target: id, Kind: EdgeContains, File: filePath, Line: lineNum,
			})
			if kind == KindMethod {
				result.Edges = append(result.Edges, Edge{
					Source: id, Target: module + "." + currentClass, Kind: EdgeReceiver, File: filePath, Line: lineNum,
				})
			}
			continue
		}

		// Import detection
		if m := pyImportPattern.FindStringSubmatch(line); m != nil {
			var importPath string
			if m[1] != "" {
				importPath = m[1]
			} else {
				importPath = m[2]
			}
			importName := importPath
			if idx := strings.LastIndex(importPath, "."); idx >= 0 {
				importName = importPath[idx+1:]
			}
			id := module + ".import." + importName
			result.Nodes = append(result.Nodes, Node{
				ID: id, Name: importName, Kind: KindImport, File: filePath,
				Line: lineNum, Package: module,
			})
			result.Edges = append(result.Edges, Edge{
				Source: module, Target: importPath, Kind: EdgeImports, File: filePath, Line: lineNum,
			})
		}

		// Reset class context on unindented non-empty, non-decorator lines
		if currentClass != "" && len(line) > 0 && line[0] != ' ' && line[0] != '\t' && line[0] != '#' && !pyDecorPattern.MatchString(line) && !pyClassPattern.MatchString(line) {
			currentClass = ""
		}
	}

	// Extract call edges from function bodies (simplified — line-level)
	extractCallEdges(result, lines, module, pyCallPattern)

	return result, nil
}

// --- Rust ---

var (
	rsFuncPattern   = regexp.MustCompile(`^\s*(?:pub\s+)?(?:async\s+)?fn\s+(\w+)\s*(?:<[^>]*>)?\s*\(([^)]*)\)(?:\s*->\s*(\S+))?`)
	rsStructPattern = regexp.MustCompile(`^\s*(?:pub\s+)?struct\s+(\w+)`)
	rsEnumPattern   = regexp.MustCompile(`^\s*(?:pub\s+)?enum\s+(\w+)`)
	rsTraitPattern  = regexp.MustCompile(`^\s*(?:pub\s+)?trait\s+(\w+)`)
	rsImplPattern   = regexp.MustCompile(`^\s*impl(?:<[^>]*>)?\s+(?:(\w+)\s+for\s+)?(\w+)`)
	rsUsePattern    = regexp.MustCompile(`^\s*use\s+([\w:]+)`)
	rsCallPattern   = regexp.MustCompile(`\b(\w+(?:::\w+)*)\s*\(`)
)

// ParseRustFile extracts nodes and edges from a Rust source file.
func ParseRustFile(filePath string) (*ParseResult, error) {
	lines, err := readLines(filePath)
	if err != nil {
		return nil, err
	}

	result := &ParseResult{File: filePath}
	crate := rustCrateName(filePath)

	result.Nodes = append(result.Nodes, Node{
		ID: crate, Name: crate, Kind: KindPackage, File: filePath, Line: 1, Package: crate,
	})

	var currentImpl string

	for i, line := range lines {
		lineNum := i + 1

		// impl block detection
		if m := rsImplPattern.FindStringSubmatch(line); m != nil {
			currentImpl = m[2]
			if m[1] != "" {
				// impl Trait for Type
				result.Edges = append(result.Edges, Edge{
					Source: crate + "." + m[2], Target: m[1], Kind: EdgeImplements, File: filePath, Line: lineNum,
				})
			}
			continue
		}

		// Function detection
		if m := rsFuncPattern.FindStringSubmatch(line); m != nil {
			funcName := m[1]
			params := m[2]
			retType := m[3]

			kind := KindFunction
			id := crate + "." + funcName
			if currentImpl != "" && strings.HasPrefix(line, "    ") {
				kind = KindMethod
				id = crate + "." + currentImpl + "." + funcName
			}

			sig := "fn " + funcName + "(" + params + ")"
			if retType != "" {
				sig += " -> " + retType
			}

			result.Nodes = append(result.Nodes, Node{
				ID: id, Name: funcName, Kind: kind, File: filePath,
				Line: lineNum, Signature: sig, Package: crate,
			})
			result.Edges = append(result.Edges, Edge{
				Source: crate, Target: id, Kind: EdgeContains, File: filePath, Line: lineNum,
			})
			if kind == KindMethod {
				result.Edges = append(result.Edges, Edge{
					Source: id, Target: crate + "." + currentImpl, Kind: EdgeReceiver, File: filePath, Line: lineNum,
				})
			}
			continue
		}

		// Struct detection
		if m := rsStructPattern.FindStringSubmatch(line); m != nil {
			id := crate + "." + m[1]
			result.Nodes = append(result.Nodes, Node{
				ID: id, Name: m[1], Kind: KindStruct, File: filePath,
				Line: lineNum, Package: crate,
			})
			result.Edges = append(result.Edges, Edge{
				Source: crate, Target: id, Kind: EdgeContains, File: filePath, Line: lineNum,
			})
			continue
		}

		// Enum detection (treated as struct for graph purposes)
		if m := rsEnumPattern.FindStringSubmatch(line); m != nil {
			id := crate + "." + m[1]
			result.Nodes = append(result.Nodes, Node{
				ID: id, Name: m[1], Kind: KindType, File: filePath,
				Line: lineNum, Package: crate,
			})
			result.Edges = append(result.Edges, Edge{
				Source: crate, Target: id, Kind: EdgeContains, File: filePath, Line: lineNum,
			})
			continue
		}

		// Trait detection
		if m := rsTraitPattern.FindStringSubmatch(line); m != nil {
			id := crate + "." + m[1]
			result.Nodes = append(result.Nodes, Node{
				ID: id, Name: m[1], Kind: KindInterface, File: filePath,
				Line: lineNum, Package: crate,
			})
			result.Edges = append(result.Edges, Edge{
				Source: crate, Target: id, Kind: EdgeContains, File: filePath, Line: lineNum,
			})
			continue
		}

		// Use/import detection
		if m := rsUsePattern.FindStringSubmatch(line); m != nil {
			usePath := m[1]
			parts := strings.Split(usePath, "::")
			importName := parts[len(parts)-1]
			id := crate + ".import." + importName
			result.Nodes = append(result.Nodes, Node{
				ID: id, Name: importName, Kind: KindImport, File: filePath,
				Line: lineNum, Package: crate,
			})
			result.Edges = append(result.Edges, Edge{
				Source: crate, Target: usePath, Kind: EdgeImports, File: filePath, Line: lineNum,
			})
		}

		// Reset impl context on unindented closing brace
		if currentImpl != "" && line == "}" {
			currentImpl = ""
		}
	}

	extractCallEdges(result, lines, crate, rsCallPattern)
	return result, nil
}

// --- JavaScript / TypeScript ---

var (
	jsFuncPattern   = regexp.MustCompile(`^\s*(?:export\s+)?(?:async\s+)?function\s+(\w+)\s*\(([^)]*)\)`)
	jsArrowPattern  = regexp.MustCompile(`^\s*(?:export\s+)?(?:const|let|var)\s+(\w+)\s*=\s*(?:async\s+)?\([^)]*\)\s*=>`)
	jsClassPattern  = regexp.MustCompile(`^\s*(?:export\s+)?class\s+(\w+)(?:\s+extends\s+(\w+))?`)
	jsMethodPattern = regexp.MustCompile(`^\s+(?:async\s+)?(\w+)\s*\(([^)]*)\)\s*\{`)
	jsImportPattern = regexp.MustCompile(`^\s*import\s+(?:\{[^}]*\}\s+from\s+|(?:\w+)\s+from\s+)?['"]([^'"]+)['"]`)
	jsCallPattern   = regexp.MustCompile(`\b(\w+(?:\.\w+)*)\s*\(`)
)

// ParseJSFile extracts nodes and edges from a JavaScript or TypeScript file.
func ParseJSFile(filePath string) (*ParseResult, error) {
	lines, err := readLines(filePath)
	if err != nil {
		return nil, err
	}

	result := &ParseResult{File: filePath}
	module := jsModuleName(filePath)

	result.Nodes = append(result.Nodes, Node{
		ID: module, Name: module, Kind: KindPackage, File: filePath, Line: 1, Package: module,
	})

	var currentClass string

	for i, line := range lines {
		lineNum := i + 1

		// Class detection
		if m := jsClassPattern.FindStringSubmatch(line); m != nil {
			className := m[1]
			currentClass = className
			id := module + "." + className
			result.Nodes = append(result.Nodes, Node{
				ID: id, Name: className, Kind: KindStruct, File: filePath,
				Line: lineNum, Package: module,
			})
			result.Edges = append(result.Edges, Edge{
				Source: module, Target: id, Kind: EdgeContains, File: filePath, Line: lineNum,
			})
			if m[2] != "" {
				result.Edges = append(result.Edges, Edge{
					Source: id, Target: m[2], Kind: EdgeImplements, File: filePath, Line: lineNum,
				})
			}
			continue
		}

		// Method detection (inside class)
		if currentClass != "" {
			if m := jsMethodPattern.FindStringSubmatch(line); m != nil {
				methodName := m[1]
				if methodName != "if" && methodName != "for" && methodName != "while" && methodName != "switch" {
					id := module + "." + currentClass + "." + methodName
					sig := methodName + "(" + m[2] + ")"
					result.Nodes = append(result.Nodes, Node{
						ID: id, Name: methodName, Kind: KindMethod, File: filePath,
						Line: lineNum, Signature: sig, Package: module,
					})
					result.Edges = append(result.Edges, Edge{
						Source: id, Target: module + "." + currentClass, Kind: EdgeReceiver, File: filePath, Line: lineNum,
					})
					continue
				}
			}
		}

		// Function detection
		if m := jsFuncPattern.FindStringSubmatch(line); m != nil {
			funcName := m[1]
			id := module + "." + funcName
			sig := "function " + funcName + "(" + m[2] + ")"
			result.Nodes = append(result.Nodes, Node{
				ID: id, Name: funcName, Kind: KindFunction, File: filePath,
				Line: lineNum, Signature: sig, Package: module,
			})
			result.Edges = append(result.Edges, Edge{
				Source: module, Target: id, Kind: EdgeContains, File: filePath, Line: lineNum,
			})
			continue
		}

		// Arrow function detection
		if m := jsArrowPattern.FindStringSubmatch(line); m != nil {
			funcName := m[1]
			id := module + "." + funcName
			result.Nodes = append(result.Nodes, Node{
				ID: id, Name: funcName, Kind: KindFunction, File: filePath,
				Line: lineNum, Package: module,
			})
			result.Edges = append(result.Edges, Edge{
				Source: module, Target: id, Kind: EdgeContains, File: filePath, Line: lineNum,
			})
			continue
		}

		// Import detection
		if m := jsImportPattern.FindStringSubmatch(line); m != nil {
			importPath := m[1]
			parts := strings.Split(importPath, "/")
			importName := parts[len(parts)-1]
			id := module + ".import." + importName
			result.Nodes = append(result.Nodes, Node{
				ID: id, Name: importName, Kind: KindImport, File: filePath,
				Line: lineNum, Package: module,
			})
			result.Edges = append(result.Edges, Edge{
				Source: module, Target: importPath, Kind: EdgeImports, File: filePath, Line: lineNum,
			})
		}

		// Reset class context on closing brace at column 0
		if currentClass != "" && line == "}" {
			currentClass = ""
		}
	}

	extractCallEdges(result, lines, module, jsCallPattern)
	return result, nil
}

// --- C / C++ ---

var (
	cFuncPattern    = regexp.MustCompile(`^(?:\w[\w\s*]+)\s+(\w+)\s*\(([^)]*)\)\s*\{`)
	cStructPattern  = regexp.MustCompile(`^\s*(?:typedef\s+)?struct\s+(\w+)`)
	cIncludePattern = regexp.MustCompile(`^\s*#include\s+[<"]([^>"]+)[>"]`)
	cCallPattern    = regexp.MustCompile(`\b(\w+)\s*\(`)
)

// ParseCFile extracts nodes and edges from a C or C++ source file.
func ParseCFile(filePath string) (*ParseResult, error) {
	lines, err := readLines(filePath)
	if err != nil {
		return nil, err
	}

	result := &ParseResult{File: filePath}
	module := cModuleName(filePath)

	result.Nodes = append(result.Nodes, Node{
		ID: module, Name: module, Kind: KindPackage, File: filePath, Line: 1, Package: module,
	})

	for i, line := range lines {
		lineNum := i + 1

		// Include detection
		if m := cIncludePattern.FindStringSubmatch(line); m != nil {
			includePath := m[1]
			parts := strings.Split(includePath, "/")
			importName := strings.TrimSuffix(parts[len(parts)-1], ".h")
			id := module + ".import." + importName
			result.Nodes = append(result.Nodes, Node{
				ID: id, Name: importName, Kind: KindImport, File: filePath,
				Line: lineNum, Package: module,
			})
			result.Edges = append(result.Edges, Edge{
				Source: module, Target: includePath, Kind: EdgeImports, File: filePath, Line: lineNum,
			})
			continue
		}

		// Struct detection
		if m := cStructPattern.FindStringSubmatch(line); m != nil {
			if m[1] != "{" && m[1] != "" {
				id := module + "." + m[1]
				result.Nodes = append(result.Nodes, Node{
					ID: id, Name: m[1], Kind: KindStruct, File: filePath,
					Line: lineNum, Package: module,
				})
				result.Edges = append(result.Edges, Edge{
					Source: module, Target: id, Kind: EdgeContains, File: filePath, Line: lineNum,
				})
			}
			continue
		}

		// Function detection (must start at column 0, have a brace)
		if m := cFuncPattern.FindStringSubmatch(line); m != nil {
			funcName := m[1]
			// Skip control flow keywords
			if funcName == "if" || funcName == "for" || funcName == "while" || funcName == "switch" || funcName == "return" {
				continue
			}
			id := module + "." + funcName
			sig := funcName + "(" + strings.TrimSpace(m[2]) + ")"
			result.Nodes = append(result.Nodes, Node{
				ID: id, Name: funcName, Kind: KindFunction, File: filePath,
				Line: lineNum, Signature: sig, Package: module,
			})
			result.Edges = append(result.Edges, Edge{
				Source: module, Target: id, Kind: EdgeContains, File: filePath, Line: lineNum,
			})
		}
	}

	extractCallEdges(result, lines, module, cCallPattern)
	return result, nil
}

// --- Java ---

var (
	javaClassPattern  = regexp.MustCompile(`^\s*(?:public\s+)?(?:abstract\s+)?(?:final\s+)?class\s+(\w+)(?:\s+extends\s+(\w+))?(?:\s+implements\s+(.+?))?(?:\s*\{)`)
	javaIfacePattern  = regexp.MustCompile(`^\s*(?:public\s+)?interface\s+(\w+)`)
	javaMethodPattern = regexp.MustCompile(`^\s+(?:public|private|protected)?\s*(?:static\s+)?(?:final\s+)?(?:\w+(?:<[^>]+>)?)\s+(\w+)\s*\(([^)]*)\)`)
	javaImportPattern = regexp.MustCompile(`^\s*import\s+([\w.]+);`)
	javaCallPattern   = regexp.MustCompile(`\b(\w+(?:\.\w+)*)\s*\(`)
)

// ParseJavaFile extracts nodes and edges from a Java source file.
func ParseJavaFile(filePath string) (*ParseResult, error) {
	lines, err := readLines(filePath)
	if err != nil {
		return nil, err
	}

	result := &ParseResult{File: filePath}
	module := javaModuleName(filePath)

	result.Nodes = append(result.Nodes, Node{
		ID: module, Name: module, Kind: KindPackage, File: filePath, Line: 1, Package: module,
	})

	var currentClass string

	for i, line := range lines {
		lineNum := i + 1

		// Class detection
		if m := javaClassPattern.FindStringSubmatch(line); m != nil {
			className := m[1]
			currentClass = className
			id := module + "." + className
			result.Nodes = append(result.Nodes, Node{
				ID: id, Name: className, Kind: KindStruct, File: filePath,
				Line: lineNum, Package: module,
			})
			result.Edges = append(result.Edges, Edge{
				Source: module, Target: id, Kind: EdgeContains, File: filePath, Line: lineNum,
			})
			if m[2] != "" {
				result.Edges = append(result.Edges, Edge{
					Source: id, Target: m[2], Kind: EdgeImplements, File: filePath, Line: lineNum,
				})
			}
			continue
		}

		// Interface detection
		if m := javaIfacePattern.FindStringSubmatch(line); m != nil {
			id := module + "." + m[1]
			result.Nodes = append(result.Nodes, Node{
				ID: id, Name: m[1], Kind: KindInterface, File: filePath,
				Line: lineNum, Package: module,
			})
			result.Edges = append(result.Edges, Edge{
				Source: module, Target: id, Kind: EdgeContains, File: filePath, Line: lineNum,
			})
			continue
		}

		// Method detection
		if currentClass != "" {
			if m := javaMethodPattern.FindStringSubmatch(line); m != nil {
				methodName := m[1]
				if methodName == "if" || methodName == "for" || methodName == "while" {
					continue
				}
				id := module + "." + currentClass + "." + methodName
				sig := methodName + "(" + strings.TrimSpace(m[2]) + ")"
				result.Nodes = append(result.Nodes, Node{
					ID: id, Name: methodName, Kind: KindMethod, File: filePath,
					Line: lineNum, Signature: sig, Package: module,
				})
				result.Edges = append(result.Edges, Edge{
					Source: id, Target: module + "." + currentClass, Kind: EdgeReceiver, File: filePath, Line: lineNum,
				})
			}
		}

		// Import detection
		if m := javaImportPattern.FindStringSubmatch(line); m != nil {
			importPath := m[1]
			parts := strings.Split(importPath, ".")
			importName := parts[len(parts)-1]
			id := module + ".import." + importName
			result.Nodes = append(result.Nodes, Node{
				ID: id, Name: importName, Kind: KindImport, File: filePath,
				Line: lineNum, Package: module,
			})
			result.Edges = append(result.Edges, Edge{
				Source: module, Target: importPath, Kind: EdgeImports, File: filePath, Line: lineNum,
			})
		}
	}

	extractCallEdges(result, lines, module, javaCallPattern)
	return result, nil
}

// --- Shared helpers ---

// readLines reads a file into a slice of strings.
func readLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB line buffer
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

// extractCallEdges scans all lines for function call patterns and creates edges.
// Simplified: attributes calls to the nearest preceding function definition.
func extractCallEdges(result *ParseResult, lines []string, module string, callPattern *regexp.Regexp) {
	// Find all function/method nodes for this file to map calls to callers
	var currentCaller string
	for i, line := range lines {
		lineNum := i + 1

		// Track which function we're inside (simplistic: last function def before this line)
		for _, node := range result.Nodes {
			if node.File == result.File && node.Line == lineNum &&
				(node.Kind == KindFunction || node.Kind == KindMethod) {
				currentCaller = node.ID
			}
		}

		if currentCaller == "" {
			continue
		}

		// Find calls on this line
		matches := callPattern.FindAllStringSubmatch(line, -1)
		for _, m := range matches {
			callee := m[1]
			// Skip common keywords and builtins
			if isCommonKeyword(callee) {
				continue
			}
			result.Edges = append(result.Edges, Edge{
				Source: currentCaller, Target: callee, Kind: EdgeCalls,
				File: result.File, Line: lineNum,
			})
		}
	}
}

var commonKeywords = map[string]bool{
	"if": true, "for": true, "while": true, "switch": true, "return": true,
	"print": true, "println": true, "printf": true, "len": true, "range": true,
	"new": true, "make": true, "append": true, "delete": true, "close": true,
	"true": true, "false": true, "nil": true, "null": true, "None": true,
	"self": true, "this": true, "super": true, "class": true, "struct": true,
	"import": true, "from": true, "def": true, "fn": true, "func": true,
	"var": true, "let": true, "const": true, "mut": true, "pub": true,
	"async": true, "await": true, "try": true, "catch": true, "throw": true,
	"sizeof": true, "typeof": true, "instanceof": true,
}

func isCommonKeyword(name string) bool {
	return commonKeywords[name]
}

// Module name helpers — derive a module/package name from the file path.

func pythonModuleName(path string) string {
	base := strings.TrimSuffix(strings.TrimSuffix(filepath.Base(path), ".py"), ".pyi")
	return base
}

func rustCrateName(path string) string {
	return strings.TrimSuffix(filepath.Base(path), ".rs")
}

func jsModuleName(path string) string {
	base := filepath.Base(path)
	base = strings.TrimSuffix(base, ".tsx")
	base = strings.TrimSuffix(base, ".ts")
	base = strings.TrimSuffix(base, ".jsx")
	base = strings.TrimSuffix(base, ".js")
	return base
}

func cModuleName(path string) string {
	base := filepath.Base(path)
	base = strings.TrimSuffix(base, ".cpp")
	base = strings.TrimSuffix(base, ".cc")
	base = strings.TrimSuffix(base, ".c")
	base = strings.TrimSuffix(base, ".h")
	base = strings.TrimSuffix(base, ".hpp")
	return base
}

func javaModuleName(path string) string {
	return strings.TrimSuffix(filepath.Base(path), ".java")
}
