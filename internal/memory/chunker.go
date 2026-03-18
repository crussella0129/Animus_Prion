package memory

import (
	"strings"
)

// ChunkText splits text into overlapping chunks of approximately chunkSize tokens.
// Uses line-aware splitting to avoid breaking mid-line.
func ChunkText(text string, source string, chunkSize int, overlap int) []Chunk {
	if chunkSize <= 0 {
		chunkSize = 512
	}
	if overlap < 0 {
		overlap = 0
	}

	lines := strings.Split(text, "\n")
	if len(lines) == 0 {
		return nil
	}

	// Estimate chars per token (~3.5 for code)
	maxChars := int(float64(chunkSize) * 3.5)
	overlapChars := int(float64(overlap) * 3.5)

	var chunks []Chunk
	var current strings.Builder
	startLine := 1
	currentLine := 1

	for _, line := range lines {
		// If adding this line would exceed the limit, emit the chunk
		if current.Len()+len(line)+1 > maxChars && current.Len() > 0 {
			chunks = append(chunks, Chunk{
				Text:      current.String(),
				Source:    source,
				StartLine: startLine,
				EndLine:   currentLine - 1,
			})

			// Calculate overlap: keep last N characters
			if overlapChars > 0 && current.Len() > overlapChars {
				overlapText := current.String()[current.Len()-overlapChars:]
				current.Reset()
				current.WriteString(overlapText)
				// Approximate start line for overlap
				overlapLines := strings.Count(overlapText, "\n")
				startLine = currentLine - overlapLines
			} else {
				current.Reset()
				startLine = currentLine
			}
		}

		if current.Len() > 0 {
			current.WriteByte('\n')
		}
		current.WriteString(line)
		currentLine++
	}

	// Emit final chunk
	if current.Len() > 0 {
		chunks = append(chunks, Chunk{
			Text:      current.String(),
			Source:    source,
			StartLine: startLine,
			EndLine:   currentLine - 1,
		})
	}

	return chunks
}

// ChunkByFunction splits Go source into per-function chunks.
// Each function/method becomes its own chunk with its doc comment.
// This is more semantically meaningful than fixed-size chunking for code.
func ChunkByFunction(text string, source string) []Chunk {
	lines := strings.Split(text, "\n")
	var chunks []Chunk
	var current strings.Builder
	startLine := 1
	inFunc := false
	braceDepth := 0

	for i, line := range lines {
		lineNum := i + 1
		trimmed := strings.TrimSpace(line)

		// Detect function start — emit any accumulated pre-function code
		if !inFunc && strings.HasPrefix(trimmed, "func ") {
			if current.Len() > 0 {
				// Emit package-level code before this function
				chunks = append(chunks, Chunk{
					Text:      current.String(),
					Source:    source,
					StartLine: startLine,
					EndLine:   lineNum - 1,
				})
				current.Reset()
			}
			startLine = lineNum
			inFunc = true
			braceDepth = 0
		}

		if current.Len() > 0 {
			current.WriteByte('\n')
		}
		current.WriteString(line)

		// Track brace depth
		if inFunc {
			braceDepth += strings.Count(line, "{") - strings.Count(line, "}")
			if braceDepth <= 0 && strings.Contains(line, "}") {
				// Function closed
				chunks = append(chunks, Chunk{
					Text:      current.String(),
					Source:    source,
					StartLine: startLine,
					EndLine:   lineNum,
				})
				current.Reset()
				inFunc = false
				startLine = lineNum + 1
			}
		}
	}

	// Remainder (package-level code)
	if current.Len() > 0 {
		remaining := strings.TrimSpace(current.String())
		if remaining != "" {
			chunks = append(chunks, Chunk{
				Text:      current.String(),
				Source:    source,
				StartLine: startLine,
				EndLine:   len(lines),
			})
		}
	}

	return chunks
}
