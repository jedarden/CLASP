// Package provider implements LLM provider backends.
package provider

import (
	"net/http"
)

// OpenRouterProvider implements the Provider interface for OpenRouter.
type OpenRouterProvider struct {
	BaseURL string
}

// NewOpenRouterProvider creates a new OpenRouter provider.
func NewOpenRouterProvider(baseURL string) *OpenRouterProvider {
	if baseURL == "" {
		baseURL = "https://openrouter.ai/api/v1"
	}
	return &OpenRouterProvider{BaseURL: baseURL}
}

// Name returns the provider name.
func (p *OpenRouterProvider) Name() string {
	return "openrouter"
}

// GetHeaders returns the HTTP headers for OpenRouter API requests.
func (p *OpenRouterProvider) GetHeaders(apiKey string) http.Header {
	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+apiKey)
	headers.Set("Content-Type", "application/json")
	headers.Set("HTTP-Referer", "https://github.com/jedarden/CLASP")
	headers.Set("X-Title", "CLASP Proxy")
	headers.Set("User-Agent", "CLASP/0.2.0 (+https://github.com/jedarden/CLASP)")
	return headers
}

// GetEndpointURL returns the chat completions endpoint URL.
func (p *OpenRouterProvider) GetEndpointURL() string {
	return p.BaseURL + "/chat/completions"
}

// TransformModelID returns the model ID as-is for OpenRouter.
func (p *OpenRouterProvider) TransformModelID(modelID string) string {
	// OpenRouter uses provider/model format, no transformation needed
	return modelID
}

// SupportsStreaming indicates that OpenRouter supports SSE streaming.
func (p *OpenRouterProvider) SupportsStreaming() bool {
	return true
}

// RequiresTransformation indicates that OpenRouter needs Anthropic->OpenAI translation.
func (p *OpenRouterProvider) RequiresTransformation() bool {
	return true
}
