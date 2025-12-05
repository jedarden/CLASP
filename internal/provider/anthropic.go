// Package provider implements LLM provider backends.
package provider

import (
	"net/http"
)

// AnthropicProvider implements the Provider interface for direct Anthropic API access.
// This provider operates in passthrough mode - no protocol translation is needed.
type AnthropicProvider struct {
	BaseURL string
	apiKey  string
}

// NewAnthropicProvider creates a new Anthropic provider.
func NewAnthropicProvider(baseURL string) *AnthropicProvider {
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}
	return &AnthropicProvider{BaseURL: baseURL}
}

// NewAnthropicProviderWithKey creates a new Anthropic provider with an embedded API key.
// Used for multi-provider routing where each tier has its own credentials.
func NewAnthropicProviderWithKey(baseURL, apiKey string) *AnthropicProvider {
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}
	return &AnthropicProvider{BaseURL: baseURL, apiKey: apiKey}
}

// Name returns the provider name.
func (p *AnthropicProvider) Name() string {
	return "anthropic"
}

// GetHeaders returns the HTTP headers for Anthropic API requests.
func (p *AnthropicProvider) GetHeaders(apiKey string) http.Header {
	headers := http.Header{}
	// Use embedded API key if set (for tier-specific routing), otherwise use provided key
	key := apiKey
	if p.apiKey != "" {
		key = p.apiKey
	}
	headers.Set("x-api-key", key)
	headers.Set("Content-Type", "application/json")
	headers.Set("anthropic-version", "2023-06-01")
	return headers
}

// GetEndpointURL returns the messages endpoint URL.
func (p *AnthropicProvider) GetEndpointURL() string {
	return p.BaseURL + "/v1/messages"
}

// TransformModelID passes through the model ID unchanged for Anthropic.
func (p *AnthropicProvider) TransformModelID(modelID string) string {
	// Anthropic models are passed through unchanged
	return modelID
}

// SupportsStreaming indicates that Anthropic supports SSE streaming.
func (p *AnthropicProvider) SupportsStreaming() bool {
	return true
}

// RequiresTransformation returns false - Anthropic is passthrough mode.
// The incoming request is already in Anthropic format, so no translation is needed.
func (p *AnthropicProvider) RequiresTransformation() bool {
	return false
}
