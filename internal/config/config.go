// Package config manages CLASP configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
)

// ProviderType represents the type of LLM provider.
type ProviderType string

const (
	ProviderOpenAI     ProviderType = "openai"
	ProviderAzure      ProviderType = "azure"
	ProviderOpenRouter ProviderType = "openrouter"
	ProviderAnthropic  ProviderType = "anthropic"
	ProviderCustom     ProviderType = "custom"
)

// Config holds the CLASP configuration.
type Config struct {
	// Provider settings
	Provider ProviderType

	// API keys
	OpenAIAPIKey     string
	AzureAPIKey      string
	OpenRouterAPIKey string
	AnthropicAPIKey  string
	CustomAPIKey     string

	// Endpoints
	OpenAIBaseURL        string
	AzureEndpoint        string
	AzureDeploymentName  string
	AzureAPIVersion      string
	OpenRouterBaseURL    string
	CustomBaseURL        string

	// Model mapping
	DefaultModel string
	ModelOpus    string
	ModelSonnet  string
	ModelHaiku   string

	// Server settings
	Port     int
	LogLevel string

	// Debug settings
	Debug          bool
	DebugRequests  bool
	DebugResponses bool
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		Provider:          ProviderOpenAI,
		OpenAIBaseURL:     "https://api.openai.com/v1",
		OpenRouterBaseURL: "https://openrouter.ai/api/v1",
		AzureAPIVersion:   "2024-02-15-preview",
		Port:              8080,
		LogLevel:          "info",
		DefaultModel:      "gpt-4o",
	}
}

