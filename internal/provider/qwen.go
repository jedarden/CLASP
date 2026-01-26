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

// QwenProvider implements the Provider interface for Alibaba's Qwen API.
// Qwen provides powerful multilingual models via an OpenAI-compatible API (DashScope).
type QwenProvider struct {
	BaseURL string
	apiKey  string
}

// DefaultQwenURL is the standard DashScope OpenAI-compatible API endpoint.
const DefaultQwenURL = "https://dashscope.aliyuncs.com/compatible-mode"

// NewQwenProvider creates a new Qwen provider with the default URL.
func NewQwenProvider(apiKey string) *QwenProvider {
	return &QwenProvider{
		BaseURL: DefaultQwenURL,
		apiKey:  apiKey,
	}
}

// NewQwenProviderWithURL creates a new Qwen provider with a custom URL.
// Useful for proxy configurations or regional deployments.
func NewQwenProviderWithURL(baseURL, apiKey string) *QwenProvider {
	if baseURL == "" {
		baseURL = DefaultQwenURL
	}
	baseURL = strings.TrimSuffix(baseURL, "/")
	return &QwenProvider{
		BaseURL: baseURL,
		apiKey:  apiKey,
	}
}

// Name returns the provider name.
func (p *QwenProvider) Name() string {
	return "qwen"
}

// GetHeaders returns the HTTP headers for Qwen API requests.
// Qwen uses Bearer token authentication like OpenAI.
func (p *QwenProvider) GetHeaders(apiKey string) http.Header {
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
// Qwen uses the standard /v1/chat/completions endpoint via DashScope.
func (p *QwenProvider) GetEndpointURL() string {
	return p.BaseURL + "/v1/chat/completions"
}

// TransformModelID transforms a model ID for Qwen.
// Maps Claude model names to appropriate Qwen equivalents.
func (p *QwenProvider) TransformModelID(modelID string) string {
	// Strip any provider prefix
	modelID = strings.TrimPrefix(modelID, "qwen/")
	modelID = strings.TrimPrefix(modelID, "alibaba/")

	// If already a Qwen model, return as-is
	modelLower := strings.ToLower(modelID)
	if strings.HasPrefix(modelLower, "qwen") {
		return modelID
	}

	// Map Claude tier names to Qwen models
	switch {
	case strings.Contains(modelLower, "opus"):
		return "qwen-max" // Highest capability
	case strings.Contains(modelLower, "sonnet"):
		return "qwen-plus" // Balanced performance
	case strings.Contains(modelLower, "haiku"):
		return "qwen-turbo" // Faster, lighter
	default:
		// Default to qwen-plus for general use
		return "qwen-plus"
	}
}

// SupportsStreaming indicates that Qwen supports SSE streaming.
func (p *QwenProvider) SupportsStreaming() bool {
	return true
}

// RequiresTransformation indicates that Qwen needs Anthropic->OpenAI translation.
func (p *QwenProvider) RequiresTransformation() bool {
	return true
}

// GetAPIKey returns the configured API key.
func (p *QwenProvider) GetAPIKey() string {
	return p.apiKey
}

// IsAvailable checks if the Qwen API is reachable.
func (p *QwenProvider) IsAvailable() bool {
	if p.apiKey == "" {
		return false
	}
	return IsQwenAvailable(p.apiKey)
}

// ListModels returns available Qwen models.
func (p *QwenProvider) ListModels() ([]string, error) {
	return ListQwenModels(p.apiKey)
}

// IsQwenAvailable checks if Qwen API is accessible with the given key.
func IsQwenAvailable(apiKey string) bool {
	if apiKey == "" {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, DefaultQwenURL+"/v1/models", http.NoBody)
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

// QwenModel represents a model from the Qwen API.
type QwenModel struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// QwenModelsResponse is the response from the /v1/models endpoint.
type QwenModelsResponse struct {
	Object string      `json:"object"`
	Data   []QwenModel `json:"data"`
}

// ListQwenModels fetches available models from the Qwen API.
func ListQwenModels(apiKey string) ([]string, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API key required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, DefaultQwenURL+"/v1/models", http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Qwen API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Qwen API returned status %d", resp.StatusCode)
	}

	var modelsResp QwenModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, fmt.Errorf("failed to decode models response: %w", err)
	}

	models := make([]string, 0, len(modelsResp.Data))
	for _, m := range modelsResp.Data {
		models = append(models, m.ID)
	}

	return models, nil
}

// WaitForQwen waits for the Qwen API to become available.
func WaitForQwen(ctx context.Context, apiKey string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if IsQwenAvailable(apiKey) {
				return nil
			}
			time.Sleep(500 * time.Millisecond)
		}
	}
	return fmt.Errorf("timeout waiting for Qwen API")
}

// RecommendedQwenModels returns recommended models for different use cases.
func RecommendedQwenModels() map[string]string {
	return map[string]string{
		"qwen-max":        "Most capable Qwen model - best for complex reasoning tasks",
		"qwen-plus":       "Balanced model - good performance and speed",
		"qwen-turbo":      "Fastest model - optimized for low latency",
		"qwen-coder-plus": "Optimized for code generation and understanding",
	}
}

// QwenModelTiers maps Claude tiers to Qwen models.
func QwenModelTiers() map[string]string {
	return map[string]string{
		"opus":   "qwen-max",   // Highest capability
		"sonnet": "qwen-plus",  // Balanced
		"haiku":  "qwen-turbo", // Faster
	}
}
