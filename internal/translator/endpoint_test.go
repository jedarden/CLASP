package translator

import (
	"testing"
)

func TestGetEndpointType(t *testing.T) {
	tests := []struct {
		name     string
		model    string
		expected EndpointType
	}{
		// Chat Completions models
		{"gpt-4o", "gpt-4o", EndpointChatCompletions},
		{"gpt-4o-mini", "gpt-4o-mini", EndpointChatCompletions},
		{"gpt-4-turbo", "gpt-4-turbo", EndpointChatCompletions},
		{"gpt-4", "gpt-4", EndpointChatCompletions},
		{"gpt-3.5-turbo", "gpt-3.5-turbo", EndpointChatCompletions},
		{"o1-preview", "o1-preview", EndpointChatCompletions},
		{"o1-mini", "o1-mini", EndpointChatCompletions},
		{"o3-mini", "o3-mini", EndpointChatCompletions},

		// With provider prefix
		{"openai/gpt-4o", "openai/gpt-4o", EndpointChatCompletions},
		{"openai/gpt-4o-mini", "openai/gpt-4o-mini", EndpointChatCompletions},

		// Responses API models (hypothetical future models)
		{"gpt-5", "gpt-5", EndpointResponses},
		{"gpt-5.1", "gpt-5.1", EndpointResponses},
		{"gpt-5.1-codex", "gpt-5.1-codex", EndpointResponses},
		{"codex-mini", "codex", EndpointResponses},

		// With provider prefix
		{"openai/gpt-5", "openai/gpt-5", EndpointResponses},

		// Case insensitivity
		{"GPT-5", "GPT-5", EndpointResponses},
		{"GPT-4O", "GPT-4O", EndpointChatCompletions},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetEndpointType(tt.model)
			if result != tt.expected {
				t.Errorf("GetEndpointType(%q) = %v, want %v", tt.model, result, tt.expected)
			}
		})
	}
}

func TestRequiresResponsesAPI(t *testing.T) {
	if RequiresResponsesAPI("gpt-4o") {
		t.Error("gpt-4o should not require Responses API")
	}

	if !RequiresResponsesAPI("gpt-5") {
		t.Error("gpt-5 should require Responses API")
	}

	if !RequiresResponsesAPI("gpt-5.1-codex") {
		t.Error("gpt-5.1-codex should require Responses API")
	}
}

func TestIsChatCompletionsModel(t *testing.T) {
	if !IsChatCompletionsModel("gpt-4o") {
		t.Error("gpt-4o should be a Chat Completions model")
	}

	if IsChatCompletionsModel("gpt-5") {
		t.Error("gpt-5 should not be a Chat Completions model")
	}
}

func TestFilterChatCompletionsModels(t *testing.T) {
	models := []string{
		"gpt-4o",
		"gpt-4o-mini",
		"gpt-5",
		"gpt-5.1-codex",
		"gpt-3.5-turbo",
	}

	filtered := FilterChatCompletionsModels(models)

	expected := []string{
		"gpt-4o",
		"gpt-4o-mini",
		"gpt-3.5-turbo",
	}

	if len(filtered) != len(expected) {
		t.Errorf("FilterChatCompletionsModels returned %d models, want %d", len(filtered), len(expected))
	}

	for i, m := range filtered {
		if m != expected[i] {
			t.Errorf("FilterChatCompletionsModels[%d] = %q, want %q", i, m, expected[i])
		}
	}
}

func TestGetSupportedChatCompletionsModels(t *testing.T) {
	models := GetSupportedChatCompletionsModels()

	if len(models) == 0 {
		t.Error("GetSupportedChatCompletionsModels should return at least one model")
	}

	// Verify all returned models are actually Chat Completions models
	for _, m := range models {
		if RequiresResponsesAPI(m) {
			t.Errorf("GetSupportedChatCompletionsModels returned Responses API model: %q", m)
		}
	}

	// Check for expected models
	expectedModels := map[string]bool{
		"gpt-4o":        false,
		"gpt-4o-mini":   false,
		"gpt-4-turbo":   false,
		"gpt-3.5-turbo": false,
		"o1-preview":    false,
	}

	for _, m := range models {
		if _, ok := expectedModels[m]; ok {
			expectedModels[m] = true
		}
	}

	for m, found := range expectedModels {
		if !found {
			t.Errorf("GetSupportedChatCompletionsModels missing expected model: %q", m)
		}
	}
}

func TestEndpointTypeString(t *testing.T) {
	if EndpointChatCompletions.String() != "chat_completions" {
		t.Errorf("EndpointChatCompletions.String() = %q, want %q", EndpointChatCompletions.String(), "chat_completions")
	}

	if EndpointResponses.String() != "responses" {
		t.Errorf("EndpointResponses.String() = %q, want %q", EndpointResponses.String(), "responses")
	}
}
