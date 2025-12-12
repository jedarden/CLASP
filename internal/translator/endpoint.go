// Package translator handles protocol translation between Anthropic and OpenAI formats.
package translator

import (
	"strings"
)

// EndpointType represents the type of OpenAI API endpoint to use.
type EndpointType int

const (
	// EndpointChatCompletions uses the /v1/chat/completions endpoint.
	EndpointChatCompletions EndpointType = iota
	// EndpointResponses uses the /v1/responses endpoint.
	EndpointResponses
)

// String returns the string representation of the endpoint type.
func (e EndpointType) String() string {
	switch e {
	case EndpointResponses:
		return "responses"
	default:
		return "chat_completions"
	}
}

// responsesModels lists model prefixes that require the /v1/responses endpoint.
// These models are only available through the Responses API.
var responsesModels = []string{
	// GPT-5 series (future models requiring Responses API)
	"gpt-5",
	"gpt-5.1",
	// Codex series that requires Responses API
	"codex",
	// Add more as OpenAI introduces them
}

// chatCompletionsModels lists models known to work with chat/completions.
// This is used for the model picker to filter supported models.
var chatCompletionsModels = []string{
	// GPT-4o series
	"gpt-4o",
	"gpt-4o-mini",
	"gpt-4o-2024-11-20",
	"gpt-4o-2024-08-06",
	"gpt-4o-2024-05-13",
	"gpt-4o-mini-2024-07-18",
	// GPT-4 Turbo
	"gpt-4-turbo",
	"gpt-4-turbo-2024-04-09",
	"gpt-4-turbo-preview",
	"gpt-4-0125-preview",
	"gpt-4-1106-preview",
	// GPT-4
	"gpt-4",
	"gpt-4-32k",
	"gpt-4-0613",
	"gpt-4-32k-0613",
	// GPT-3.5 Turbo
	"gpt-3.5-turbo",
	"gpt-3.5-turbo-0125",
	"gpt-3.5-turbo-1106",
	"gpt-3.5-turbo-16k",
	// O1 reasoning models
	"o1",
	"o1-preview",
	"o1-mini",
	// O3 reasoning models
	"o3",
	"o3-mini",
}

// GetEndpointType determines which API endpoint a model requires.
// Returns EndpointResponses for models that require /v1/responses,
// or EndpointChatCompletions for standard chat models.
func GetEndpointType(model string) EndpointType {
	m := strings.ToLower(model)

	// Strip provider prefix if present (e.g., "openai/gpt-4o")
	if idx := strings.Index(m, "/"); idx != -1 {
		m = m[idx+1:]
	}

	// Check if model requires Responses API
	for _, prefix := range responsesModels {
		if strings.HasPrefix(m, prefix) {
			return EndpointResponses
		}
	}

	return EndpointChatCompletions
}

// RequiresResponsesAPI checks if a model requires the Responses API.
func RequiresResponsesAPI(model string) bool {
	return GetEndpointType(model) == EndpointResponses
}

// IsChatCompletionsModel checks if a model is supported by Chat Completions API.
func IsChatCompletionsModel(model string) bool {
	return GetEndpointType(model) == EndpointChatCompletions
}

// GetSupportedChatCompletionsModels returns a list of known Chat Completions models.
// Used by the model picker to show only fully supported models.
func GetSupportedChatCompletionsModels() []string {
	return chatCompletionsModels
}

// FilterChatCompletionsModels filters a list of models to only include
// those that work with the Chat Completions API.
func FilterChatCompletionsModels(models []string) []string {
	result := make([]string, 0, len(models))
	for _, m := range models {
		if !RequiresResponsesAPI(m) {
			result = append(result, m)
		}
	}
	return result
}
