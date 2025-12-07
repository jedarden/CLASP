// Package provider implements LLM provider backends.
package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// DeepSeekProvider implements the Provider interface for DeepSeek API.
// DeepSeek provides powerful coding and reasoning models via an OpenAI-compatible API.
type DeepSeekProvider struct {
	BaseURL string
	apiKey  string
}

// DefaultDeepSeekURL is the standard DeepSeek API endpoint.
const DefaultDeepSeekURL = "https://api.deepseek.com"

// NewDeepSeekProvider creates a new DeepSeek provider with the default URL.
func NewDeepSeekProvider(apiKey string) *DeepSeekProvider {
	return &DeepSeekProvider{
		BaseURL: DefaultDeepSeekURL,
		apiKey:  apiKey,
	}
}

// NewDeepSeekProviderWithURL creates a new DeepSeek provider with a custom URL.
// Useful for proxy configurations or self-hosted deployments.
func NewDeepSeekProviderWithURL(baseURL, apiKey string) *DeepSeekProvider {
	if baseURL == "" {
		baseURL = DefaultDeepSeekURL
	}
	baseURL = strings.TrimSuffix(baseURL, "/")
	return &DeepSeekProvider{
		BaseURL: baseURL,
		apiKey:  apiKey,
	}
}

// Name returns the provider name.
func (p *DeepSeekProvider) Name() string {
	return "deepseek"
}

// GetHeaders returns the HTTP headers for DeepSeek API requests.
// DeepSeek uses Bearer token authentication like OpenAI.
func (p *DeepSeekProvider) GetHeaders(apiKey string) http.Header {
	headers := http.Header{}
	// Use embedded API key if set, otherwise use provided key
	key := apiKey
	if p.apiKey != "" {
		key = p.apiKey
	}
	if key != "" {
		headers.Set("Authorization", "Bearer "+key)
	}
	headers.Set("Content-Type", "application/json")
	return headers
}

// GetEndpointURL returns the OpenAI-compatible chat completions endpoint URL.
// DeepSeek uses the standard /v1/chat/completions endpoint.
func (p *DeepSeekProvider) GetEndpointURL() string {
	return p.BaseURL + "/v1/chat/completions"
}

// TransformModelID transforms a model ID for DeepSeek.
// Maps Claude model names to appropriate DeepSeek equivalents.
func (p *DeepSeekProvider) TransformModelID(modelID string) string {
	// Strip any provider prefix
	if strings.HasPrefix(modelID, "deepseek/") {
		modelID = strings.TrimPrefix(modelID, "deepseek/")
	}

	// If already a DeepSeek model, return as-is
	modelLower := strings.ToLower(modelID)
	if strings.HasPrefix(modelLower, "deepseek-") {
		return modelID
	}

	// Map Claude tier names to DeepSeek models
	switch {
	case strings.Contains(modelLower, "opus"):
		return "deepseek-reasoner" // Highest capability with reasoning
	case strings.Contains(modelLower, "sonnet"):
		return "deepseek-chat" // Balanced performance
	case strings.Contains(modelLower, "haiku"):
		return "deepseek-chat" // DeepSeek doesn't have a "mini" model currently
	default:
		// Default to deepseek-chat for general use
		return "deepseek-chat"
	}
}

// SupportsStreaming indicates that DeepSeek supports SSE streaming.
func (p *DeepSeekProvider) SupportsStreaming() bool {
	return true
}

// RequiresTransformation indicates that DeepSeek needs Anthropic->OpenAI translation.
func (p *DeepSeekProvider) RequiresTransformation() bool {
	return true
}

// GetAPIKey returns the configured API key.
func (p *DeepSeekProvider) GetAPIKey() string {
	return p.apiKey
}

// IsAvailable checks if the DeepSeek API is reachable.
func (p *DeepSeekProvider) IsAvailable() bool {
	if p.apiKey == "" {
		return false
	}
	return IsDeepSeekAvailable(p.apiKey)
}

// ListModels returns available DeepSeek models.
func (p *DeepSeekProvider) ListModels() ([]string, error) {
	return ListDeepSeekModels(p.apiKey)
}

// IsDeepSeekAvailable checks if DeepSeek API is accessible with the given key.
func IsDeepSeekAvailable(apiKey string) bool {
	if apiKey == "" {
		return false
	}

	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest("GET", DefaultDeepSeekURL+"/v1/models", nil)
	if err != nil {
		return false
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// DeepSeekModel represents a model from the DeepSeek API.
type DeepSeekModel struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// DeepSeekModelsResponse is the response from the /v1/models endpoint.
type DeepSeekModelsResponse struct {
	Object string          `json:"object"`
	Data   []DeepSeekModel `json:"data"`
}

// ListDeepSeekModels fetches available models from the DeepSeek API.
func ListDeepSeekModels(apiKey string) ([]string, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API key required")
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", DefaultDeepSeekURL+"/v1/models", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to DeepSeek API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("DeepSeek API returned status %d", resp.StatusCode)
	}

	var modelsResp DeepSeekModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, fmt.Errorf("failed to decode models response: %w", err)
	}

	models := make([]string, 0, len(modelsResp.Data))
	for _, m := range modelsResp.Data {
		models = append(models, m.ID)
	}

	return models, nil
}

// WaitForDeepSeek waits for the DeepSeek API to become available.
func WaitForDeepSeek(ctx context.Context, apiKey string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if IsDeepSeekAvailable(apiKey) {
				return nil
			}
			time.Sleep(500 * time.Millisecond)
		}
	}
	return fmt.Errorf("timeout waiting for DeepSeek API")
}

// RecommendedDeepSeekModels returns recommended models for different use cases.
func RecommendedDeepSeekModels() map[string]string {
	return map[string]string{
		"deepseek-chat":     "General purpose chat model - fast and capable (recommended)",
		"deepseek-coder":    "Optimized for code generation and understanding",
		"deepseek-reasoner": "Enhanced reasoning capabilities for complex tasks",
	}
}

// DeepSeekModelTiers maps Claude tiers to DeepSeek models.
func DeepSeekModelTiers() map[string]string {
	return map[string]string{
		"opus":   "deepseek-reasoner", // Highest capability
		"sonnet": "deepseek-chat",     // Balanced
		"haiku":  "deepseek-chat",     // DeepSeek doesn't have mini model
	}
}
