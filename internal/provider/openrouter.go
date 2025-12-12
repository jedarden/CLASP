// Package provider implements LLM provider backends.
package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

// OpenRouterProvider implements the Provider interface for OpenRouter.
type OpenRouterProvider struct {
	BaseURL string
	apiKey  string // Optional: used for tier-specific routing
}

// NewOpenRouterProvider creates a new OpenRouter provider.
func NewOpenRouterProvider(baseURL string) *OpenRouterProvider {
	if baseURL == "" {
		baseURL = "https://openrouter.ai/api/v1"
	}
	return &OpenRouterProvider{BaseURL: baseURL}
}

// NewOpenRouterProviderWithKey creates a new OpenRouter provider with an embedded API key.
// Used for multi-provider routing where each tier has its own credentials.
func NewOpenRouterProviderWithKey(baseURL, apiKey string) *OpenRouterProvider {
	if baseURL == "" {
		baseURL = "https://openrouter.ai/api/v1"
	}
	return &OpenRouterProvider{BaseURL: baseURL, apiKey: apiKey}
}

// Name returns the provider name.
func (p *OpenRouterProvider) Name() string {
	return "openrouter"
}

// GetHeaders returns the HTTP headers for OpenRouter API requests.
func (p *OpenRouterProvider) GetHeaders(apiKey string) http.Header {
	headers := http.Header{}
	// Use embedded API key if set (for tier-specific routing), otherwise use provided key
	key := apiKey
	if p.apiKey != "" {
		key = p.apiKey
	}
	headers.Set("Authorization", "Bearer "+key)
	headers.Set("Content-Type", "application/json")
	headers.Set("HTTP-Referer", "https://github.com/jedarden/CLASP")
	headers.Set("X-Title", "CLASP Proxy")
	headers.Set("User-Agent", "CLASP/0.2.5 (+https://github.com/jedarden/CLASP)")
	return headers
}

// GetEndpointURL returns the chat completions endpoint URL.
func (p *OpenRouterProvider) GetEndpointURL() string {
	return p.BaseURL + "/chat/completions"
}

// TransformModelID returns the model ID as-is for OpenRouter.
func (p *OpenRouterProvider) TransformModelID(modelID string) string {
	// OpenRouter uses provider/model format, no transformation needed
	return modelID
}

// SupportsStreaming indicates that OpenRouter supports SSE streaming.
func (p *OpenRouterProvider) SupportsStreaming() bool {
	return true
}

// RequiresTransformation indicates that OpenRouter needs Anthropic->OpenAI translation.
func (p *OpenRouterProvider) RequiresTransformation() bool {
	return true
}

// OpenRouterModelPricing contains pricing information for a model.
type OpenRouterModelPricing struct {
	Prompt     string `json:"prompt"`     // Price per token for prompt
	Completion string `json:"completion"` // Price per token for completion
}

// OpenRouterModelTopProvider contains top provider information.
type OpenRouterModelTopProvider struct {
	ContextLength  int  `json:"context_length"`
	MaxCompletions int  `json:"max_completion_tokens"`
	IsModerated    bool `json:"is_moderated"`
}

// OpenRouterModelLimits contains per-request limits.
type OpenRouterModelLimits struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

// OpenRouterModel represents a model from OpenRouter's models endpoint.
type OpenRouterModel struct {
	ID               string                     `json:"id"`
	Name             string                     `json:"name"`
	Description      string                     `json:"description"`
	ContextLength    int                        `json:"context_length"`
	Pricing          OpenRouterModelPricing     `json:"pricing"`
	TopProvider      OpenRouterModelTopProvider `json:"top_provider"`
	PerRequestLimits *OpenRouterModelLimits     `json:"per_request_limits,omitempty"`
}

// OpenRouterModelsResponse is the response from /models endpoint.
type OpenRouterModelsResponse struct {
	Data []OpenRouterModel `json:"data"`
}

