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

// GeminiProvider implements the Provider interface for Google Gemini API.
// Google's Gemini models are accessed via the generativelanguage.googleapis.com API.
type GeminiProvider struct {
	BaseURL string
	apiKey  string
}

// DefaultGeminiURL is the standard Google AI Studio API endpoint.
const DefaultGeminiURL = "https://generativelanguage.googleapis.com/v1beta"

// NewGeminiProvider creates a new Gemini provider with the default URL.
func NewGeminiProvider(apiKey string) *GeminiProvider {
	return &GeminiProvider{
		BaseURL: DefaultGeminiURL,
		apiKey:  apiKey,
	}
}

// NewGeminiProviderWithURL creates a new Gemini provider with a custom URL.
// Useful for Vertex AI or proxy configurations.
func NewGeminiProviderWithURL(baseURL, apiKey string) *GeminiProvider {
	if baseURL == "" {
		baseURL = DefaultGeminiURL
	}
	baseURL = strings.TrimSuffix(baseURL, "/")
	return &GeminiProvider{
		BaseURL: baseURL,
		apiKey:  apiKey,
	}
}

// Name returns the provider name.
func (p *GeminiProvider) Name() string {
	return "gemini"
}

// GetHeaders returns the HTTP headers for Gemini API requests.
// Gemini uses an API key in the URL query parameter, not headers.
func (p *GeminiProvider) GetHeaders(apiKey string) http.Header {
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	return headers
}

// GetEndpointURL returns the OpenAI-compatible chat completions endpoint URL.
// Google provides an OpenAI-compatible endpoint at /v1beta/openai/chat/completions
func (p *GeminiProvider) GetEndpointURL() string {
	// Use the OpenAI-compatible endpoint
	return p.BaseURL + "/openai/chat/completions"
}

// GetEndpointURLWithKey returns the endpoint URL with the API key as query param.
// This is used for the native Gemini endpoint.
func (p *GeminiProvider) GetEndpointURLWithKey(model string) string {
	// Native endpoint format: /models/{model}:generateContent?key={apiKey}
	return fmt.Sprintf("%s/models/%s:generateContent?key=%s", p.BaseURL, model, p.apiKey)
}

// GetStreamEndpointURL returns the streaming endpoint URL.
func (p *GeminiProvider) GetStreamEndpointURL(model string) string {
	// Streaming format: /models/{model}:streamGenerateContent?key={apiKey}
	return fmt.Sprintf("%s/models/%s:streamGenerateContent?key=%s", p.BaseURL, model, p.apiKey)
}

// TransformModelID transforms a model ID for Gemini.
// Maps Claude model names to appropriate Gemini equivalents.
func (p *GeminiProvider) TransformModelID(modelID string) string {
	// Strip any provider prefix
	if strings.HasPrefix(modelID, "gemini/") {
		modelID = strings.TrimPrefix(modelID, "gemini/")
	}
	if strings.HasPrefix(modelID, "google/") {
		modelID = strings.TrimPrefix(modelID, "google/")
	}

	// Map Claude model names to Gemini equivalents
	modelLower := strings.ToLower(modelID)

	// If already a Gemini model, return as-is
	if strings.HasPrefix(modelLower, "gemini-") || strings.HasPrefix(modelLower, "models/gemini-") {
		return modelID
	}

	// Map Claude tier names to Gemini models
	switch {
	case strings.Contains(modelLower, "opus"):
		return "gemini-2.0-flash-thinking-exp" // Highest capability
	case strings.Contains(modelLower, "sonnet"):
		return "gemini-2.0-flash-exp" // Balanced
	case strings.Contains(modelLower, "haiku"):
		return "gemini-1.5-flash" // Fast/cheap
	default:
		// Default to gemini-2.0-flash-exp for general use
		return "gemini-2.0-flash-exp"
	}
}

// SupportsStreaming indicates that Gemini supports SSE streaming.
func (p *GeminiProvider) SupportsStreaming() bool {
	return true
}

