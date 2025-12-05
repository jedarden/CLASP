// Package provider implements LLM provider backends.
package provider

import (
	"net/http"
	"strings"
)

// OpenAIProvider implements the Provider interface for OpenAI.
type OpenAIProvider struct {
	BaseURL string
}

// NewOpenAIProvider creates a new OpenAI provider.
func NewOpenAIProvider(baseURL string) *OpenAIProvider {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	return &OpenAIProvider{BaseURL: baseURL}
}

// Name returns the provider name.
func (p *OpenAIProvider) Name() string {
	return "openai"
}

// GetHeaders returns the HTTP headers for OpenAI API requests.
func (p *OpenAIProvider) GetHeaders(apiKey string) http.Header {
	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+apiKey)
	headers.Set("Content-Type", "application/json")
	return headers
}

// GetEndpointURL returns the chat completions endpoint URL.
func (p *OpenAIProvider) GetEndpointURL() string {
	return p.BaseURL + "/chat/completions"
}

// TransformModelID transforms the model ID for OpenAI.
func (p *OpenAIProvider) TransformModelID(modelID string) string {
	// Strip "openai/" prefix if present
	if strings.HasPrefix(modelID, "openai/") {
		return strings.TrimPrefix(modelID, "openai/")
	}
	return modelID
}

// SupportsStreaming indicates that OpenAI supports SSE streaming.
func (p *OpenAIProvider) SupportsStreaming() bool {
	return true
}

// RequiresTransformation indicates that OpenAI needs Anthropic->OpenAI translation.
func (p *OpenAIProvider) RequiresTransformation() bool {
	return true
}