// OpenRouterModelInfo contains detailed model information with pricing.
type OpenRouterModelInfo struct {
	ID            string
	Name          string
	Description   string
	ContextLength int
	InputPrice    float64 // Price per 1M tokens
	OutputPrice   float64 // Price per 1M tokens
	Provider      string  // Extracted from ID (e.g., "openai" from "openai/gpt-4o")
}

// ListModels returns available models from OpenRouter's /models endpoint.
// This endpoint is publicly accessible without an API key.
func (p *OpenRouterProvider) ListModels() ([]string, error) {
	models, err := p.ListModelsWithInfo()
	if err != nil {
		return nil, err
	}

	ids := make([]string, len(models))
	for i, m := range models {
		ids[i] = m.ID
	}
	return ids, nil
}

// ListModelsWithInfo returns models with full pricing and context info.
func (p *OpenRouterProvider) ListModelsWithInfo() ([]OpenRouterModelInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// OpenRouter's /models endpoint is public
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.BaseURL+"/models", http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("HTTP-Referer", "https://github.com/jedarden/CLASP")
	req.Header.Set("X-Title", "CLASP Proxy")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OpenRouter returned status %d", resp.StatusCode)
	}

	var modelsResp OpenRouterModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert to our info structure with parsed pricing
	models := make([]OpenRouterModelInfo, 0, len(modelsResp.Data))
	for _, m := range modelsResp.Data {
		info := OpenRouterModelInfo{
			ID:            m.ID,
			Name:          m.Name,
			Description:   m.Description,
			ContextLength: m.ContextLength,
		}

		// Extract provider from ID (e.g., "openai/gpt-4o" -> "openai")
		if idx := strings.Index(m.ID, "/"); idx > 0 {
			info.Provider = m.ID[:idx]
		}

		// Parse pricing (OpenRouter prices are per-token, convert to per-1M)
		if m.Pricing.Prompt != "" {
			var price float64
			if _, scanErr := fmt.Sscanf(m.Pricing.Prompt, "%f", &price); scanErr == nil {
				info.InputPrice = price * 1000000 // Convert to per-1M
			}
		}
		if m.Pricing.Completion != "" {
			var price float64
			if _, scanErr := fmt.Sscanf(m.Pricing.Completion, "%f", &price); scanErr == nil {
				info.OutputPrice = price * 1000000 // Convert to per-1M
			}
		}

		models = append(models, info)
	}

	// Sort by provider then model name for easier browsing
	sort.Slice(models, func(i, j int) bool {
		if models[i].Provider != models[j].Provider {
			return models[i].Provider < models[j].Provider
		}
		return models[i].ID < models[j].ID
	})

	return models, nil
}

// ListModelsByProvider returns models filtered by provider (e.g., "openai", "anthropic").
func (p *OpenRouterProvider) ListModelsByProvider(provider string) ([]OpenRouterModelInfo, error) {
	models, err := p.ListModelsWithInfo()
	if err != nil {
		return nil, err
	}

	var filtered []OpenRouterModelInfo
	for _, m := range models {
		if strings.EqualFold(m.Provider, provider) {
			filtered = append(filtered, m)
		}
	}

	return filtered, nil
}

// GetChatModels returns only models suitable for chat completions.
// Filters out embedding, moderation, and non-chat models.
func (p *OpenRouterProvider) GetChatModels() ([]OpenRouterModelInfo, error) {
	models, err := p.ListModelsWithInfo()
	if err != nil {
		return nil, err
	}

	chatModels := make([]OpenRouterModelInfo, 0, len(models))
	for _, m := range models {
		id := strings.ToLower(m.ID)
		// Skip embedding models
		if strings.Contains(id, "embed") {
			continue
		}
		// Skip moderation models
		if strings.Contains(id, "moderation") {
			continue
		}
		// Skip image-only models
		if strings.Contains(id, "dall-e") || strings.Contains(id, "stable-diffusion") {
			continue
		}
		// Skip audio models
		if strings.Contains(id, "whisper") || strings.Contains(id, "tts") {
			continue
		}
		chatModels = append(chatModels, m)
	}

	return chatModels, nil
}
