// Package provider implements LLM provider backends.
package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

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

// OpenAIModel represents a model from OpenAI's models endpoint.
type OpenAIModel struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// OpenAIModelsResponse is the response from /v1/models endpoint.
type OpenAIModelsResponse struct {
	Data   []OpenAIModel `json:"data"`
	Object string        `json:"object"`
}

// ListModels returns available models from OpenAI's /v1/models endpoint.
// Requires a valid API key to be set.
func (p *OpenAIProvider) ListModels(apiKey string) ([]string, error) {
	key := apiKey
	if p.apiKey != "" {
		key = p.apiKey
	}
	if key == "" {
		return nil, fmt.Errorf("API key required to list models")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.BaseURL+"/models", http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+key)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OpenAI returned status %d", resp.StatusCode)
	}

	var modelsResp OpenAIModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Filter to only chat-capable models
	var models []string
	for _, m := range modelsResp.Data {
		// Include GPT models, O1/O3 reasoning models, and exclude embedding/audio/tts models
		if isChatModel(m.ID) {
			models = append(models, m.ID)
		}
	}

	return models, nil
}

// isChatModel checks if a model ID is a chat-capable model.
func isChatModel(id string) bool {
	// Include GPT models
	if strings.HasPrefix(id, "gpt-") {
		return true
	}
	// Include O1/O3 reasoning models
	if strings.HasPrefix(id, "o1") || strings.HasPrefix(id, "o3") {
		return true
	}
	// Include Codex models (GPT-5.1 series)
	if strings.Contains(id, "codex") {
		return true
	}
	// Include GPT-5 series
	if strings.HasPrefix(id, "gpt-5") {
		return true
	}
	return false
}
