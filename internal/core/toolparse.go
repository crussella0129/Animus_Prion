package core

import (
	"encoding/json"
	"regexp"
	"strings"
)

// ParsedToolCall represents a tool invocation extracted from LLM text output.
// Distinct from core.ToolCall which is the OpenAI-format function call metadata.
type ParsedToolCall struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// jsonBlockPattern matches ```json ... ``` code blocks.
var jsonBlockPattern = regexp.MustCompile("(?s)```(?:json)?\\s*\\n?(\\{.*?\\})\\s*```")

// inlineJSONPattern matches inline JSON objects with "name" and "arguments" keys.
var inlineJSONPattern = regexp.MustCompile(`\{[^{}]*"name"\s*:\s*"[^"]+"\s*,\s*"arguments"\s*:\s*\{[^}]*\}[^}]*\}`)

// ParseToolCalls extracts tool calls from LLM output text.
// Tries three strategies in order:
//  1. Raw JSON (entire response is a single tool call)
//  2. JSON code blocks (```json { ... } ```)
//  3. Inline JSON objects with name + arguments
func ParseToolCalls(text string) []ParsedToolCall {
	text = strings.TrimSpace(text)
	if len(text) == 0 {
		return nil
	}

	// Strategy 1: Raw JSON — entire response is a tool call
	if strings.HasPrefix(text, "{") {
		var tc ParsedToolCall
		if err := json.Unmarshal([]byte(text), &tc); err == nil && tc.Name != "" {
			return []ParsedToolCall{tc}
		}
	}

	// Strategy 2: JSON code blocks
	matches := jsonBlockPattern.FindAllStringSubmatch(text, -1)
	if len(matches) > 0 {
		var calls []ParsedToolCall
		for _, m := range matches {
			var tc ParsedToolCall
			if err := json.Unmarshal([]byte(m[1]), &tc); err == nil && tc.Name != "" {
				calls = append(calls, tc)
			}
		}
		if len(calls) > 0 {
			return calls
		}
	}

	// Strategy 3: Inline JSON (regex — may miss nested braces)
	inlineMatches := inlineJSONPattern.FindAllString(text, -1)
	if len(inlineMatches) > 0 {
		var calls []ParsedToolCall
		for _, m := range inlineMatches {
			var tc ParsedToolCall
			if err := json.Unmarshal([]byte(m), &tc); err == nil && tc.Name != "" {
				calls = append(calls, tc)
			}
		}
		if len(calls) > 0 {
			return calls
		}
	}

	// Strategy 4: Brace-counting extraction for nested JSON the regex missed
	extracted := extractBalancedJSON(text)
	if len(extracted) > 0 {
		var calls []ParsedToolCall
		for _, obj := range extracted {
			var tc ParsedToolCall
			if err := json.Unmarshal([]byte(obj), &tc); err == nil && tc.Name != "" {
				calls = append(calls, tc)
			}
		}
		if len(calls) > 0 {
			return calls
		}
	}

	return nil
}

// extractBalancedJSON finds JSON objects with "name" key using brace counting.
// Handles nested braces that the regex strategies miss.
func extractBalancedJSON(text string) []string {
	var results []string
	i := 0
	for i < len(text) {
		// Find potential JSON start
		idx := strings.Index(text[i:], `{"name"`)
		if idx == -1 {
			// Also try with spaces: { "name"
			idx = strings.Index(text[i:], `{ "name"`)
			if idx == -1 {
				break
			}
		}
		start := i + idx

		// Count braces to find the matching close
		depth := 0
		inString := false
		escaped := false
		end := -1

		for j := start; j < len(text); j++ {
			ch := text[j]
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' && inString {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = !inString
				continue
			}
			if inString {
				continue
			}
			if ch == '{' {
				depth++
			} else if ch == '}' {
				depth--
				if depth == 0 {
					end = j + 1
					break
				}
			}
		}

		if end > start {
			candidate := text[start:end]
			// Quick validation: does it parse as JSON?
			var raw map[string]interface{}
			if json.Unmarshal([]byte(candidate), &raw) == nil {
				if _, hasName := raw["name"]; hasName {
					results = append(results, candidate)
				}
			}
			i = end
		} else {
			i = start + 1
		}
	}
	return results
}

// DeduplicateToolCalls removes duplicate tool calls within a single response.
// Two calls are considered duplicates if they have the same name and arguments JSON.
func DeduplicateToolCalls(calls []ParsedToolCall) []ParsedToolCall {
	if len(calls) <= 1 {
		return calls
	}

	seen := make(map[string]bool)
	var unique []ParsedToolCall

	for _, tc := range calls {
		key := tc.Name
		if argBytes, err := json.Marshal(tc.Arguments); err == nil {
			key += ":" + string(argBytes)
		}
		if !seen[key] {
			seen[key] = true
			unique = append(unique, tc)
		}
	}

	return unique
}
