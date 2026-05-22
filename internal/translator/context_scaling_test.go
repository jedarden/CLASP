package translator

import (
	"math"
	"testing"
)

func TestGetModelContextWindow_KnownModels(t *testing.T) {
	tests := []struct {
		name     string
		model    string
		expected int
	}{
		// Claude models
		{"claude-3-5-sonnet-20241022", "claude-3-5-sonnet-20241022", 200000},
		{"claude-3-5-haiku-20241022", "claude-3-5-haiku-20241022", 200000},
		{"claude-3-opus-20240229", "claude-3-opus-20240229", 200000},

		// OpenAI GPT-4o series
		{"gpt-4o", "gpt-4o", 128000},
		{"gpt-4o-mini", "gpt-4o-mini", 128000},

		// OpenAI O1/O3 reasoning models
		{"o1", "o1", 200000},
		{"o1-preview", "o1-preview", 128000},
		{"o3", "o3", 200000},
		{"o3-mini", "o3-mini", 200000},

		// GPT-5 series
		{"gpt-5", "gpt-5", 1000000},
		{"gpt-5-turbo", "gpt-5-turbo", 500000},
		{"codex", "codex", 1000000},

		// GPT-4
		{"gpt-4", "gpt-4", 8192},
		{"gpt-4-32k", "gpt-4-32k", 32768},

		// GPT-3.5 Turbo
		{"gpt-3.5-turbo", "gpt-3.5-turbo", 16385},

		// Google Gemini
		{"gemini-2.5-pro", "gemini-2.5-pro", 2000000},
		{"gemini-2.5-flash", "gemini-2.5-flash", 1000000},
		{"gemini-pro", "gemini-pro", 1000000},

		// Grok
		{"grok", "grok", 131072},
		{"grok-2", "grok-2", 131072},

		// Qwen
		{"qwen-turbo", "qwen-turbo", 1000000},
		{"qwen-plus", "qwen-plus", 131072},

		// DeepSeek
		{"deepseek-chat", "deepseek-chat", 128000},
		{"deepseek-coder", "deepseek-coder", 128000},

		// Ollama
		{"llama3", "llama3", 8192},
		{"mistral", "mistral", 32768},

		// OpenRouter
		{"anthropic/claude-3-5-sonnet", "anthropic/claude-3-5-sonnet", 200000},
		{"openai/gpt-4o", "openai/gpt-4o", 128000},
		{"google/gemini-pro", "google/gemini-pro", 1000000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetModelContextWindow(tt.model)
			if result != tt.expected {
				t.Errorf("GetModelContextWindow(%q) = %d, want %d", tt.model, result, tt.expected)
			}
		})
	}
}

func TestGetModelContextWindow_PrefixMatch(t *testing.T) {
	tests := []struct {
		name     string
		model    string
		expected int
	}{
		// Prefix matching for model variants (avoiding prefix collisions)
		{"claude-3.5 variant", "claude-3-5-sonnet-20241022:preview", 200000},
		{"gemini variant", "gemini-2.5-flash-experimental", 1000000},
		{"deepseek variant", "deepseek-chat-v2", 128000},
		{"qwen variant", "qwen-plus-latest", 131072},
		{"ollama tag variant", "phi3:medium", 128000},
		{"openrouter variant", "openai/gpt-4o-mini-turbo", 128000},
		{"o3 variant", "o3-mini-experimental", 200000},
		{"grok variant", "grok-code-fast-1-tweaked", 131072},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetModelContextWindow(tt.model)
			if result != tt.expected {
				t.Errorf("GetModelContextWindow(%q) = %d, want %d", tt.model, result, tt.expected)
			}
		})
	}
}

func TestGetModelContextWindow_SuffixMatch(t *testing.T) {
	tests := []struct {
		name     string
		model    string
		expected int
	}{
		// Suffix matching for custom model names
		{"custom-gpt-4o", "my-custom-gpt-4o", 128000},
		{"company-gpt-4o", "company-internal-gpt-4o", 128000},
		{"custom-gpt-5", "custom-provider-gpt-5", 1000000},
		{"custom-gemini-pro", "x-gemini-pro", 1000000},
		{"custom-mistral", "my-mistral", 32768},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetModelContextWindow(tt.model)
			if result != tt.expected {
				t.Errorf("GetModelContextWindow(%q) = %d, want %d", tt.model, result, tt.expected)
			}
		})
	}
}

func TestGetModelContextWindow_UnknownModel(t *testing.T) {
	tests := []struct {
		name     string
		model    string
		expected int
	}{
		{"unknown model", "completely-unknown-model-xyz", DefaultContextWindow},
		{"empty string", "", DefaultContextWindow},
		{"random name", "random-model-name-12345", DefaultContextWindow},
		{"variant of unknown", "unknown-model-v2", DefaultContextWindow},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetModelContextWindow(tt.model)
			if result != tt.expected {
				t.Errorf("GetModelContextWindow(%q) = %d, want %d", tt.model, result, tt.expected)
			}
		})
	}
}

