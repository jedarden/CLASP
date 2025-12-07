// Package provider implements LLM provider backends.
package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

// OllamaProvider implements the Provider interface for Ollama local models.
// Ollama is an OpenAI-compatible local model server that requires no API key.
type OllamaProvider struct {
	BaseURL string
	apiKey  string // Usually empty for local Ollama
}

// DefaultOllamaURL is the standard Ollama server address.
const DefaultOllamaURL = "http://localhost:11434"

// NewOllamaProvider creates a new Ollama provider with the default URL.
func NewOllamaProvider(baseURL string) *OllamaProvider {
	if baseURL == "" {
		baseURL = DefaultOllamaURL
	}
	// Ensure we strip any /v1 suffix since Ollama uses /api
	baseURL = strings.TrimSuffix(baseURL, "/v1")
	baseURL = strings.TrimSuffix(baseURL, "/")
	return &OllamaProvider{BaseURL: baseURL}
}

// NewOllamaProviderWithKey creates a new Ollama provider with an optional API key.
// Used for Ollama instances that require authentication.
func NewOllamaProviderWithKey(baseURL, apiKey string) *OllamaProvider {
	if baseURL == "" {
		baseURL = DefaultOllamaURL
	}
	baseURL = strings.TrimSuffix(baseURL, "/v1")
	baseURL = strings.TrimSuffix(baseURL, "/")
	return &OllamaProvider{BaseURL: baseURL, apiKey: apiKey}
}

// Name returns the provider name.
func (p *OllamaProvider) Name() string {
	return "ollama"
}

// GetHeaders returns the HTTP headers for Ollama API requests.
// Ollama typically doesn't require authentication for local use.
func (p *OllamaProvider) GetHeaders(apiKey string) http.Header {
	headers := http.Header{}
	// Use embedded API key if set, otherwise use provided key
	key := apiKey
	if p.apiKey != "" {
		key = p.apiKey
	}
	// Only set Authorization if key is provided and non-empty
	if key != "" && key != "not-required" && key != "ollama" {
		headers.Set("Authorization", "Bearer "+key)
	}
	headers.Set("Content-Type", "application/json")
	return headers
}

// GetEndpointURL returns the OpenAI-compatible chat completions endpoint URL.
// Ollama exposes an OpenAI-compatible API at /v1/chat/completions
func (p *OllamaProvider) GetEndpointURL() string {
	return p.BaseURL + "/v1/chat/completions"
}

// TransformModelID returns the model ID as-is for Ollama.
// Ollama model names are used directly (e.g., "llama3.2", "codellama", "mistral").
func (p *OllamaProvider) TransformModelID(modelID string) string {
	// Strip any provider prefix if present
	if strings.HasPrefix(modelID, "ollama/") {
		return strings.TrimPrefix(modelID, "ollama/")
	}
	return modelID
}

// SupportsStreaming indicates that Ollama supports SSE streaming.
func (p *OllamaProvider) SupportsStreaming() bool {
	return true
}

// RequiresTransformation indicates that Ollama needs Anthropic->OpenAI translation.
func (p *OllamaProvider) RequiresTransformation() bool {
	return true
}

// IsRunning checks if Ollama is running and accessible.
func (p *OllamaProvider) IsRunning() bool {
	return IsOllamaRunning(p.BaseURL)
}

// ListModels returns a list of available models from the Ollama server.
func (p *OllamaProvider) ListModels() ([]string, error) {
	return ListOllamaModels(p.BaseURL)
}

