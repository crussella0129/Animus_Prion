package core

import (
	"encoding/json"
	"regexp"
	"strings"
)

// ToolCall represents a parsed tool invocation from LLM output.
type ToolCall struct {
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
func ParseToolCalls(text string) []ToolCall {
	text = strings.TrimSpace(text)
	if len(text) == 0 {
		return nil
	}

	// Strategy 1: Raw JSON — entire response is a tool call
	if strings.HasPrefix(text, "{") {
		var tc ToolCall
		if err := json.Unmarshal([]byte(text), &tc); err == nil && tc.Name != "" {
			return []ToolCall{tc}
		}
	}

	// Strategy 2: JSON code blocks
	matches := jsonBlockPattern.FindAllStringSubmatch(text, -1)
	if len(matches) > 0 {
		var calls []ToolCall
		for _, m := range matches {
			var tc ToolCall
			if err := json.Unmarshal([]byte(m[1]), &tc); err == nil && tc.Name != "" {
				calls = append(calls, tc)
			}
		}
		if len(calls) > 0 {
			return calls
		}
	}

	// Strategy 3: Inline JSON
	inlineMatches := inlineJSONPattern.FindAllString(text, -1)
	if len(inlineMatches) > 0 {
		var calls []ToolCall
		for _, m := range inlineMatches {
			var tc ToolCall
			if err := json.Unmarshal([]byte(m), &tc); err == nil && tc.Name != "" {
				calls = append(calls, tc)
			}
		}
		if len(calls) > 0 {
			return calls
		}
	}

	return nil
}

// DeduplicateToolCalls removes duplicate tool calls within a single response.
// Two calls are considered duplicates if they have the same name and arguments JSON.
func DeduplicateToolCalls(calls []ToolCall) []ToolCall {
	if len(calls) <= 1 {
		return calls
	}

	seen := make(map[string]bool)
	var unique []ToolCall

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
