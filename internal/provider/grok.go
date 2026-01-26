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

// GrokProvider implements the Provider interface for xAI's Grok API.
// Grok provides powerful reasoning models via an OpenAI-compatible API.
type GrokProvider struct {
	BaseURL string
	apiKey  string
}

// DefaultGrokURL is the standard xAI API endpoint.
const DefaultGrokURL = "https://api.x.ai"

// NewGrokProvider creates a new Grok provider with the default URL.
func NewGrokProvider(apiKey string) *GrokProvider {
	return &GrokProvider{
		BaseURL: DefaultGrokURL,
		apiKey:  apiKey,
	}
}

// NewGrokProviderWithURL creates a new Grok provider with a custom URL.
// Useful for proxy configurations or self-hosted deployments.
func NewGrokProviderWithURL(baseURL, apiKey string) *GrokProvider {
	if baseURL == "" {
		baseURL = DefaultGrokURL
	}
	baseURL = strings.TrimSuffix(baseURL, "/")
	return &GrokProvider{
		BaseURL: baseURL,
		apiKey:  apiKey,
	}
}

// Name returns the provider name.
func (p *GrokProvider) Name() string {
	return "grok"
}

// GetHeaders returns the HTTP headers for Grok API requests.
// Grok uses Bearer token authentication like OpenAI.
func (p *GrokProvider) GetHeaders(apiKey string) http.Header {
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
// Grok uses the standard /v1/chat/completions endpoint.
func (p *GrokProvider) GetEndpointURL() string {
	return p.BaseURL + "/v1/chat/completions"
}

// TransformModelID transforms a model ID for Grok.
// Maps Claude model names to appropriate Grok equivalents.
func (p *GrokProvider) TransformModelID(modelID string) string {
	// Strip any provider prefix
	modelID = strings.TrimPrefix(modelID, "x-ai/")
	modelID = strings.TrimPrefix(modelID, "grok/")

	// If already a Grok model, return as-is
	modelLower := strings.ToLower(modelID)
	if strings.HasPrefix(modelLower, "grok-") {
		return modelID
	}

	// Map Claude tier names to Grok models
	switch {
	case strings.Contains(modelLower, "opus"):
		return "grok-3-beta" // Highest capability
	case strings.Contains(modelLower, "sonnet"):
		return "grok-3-beta" // Balanced performance
	case strings.Contains(modelLower, "haiku"):
		return "grok-3-mini-beta" // Faster, lighter
	default:
		// Default to grok-3-beta for general use
		return "grok-3-beta"
	}
}

// SupportsStreaming indicates that Grok supports SSE streaming.
func (p *GrokProvider) SupportsStreaming() bool {
	return true
}

// RequiresTransformation indicates that Grok needs Anthropic->OpenAI translation.
func (p *GrokProvider) RequiresTransformation() bool {
	return true
}

// GetAPIKey returns the configured API key.
func (p *GrokProvider) GetAPIKey() string {
	return p.apiKey
}

// IsAvailable checks if the Grok API is reachable.
func (p *GrokProvider) IsAvailable() bool {
	if p.apiKey == "" {
		return false
	}
	return IsGrokAvailable(p.apiKey)
}

// ListModels returns available Grok models.
func (p *GrokProvider) ListModels() ([]string, error) {
	return ListGrokModels(p.apiKey)
}

// IsGrokAvailable checks if Grok API is accessible with the given key.
func IsGrokAvailable(apiKey string) bool {
	if apiKey == "" {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, DefaultGrokURL+"/v1/models", http.NoBody)
	if err != nil {
		return false
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// GrokModel represents a model from the Grok API.
type GrokModel struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// GrokModelsResponse is the response from the /v1/models endpoint.
type GrokModelsResponse struct {
	Object string      `json:"object"`
	Data   []GrokModel `json:"data"`
}

// ListGrokModels fetches available models from the Grok API.
func ListGrokModels(apiKey string) ([]string, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API key required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, DefaultGrokURL+"/v1/models", http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Grok API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Grok API returned status %d", resp.StatusCode)
	}

	var modelsResp GrokModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, fmt.Errorf("failed to decode models response: %w", err)
	}

	models := make([]string, 0, len(modelsResp.Data))
	for _, m := range modelsResp.Data {
		models = append(models, m.ID)
	}

	return models, nil
}

// WaitForGrok waits for the Grok API to become available.
func WaitForGrok(ctx context.Context, apiKey string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if IsGrokAvailable(apiKey) {
				return nil
			}
			time.Sleep(500 * time.Millisecond)
		}
	}
	return fmt.Errorf("timeout waiting for Grok API")
}

// RecommendedGrokModels returns recommended models for different use cases.
func RecommendedGrokModels() map[string]string {
	return map[string]string{
		"grok-3-beta":      "Most capable Grok model - excellent reasoning and coding",
		"grok-3-mini-beta": "Faster model for simpler tasks - good balance of speed and capability",
		"grok-2":           "Previous generation model - stable and reliable",
	}
}

// GrokModelTiers maps Claude tiers to Grok models.
func GrokModelTiers() map[string]string {
	return map[string]string{
		"opus":   "grok-3-beta",      // Highest capability
		"sonnet": "grok-3-beta",      // Balanced
		"haiku":  "grok-3-mini-beta", // Faster
	}
}