func TestNewContextScaler(t *testing.T) {
	tests := []struct {
		name               string
		model              string
		expectedModelLimit int
		expectedRatio      float64
	}{
		{
			name:               "claude 200k - ratio 1.0",
			model:              "claude-3-5-sonnet-20241022",
			expectedModelLimit: 200000,
			expectedRatio:      1.0,
		},
		{
			name:               "gpt-4o 128k - ratio > 1.0",
			model:              "gpt-4o",
			expectedModelLimit: 128000,
			expectedRatio:      200000.0 / 128000.0,
		},
		{
			name:               "gemini-2.5-pro 2M - ratio < 1.0",
			model:              "gemini-2.5-pro",
			expectedModelLimit: 2000000,
			expectedRatio:      200000.0 / 2000000.0,
		},
		{
			name:               "gpt-5 1M - ratio < 1.0",
			model:              "gpt-5",
			expectedModelLimit: 1000000,
			expectedRatio:      200000.0 / 1000000.0,
		},
		{
			name:               "unknown model - default 128k",
			model:              "unknown-model",
			expectedModelLimit: DefaultContextWindow,
			expectedRatio:      200000.0 / 128000.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scaler := NewContextScaler(tt.model)

			if scaler.ModelContextLimit != tt.expectedModelLimit {
				t.Errorf("ModelContextLimit = %d, want %d", scaler.ModelContextLimit, tt.expectedModelLimit)
			}

			if scaler.ClaudeContextLimit != ClaudeContextWindow {
				t.Errorf("ClaudeContextLimit = %d, want %d", scaler.ClaudeContextLimit, ClaudeContextWindow)
			}

			if math.Abs(scaler.ScalingRatio-tt.expectedRatio) > 1e-9 {
				t.Errorf("ScalingRatio = %f, want %f", scaler.ScalingRatio, tt.expectedRatio)
			}
		})
	}
}

func TestContextScaler_ScaleTokensForClaude(t *testing.T) {
	tests := []struct {
		name         string
		model        string
		actualTokens int
		expectedScaled int
	}{
		{
			name:           "same context (claude) - no scaling",
			model:          "claude-3-5-sonnet-20241022",
			actualTokens:   100000,
			expectedScaled: 100000,
		},
		{
			name:           "smaller context (gpt-4o 128k) - scale up",
			model:          "gpt-4o",
			actualTokens:   64000,
			expectedScaled: 64000,
		},
		{
			name:           "larger context (gemini 2M) - scale down",
			model:          "gemini-2.5-pro",
			actualTokens:   500000,
			expectedScaled: 50000,
		},
		{
			name:           "gpt-5 1M - scale down",
			model:          "gpt-5",
			actualTokens:   250000,
			expectedScaled: 50000,
		},
		{
			name:           "tiny context (gpt-4 8192)",
			model:          "gpt-4",
			actualTokens:   4096,
			expectedScaled: 4096,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scaler := NewContextScaler(tt.model)
			result := scaler.ScaleTokensForClaude(tt.actualTokens)

			if result != tt.expectedScaled {
				t.Errorf("ScaleTokensForClaude(%d) = %d, want %d", tt.actualTokens, result, tt.expectedScaled)
			}
		})
	}
}

func TestContextScaler_GetRealUsagePercent(t *testing.T) {
	tests := []struct {
		name              string
		model             string
		actualTokens      int
		expectedPercent   float64
	}{
		{
			name:            "claude 50% used",
			model:           "claude-3-5-sonnet-20241022",
			actualTokens:    100000,
			expectedPercent: 50.0,
		},
		{
			name:            "gpt-4o 50% used",
			model:           "gpt-4o",
			actualTokens:    64000,
			expectedPercent: 50.0,
		},
		{
			name:            "gemini 2M 25% used",
			model:           "gemini-2.5-pro",
			actualTokens:    500000,
			expectedPercent: 25.0,
		},
		{
			name:            "gpt-5 1M 10% used",
			model:           "gpt-5",
			actualTokens:    100000,
			expectedPercent: 10.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scaler := NewContextScaler(tt.model)
			result := scaler.GetRealUsagePercent(tt.actualTokens)

			if math.Abs(result-tt.expectedPercent) > 1e-9 {
				t.Errorf("GetRealUsagePercent(%d) = %f, want %f", tt.actualTokens, result, tt.expectedPercent)
			}
		})
	}
}

func TestContextScaler_ScaleUsage(t *testing.T) {
	scaler := NewContextScaler("gemini-2.5-pro")

	inputTokens := 400000
	outputTokens := 100000

	scaledInput, scaledOutput := scaler.ScaleUsage(inputTokens, outputTokens)

	expectedInput := 40000
	expectedOutput := 10000

	if scaledInput != expectedInput {
		t.Errorf("ScaleUsage input = %d, want %d", scaledInput, expectedInput)
	}

	if scaledOutput != expectedOutput {
		t.Errorf("ScaleUsage output = %d, want %d", scaledOutput, expectedOutput)
	}
}

