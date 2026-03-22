package core

// ContextWindow manages token budgets for LLM interactions.
// Scales budgets based on model size tier (small/medium/large).
type ContextWindow struct {
	ContextLength int
	SizeTier      string // "small", "medium", "large"
}

// TierRatios defines how context budget is allocated per tier.
type TierRatios struct {
	HistoryRatio  float64
	OutputRatio   float64
	ChunkRatio    float64
}

// tierConfigs maps size tiers to their budget ratios.
var tierConfigs = map[string]TierRatios{
	"small":  {HistoryRatio: 0.30, OutputRatio: 0.25, ChunkRatio: 0.125},
	"medium": {HistoryRatio: 0.50, OutputRatio: 0.25, ChunkRatio: 0.125},
	"large":  {HistoryRatio: 0.70, OutputRatio: 0.25, ChunkRatio: 0.0},
}

// Budget represents the computed token budget for a generation call.
type Budget struct {
	TotalTokens   int
	HistoryTokens int
	OutputTokens  int
	ChunkSize     int
	SystemTokens  int
}

// ComputeBudget calculates token allocation based on context length and tier.
func (cw *ContextWindow) ComputeBudget(systemPromptTokens int) Budget {
	ratios, ok := tierConfigs[cw.SizeTier]
	if !ok {
		ratios = tierConfigs["medium"] // fallback
	}

	total := cw.ContextLength
	history := int(float64(total) * ratios.HistoryRatio)
	output := int(float64(total) * ratios.OutputRatio)
	chunk := int(float64(total) * ratios.ChunkRatio)

	return Budget{
		TotalTokens:   total,
		HistoryTokens: history,
		OutputTokens:  output,
		ChunkSize:     chunk,
		SystemTokens:  systemPromptTokens,
	}
}

// EstimateTokens provides a rough token count for text.
// Uses the heuristic: ~3.1 chars/token for code, ~4.0 for prose.
// This avoids a tiktoken dependency while remaining useful for budgeting.
func EstimateTokens(text string, isCode bool) int {
	if len(text) == 0 {
		return 0
	}
	ratio := 4.0
	if isCode {
		ratio = 3.1
	}
	return int(float64(len(text)) / ratio)
}

// EstimateMessagesTokens estimates total tokens across a slice of messages.
// Each message adds ~4 tokens of overhead (role, separators).
func EstimateMessagesTokens(messages []Message) int {
	total := 0
	for _, m := range messages {
		total += EstimateTokens(m.Content, false) + 4 // overhead per message
	}
	return total
}

// TrimMessages removes oldest messages to fit within a token budget.
// Always preserves the system message (index 0) and the latest user message.
func TrimMessages(messages []Message, maxTokens int) []Message {
	if len(messages) <= 2 {
		return messages
	}

	current := EstimateMessagesTokens(messages)
	if current <= maxTokens {
		return messages
	}

	// Keep first (system) and last (user) messages; trim from the middle
	result := make([]Message, 0, len(messages))
	result = append(result, messages[0]) // system prompt

	// Walk backwards from second-to-last, collecting until budget exceeded
	remaining := messages[1:]
	budget := maxTokens - EstimateTokens(messages[0].Content, false) - 4

	// If the system prompt alone exceeds the budget, return system + last message only
	if budget <= 0 {
		result = append(result, remaining[len(remaining)-1])
		return result
	}

	// Find the cutoff index (oldest message to keep)
	cutoff := len(remaining)
	for i := len(remaining) - 1; i >= 0; i-- {
		cost := EstimateTokens(remaining[i].Content, false) + 4
		if budget-cost < 0 && i < len(remaining)-1 {
			break
		}
		budget -= cost
		cutoff = i
	}

	// Append from cutoff to end (O(n), no prepend)
	result = append(result, remaining[cutoff:]...)
	return result
}
