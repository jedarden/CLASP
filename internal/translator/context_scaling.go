// Package translator handles protocol translation between Anthropic and OpenAI formats.
package translator

// ModelContextWindow stores the context window size for known models.
// This enables dynamic context window scaling to allow Claude Code's
// auto-compaction to work correctly with models of varying context sizes.
var ModelContextWindow = map[string]int{
	// Claude models (reference - these are passed through)
	"claude-3-5-sonnet-20241022": 200000,
	"claude-3-5-haiku-20241022":  200000,
	"claude-3-opus-20240229":     200000,
	"claude-3-sonnet-20240229":   200000,
	"claude-3-haiku-20240307":    200000,

	// OpenAI GPT-4o series
	"gpt-4o":            128000,
	"gpt-4o-2024-11-20": 128000,
	"gpt-4o-2024-08-06": 128000,
	"gpt-4o-2024-05-13": 128000,
	"gpt-4o-mini":       128000,

	// OpenAI O1/O3 reasoning models
	"o1":         200000,
	"o1-preview": 128000,
	"o1-mini":    128000,
	"o3":         200000,
	"o3-mini":    200000,

	// OpenAI GPT-5 series (Responses API)
	"gpt-5":       1000000,
	"gpt-5-turbo": 500000,
	"gpt-5.1":     1000000,
	"codex":       1000000,

	// GPT-4 Turbo
	"gpt-4-turbo":            128000,
	"gpt-4-turbo-2024-04-09": 128000,
	"gpt-4-turbo-preview":    128000,

	// GPT-4
	"gpt-4":     8192,
	"gpt-4-32k": 32768,

	// GPT-3.5 Turbo
	"gpt-3.5-turbo":     16385,
	"gpt-3.5-turbo-16k": 16385,

	// Google Gemini models
	"gemini-2.5-pro":      2000000,
	"gemini-2.5-flash":    1000000,
	"gemini-3-pro":        2000000,
	"gemini-3-flash":      1000000,
	"gemini-pro":          1000000,
	"gemini-1.5-pro":      2000000,
	"gemini-1.5-flash":    1000000,
	"gemini-1.0-pro":      32000,
	"gemini-ultra":        1000000,

	// Grok models (x-ai)
	"grok":              131072,
	"grok-2":            131072,
	"grok-3":            131072,
	"grok-code-fast-1":  131072,
	"x-ai/grok":         131072,
	"x-ai/grok-2":       131072,
	"x-ai/grok-3":       131072,

	// Qwen models
	"qwen-turbo":        1000000,
	"qwen-plus":         131072,
	"qwen-max":          32768,
	"qwen3":             131072,
	"qwen3-vl-235b":     131072,
	"qwen/qwen-turbo":   1000000,
	"qwen/qwen3":        131072,

	// MiniMax models
	"minimax-m2":        1000000,
	"minimax/minimax-m2": 1000000,

	// DeepSeek models
	"deepseek-chat":     128000,
	"deepseek-coder":    128000,
	"deepseek-r1":       128000,
	"deepseek/chat":     128000,
	"deepseek/coder":    128000,

	// Ollama common models (local)
	"llama3":           8192,
	"llama3:70b":       8192,
	"llama3.1":         131072,
	"llama3.1:405b":    131072,
	"llama3.2":         131072,
	"mistral":          32768,
	"mixtral":          32768,
	"codellama":        16384,
	"phi3":             128000,
	"phi3:medium":      128000,
	"gemma2":           8192,
	"command-r":        128000,
	"command-r-plus":   128000,

	// OpenRouter routed models (common patterns)
	"anthropic/claude-3-5-sonnet": 200000,
	"anthropic/claude-3-opus":     200000,
	"anthropic/claude-3-haiku":    200000,
	"openai/gpt-4o":               128000,
	"openai/gpt-4-turbo":          128000,
	"openai/o1":                   200000,
	"openai/o1-mini":              128000,
	"google/gemini-pro":           1000000,
	"google/gemini-2.5-pro":       2000000,
	"meta-llama/llama-3.1-405b":   131072,
	"mistralai/mistral-large":     128000,
}

