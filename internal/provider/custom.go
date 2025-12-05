// Package provider implements LLM provider backends.
package provider

import (
	"net/http"
)

// CustomProvider implements the Provider interface for custom OpenAI-compatible endpoints.
type CustomProvider struct {
	BaseURL string
	apiKey  string // Optional: used for tier-specific routing
}

// NewCustomProvider creates a new custom provider.
func NewCustomProvider(baseURL string) *CustomProvider {
	return &CustomProvider{BaseURL: baseURL}
}

// NewCustomProviderWithKey creates a new custom provider with an embedded API key.
// Used for multi-provider routing where each tier has its own credentials.
func NewCustomProviderWithKey(baseURL, apiKey string) *CustomProvider {
	return &CustomProvider{BaseURL: baseURL, apiKey: apiKey}
}

// Name returns the provider name.
func (p *CustomProvider) Name() string {
	return "custom"
}

// GetHeaders returns the HTTP headers for custom API requests.
func (p *CustomProvider) GetHeaders(apiKey string) http.Header {
	headers := http.Header{}
	// Use embedded API key if set (for tier-specific routing), otherwise use provided key
	key := apiKey
	if p.apiKey != "" {
		key = p.apiKey
	}
	if key != "" && key != "not-required" {
		headers.Set("Authorization", "Bearer "+key)
	}
	headers.Set("Content-Type", "application/json")
	return headers
}

// GetEndpointURL returns the chat completions endpoint URL.
func (p *CustomProvider) GetEndpointURL() string {
	return p.BaseURL + "/chat/completions"
}

// TransformModelID returns the model ID as-is for custom providers.
func (p *CustomProvider) TransformModelID(modelID string) string {
	return modelID
}

// SupportsStreaming indicates that custom providers support SSE streaming.
func (p *CustomProvider) SupportsStreaming() bool {
	return true
}

// RequiresTransformation indicates that custom providers need Anthropic->OpenAI translation.
func (p *CustomProvider) RequiresTransformation() bool {
	return true
}