func TestContextScaler_ShouldScale(t *testing.T) {
	tests := []struct {
		name        string
		model       string
		shouldScale bool
	}{
		{
			name:        "claude same context - no scale",
			model:       "claude-3-5-sonnet-20241022",
			shouldScale: false,
		},
		{
			name:        "gpt-4o smaller - should scale",
			model:       "gpt-4o",
			shouldScale: true,
		},
		{
			name:        "gemini larger - should scale",
			model:       "gemini-2.5-pro",
			shouldScale: true,
		},
		{
			name:        "gpt-5 larger - should scale",
			model:       "gpt-5",
			shouldScale: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scaler := NewContextScaler(tt.model)
			result := scaler.ShouldScale()

			if result != tt.shouldScale {
				t.Errorf("ShouldScale() = %v, want %v", result, tt.shouldScale)
			}
		})
	}
}

func TestContextScaler_GetContextInfo(t *testing.T) {
	model := "gemini-2.5-pro"
	actualTokens := 500000

	scaler := NewContextScaler(model)
	info := scaler.GetContextInfo(model, actualTokens)

	expectedInfo := ContextInfo{
		ModelName:          model,
		ModelContextLimit:  2000000,
		ClaudeContextLimit: 200000,
		ScalingRatio:       0.1,
		ActualTokens:       actualTokens,
		ScaledTokens:       50000,
		ActualPercent:      25.0,
	}

	if info.ModelName != expectedInfo.ModelName {
		t.Errorf("ModelName = %q, want %q", info.ModelName, expectedInfo.ModelName)
	}

	if info.ModelContextLimit != expectedInfo.ModelContextLimit {
		t.Errorf("ModelContextLimit = %d, want %d", info.ModelContextLimit, expectedInfo.ModelContextLimit)
	}

	if info.ClaudeContextLimit != expectedInfo.ClaudeContextLimit {
		t.Errorf("ClaudeContextLimit = %d, want %d", info.ClaudeContextLimit, expectedInfo.ClaudeContextLimit)
	}

	if math.Abs(info.ScalingRatio-expectedInfo.ScalingRatio) > 1e-9 {
		t.Errorf("ScalingRatio = %f, want %f", info.ScalingRatio, expectedInfo.ScalingRatio)
	}

	if info.ActualTokens != expectedInfo.ActualTokens {
		t.Errorf("ActualTokens = %d, want %d", info.ActualTokens, expectedInfo.ActualTokens)
	}

	if info.ScaledTokens != expectedInfo.ScaledTokens {
		t.Errorf("ScaledTokens = %d, want %d", info.ScaledTokens, expectedInfo.ScaledTokens)
	}

	if math.Abs(info.ActualPercent-expectedInfo.ActualPercent) > 1e-9 {
		t.Errorf("ActualPercent = %f, want %f", info.ActualPercent, expectedInfo.ActualPercent)
	}
}

func TestContextScaler_ScaleTokensForClaude_RatioBehavior(t *testing.T) {
	// Test that scaling ratio >= 1.0 returns actual tokens (no scaling up)
	t.Run("ratio >= 1.0 returns actual tokens", func(t *testing.T) {
		// gpt-4 has 8192 context, ratio = 200000/8192 > 1
		scaler := NewContextScaler("gpt-4")
		actualTokens := 4096

		result := scaler.ScaleTokensForClaude(actualTokens)

		// Should return actual tokens, not scaled up
		if result != actualTokens {
			t.Errorf("ScaleTokensForClaude with ratio >= 1.0 should return actual tokens, got %d want %d", result, actualTokens)
		}
	})

	// Test that scaling ratio < 1.0 scales down
	t.Run("ratio < 1.0 scales down", func(t *testing.T) {
		// gemini-2.5-pro has 2M context, ratio = 200000/2000000 = 0.1
		scaler := NewContextScaler("gemini-2.5-pro")
		actualTokens := 1000000

		result := scaler.ScaleTokensForClaude(actualTokens)
		expected := 100000 // 1000000 * 0.1

		if result != expected {
			t.Errorf("ScaleTokensForClaude with ratio < 1.0 = %d, want %d", result, expected)
		}
	})
}

func TestGetModelContextWindow_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		model    string
		expected int
	}{
		{"model with colon tag", "llama3:latest", 8192},
		{"model with slash", "openai/gpt-4o", 128000},
		{"exact match with variant", "gpt-4o-2024-05-13", 128000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetModelContextWindow(tt.model)
			if result != tt.expected {
				t.Errorf("GetModelContextWindow(%q) = %d, want %d", tt.model, result, tt.expected)
			}
		})
	}
}