// RequiresTransformation indicates that Gemini needs Anthropic->OpenAI translation.
// Google's OpenAI-compatible endpoint accepts OpenAI format directly.
func (p *GeminiProvider) RequiresTransformation() bool {
	return true
}

// GetAPIKey returns the configured API key.
func (p *GeminiProvider) GetAPIKey() string {
	return p.apiKey
}

// IsAvailable checks if the Gemini API is reachable.
func (p *GeminiProvider) IsAvailable() bool {
	if p.apiKey == "" {
		return false
	}
	return IsGeminiAvailable(p.apiKey)
}

// ListModels returns available Gemini models.
func (p *GeminiProvider) ListModels() ([]string, error) {
	return ListGeminiModels(p.apiKey)
}

// IsGeminiAvailable checks if Gemini API is accessible with the given key.
func IsGeminiAvailable(apiKey string) bool {
	if apiKey == "" {
		return false
	}

	client := &http.Client{Timeout: 5 * time.Second}
	url := fmt.Sprintf("%s/models?key=%s", DefaultGeminiURL, apiKey)

	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// GeminiModel represents a model from the Gemini API.
type GeminiModel struct {
	Name                       string   `json:"name"`
	BaseModelID                string   `json:"baseModelId,omitempty"`
	Version                    string   `json:"version"`
	DisplayName                string   `json:"displayName"`
	Description                string   `json:"description"`
	InputTokenLimit            int      `json:"inputTokenLimit"`
	OutputTokenLimit           int      `json:"outputTokenLimit"`
	SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
}

// GeminiModelsResponse is the response from the /models endpoint.
type GeminiModelsResponse struct {
	Models []GeminiModel `json:"models"`
}

// ListGeminiModels fetches available models from the Gemini API.
func ListGeminiModels(apiKey string) ([]string, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API key required")
	}

	client := &http.Client{Timeout: 10 * time.Second}
	url := fmt.Sprintf("%s/models?key=%s", DefaultGeminiURL, apiKey)

	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Gemini API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Gemini API returned status %d", resp.StatusCode)
	}

	var modelsResp GeminiModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, fmt.Errorf("failed to decode models response: %w", err)
	}

	// Filter to only models that support generateContent
	models := make([]string, 0)
	for _, m := range modelsResp.Models {
		for _, method := range m.SupportedGenerationMethods {
			if method == "generateContent" {
				// Extract just the model name from "models/gemini-xxx"
				name := m.Name
				if strings.HasPrefix(name, "models/") {
					name = strings.TrimPrefix(name, "models/")
				}
				models = append(models, name)
				break
			}
		}
	}

	return models, nil
}

// WaitForGemini waits for the Gemini API to become available.
func WaitForGemini(ctx context.Context, apiKey string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if IsGeminiAvailable(apiKey) {
				return nil
			}
			time.Sleep(500 * time.Millisecond)
		}
	}
	return fmt.Errorf("timeout waiting for Gemini API")
}

// RecommendedGeminiModels returns recommended models for different use cases.
func RecommendedGeminiModels() map[string]string {
	return map[string]string{
		"gemini-2.0-flash-exp":           "Latest flash model - fast and capable (recommended)",
		"gemini-2.0-flash-thinking-exp":  "Enhanced reasoning - best for complex tasks",
		"gemini-1.5-pro":                 "Production-ready pro model",
		"gemini-1.5-flash":               "Fast and efficient for high-volume tasks",
		"gemini-1.5-flash-8b":            "Lightweight 8B model for simple tasks",
		"gemini-exp-1206":                "Experimental model with latest features",
	}
}

// GeminiModelTiers maps Claude tiers to Gemini models.
func GeminiModelTiers() map[string]string {
	return map[string]string{
		"opus":   "gemini-2.0-flash-thinking-exp", // Highest capability
		"sonnet": "gemini-2.0-flash-exp",          // Balanced
		"haiku":  "gemini-1.5-flash",              // Fast/cheap
	}
}
