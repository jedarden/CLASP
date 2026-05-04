// Package provider implements LLM provider backends.
package provider

import (
	"net/http"
	"strings"
)

// LiteLLMProvider implements the Provider interface for LiteLLM.
// LiteLLM is an OpenAI-compatible proxy that can route to 100+ providers.
// See: https://docs.litellm.ai/
type LiteLLMProvider struct {
	BaseURL string
	apiKey  string // Optional: used for tier-specific routing
}

// DefaultLiteLLMURL is the standard LiteLLM server address.
const DefaultLiteLLMURL = "http://localhost:4000"

// NewLiteLLMProvider creates a new LiteLLM provider with the default URL.
func NewLiteLLMProvider(baseURL string) *LiteLLMProvider {
	if baseURL == "" {
		baseURL = DefaultLiteLLMURL
	}
	baseURL = strings.TrimSuffix(baseURL, "/")
	return &LiteLLMProvider{BaseURL: baseURL}
}

// NewLiteLLMProviderWithKey creates a new LiteLLM provider with an optional API key.
// Used for multi-provider routing where each tier has its own credentials.
func NewLiteLLMProviderWithKey(baseURL, apiKey string) *LiteLLMProvider {
	if baseURL == "" {
		baseURL = DefaultLiteLLMURL
	}
	baseURL = strings.TrimSuffix(baseURL, "/")
	return &LiteLLMProvider{BaseURL: baseURL, apiKey: apiKey}
}

// Name returns the provider name.
func (p *LiteLLMProvider) Name() string {
	return "litellm"
}

// GetHeaders returns the HTTP headers for LiteLLM API requests.
// LiteLLM may require authentication depending on server configuration.
func (p *LiteLLMProvider) GetHeaders(apiKey string) http.Header {
	headers := http.Header{}
	// Use embedded API key if set (for tier-specific routing), otherwise use provided key
	key := apiKey
	if p.apiKey != "" {
		key = p.apiKey
	}
	// Only set Authorization if key is provided and non-empty
	if key != "" && key != "not-required" {
		headers.Set("Authorization", "Bearer "+key)
	}
	headers.Set("Content-Type", "application/json")
	// Set attribution header for LiteLLM analytics
	headers.Set("X-LiteLLM-Tag", "clasp-proxy")
	return headers
}

// GetEndpointURL returns the OpenAI-compatible chat completions endpoint URL.
// LiteLLM exposes an OpenAI-compatible API at /v1/chat/completions
func (p *LiteLLMProvider) GetEndpointURL() string {
	return p.BaseURL + "/v1/chat/completions"
}

// TransformModelID returns the model ID as-is for LiteLLM.
// LiteLLM accepts model IDs in provider/model format (e.g., "openai/gpt-4o")
// or direct model names depending on configuration.
func (p *LiteLLMProvider) TransformModelID(modelID string) string {
	// Strip any provider prefix if present (litellm/)
	if strings.HasPrefix(modelID, "litellm/") {
		return strings.TrimPrefix(modelID, "litellm/")
	}
	return modelID
}

// SupportsStreaming indicates that LiteLLM supports SSE streaming.
func (p *LiteLLMProvider) SupportsStreaming() bool {
	return true
}

// RequiresTransformation indicates that LiteLLM needs Anthropic->OpenAI translation.
func (p *LiteLLMProvider) RequiresTransformation() bool {
	return true
}