// DefaultContextWindow is used when the model is not in the known list.
const DefaultContextWindow = 128000

// ClaudeContextWindow is Claude Code's assumed context window size.
const ClaudeContextWindow = 200000

// ContextScaler handles context window normalization between Claude Code
// and the actual model being used.
type ContextScaler struct {
	ModelContextLimit  int
	ClaudeContextLimit int
	ScalingRatio       float64
}

// NewContextScaler creates a new scaler for the given model.
func NewContextScaler(modelID string) *ContextScaler {
	modelContext := GetModelContextWindow(modelID)
	ratio := float64(ClaudeContextWindow) / float64(modelContext)

	return &ContextScaler{
		ModelContextLimit:  modelContext,
		ClaudeContextLimit: ClaudeContextWindow,
		ScalingRatio:       ratio,
	}
}

// GetModelContextWindow returns the context window for a model.
// Falls back to DefaultContextWindow if the model is unknown.
func GetModelContextWindow(modelID string) int {
	// Direct lookup
	if limit, ok := ModelContextWindow[modelID]; ok {
		return limit
	}

	// Try prefix matching for model variants
	for modelPrefix, limit := range ModelContextWindow {
		if len(modelID) >= len(modelPrefix) && modelID[:len(modelPrefix)] == modelPrefix {
			return limit
		}
	}

	// Try suffix matching (e.g., "my-custom-gpt-4o" should match "gpt-4o")
	for model, limit := range ModelContextWindow {
		if len(modelID) > len(model) && modelID[len(modelID)-len(model):] == model {
			return limit
		}
	}

	return DefaultContextWindow
}

// ScaleTokensForClaude scales actual token usage so Claude Code
// perceives the model's context as exactly 200k tokens.
// This enables auto-compaction to trigger at the correct percentage.
//
// Formula: scaled_tokens = (actual_tokens / model_context_limit) * 200000
//
// Example: If using Gemini 2M context and 500k tokens are used:
// - Actual: 500k / 2M = 25% used
// - Scaled: 500k * (200k / 2M) = 50k tokens
// - Claude sees: 50k / 200k = 25% used (same percentage)
func (cs *ContextScaler) ScaleTokensForClaude(actualTokens int) int {
	if cs.ScalingRatio >= 1.0 {
		// Model has same or smaller context than Claude's 200k
		// No scaling needed, but cap to avoid overflow perception
		return actualTokens
	}
	return int(float64(actualTokens) * cs.ScalingRatio)
}

// GetRealUsagePercent calculates the actual context usage percentage.
func (cs *ContextScaler) GetRealUsagePercent(actualTokens int) float64 {
	return (float64(actualTokens) / float64(cs.ModelContextLimit)) * 100
}

// ScaleUsage scales both input and output tokens for Claude Code reporting.
func (cs *ContextScaler) ScaleUsage(inputTokens, outputTokens int) (scaledInput, scaledOutput int) {
	return cs.ScaleTokensForClaude(inputTokens), cs.ScaleTokensForClaude(outputTokens)
}

// ShouldScale returns true if token scaling is needed for this model.
func (cs *ContextScaler) ShouldScale() bool {
	// Scale if model context differs significantly from Claude's 200k
	return cs.ModelContextLimit != ClaudeContextWindow
}

// GetContextInfo returns context scaling information for status display.
type ContextInfo struct {
	ModelName         string
	ModelContextLimit int
	ClaudeContextLimit int
	ScalingRatio      float64
	ActualTokens      int
	ScaledTokens      int
	ActualPercent     float64
}

// GetContextInfo returns detailed context information for status/metrics.
func (cs *ContextScaler) GetContextInfo(modelName string, actualTokens int) ContextInfo {
	scaledTokens := cs.ScaleTokensForClaude(actualTokens)
	return ContextInfo{
		ModelName:          modelName,
		ModelContextLimit:  cs.ModelContextLimit,
		ClaudeContextLimit: cs.ClaudeContextLimit,
		ScalingRatio:       cs.ScalingRatio,
		ActualTokens:       actualTokens,
		ScaledTokens:       scaledTokens,
		ActualPercent:      cs.GetRealUsagePercent(actualTokens),
	}
}
