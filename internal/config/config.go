// Package config manages CLASP configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// ProviderType represents the type of LLM provider.
type ProviderType string

const (
	ProviderOpenAI     ProviderType = "openai"
	ProviderAzure      ProviderType = "azure"
	ProviderOpenRouter ProviderType = "openrouter"
	ProviderAnthropic  ProviderType = "anthropic"
	ProviderOllama     ProviderType = "ollama"
	ProviderGemini     ProviderType = "gemini"
	ProviderDeepSeek   ProviderType = "deepseek"
	ProviderCustom     ProviderType = "custom"
)

// TierConfig holds configuration for a specific model tier.
type TierConfig struct {
	Provider ProviderType
	Model    string
	APIKey   string
	BaseURL  string
	// Fallback configuration
	FallbackProvider ProviderType
	FallbackModel    string
	FallbackAPIKey   string
	FallbackBaseURL  string
}

// Config holds the CLASP configuration.
type Config struct {
	// Provider settings
	Provider ProviderType

	// API keys
	OpenAIAPIKey     string
	AzureAPIKey      string
	OpenRouterAPIKey string
	AnthropicAPIKey  string
	OllamaAPIKey     string // Optional, most Ollama instances don't need auth
	GeminiAPIKey     string // Google AI Studio API key
	DeepSeekAPIKey   string // DeepSeek API key
	CustomAPIKey     string

	// Endpoints
	OpenAIBaseURL        string
	AzureEndpoint        string
	AzureDeploymentName  string
	AzureAPIVersion      string
	OpenRouterBaseURL    string
	OllamaBaseURL        string // Default: http://localhost:11434
	GeminiBaseURL        string // Default: https://generativelanguage.googleapis.com/v1beta
	DeepSeekBaseURL      string // Default: https://api.deepseek.com
	CustomBaseURL        string

	// Model mapping
	DefaultModel string
	ModelOpus    string
	ModelSonnet  string
	ModelHaiku   string

	// Multi-provider routing (per-tier provider configuration)
	MultiProviderEnabled bool
	TierOpus             *TierConfig
	TierSonnet           *TierConfig
	TierHaiku            *TierConfig

	// Fallback routing (global fallback provider)
	FallbackEnabled  bool
	FallbackProvider ProviderType
	FallbackModel    string
	FallbackAPIKey   string
	FallbackBaseURL  string

	// Server settings
	Port     int
	LogLevel string

	// Debug settings
	Debug          bool
	DebugRequests  bool
	DebugResponses bool

	// Rate limiting settings
	RateLimitEnabled   bool
	RateLimitRequests  int     // Requests per window
	RateLimitWindow    int     // Window in seconds
	RateLimitBurst     int     // Burst allowance

	// Cache settings
	CacheEnabled  bool
	CacheMaxSize  int   // Maximum number of entries
	CacheTTL      int   // Time-to-live in seconds (0 = no expiry)

	// Authentication settings
	AuthEnabled              bool
	AuthAPIKey               string
	AuthAllowAnonymousHealth  bool
	AuthAllowAnonymousMetrics bool

	// Queue settings
	QueueEnabled           bool
	QueueMaxSize           int   // Maximum requests to queue
	QueueMaxWaitSeconds    int   // Maximum time a request can wait
	QueueRetryDelayMs      int   // Delay between retries in milliseconds
	QueueMaxRetries        int   // Maximum retries per request

	// Circuit breaker settings
	CircuitBreakerEnabled      bool
	CircuitBreakerThreshold    int   // Failures before opening
	CircuitBreakerRecovery     int   // Successes to close
	CircuitBreakerTimeoutSec   int   // Timeout before half-open

	// HTTP client settings
	HTTPClientTimeoutSec int // Timeout for upstream requests (default: 300 = 5 minutes)

	// Model aliasing - map custom model names to provider models
	ModelAliases map[string]string
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		Provider:           ProviderOpenAI,
		OpenAIBaseURL:      "https://api.openai.com/v1",
		OpenRouterBaseURL:  "https://openrouter.ai/api/v1",
		OllamaBaseURL:      "http://localhost:11434",
		GeminiBaseURL:      "https://generativelanguage.googleapis.com/v1beta",
		DeepSeekBaseURL:    "https://api.deepseek.com",
		AzureAPIVersion:    "2024-02-15-preview",
		Port:               8080,
		LogLevel:           "info",
		DefaultModel:       "gpt-4o",
		RateLimitEnabled:   false,
		RateLimitRequests:  60,   // 60 requests per window (default)
		RateLimitWindow:    60,   // 60 second window (default)
		RateLimitBurst:     10,   // Allow burst of 10 (default)
		CacheEnabled:             false,
		CacheMaxSize:             1000, // Default 1000 entries
		CacheTTL:                 3600, // Default 1 hour TTL
		AuthEnabled:              false,
		AuthAllowAnonymousHealth:  true, // Allow health checks without auth by default
		AuthAllowAnonymousMetrics: false,
		// Queue defaults
		QueueEnabled:           false,
		QueueMaxSize:           100,
		QueueMaxWaitSeconds:    30,
		QueueRetryDelayMs:      1000,
		QueueMaxRetries:        3,
		// Circuit breaker defaults
		CircuitBreakerEnabled:    false,
		CircuitBreakerThreshold:  5,  // Open after 5 failures
		CircuitBreakerRecovery:   2,  // Close after 2 successes
		CircuitBreakerTimeoutSec: 30, // Try again after 30 seconds
		// HTTP client defaults
		HTTPClientTimeoutSec: 300, // 5 minutes for reasoning models
		// Model aliases (empty by default)
		ModelAliases: make(map[string]string),
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
	cfg.OllamaAPIKey = os.Getenv("OLLAMA_API_KEY") // Optional
	cfg.GeminiAPIKey = os.Getenv("GEMINI_API_KEY") // Google AI Studio key
	cfg.DeepSeekAPIKey = os.Getenv("DEEPSEEK_API_KEY") // DeepSeek API key
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
	if baseURL := os.Getenv("OLLAMA_BASE_URL"); baseURL != "" {
		cfg.OllamaBaseURL = baseURL
	}
	if baseURL := os.Getenv("GEMINI_BASE_URL"); baseURL != "" {
		cfg.GeminiBaseURL = baseURL
	}
	if baseURL := os.Getenv("DEEPSEEK_BASE_URL"); baseURL != "" {
		cfg.DeepSeekBaseURL = baseURL
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

	// Rate limiting settings
	cfg.RateLimitEnabled = os.Getenv("CLASP_RATE_LIMIT") == "true" || os.Getenv("CLASP_RATE_LIMIT") == "1"
	if rps := os.Getenv("CLASP_RATE_LIMIT_REQUESTS"); rps != "" {
		r, err := strconv.Atoi(rps)
		if err != nil {
			return nil, fmt.Errorf("invalid CLASP_RATE_LIMIT_REQUESTS: %w", err)
		}
		cfg.RateLimitRequests = r
	}
	if window := os.Getenv("CLASP_RATE_LIMIT_WINDOW"); window != "" {
		w, err := strconv.Atoi(window)
		if err != nil {
			return nil, fmt.Errorf("invalid CLASP_RATE_LIMIT_WINDOW: %w", err)
		}
		cfg.RateLimitWindow = w
	}
	if burst := os.Getenv("CLASP_RATE_LIMIT_BURST"); burst != "" {
		b, err := strconv.Atoi(burst)
		if err != nil {
			return nil, fmt.Errorf("invalid CLASP_RATE_LIMIT_BURST: %w", err)
		}
		cfg.RateLimitBurst = b
	}

	// Cache settings
	cfg.CacheEnabled = os.Getenv("CLASP_CACHE") == "true" || os.Getenv("CLASP_CACHE") == "1"
	if maxSize := os.Getenv("CLASP_CACHE_MAX_SIZE"); maxSize != "" {
		m, err := strconv.Atoi(maxSize)
		if err != nil {
			return nil, fmt.Errorf("invalid CLASP_CACHE_MAX_SIZE: %w", err)
		}
		cfg.CacheMaxSize = m
	}
	if ttl := os.Getenv("CLASP_CACHE_TTL"); ttl != "" {
		t, err := strconv.Atoi(ttl)
		if err != nil {
			return nil, fmt.Errorf("invalid CLASP_CACHE_TTL: %w", err)
		}
		cfg.CacheTTL = t
	}

	// Authentication settings
	cfg.AuthEnabled = os.Getenv("CLASP_AUTH") == "true" || os.Getenv("CLASP_AUTH") == "1"
	cfg.AuthAPIKey = os.Getenv("CLASP_AUTH_API_KEY")
	if os.Getenv("CLASP_AUTH_ALLOW_ANONYMOUS_HEALTH") == "false" || os.Getenv("CLASP_AUTH_ALLOW_ANONYMOUS_HEALTH") == "0" {
		cfg.AuthAllowAnonymousHealth = false
	}
	if os.Getenv("CLASP_AUTH_ALLOW_ANONYMOUS_METRICS") == "true" || os.Getenv("CLASP_AUTH_ALLOW_ANONYMOUS_METRICS") == "1" {
		cfg.AuthAllowAnonymousMetrics = true
	}

	// Queue settings
	cfg.QueueEnabled = os.Getenv("CLASP_QUEUE") == "true" || os.Getenv("CLASP_QUEUE") == "1"
	if maxSize := os.Getenv("CLASP_QUEUE_MAX_SIZE"); maxSize != "" {
		m, err := strconv.Atoi(maxSize)
		if err != nil {
			return nil, fmt.Errorf("invalid CLASP_QUEUE_MAX_SIZE: %w", err)
		}
		cfg.QueueMaxSize = m
	}
	if maxWait := os.Getenv("CLASP_QUEUE_MAX_WAIT"); maxWait != "" {
		m, err := strconv.Atoi(maxWait)
		if err != nil {
			return nil, fmt.Errorf("invalid CLASP_QUEUE_MAX_WAIT: %w", err)
		}
		cfg.QueueMaxWaitSeconds = m
	}
	if retryDelay := os.Getenv("CLASP_QUEUE_RETRY_DELAY"); retryDelay != "" {
		r, err := strconv.Atoi(retryDelay)
		if err != nil {
			return nil, fmt.Errorf("invalid CLASP_QUEUE_RETRY_DELAY: %w", err)
		}
		cfg.QueueRetryDelayMs = r
	}
	if maxRetries := os.Getenv("CLASP_QUEUE_MAX_RETRIES"); maxRetries != "" {
		m, err := strconv.Atoi(maxRetries)
		if err != nil {
			return nil, fmt.Errorf("invalid CLASP_QUEUE_MAX_RETRIES: %w", err)
		}
		cfg.QueueMaxRetries = m
	}

	// Model aliasing - load aliases from environment
	// Pattern: CLASP_ALIAS_<alias>=<target_model>
	// Also supports: CLASP_MODEL_ALIASES=alias1:model1,alias2:model2
	cfg.ModelAliases = loadModelAliases()

	// Circuit breaker settings
	cfg.CircuitBreakerEnabled = os.Getenv("CLASP_CIRCUIT_BREAKER") == "true" || os.Getenv("CLASP_CIRCUIT_BREAKER") == "1"
	if threshold := os.Getenv("CLASP_CIRCUIT_BREAKER_THRESHOLD"); threshold != "" {
		t, err := strconv.Atoi(threshold)
		if err != nil {
			return nil, fmt.Errorf("invalid CLASP_CIRCUIT_BREAKER_THRESHOLD: %w", err)
		}
		cfg.CircuitBreakerThreshold = t
	}
	if recovery := os.Getenv("CLASP_CIRCUIT_BREAKER_RECOVERY"); recovery != "" {
		r, err := strconv.Atoi(recovery)
		if err != nil {
			return nil, fmt.Errorf("invalid CLASP_CIRCUIT_BREAKER_RECOVERY: %w", err)
		}
		cfg.CircuitBreakerRecovery = r
	}
	if timeout := os.Getenv("CLASP_CIRCUIT_BREAKER_TIMEOUT"); timeout != "" {
		t, err := strconv.Atoi(timeout)
		if err != nil {
			return nil, fmt.Errorf("invalid CLASP_CIRCUIT_BREAKER_TIMEOUT: %w", err)
		}
		cfg.CircuitBreakerTimeoutSec = t
	}

	// HTTP client settings
	if httpTimeout := os.Getenv("CLASP_HTTP_TIMEOUT"); httpTimeout != "" {
		t, err := strconv.Atoi(httpTimeout)
		if err != nil {
			return nil, fmt.Errorf("invalid CLASP_HTTP_TIMEOUT: %w", err)
		}
		cfg.HTTPClientTimeoutSec = t
	}

	// Multi-provider routing settings
	cfg.MultiProviderEnabled = os.Getenv("CLASP_MULTI_PROVIDER") == "true" || os.Getenv("CLASP_MULTI_PROVIDER") == "1"
	cfg.TierOpus = loadTierConfig("OPUS", cfg)
	cfg.TierSonnet = loadTierConfig("SONNET", cfg)
	cfg.TierHaiku = loadTierConfig("HAIKU", cfg)

	// Fallback routing settings
	cfg.FallbackEnabled = os.Getenv("CLASP_FALLBACK") == "true" || os.Getenv("CLASP_FALLBACK") == "1"
	if fallbackProvider := os.Getenv("CLASP_FALLBACK_PROVIDER"); fallbackProvider != "" {
		cfg.FallbackProvider = ProviderType(fallbackProvider)
	}
	cfg.FallbackModel = os.Getenv("CLASP_FALLBACK_MODEL")
	cfg.FallbackAPIKey = os.Getenv("CLASP_FALLBACK_API_KEY")
	cfg.FallbackBaseURL = os.Getenv("CLASP_FALLBACK_BASE_URL")

	// Inherit API key from main config if not specified
	if cfg.FallbackEnabled && cfg.FallbackAPIKey == "" {
		switch cfg.FallbackProvider {
		case ProviderOpenAI:
			cfg.FallbackAPIKey = cfg.OpenAIAPIKey
		case ProviderOpenRouter:
			cfg.FallbackAPIKey = cfg.OpenRouterAPIKey
		case ProviderAzure:
			cfg.FallbackAPIKey = cfg.AzureAPIKey
		case ProviderAnthropic:
			cfg.FallbackAPIKey = cfg.AnthropicAPIKey
		case ProviderCustom:
			cfg.FallbackAPIKey = cfg.CustomAPIKey
		}
	}

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
	if cfg.GeminiAPIKey != "" {
		return ProviderGemini
	}
	if cfg.DeepSeekAPIKey != "" {
		return ProviderDeepSeek
	}
	// Ollama doesn't require API key, check if base URL is set or use detection
	if cfg.OllamaBaseURL != "" && cfg.OllamaBaseURL != "http://localhost:11434" {
		return ProviderOllama
	}
	if cfg.CustomAPIKey != "" && cfg.CustomBaseURL != "" {
		return ProviderCustom
	}
	return ProviderOpenAI // Default
}

// loadTierConfig loads tier-specific configuration from environment variables.
// Pattern: CLASP_<TIER>_PROVIDER, CLASP_<TIER>_MODEL, CLASP_<TIER>_API_KEY, CLASP_<TIER>_BASE_URL
// Fallback: CLASP_<TIER>_FALLBACK_PROVIDER, CLASP_<TIER>_FALLBACK_MODEL, etc.
func loadTierConfig(tier string, cfg *Config) *TierConfig {
	provider := os.Getenv("CLASP_" + tier + "_PROVIDER")
	model := os.Getenv("CLASP_" + tier + "_MODEL")
	apiKey := os.Getenv("CLASP_" + tier + "_API_KEY")
	baseURL := os.Getenv("CLASP_" + tier + "_BASE_URL")

	// If no provider specified for this tier, return nil
	if provider == "" && model == "" {
		return nil
	}

	tierCfg := &TierConfig{
		Provider: ProviderType(provider),
		Model:    model,
		APIKey:   apiKey,
		BaseURL:  baseURL,
	}

	// If no explicit API key, inherit from main config based on provider
	if tierCfg.APIKey == "" {
		switch tierCfg.Provider {
		case ProviderOpenAI:
			tierCfg.APIKey = cfg.OpenAIAPIKey
		case ProviderOpenRouter:
			tierCfg.APIKey = cfg.OpenRouterAPIKey
		case ProviderAzure:
			tierCfg.APIKey = cfg.AzureAPIKey
		case ProviderAnthropic:
			tierCfg.APIKey = cfg.AnthropicAPIKey
		case ProviderOllama:
			tierCfg.APIKey = cfg.OllamaAPIKey // Usually empty
		case ProviderGemini:
			tierCfg.APIKey = cfg.GeminiAPIKey
		case ProviderDeepSeek:
			tierCfg.APIKey = cfg.DeepSeekAPIKey
		case ProviderCustom:
			tierCfg.APIKey = cfg.CustomAPIKey
		}
	}

	// If no explicit base URL, use defaults based on provider
	if tierCfg.BaseURL == "" {
		switch tierCfg.Provider {
		case ProviderOpenAI:
			tierCfg.BaseURL = cfg.OpenAIBaseURL
		case ProviderOpenRouter:
			tierCfg.BaseURL = cfg.OpenRouterBaseURL
		case ProviderOllama:
			tierCfg.BaseURL = cfg.OllamaBaseURL + "/v1"
		case ProviderGemini:
			tierCfg.BaseURL = cfg.GeminiBaseURL + "/openai"
		case ProviderDeepSeek:
			tierCfg.BaseURL = cfg.DeepSeekBaseURL + "/v1"
		case ProviderCustom:
			tierCfg.BaseURL = cfg.CustomBaseURL
		}
	}

	// Load tier-specific fallback configuration
	fallbackProvider := os.Getenv("CLASP_" + tier + "_FALLBACK_PROVIDER")
	if fallbackProvider != "" {
		tierCfg.FallbackProvider = ProviderType(fallbackProvider)
		tierCfg.FallbackModel = os.Getenv("CLASP_" + tier + "_FALLBACK_MODEL")
		tierCfg.FallbackAPIKey = os.Getenv("CLASP_" + tier + "_FALLBACK_API_KEY")
		tierCfg.FallbackBaseURL = os.Getenv("CLASP_" + tier + "_FALLBACK_BASE_URL")

		// Inherit fallback API key if not specified
		if tierCfg.FallbackAPIKey == "" {
			switch tierCfg.FallbackProvider {
			case ProviderOpenAI:
				tierCfg.FallbackAPIKey = cfg.OpenAIAPIKey
			case ProviderOpenRouter:
				tierCfg.FallbackAPIKey = cfg.OpenRouterAPIKey
			case ProviderAzure:
				tierCfg.FallbackAPIKey = cfg.AzureAPIKey
			case ProviderAnthropic:
				tierCfg.FallbackAPIKey = cfg.AnthropicAPIKey
			case ProviderOllama:
				tierCfg.FallbackAPIKey = cfg.OllamaAPIKey
			case ProviderGemini:
				tierCfg.FallbackAPIKey = cfg.GeminiAPIKey
			case ProviderDeepSeek:
				tierCfg.FallbackAPIKey = cfg.DeepSeekAPIKey
			case ProviderCustom:
				tierCfg.FallbackAPIKey = cfg.CustomAPIKey
			}
		}
	}

	return tierCfg
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
	case ProviderOllama:
		// Ollama doesn't require API key - it runs locally
		// Base URL defaults to http://localhost:11434
	case ProviderGemini:
		if c.GeminiAPIKey == "" {
			return fmt.Errorf("GEMINI_API_KEY is required for provider 'gemini'")
		}
	case ProviderDeepSeek:
		if c.DeepSeekAPIKey == "" {
			return fmt.Errorf("DEEPSEEK_API_KEY is required for provider 'deepseek'")
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
	case ProviderOllama:
		return c.OllamaAPIKey // Usually empty for local Ollama
	case ProviderGemini:
		return c.GeminiAPIKey
	case ProviderDeepSeek:
		return c.DeepSeekAPIKey
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
	case ProviderOllama:
		// Ollama exposes OpenAI-compatible API at /v1
		return c.OllamaBaseURL + "/v1"
	case ProviderGemini:
		// Gemini uses OpenAI-compatible endpoint at /v1beta/openai
		return c.GeminiBaseURL + "/openai"
	case ProviderDeepSeek:
		// DeepSeek uses standard OpenAI-compatible /v1 endpoint
		return c.DeepSeekBaseURL + "/v1"
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

// GetTierConfig returns the tier-specific configuration for a given model.
// Returns nil if multi-provider routing is disabled or no tier config exists.
func (c *Config) GetTierConfig(requestedModel string) *TierConfig {
	if !c.MultiProviderEnabled {
		return nil
	}

	// Match model to tier
	switch {
	case contains(requestedModel, "opus"):
		return c.TierOpus
	case contains(requestedModel, "sonnet"):
		return c.TierSonnet
	case contains(requestedModel, "haiku"):
		return c.TierHaiku
	}

	return nil
}

// HasFallback checks if the tier config has a fallback provider configured.
func (tc *TierConfig) HasFallback() bool {
	return tc != nil && tc.FallbackProvider != ""
}

// GetFallbackConfig returns a new TierConfig representing the fallback.
func (tc *TierConfig) GetFallbackConfig() *TierConfig {
	if tc == nil || tc.FallbackProvider == "" {
		return nil
	}
	return &TierConfig{
		Provider: tc.FallbackProvider,
		Model:    tc.FallbackModel,
		APIKey:   tc.FallbackAPIKey,
		BaseURL:  tc.FallbackBaseURL,
	}
}

// HasGlobalFallback checks if global fallback is configured.
func (c *Config) HasGlobalFallback() bool {
	return c.FallbackEnabled && c.FallbackProvider != ""
}

// GetGlobalFallbackConfig returns a TierConfig for the global fallback.
func (c *Config) GetGlobalFallbackConfig() *TierConfig {
	if !c.HasGlobalFallback() {
		return nil
	}
	baseURL := c.FallbackBaseURL
	if baseURL == "" {
		switch c.FallbackProvider {
		case ProviderOpenAI:
			baseURL = c.OpenAIBaseURL
		case ProviderOpenRouter:
			baseURL = c.OpenRouterBaseURL
		case ProviderCustom:
			baseURL = c.CustomBaseURL
		}
	}
	return &TierConfig{
		Provider: c.FallbackProvider,
		Model:    c.FallbackModel,
		APIKey:   c.FallbackAPIKey,
		BaseURL:  baseURL,
	}
}

// ModelTier represents a Claude model tier.
type ModelTier string

const (
	TierOpus   ModelTier = "opus"
	TierSonnet ModelTier = "sonnet"
	TierHaiku  ModelTier = "haiku"
)

// GetModelTier returns the tier for a given model name.
func GetModelTier(model string) ModelTier {
	switch {
	case contains(model, "opus"):
		return TierOpus
	case contains(model, "sonnet"):
		return TierSonnet
	case contains(model, "haiku"):
		return TierHaiku
	default:
		return TierSonnet // Default to sonnet tier
	}
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

// loadModelAliases loads model aliases from environment variables.
// Supports two patterns:
// 1. CLASP_ALIAS_<alias>=<target_model> (e.g., CLASP_ALIAS_FAST=gpt-4o-mini)
// 2. CLASP_MODEL_ALIASES=alias1:model1,alias2:model2 (comma-separated list)
func loadModelAliases() map[string]string {
	aliases := make(map[string]string)

	// Load from CLASP_ALIAS_* environment variables
	const aliasPrefix = "CLASP_ALIAS_"
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, aliasPrefix) {
			parts := strings.SplitN(env, "=", 2)
			if len(parts) == 2 {
				aliasName := strings.ToLower(strings.TrimPrefix(parts[0], aliasPrefix))
				targetModel := parts[1]
				if aliasName != "" && targetModel != "" {
					aliases[aliasName] = targetModel
				}
			}
		}
	}

	// Load from CLASP_MODEL_ALIASES (comma-separated list)
	if aliasesStr := os.Getenv("CLASP_MODEL_ALIASES"); aliasesStr != "" {
		for _, pair := range strings.Split(aliasesStr, ",") {
			parts := strings.SplitN(strings.TrimSpace(pair), ":", 2)
			if len(parts) == 2 {
				aliasName := strings.ToLower(strings.TrimSpace(parts[0]))
				targetModel := strings.TrimSpace(parts[1])
				if aliasName != "" && targetModel != "" {
					aliases[aliasName] = targetModel
				}
			}
		}
	}

	return aliases
}

// ResolveAlias resolves a model alias to its target model.
// If the model is not an alias, returns the original model unchanged.
func (c *Config) ResolveAlias(model string) string {
	// Check if this model is an alias (case-insensitive lookup)
	modelLower := strings.ToLower(model)
	if target, ok := c.ModelAliases[modelLower]; ok {
		return target
	}
	return model
}

// AddAlias adds a model alias at runtime.
func (c *Config) AddAlias(alias, targetModel string) {
	if c.ModelAliases == nil {
		c.ModelAliases = make(map[string]string)
	}
	c.ModelAliases[strings.ToLower(alias)] = targetModel
}

// GetAliases returns all configured model aliases.
func (c *Config) GetAliases() map[string]string {
	result := make(map[string]string, len(c.ModelAliases))
	for k, v := range c.ModelAliases {
		result[k] = v
	}
	return result
}
