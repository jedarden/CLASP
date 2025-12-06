// Package provider implements LLM provider backends.
package provider

import (
	"net/http"
	"strings"

	"github.com/jedarden/clasp/internal/translator"
)

// OpenAIProvider implements the Provider interface for OpenAI.
type OpenAIProvider struct {
	BaseURL      string
	apiKey       string // Optional: used for tier-specific routing
	endpointType translator.EndpointType
	targetModel  string // Cached for endpoint determination
}

// NewOpenAIProvider creates a new OpenAI provider.
func NewOpenAIProvider(baseURL string) *OpenAIProvider {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	return &OpenAIProvider{
		BaseURL:      baseURL,
		endpointType: translator.EndpointChatCompletions, // Default
	}
}

// NewOpenAIProviderWithKey creates a new OpenAI provider with an embedded API key.
// Used for multi-provider routing where each tier has its own credentials.
func NewOpenAIProviderWithKey(baseURL, apiKey string) *OpenAIProvider {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	return &OpenAIProvider{
		BaseURL:      baseURL,
		apiKey:       apiKey,
		endpointType: translator.EndpointChatCompletions,
	}
}

// Name returns the provider name.
func (p *OpenAIProvider) Name() string {
	return "openai"
}

// GetHeaders returns the HTTP headers for OpenAI API requests.
func (p *OpenAIProvider) GetHeaders(apiKey string) http.Header {
	headers := http.Header{}
	// Use embedded API key if set (for tier-specific routing), otherwise use provided key
	key := apiKey
	if p.apiKey != "" {
		key = p.apiKey
	}
	headers.Set("Authorization", "Bearer "+key)
	headers.Set("Content-Type", "application/json")
	return headers
}

// GetEndpointURL returns the appropriate endpoint URL based on the target model.
func (p *OpenAIProvider) GetEndpointURL() string {
	if p.endpointType == translator.EndpointResponses {
		return p.BaseURL + "/responses"
	}
	return p.BaseURL + "/chat/completions"
}

// GetEndpointURLForModel returns the endpoint URL for a specific model.
// This allows dynamic endpoint selection based on the model being used.
func (p *OpenAIProvider) GetEndpointURLForModel(model string) string {
	endpointType := translator.GetEndpointType(model)
	if endpointType == translator.EndpointResponses {
		return p.BaseURL + "/responses"
	}
	return p.BaseURL + "/chat/completions"
}

// SetTargetModel sets the target model and updates the endpoint type accordingly.
// Call this before GetEndpointURL() to ensure the correct endpoint is used.
func (p *OpenAIProvider) SetTargetModel(model string) {
	p.targetModel = model
	p.endpointType = translator.GetEndpointType(model)
}

// GetEndpointType returns the current endpoint type.
func (p *OpenAIProvider) GetEndpointType() translator.EndpointType {
	return p.endpointType
}

// RequiresResponsesAPI checks if the current target model requires the Responses API.
func (p *OpenAIProvider) RequiresResponsesAPI() bool {
	return p.endpointType == translator.EndpointResponses
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
