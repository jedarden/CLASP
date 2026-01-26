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

// MiniMaxProvider implements the Provider interface for MiniMax API.
// MiniMax provides powerful Chinese language models via an OpenAI-compatible API.
type MiniMaxProvider struct {
	BaseURL string
	apiKey  string
	groupID string // MiniMax requires a group ID for some endpoints
}

// DefaultMiniMaxURL is the standard MiniMax API endpoint.
const DefaultMiniMaxURL = "https://api.minimax.chat"

// NewMiniMaxProvider creates a new MiniMax provider with the default URL.
func NewMiniMaxProvider(apiKey string) *MiniMaxProvider {
	return &MiniMaxProvider{
		BaseURL: DefaultMiniMaxURL,
		apiKey:  apiKey,
	}
}

// NewMiniMaxProviderWithGroup creates a new MiniMax provider with a group ID.
// Some MiniMax endpoints require a group ID for authentication.
func NewMiniMaxProviderWithGroup(apiKey, groupID string) *MiniMaxProvider {
	return &MiniMaxProvider{
		BaseURL: DefaultMiniMaxURL,
		apiKey:  apiKey,
		groupID: groupID,
	}
}

// NewMiniMaxProviderWithURL creates a new MiniMax provider with a custom URL.
// Useful for proxy configurations or regional deployments.
func NewMiniMaxProviderWithURL(baseURL, apiKey, groupID string) *MiniMaxProvider {
	if baseURL == "" {
		baseURL = DefaultMiniMaxURL
	}
	baseURL = strings.TrimSuffix(baseURL, "/")
	return &MiniMaxProvider{
		BaseURL: baseURL,
		apiKey:  apiKey,
		groupID: groupID,
	}
}

// Name returns the provider name.
func (p *MiniMaxProvider) Name() string {
	return "minimax"
}

// GetHeaders returns the HTTP headers for MiniMax API requests.
// MiniMax uses Bearer token authentication like OpenAI.
func (p *MiniMaxProvider) GetHeaders(apiKey string) http.Header {
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
// MiniMax uses the standard /v1/chat/completions endpoint.
func (p *MiniMaxProvider) GetEndpointURL() string {
	return p.BaseURL + "/v1/chat/completions"
}

// TransformModelID transforms a model ID for MiniMax.
// Maps Claude model names to appropriate MiniMax equivalents.
func (p *MiniMaxProvider) TransformModelID(modelID string) string {
	// Strip any provider prefix
	modelID = strings.TrimPrefix(modelID, "minimax/")

	// If already a MiniMax model, return as-is
	modelLower := strings.ToLower(modelID)
	if strings.HasPrefix(modelLower, "minimax") || strings.HasPrefix(modelLower, "abab") {
		return modelID
	}

	// Map Claude tier names to MiniMax models
	switch {
	case strings.Contains(modelLower, "opus"):
		return "abab6.5s-chat" // Highest capability
	case strings.Contains(modelLower, "sonnet"):
		return "abab6.5s-chat" // Balanced performance
	case strings.Contains(modelLower, "haiku"):
		return "abab5.5s-chat" // Faster, lighter
	default:
		// Default to abab6.5s-chat for general use
		return "abab6.5s-chat"
	}
}

// SupportsStreaming indicates that MiniMax supports SSE streaming.
func (p *MiniMaxProvider) SupportsStreaming() bool {
	return true
}

// RequiresTransformation indicates that MiniMax needs Anthropic->OpenAI translation.
func (p *MiniMaxProvider) RequiresTransformation() bool {
	return true
}

// GetAPIKey returns the configured API key.
func (p *MiniMaxProvider) GetAPIKey() string {
	return p.apiKey
}

// GetGroupID returns the configured group ID.
func (p *MiniMaxProvider) GetGroupID() string {
	return p.groupID
}

// IsAvailable checks if the MiniMax API is reachable.
func (p *MiniMaxProvider) IsAvailable() bool {
	if p.apiKey == "" {
		return false
	}
	return IsMiniMaxAvailable(p.apiKey)
}

// ListModels returns available MiniMax models.
func (p *MiniMaxProvider) ListModels() ([]string, error) {
	return ListMiniMaxModels(p.apiKey)
}

// IsMiniMaxAvailable checks if MiniMax API is accessible with the given key.
func IsMiniMaxAvailable(apiKey string) bool {
	if apiKey == "" {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, DefaultMiniMaxURL+"/v1/models", http.NoBody)
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

// MiniMaxModel represents a model from the MiniMax API.
type MiniMaxModel struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// MiniMaxModelsResponse is the response from the /v1/models endpoint.
type MiniMaxModelsResponse struct {
	Object string         `json:"object"`
	Data   []MiniMaxModel `json:"data"`
}

// ListMiniMaxModels fetches available models from the MiniMax API.
func ListMiniMaxModels(apiKey string) ([]string, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API key required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, DefaultMiniMaxURL+"/v1/models", http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MiniMax API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MiniMax API returned status %d", resp.StatusCode)
	}

	var modelsResp MiniMaxModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, fmt.Errorf("failed to decode models response: %w", err)
	}

	models := make([]string, 0, len(modelsResp.Data))
	for _, m := range modelsResp.Data {
		models = append(models, m.ID)
	}

	return models, nil
}

// WaitForMiniMax waits for the MiniMax API to become available.
func WaitForMiniMax(ctx context.Context, apiKey string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if IsMiniMaxAvailable(apiKey) {
				return nil
			}
			time.Sleep(500 * time.Millisecond)
		}
	}
	return fmt.Errorf("timeout waiting for MiniMax API")
}

// RecommendedMiniMaxModels returns recommended models for different use cases.
func RecommendedMiniMaxModels() map[string]string {
	return map[string]string{
		"abab6.5s-chat": "Most capable MiniMax model - best for complex tasks",
		"abab6.5t-chat": "Turbo variant - faster responses",
		"abab5.5s-chat": "Previous generation - stable and reliable",
	}
}

// MiniMaxModelTiers maps Claude tiers to MiniMax models.
func MiniMaxModelTiers() map[string]string {
	return map[string]string{
		"opus":   "abab6.5s-chat", // Highest capability
		"sonnet": "abab6.5s-chat", // Balanced
		"haiku":  "abab5.5s-chat", // Faster
	}
}
