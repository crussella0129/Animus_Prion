package llm

import (
	"fmt"
	"strings"
)

// ToolCallGrammar generates a GBNF grammar that constrains model output to
// valid tool-call JSON: {"name": "<tool_name>", "arguments": {<key-value pairs>}}
//
// The grammar accepts exactly the format that core.ParseToolCalls expects.
// Tool names are enumerated from the registry so the model can only call
// tools that actually exist.
//
// GBNF (GGML BNF) is the grammar format used by llama.cpp / llama-server.
func ToolCallGrammar(toolNames []string) string {
	if len(toolNames) == 0 {
		return ""
	}

	// Build the tool-name alternatives for the GBNF grammar.
	// In GBNF, literal strings are in double quotes. To match the JSON string
	// "read_file" (with surrounding quotes), the GBNF rule is: "\"read_file\""
	// In Go, this is: "\"\\\"" + name + "\\\"\""
	var quoted []string
	for _, name := range toolNames {
		quoted = append(quoted, "\"\\\""+name+"\\\"\"")
	}
	nameAlternatives := strings.Join(quoted, " | ")

	// GBNF grammar for a JSON tool call.
	// The grammar allows:
	//   1. A tool call: {"name": "<tool>", "arguments": {<json-object>}}
	//   2. Whitespace is allowed between tokens
	//
	// This is intentionally simple — it constrains the structure but allows
	// arbitrary JSON values in the arguments object.
	grammar := fmt.Sprintf(`root ::= ws tool-call ws

tool-call ::= "{" ws "\"name\"" ws ":" ws tool-name ws "," ws "\"arguments\"" ws ":" ws json-object ws "}"

tool-name ::= %s

json-object ::= "{" ws "}" | "{" ws json-pair (ws "," ws json-pair)* ws "}"

json-pair ::= json-string ws ":" ws json-value

json-value ::= json-string | json-number | json-object | json-array | "true" | "false" | "null"

json-string ::= "\"" json-chars "\""

json-chars ::= "" | json-char json-chars

json-char ::= [^"\\] | "\\" json-escape

json-escape ::= "\"" | "\\" | "/" | "b" | "f" | "n" | "r" | "t" | "u" [0-9a-fA-F] [0-9a-fA-F] [0-9a-fA-F] [0-9a-fA-F]

json-array ::= "[" ws "]" | "[" ws json-value (ws "," ws json-value)* ws "]"

json-number ::= "-"? json-int json-frac? json-exp?

json-int ::= "0" | [1-9] [0-9]*

json-frac ::= "." [0-9]+

json-exp ::= [eE] [+-]? [0-9]+

ws ::= [ \t\n\r]*
`, nameAlternatives)

	return grammar
}

// ToolCallOrTextGrammar generates a GBNF grammar that allows EITHER:
//  1. A valid tool-call JSON object, OR
//  2. Free-form text (for when the model wants to respond without calling a tool)
//
// This is used when the model might need to either call a tool or give a text response.
// The grammar tries tool-call first (preferred), falling back to text.
func ToolCallOrTextGrammar(toolNames []string) string {
	if len(toolNames) == 0 {
		return ""
	}

	var quoted []string
	for _, name := range toolNames {
		quoted = append(quoted, "\"\\\""+name+"\\\"\"")
	}
	nameAlternatives := strings.Join(quoted, " | ")

	grammar := fmt.Sprintf(`root ::= tool-call | free-text

tool-call ::= ws "{" ws "\"name\"" ws ":" ws tool-name ws "," ws "\"arguments\"" ws ":" ws json-object ws "}" ws

tool-name ::= %s

free-text ::= [^\x00]+

json-object ::= "{" ws "}" | "{" ws json-pair (ws "," ws json-pair)* ws "}"

json-pair ::= json-string ws ":" ws json-value

json-value ::= json-string | json-number | json-object | json-array | "true" | "false" | "null"

json-string ::= "\"" json-chars "\""

json-chars ::= "" | json-char json-chars

json-char ::= [^"\\] | "\\" json-escape

json-escape ::= "\"" | "\\" | "/" | "b" | "f" | "n" | "r" | "t" | "u" [0-9a-fA-F] [0-9a-fA-F] [0-9a-fA-F] [0-9a-fA-F]

json-array ::= "[" ws "]" | "[" ws json-value (ws "," ws json-value)* ws "]"

json-number ::= "-"? json-int json-frac? json-exp?

json-int ::= "0" | [1-9] [0-9]*

json-frac ::= "." [0-9]+

json-exp ::= [eE] [+-]? [0-9]+

ws ::= [ \t\n\r]*
`, nameAlternatives)

	return grammar
}