// LoadFromEnv loads configuration from environment variables.
func LoadFromEnv() (*Config, error) {
	cfg := DefaultConfig()

	// Provider selection
	if provider := os.Getenv("PROVIDER"); provider != "" {
		cfg.Provider = ProviderType(provider)
	}

	// API keys
	cfg.OpenAIAPIKey = os.Getenv("OPENAI_API_KEY")
	cfg.AzureAPIKey = os.Getenv("AZURE_API_KEY")
	cfg.OpenRouterAPIKey = os.Getenv("OPENROUTER_API_KEY")
	cfg.AnthropicAPIKey = os.Getenv("ANTHROPIC_API_KEY")
	cfg.CustomAPIKey = os.Getenv("CUSTOM_API_KEY")

	// Endpoints
	if baseURL := os.Getenv("OPENAI_BASE_URL"); baseURL != "" {
		cfg.OpenAIBaseURL = baseURL
	}
	cfg.AzureEndpoint = os.Getenv("AZURE_OPENAI_ENDPOINT")
	cfg.AzureDeploymentName = os.Getenv("AZURE_DEPLOYMENT_NAME")
	if apiVersion := os.Getenv("AZURE_API_VERSION"); apiVersion != "" {
		cfg.AzureAPIVersion = apiVersion
	}
	if baseURL := os.Getenv("OPENROUTER_BASE_URL"); baseURL != "" {
		cfg.OpenRouterBaseURL = baseURL
	}
	cfg.CustomBaseURL = os.Getenv("CUSTOM_BASE_URL")

	// Model configuration
	if model := os.Getenv("CLASP_MODEL"); model != "" {
		cfg.DefaultModel = model
	}
	cfg.ModelOpus = os.Getenv("CLASP_MODEL_OPUS")
	cfg.ModelSonnet = os.Getenv("CLASP_MODEL_SONNET")
	cfg.ModelHaiku = os.Getenv("CLASP_MODEL_HAIKU")

	// Server settings
	if port := os.Getenv("CLASP_PORT"); port != "" {
		p, err := strconv.Atoi(port)
		if err != nil {
			return nil, fmt.Errorf("invalid CLASP_PORT: %w", err)
		}
		cfg.Port = p
	}
	if logLevel := os.Getenv("CLASP_LOG_LEVEL"); logLevel != "" {
		cfg.LogLevel = logLevel
	}

	// Debug settings
	cfg.Debug = os.Getenv("CLASP_DEBUG") == "true" || os.Getenv("CLASP_DEBUG") == "1"
	cfg.DebugRequests = cfg.Debug || os.Getenv("CLASP_DEBUG_REQUESTS") == "true"
	cfg.DebugResponses = cfg.Debug || os.Getenv("CLASP_DEBUG_RESPONSES") == "true"

	// Auto-detect provider from available API keys if not explicitly set
	if os.Getenv("PROVIDER") == "" {
		cfg.Provider = detectProvider(cfg)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// detectProvider determines the provider based on available API keys.
func detectProvider(cfg *Config) ProviderType {
	if cfg.OpenAIAPIKey != "" {
		return ProviderOpenAI
	}
	if cfg.OpenRouterAPIKey != "" {
		return ProviderOpenRouter
	}
	if cfg.AzureAPIKey != "" && cfg.AzureEndpoint != "" {
		return ProviderAzure
	}
	if cfg.AnthropicAPIKey != "" {
		return ProviderAnthropic
	}
	if cfg.CustomAPIKey != "" && cfg.CustomBaseURL != "" {
		return ProviderCustom
	}
	return ProviderOpenAI // Default
}

// Validate checks that the configuration is valid.
func (c *Config) Validate() error {
	switch c.Provider {
	case ProviderOpenAI:
		if c.OpenAIAPIKey == "" {
			return fmt.Errorf("OPENAI_API_KEY is required for provider 'openai'")
		}
	case ProviderAzure:
		if c.AzureAPIKey == "" {
			return fmt.Errorf("AZURE_API_KEY is required for provider 'azure'")
		}
		if c.AzureEndpoint == "" {
			return fmt.Errorf("AZURE_OPENAI_ENDPOINT is required for provider 'azure'")
		}
		if c.AzureDeploymentName == "" {
			return fmt.Errorf("AZURE_DEPLOYMENT_NAME is required for provider 'azure'")
		}
	case ProviderOpenRouter:
		if c.OpenRouterAPIKey == "" {
			return fmt.Errorf("OPENROUTER_API_KEY is required for provider 'openrouter'")
		}
	case ProviderAnthropic:
		if c.AnthropicAPIKey == "" {
			return fmt.Errorf("ANTHROPIC_API_KEY is required for provider 'anthropic'")
		}
	case ProviderCustom:
		if c.CustomBaseURL == "" {
			return fmt.Errorf("CUSTOM_BASE_URL is required for provider 'custom'")
		}
	default:
		return fmt.Errorf("unknown provider: %s", c.Provider)
	}

	return nil
}

// GetAPIKey returns the API key for the configured provider.
func (c *Config) GetAPIKey() string {
	switch c.Provider {
	case ProviderOpenAI:
		return c.OpenAIAPIKey
	case ProviderAzure:
		return c.AzureAPIKey
	case ProviderOpenRouter:
		return c.OpenRouterAPIKey
	case ProviderAnthropic:
		return c.AnthropicAPIKey
	case ProviderCustom:
		return c.CustomAPIKey
	default:
		return ""
	}
}

// GetBaseURL returns the base URL for the configured provider.
func (c *Config) GetBaseURL() string {
	switch c.Provider {
	case ProviderOpenAI:
		return c.OpenAIBaseURL
	case ProviderAzure:
		return fmt.Sprintf("%s/openai/deployments/%s", c.AzureEndpoint, c.AzureDeploymentName)
	case ProviderOpenRouter:
		return c.OpenRouterBaseURL
	case ProviderAnthropic:
		return "https://api.anthropic.com"
	case ProviderCustom:
		return c.CustomBaseURL
	default:
		return ""
	}
}

// GetChatCompletionsURL returns the full URL for chat completions.
func (c *Config) GetChatCompletionsURL() string {
	switch c.Provider {
	case ProviderAzure:
		return fmt.Sprintf("%s/chat/completions?api-version=%s", c.GetBaseURL(), c.AzureAPIVersion)
	default:
		return c.GetBaseURL() + "/chat/completions"
	}
}

// MapModel applies model mapping based on the requested model.
func (c *Config) MapModel(requestedModel string) string {
	// Check for tier-based mapping
	switch {
	case contains(requestedModel, "opus") && c.ModelOpus != "":
		return c.ModelOpus
	case contains(requestedModel, "sonnet") && c.ModelSonnet != "":
		return c.ModelSonnet
	case contains(requestedModel, "haiku") && c.ModelHaiku != "":
		return c.ModelHaiku
	}

	// Return default model if set, otherwise use requested model
	if c.DefaultModel != "" {
		return c.DefaultModel
	}
	return requestedModel
}

// contains checks if s contains substr (case-insensitive).
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
		 len(s) > len(substr) &&
		 (s[:len(substr)] == substr ||
		  s[len(s)-len(substr):] == substr ||
		  containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 1; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