// IsOllamaRunning checks if Ollama is running at the given URL.
func IsOllamaRunning(baseURL string) bool {
	if baseURL == "" {
		baseURL = DefaultOllamaURL
	}
	baseURL = strings.TrimSuffix(baseURL, "/v1")
	baseURL = strings.TrimSuffix(baseURL, "/")

	// Quick connection check with timeout
	client := &http.Client{
		Timeout: 2 * time.Second,
	}

	// Try the Ollama-specific endpoint first
	resp, err := client.Get(baseURL + "/api/tags")
	if err != nil {
		// Try OpenAI-compatible endpoint as fallback
		resp, err = client.Get(baseURL + "/v1/models")
		if err != nil {
			return false
		}
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// DetectOllama attempts to detect if Ollama is running on common ports.
// Returns the URL if found, empty string otherwise.
func DetectOllama() string {
	// Common Ollama ports
	ports := []string{"11434", "11435", "8080"}

	for _, port := range ports {
		url := fmt.Sprintf("http://localhost:%s", port)
		if IsOllamaRunning(url) {
			return url
		}
	}

	return ""
}

// OllamaModel represents a model available in Ollama.
type OllamaModel struct {
	Name       string    `json:"name"`
	Model      string    `json:"model"`
	ModifiedAt time.Time `json:"modified_at"`
	Size       int64     `json:"size"`
	Digest     string    `json:"digest"`
	Details    struct {
		Format            string   `json:"format"`
		Family            string   `json:"family"`
		Families          []string `json:"families"`
		ParameterSize     string   `json:"parameter_size"`
		QuantizationLevel string   `json:"quantization_level"`
	} `json:"details"`
}

// OllamaTagsResponse is the response from /api/tags endpoint.
type OllamaTagsResponse struct {
	Models []OllamaModel `json:"models"`
}

// ListOllamaModels returns a list of available models from Ollama.
func ListOllamaModels(baseURL string) ([]string, error) {
	if baseURL == "" {
		baseURL = DefaultOllamaURL
	}
	baseURL = strings.TrimSuffix(baseURL, "/v1")
	baseURL = strings.TrimSuffix(baseURL, "/")

	client := &http.Client{Timeout: 10 * time.Second}

	// Try Ollama native endpoint first
	resp, err := client.Get(baseURL + "/api/tags")
	if err != nil {
		// Fall back to OpenAI-compatible endpoint
		return listModelsOpenAICompat(client, baseURL)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return listModelsOpenAICompat(client, baseURL)
	}

	var tagsResp OllamaTagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tagsResp); err != nil {
		return nil, fmt.Errorf("failed to decode Ollama response: %w", err)
	}

	models := make([]string, 0, len(tagsResp.Models))
	for _, m := range tagsResp.Models {
		// Use the short name (without tag) for cleaner display
		name := m.Name
		if idx := strings.Index(name, ":"); idx > 0 {
			// Keep full name including tag for precision
			models = append(models, name)
		} else {
			models = append(models, name)
		}
	}

	return models, nil
}

// listModelsOpenAICompat tries to list models using the OpenAI-compatible endpoint.
func listModelsOpenAICompat(client *http.Client, baseURL string) ([]string, error) {
	resp, err := client.Get(baseURL + "/v1/models")
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Ollama returned status %d", resp.StatusCode)
	}

	var modelsResp struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, fmt.Errorf("failed to decode models response: %w", err)
	}

	models := make([]string, 0, len(modelsResp.Data))
	for _, m := range modelsResp.Data {
		models = append(models, m.ID)
	}

	return models, nil
}

// PullOllamaModel pulls (downloads) a model to Ollama.
// This is useful for first-time setup.
func PullOllamaModel(baseURL, modelName string) error {
	if baseURL == "" {
		baseURL = DefaultOllamaURL
	}
	baseURL = strings.TrimSuffix(baseURL, "/v1")
	baseURL = strings.TrimSuffix(baseURL, "/")

	payload := fmt.Sprintf(`{"name":"%s"}`, modelName)

	client := &http.Client{Timeout: 30 * time.Minute} // Pulling can take a long time
	resp, err := client.Post(baseURL+"/api/pull", "application/json", strings.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to start model pull: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("pull failed with status %d", resp.StatusCode)
	}

	return nil
}

// WaitForOllama waits for Ollama to become available with a timeout.
func WaitForOllama(ctx context.Context, baseURL string, timeout time.Duration) error {
	if baseURL == "" {
		baseURL = DefaultOllamaURL
	}

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if IsOllamaRunning(baseURL) {
				return nil
			}
			time.Sleep(500 * time.Millisecond)
		}
	}

	return fmt.Errorf("timeout waiting for Ollama at %s", baseURL)
}

// IsPortAvailable checks if a port is available for binding.
func IsPortAvailable(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	ln.Close()
	return true
}

// GetOllamaModelInfo returns detailed info about a specific model.
func GetOllamaModelInfo(baseURL, modelName string) (*OllamaModel, error) {
	models, err := ListOllamaModels(baseURL)
	if err != nil {
		return nil, err
	}

	for _, m := range models {
		if m == modelName || strings.HasPrefix(m, modelName+":") {
			// For detailed info, we'd need to parse the full response
			// For now, return a basic model struct
			return &OllamaModel{Name: m}, nil
		}
	}

	return nil, fmt.Errorf("model %s not found", modelName)
}

// RecommendedOllamaModels returns a list of recommended models for coding tasks.
func RecommendedOllamaModels() []string {
	return []string{
		"llama3.2",       // Latest Llama, good general purpose
		"codellama",      // Optimized for code
		"deepseek-coder", // Strong coding model
		"mistral",        // Fast and capable
		"qwen2.5-coder",  // Alibaba's code model
		"phi3",           // Microsoft's efficient model
		"gemma2",         // Google's open model
	}
}
