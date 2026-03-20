// Package config manages CLASP configuration from files and environment variables.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// FileConfig represents the structure of a CLASP configuration file.
// It uses a hierarchical structure that maps cleanly to YAML/TOML.
type FileConfig struct {
	// Provider settings
	Provider string `yaml:"provider"`

	// API keys (can reference environment variables with ${VAR} syntax)
	APIKeys APIKeysConfig `yaml:"api_keys,omitempty"`

	// Endpoints configuration
	Endpoints EndpointsConfig `yaml:"endpoints,omitempty"`

	// Model configuration
	Models ModelsConfig `yaml:"models,omitempty"`

	// Multi-provider routing (per-tier provider configuration)
	MultiProvider MultiProviderConfig `yaml:"multi_provider,omitempty"`

	// Fallback routing (global fallback provider)
	Fallback FallbackConfig `yaml:"fallback,omitempty"`

	// Server settings
	Server ServerConfig `yaml:"server,omitempty"`

	// Debug settings
	Debug DebugConfig `yaml:"debug,omitempty"`

	// Rate limiting settings
	RateLimit RateLimitConfig `yaml:"rate_limit,omitempty"`

	// Cache settings
	Cache CacheConfig `yaml:"cache,omitempty"`

	// Prompt cache settings
	PromptCache PromptCacheConfig `yaml:"prompt_cache,omitempty"`

	// Authentication settings
	Auth AuthConfig `yaml:"auth,omitempty"`

	// Queue settings
	Queue QueueConfig `yaml:"queue,omitempty"`

	// Circuit breaker settings
	CircuitBreaker CircuitBreakerConfig `yaml:"circuit_breaker,omitempty"`

	// HTTP client settings
	HTTPClient HTTPClientConfig `yaml:"http_client,omitempty"`

	// Model aliasing
	Aliases map[string]string `yaml:"aliases,omitempty"`
}

// APIKeysConfig holds API keys for all providers.
type APIKeysConfig struct {
	OpenAI     string `yaml:"openai,omitempty"`
	Azure      string `yaml:"azure,omitempty"`
	OpenRouter string `yaml:"openrouter,omitempty"`
	Anthropic  string `yaml:"anthropic,omitempty"`
	Ollama     string `yaml:"ollama,omitempty"`
	Gemini     string `yaml:"gemini,omitempty"`
	DeepSeek   string `yaml:"deepseek,omitempty"`
	Grok       string `yaml:"grok,omitempty"`
	Qwen       string `yaml:"qwen,omitempty"`
	MiniMax    string `yaml:"minimax,omitempty"`
	Custom     string `yaml:"custom,omitempty"`
}

// EndpointsConfig holds endpoint URLs for all providers.
type EndpointsConfig struct {
	OpenAI       string `yaml:"openai,omitempty"`
	Azure        AzureEndpointConfig `yaml:"azure,omitempty"`
	OpenRouter   string `yaml:"openrouter,omitempty"`
	Ollama       string `yaml:"ollama,omitempty"`
	Gemini       string `yaml:"gemini,omitempty"`
	DeepSeek     string `yaml:"deepseek,omitempty"`
	Grok         string `yaml:"grok,omitempty"`
	Qwen         string `yaml:"qwen,omitempty"`
	MiniMax      string `yaml:"minimax,omitempty"`
	Custom       string `yaml:"custom,omitempty"`
}

// AzureEndpointConfig holds Azure-specific endpoint configuration.
type AzureEndpointConfig struct {
	Endpoint       string `yaml:"endpoint,omitempty"`
	DeploymentName string `yaml:"deployment_name,omitempty"`
	APIVersion     string `yaml:"api_version,omitempty"`
}

// ModelsConfig holds model mapping configuration.
type ModelsConfig struct {
	Default string `yaml:"default,omitempty"`
	Opus    string `yaml:"opus,omitempty"`
	Sonnet  string `yaml:"sonnet,omitempty"`
	Haiku   string `yaml:"haiku,omitempty"`
}

// TierFileConfig holds configuration for a specific model tier in config file.
type TierFileConfig struct {
	Provider string `yaml:"provider,omitempty"`
	Model    string `yaml:"model,omitempty"`
	APIKey   string `yaml:"api_key,omitempty"`
	BaseURL  string `yaml:"base_url,omitempty"`
	// Fallback configuration
	Fallback TierFallbackConfig `yaml:"fallback,omitempty"`
}

// TierFallbackConfig holds fallback configuration for a tier.
type TierFallbackConfig struct {
	Provider string `yaml:"provider,omitempty"`
	Model    string `yaml:"model,omitempty"`
	APIKey   string `yaml:"api_key,omitempty"`
	BaseURL  string `yaml:"base_url,omitempty"`
}

// MultiProviderConfig holds multi-provider routing configuration.
type MultiProviderConfig struct {
	Enabled bool            `yaml:"enabled,omitempty"`
	Opus    *TierFileConfig `yaml:"opus,omitempty"`
	Sonnet  *TierFileConfig `yaml:"sonnet,omitempty"`
	Haiku   *TierFileConfig `yaml:"haiku,omitempty"`
}

// FallbackConfig holds global fallback configuration.
type FallbackConfig struct {
	Enabled  bool   `yaml:"enabled,omitempty"`
	Provider string `yaml:"provider,omitempty"`
	Model    string `yaml:"model,omitempty"`
	APIKey   string `yaml:"api_key,omitempty"`
	BaseURL  string `yaml:"base_url,omitempty"`
}

// ServerConfig holds server settings.
type ServerConfig struct {
	Port     int    `yaml:"port,omitempty"`
	LogLevel string `yaml:"log_level,omitempty"`
}

// DebugConfig holds debug settings.
type DebugConfig struct {
	Enabled  bool `yaml:"enabled,omitempty"`
	Requests bool `yaml:"requests,omitempty"`
	Responses bool `yaml:"responses,omitempty"`
}

// RateLimitConfig holds rate limiting settings.
type RateLimitConfig struct {
	Enabled  bool `yaml:"enabled,omitempty"`
	Requests int  `yaml:"requests,omitempty"`
	Window   int  `yaml:"window,omitempty"`
	Burst    int  `yaml:"burst,omitempty"`
}

// CacheConfig holds cache settings.
type CacheConfig struct {
	Enabled bool `yaml:"enabled,omitempty"`
	MaxSize int  `yaml:"max_size,omitempty"`
	TTL     int  `yaml:"ttl,omitempty"`
}

// PromptCacheConfig holds prompt cache settings.
type PromptCacheConfig struct {
	Enabled bool `yaml:"enabled,omitempty"`
	MaxSize int  `yaml:"max_size,omitempty"`
}

// AuthConfig holds authentication settings.
type AuthConfig struct {
	Enabled               bool   `yaml:"enabled,omitempty"`
	APIKey                string `yaml:"api_key,omitempty"`
	AllowAnonymousHealth  *bool  `yaml:"allow_anonymous_health,omitempty"`
	AllowAnonymousMetrics *bool  `yaml:"allow_anonymous_metrics,omitempty"`
}

// QueueConfig holds queue settings.
type QueueConfig struct {
	Enabled        bool `yaml:"enabled,omitempty"`
	MaxSize        int  `yaml:"max_size,omitempty"`
	MaxWaitSeconds int  `yaml:"max_wait_seconds,omitempty"`
	RetryDelayMs   int  `yaml:"retry_delay_ms,omitempty"`
	MaxRetries     int  `yaml:"max_retries,omitempty"`
}

// CircuitBreakerConfig holds circuit breaker settings.
type CircuitBreakerConfig struct {
	Enabled    bool `yaml:"enabled,omitempty"`
	Threshold  int  `yaml:"threshold,omitempty"`
	Recovery   int  `yaml:"recovery,omitempty"`
	TimeoutSec int  `yaml:"timeout_sec,omitempty"`
}

// HTTPClientConfig holds HTTP client settings.
type HTTPClientConfig struct {
	TimeoutSec int `yaml:"timeout_sec,omitempty"`
}

// DefaultFileConfig returns a FileConfig with default values.
func DefaultFileConfig() *FileConfig {
	return &FileConfig{
		Provider: "openai",
		Endpoints: EndpointsConfig{
			OpenAI:     "https://api.openai.com/v1",
			OpenRouter: "https://openrouter.ai/api/v1",
			Ollama:     "http://localhost:11434",
			Gemini:     "https://generativelanguage.googleapis.com/v1beta",
			DeepSeek:   "https://api.deepseek.com",
			Azure: AzureEndpointConfig{
				APIVersion: "2024-02-15-preview",
			},
		},
		Server: ServerConfig{
			Port:     8080,
			LogLevel: "info",
		},
		Models: ModelsConfig{
			Default: "gpt-4o",
		},
		RateLimit: RateLimitConfig{
			Requests: 60,
			Window:   60,
			Burst:    10,
		},
		Cache: CacheConfig{
			MaxSize: 1000,
			TTL:     3600,
		},
		Auth: AuthConfig{
			AllowAnonymousHealth: boolPtr(true),
		},
		Queue: QueueConfig{
			MaxSize:        100,
			MaxWaitSeconds: 30,
			RetryDelayMs:   1000,
			MaxRetries:     3,
		},
		CircuitBreaker: CircuitBreakerConfig{
			Threshold:  5,
			Recovery:   2,
			TimeoutSec: 30,
		},
		HTTPClient: HTTPClientConfig{
			TimeoutSec: 300,
		},
		Aliases: make(map[string]string),
	}
}

// LoadFromFile loads configuration from a YAML file.
// The path can be specified via CLASP_CONFIG_FILE environment variable.
// If no path is specified, it looks for clasp.yaml in the current directory.
func LoadFromFile(path string) (*FileConfig, error) {
	// If no path specified, check environment variable
	if path == "" {
		path = os.Getenv("CLASP_CONFIG_FILE")
	}

	// If still no path, look for default locations
	if path == "" {
		// Check for config files in standard locations
		candidates := []string{
			"clasp.yaml",
			"clasp.yml",
			".clasp.yaml",
			".clasp.yml",
		}

		// Also check home directory
		homeDir, err := os.UserHomeDir()
		if err == nil {
			candidates = append(candidates,
				filepath.Join(homeDir, ".clasp", "config.yaml"),
				filepath.Join(homeDir, ".clasp", "config.yml"),
				filepath.Join(homeDir, ".config", "clasp", "config.yaml"),
				filepath.Join(homeDir, ".config", "clasp", "config.yml"),
			)
		}

		// Also check /etc for system-wide config
		candidates = append(candidates, "/etc/clasp/config.yaml", "/etc/clasp/config.yml")

		for _, candidate := range candidates {
			if _, err := os.Stat(candidate); err == nil {
				path = candidate
				break
			}
		}
	}

	// If no config file found, return nil (not an error - use env vars)
	if path == "" {
		return nil, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}

	cfg := DefaultFileConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", path, err)
	}

	// Expand environment variables in the config
	expandEnvVars(cfg)

	// Validate the configuration
	if err := ValidateFileConfig(cfg); err != nil {
		return nil, fmt.Errorf("config validation error: %w", err)
	}

	return cfg, nil
}

// expandEnvVars expands environment variables in the config.
// Supports ${VAR} and ${VAR:-default} syntax.
func expandEnvVars(cfg *FileConfig) {
	// Expand API keys
	cfg.APIKeys.OpenAI = expandString(cfg.APIKeys.OpenAI)
	cfg.APIKeys.Azure = expandString(cfg.APIKeys.Azure)
	cfg.APIKeys.OpenRouter = expandString(cfg.APIKeys.OpenRouter)
	cfg.APIKeys.Anthropic = expandString(cfg.APIKeys.Anthropic)
	cfg.APIKeys.Ollama = expandString(cfg.APIKeys.Ollama)
	cfg.APIKeys.Gemini = expandString(cfg.APIKeys.Gemini)
	cfg.APIKeys.DeepSeek = expandString(cfg.APIKeys.DeepSeek)
	cfg.APIKeys.Grok = expandString(cfg.APIKeys.Grok)
	cfg.APIKeys.Qwen = expandString(cfg.APIKeys.Qwen)
	cfg.APIKeys.MiniMax = expandString(cfg.APIKeys.MiniMax)
	cfg.APIKeys.Custom = expandString(cfg.APIKeys.Custom)

	// Expand endpoints
	cfg.Endpoints.OpenAI = expandString(cfg.Endpoints.OpenAI)
	cfg.Endpoints.OpenRouter = expandString(cfg.Endpoints.OpenRouter)
	cfg.Endpoints.Ollama = expandString(cfg.Endpoints.Ollama)
	cfg.Endpoints.Gemini = expandString(cfg.Endpoints.Gemini)
	cfg.Endpoints.DeepSeek = expandString(cfg.Endpoints.DeepSeek)
	cfg.Endpoints.Grok = expandString(cfg.Endpoints.Grok)
	cfg.Endpoints.Qwen = expandString(cfg.Endpoints.Qwen)
	cfg.Endpoints.MiniMax = expandString(cfg.Endpoints.MiniMax)
	cfg.Endpoints.Custom = expandString(cfg.Endpoints.Custom)
	cfg.Endpoints.Azure.Endpoint = expandString(cfg.Endpoints.Azure.Endpoint)
	cfg.Endpoints.Azure.DeploymentName = expandString(cfg.Endpoints.Azure.DeploymentName)
	cfg.Endpoints.Azure.APIVersion = expandString(cfg.Endpoints.Azure.APIVersion)

	// Expand auth API key
	cfg.Auth.APIKey = expandString(cfg.Auth.APIKey)

	// Expand multi-provider configs
	expandTierConfig(cfg.MultiProvider.Opus)
	expandTierConfig(cfg.MultiProvider.Sonnet)
	expandTierConfig(cfg.MultiProvider.Haiku)

	// Expand fallback config
	cfg.Fallback.APIKey = expandString(cfg.Fallback.APIKey)
	cfg.Fallback.BaseURL = expandString(cfg.Fallback.BaseURL)
}

// expandTierConfig expands environment variables in a tier config.
func expandTierConfig(tier *TierFileConfig) {
	if tier == nil {
		return
	}
	tier.APIKey = expandString(tier.APIKey)
	tier.BaseURL = expandString(tier.BaseURL)
	tier.Fallback.APIKey = expandString(tier.Fallback.APIKey)
	tier.Fallback.BaseURL = expandString(tier.Fallback.BaseURL)
}

// expandString expands environment variables in a string.
// Supports ${VAR} and ${VAR:-default} syntax.
func expandString(s string) string {
	if s == "" {
		return ""
	}

	// Handle ${VAR:-default} syntax
	for {
		startIdx := strings.Index(s, "${")
		if startIdx == -1 {
			break
		}

		endIdx := strings.Index(s[startIdx:], "}")
		if endIdx == -1 {
			break
		}
		endIdx += startIdx

		varContent := s[startIdx+2 : endIdx]

		// Check for default value syntax
		var varName, defaultValue string
		if sepIdx := strings.Index(varContent, ":-"); sepIdx != -1 {
			varName = varContent[:sepIdx]
			defaultValue = varContent[sepIdx+2:]
		} else {
			varName = varContent
		}

		// Get environment variable
		value := os.Getenv(varName)
		if value == "" {
			value = defaultValue
		}

		s = s[:startIdx] + value + s[endIdx+1:]
	}

	return s
}

// MergeWithEnv merges file config with environment variables.
// Environment variables take precedence over file config.
func MergeWithEnv(fileCfg *FileConfig, envCfg *Config) *Config {
	if fileCfg == nil {
		return envCfg
	}

	// Start with file config as base
	cfg := DefaultConfig()

	// Provider
	if fileCfg.Provider != "" {
		cfg.Provider = ProviderType(fileCfg.Provider)
	}

	// API Keys from file (will be overridden by env if set)
	cfg.OpenAIAPIKey = fileCfg.APIKeys.OpenAI
	cfg.AzureAPIKey = fileCfg.APIKeys.Azure
	cfg.OpenRouterAPIKey = fileCfg.APIKeys.OpenRouter
	cfg.AnthropicAPIKey = fileCfg.APIKeys.Anthropic
	cfg.OllamaAPIKey = fileCfg.APIKeys.Ollama
	cfg.GeminiAPIKey = fileCfg.APIKeys.Gemini
	cfg.DeepSeekAPIKey = fileCfg.APIKeys.DeepSeek
	cfg.GrokAPIKey = fileCfg.APIKeys.Grok
	cfg.QwenAPIKey = fileCfg.APIKeys.Qwen
	cfg.MiniMaxAPIKey = fileCfg.APIKeys.MiniMax
	cfg.CustomAPIKey = fileCfg.APIKeys.Custom

	// Endpoints from file
	if fileCfg.Endpoints.OpenAI != "" {
		cfg.OpenAIBaseURL = fileCfg.Endpoints.OpenAI
	}
	if fileCfg.Endpoints.Azure.Endpoint != "" {
		cfg.AzureEndpoint = fileCfg.Endpoints.Azure.Endpoint
	}
	if fileCfg.Endpoints.Azure.DeploymentName != "" {
		cfg.AzureDeploymentName = fileCfg.Endpoints.Azure.DeploymentName
	}
	if fileCfg.Endpoints.Azure.APIVersion != "" {
		cfg.AzureAPIVersion = fileCfg.Endpoints.Azure.APIVersion
	}
	if fileCfg.Endpoints.OpenRouter != "" {
		cfg.OpenRouterBaseURL = fileCfg.Endpoints.OpenRouter
	}
	if fileCfg.Endpoints.Ollama != "" {
		cfg.OllamaBaseURL = fileCfg.Endpoints.Ollama
	}
	if fileCfg.Endpoints.Gemini != "" {
		cfg.GeminiBaseURL = fileCfg.Endpoints.Gemini
	}
	if fileCfg.Endpoints.DeepSeek != "" {
		cfg.DeepSeekBaseURL = fileCfg.Endpoints.DeepSeek
	}
	if fileCfg.Endpoints.Grok != "" {
		cfg.GrokBaseURL = fileCfg.Endpoints.Grok
	}
	if fileCfg.Endpoints.Qwen != "" {
		cfg.QwenBaseURL = fileCfg.Endpoints.Qwen
	}
	if fileCfg.Endpoints.MiniMax != "" {
		cfg.MiniMaxBaseURL = fileCfg.Endpoints.MiniMax
	}
	cfg.CustomBaseURL = fileCfg.Endpoints.Custom

	// Models from file
	if fileCfg.Models.Default != "" {
		cfg.DefaultModel = fileCfg.Models.Default
	}
	cfg.ModelOpus = fileCfg.Models.Opus
	cfg.ModelSonnet = fileCfg.Models.Sonnet
	cfg.ModelHaiku = fileCfg.Models.Haiku

	// Multi-provider routing
	cfg.MultiProviderEnabled = fileCfg.MultiProvider.Enabled
	cfg.TierOpus = convertTierFileConfig(fileCfg.MultiProvider.Opus, cfg)
	cfg.TierSonnet = convertTierFileConfig(fileCfg.MultiProvider.Sonnet, cfg)
	cfg.TierHaiku = convertTierFileConfig(fileCfg.MultiProvider.Haiku, cfg)

	// Fallback routing
	cfg.FallbackEnabled = fileCfg.Fallback.Enabled
	if fileCfg.Fallback.Provider != "" {
		cfg.FallbackProvider = ProviderType(fileCfg.Fallback.Provider)
	}
	cfg.FallbackModel = fileCfg.Fallback.Model
	cfg.FallbackAPIKey = fileCfg.Fallback.APIKey
	cfg.FallbackBaseURL = fileCfg.Fallback.BaseURL

	// Server settings
	if fileCfg.Server.Port > 0 {
		cfg.Port = fileCfg.Server.Port
	}
	if fileCfg.Server.LogLevel != "" {
		cfg.LogLevel = fileCfg.Server.LogLevel
	}

	// Debug settings
	cfg.Debug = fileCfg.Debug.Enabled
	cfg.DebugRequests = fileCfg.Debug.Requests || cfg.Debug
	cfg.DebugResponses = fileCfg.Debug.Responses || cfg.Debug

	// Rate limiting
	cfg.RateLimitEnabled = fileCfg.RateLimit.Enabled
	if fileCfg.RateLimit.Requests > 0 {
		cfg.RateLimitRequests = fileCfg.RateLimit.Requests
	}
	if fileCfg.RateLimit.Window > 0 {
		cfg.RateLimitWindow = fileCfg.RateLimit.Window
	}
	if fileCfg.RateLimit.Burst > 0 {
		cfg.RateLimitBurst = fileCfg.RateLimit.Burst
	}

	// Cache
	cfg.CacheEnabled = fileCfg.Cache.Enabled
	if fileCfg.Cache.MaxSize > 0 {
		cfg.CacheMaxSize = fileCfg.Cache.MaxSize
	}
	if fileCfg.Cache.TTL > 0 {
		cfg.CacheTTL = fileCfg.Cache.TTL
	}

	// Prompt cache
	cfg.PromptCacheEnabled = fileCfg.PromptCache.Enabled
	if fileCfg.PromptCache.MaxSize > 0 {
		cfg.PromptCacheMaxSize = fileCfg.PromptCache.MaxSize
	}

	// Auth
	cfg.AuthEnabled = fileCfg.Auth.Enabled
	cfg.AuthAPIKey = fileCfg.Auth.APIKey
	if fileCfg.Auth.AllowAnonymousHealth != nil {
		cfg.AuthAllowAnonymousHealth = *fileCfg.Auth.AllowAnonymousHealth
	}
	if fileCfg.Auth.AllowAnonymousMetrics != nil {
		cfg.AuthAllowAnonymousMetrics = *fileCfg.Auth.AllowAnonymousMetrics
	}

	// Queue
	cfg.QueueEnabled = fileCfg.Queue.Enabled
	if fileCfg.Queue.MaxSize > 0 {
		cfg.QueueMaxSize = fileCfg.Queue.MaxSize
	}
	if fileCfg.Queue.MaxWaitSeconds > 0 {
		cfg.QueueMaxWaitSeconds = fileCfg.Queue.MaxWaitSeconds
	}
	if fileCfg.Queue.RetryDelayMs > 0 {
		cfg.QueueRetryDelayMs = fileCfg.Queue.RetryDelayMs
	}
	if fileCfg.Queue.MaxRetries > 0 {
		cfg.QueueMaxRetries = fileCfg.Queue.MaxRetries
	}

	// Circuit breaker
	cfg.CircuitBreakerEnabled = fileCfg.CircuitBreaker.Enabled
	if fileCfg.CircuitBreaker.Threshold > 0 {
		cfg.CircuitBreakerThreshold = fileCfg.CircuitBreaker.Threshold
	}
	if fileCfg.CircuitBreaker.Recovery > 0 {
		cfg.CircuitBreakerRecovery = fileCfg.CircuitBreaker.Recovery
	}
	if fileCfg.CircuitBreaker.TimeoutSec > 0 {
		cfg.CircuitBreakerTimeoutSec = fileCfg.CircuitBreaker.TimeoutSec
	}

	// HTTP client
	if fileCfg.HTTPClient.TimeoutSec > 0 {
		cfg.HTTPClientTimeoutSec = fileCfg.HTTPClient.TimeoutSec
	}

	// Model aliases
	cfg.ModelAliases = fileCfg.Aliases
	if cfg.ModelAliases == nil {
		cfg.ModelAliases = make(map[string]string)
	}

	// Now overlay environment variables (they take precedence)
	overlayEnvVars(cfg)

	// Auto-detect provider if not set
	if os.Getenv("PROVIDER") == "" && cfg.Provider == "" {
		cfg.Provider = detectProvider(cfg)
	}

	return cfg
}

// convertTierFileConfig converts a TierFileConfig to TierConfig.
func convertTierFileConfig(tier *TierFileConfig, cfg *Config) *TierConfig {
	if tier == nil || (tier.Provider == "" && tier.Model == "") {
		return nil
	}

	tc := &TierConfig{
		Provider: ProviderType(tier.Provider),
		Model:    tier.Model,
		APIKey:   tier.APIKey,
		BaseURL:  tier.BaseURL,
	}

	// Inherit API key if not specified
	if tc.APIKey == "" {
		switch tc.Provider {
		case ProviderOpenAI:
			tc.APIKey = cfg.OpenAIAPIKey
		case ProviderOpenRouter:
			tc.APIKey = cfg.OpenRouterAPIKey
		case ProviderAzure:
			tc.APIKey = cfg.AzureAPIKey
		case ProviderAnthropic:
			tc.APIKey = cfg.AnthropicAPIKey
		case ProviderOllama:
			tc.APIKey = cfg.OllamaAPIKey
		case ProviderGemini:
			tc.APIKey = cfg.GeminiAPIKey
		case ProviderDeepSeek:
			tc.APIKey = cfg.DeepSeekAPIKey
		case ProviderGrok:
			tc.APIKey = cfg.GrokAPIKey
		case ProviderQwen:
			tc.APIKey = cfg.QwenAPIKey
		case ProviderMiniMax:
			tc.APIKey = cfg.MiniMaxAPIKey
		case ProviderCustom:
			tc.APIKey = cfg.CustomAPIKey
		}
	}

	// Inherit base URL if not specified
	if tc.BaseURL == "" {
		switch tc.Provider {
		case ProviderOpenAI:
			tc.BaseURL = cfg.OpenAIBaseURL
		case ProviderOpenRouter:
			tc.BaseURL = cfg.OpenRouterBaseURL
		case ProviderOllama:
			tc.BaseURL = cfg.OllamaBaseURL + "/v1"
		case ProviderGemini:
			tc.BaseURL = cfg.GeminiBaseURL + "/openai"
		case ProviderDeepSeek:
			tc.BaseURL = cfg.DeepSeekBaseURL + "/v1"
		case ProviderGrok:
			tc.BaseURL = cfg.GrokBaseURL + "/v1"
		case ProviderQwen:
			tc.BaseURL = cfg.QwenBaseURL + "/v1"
		case ProviderMiniMax:
			tc.BaseURL = cfg.MiniMaxBaseURL + "/v1"
		case ProviderCustom:
			tc.BaseURL = cfg.CustomBaseURL
		}
	}

	// Handle fallback
	if tier.Fallback.Provider != "" {
		tc.FallbackProvider = ProviderType(tier.Fallback.Provider)
		tc.FallbackModel = tier.Fallback.Model
		tc.FallbackAPIKey = tier.Fallback.APIKey
		tc.FallbackBaseURL = tier.Fallback.BaseURL

		// Inherit fallback API key if not specified
		if tc.FallbackAPIKey == "" {
			switch tc.FallbackProvider {
			case ProviderOpenAI:
				tc.FallbackAPIKey = cfg.OpenAIAPIKey
			case ProviderOpenRouter:
				tc.FallbackAPIKey = cfg.OpenRouterAPIKey
			case ProviderAzure:
				tc.FallbackAPIKey = cfg.AzureAPIKey
			case ProviderAnthropic:
				tc.FallbackAPIKey = cfg.AnthropicAPIKey
			case ProviderOllama:
				tc.FallbackAPIKey = cfg.OllamaAPIKey
			case ProviderGemini:
				tc.FallbackAPIKey = cfg.GeminiAPIKey
			case ProviderDeepSeek:
				tc.FallbackAPIKey = cfg.DeepSeekAPIKey
			case ProviderGrok:
				tc.FallbackAPIKey = cfg.GrokAPIKey
			case ProviderQwen:
				tc.FallbackAPIKey = cfg.QwenAPIKey
			case ProviderMiniMax:
				tc.FallbackAPIKey = cfg.MiniMaxAPIKey
			case ProviderCustom:
				tc.FallbackAPIKey = cfg.CustomAPIKey
			}
		}
	}

	return tc
}

// overlayEnvVars overlays environment variables onto the config.
// Environment variables take precedence over file config.
func overlayEnvVars(cfg *Config) {
	// Provider
	if provider := os.Getenv("PROVIDER"); provider != "" {
		cfg.Provider = ProviderType(provider)
	}

	// API Keys
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		cfg.OpenAIAPIKey = key
	}
	if key := os.Getenv("AZURE_API_KEY"); key != "" {
		cfg.AzureAPIKey = key
	}
	if key := os.Getenv("OPENROUTER_API_KEY"); key != "" {
		cfg.OpenRouterAPIKey = key
	}
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		cfg.AnthropicAPIKey = key
	}
	if key := os.Getenv("OLLAMA_API_KEY"); key != "" {
		cfg.OllamaAPIKey = key
	}
	if key := os.Getenv("GEMINI_API_KEY"); key != "" {
		cfg.GeminiAPIKey = key
	}
	if key := os.Getenv("DEEPSEEK_API_KEY"); key != "" {
		cfg.DeepSeekAPIKey = key
	}
	if key := os.Getenv("GROK_API_KEY"); key != "" {
		cfg.GrokAPIKey = key
	}
	if key := os.Getenv("QWEN_API_KEY"); key != "" {
		cfg.QwenAPIKey = key
	}
	if key := os.Getenv("MINIMAX_API_KEY"); key != "" {
		cfg.MiniMaxAPIKey = key
	}
	if key := os.Getenv("CUSTOM_API_KEY"); key != "" {
		cfg.CustomAPIKey = key
	}

	// Endpoints
	if baseURL := os.Getenv("OPENAI_BASE_URL"); baseURL != "" {
		cfg.OpenAIBaseURL = baseURL
	}
	if endpoint := os.Getenv("AZURE_OPENAI_ENDPOINT"); endpoint != "" {
		cfg.AzureEndpoint = endpoint
	}
	if deployment := os.Getenv("AZURE_DEPLOYMENT_NAME"); deployment != "" {
		cfg.AzureDeploymentName = deployment
	}
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
	if baseURL := os.Getenv("GROK_BASE_URL"); baseURL != "" {
		cfg.GrokBaseURL = baseURL
	}
	if baseURL := os.Getenv("QWEN_BASE_URL"); baseURL != "" {
		cfg.QwenBaseURL = baseURL
	}
	if baseURL := os.Getenv("MINIMAX_BASE_URL"); baseURL != "" {
		cfg.MiniMaxBaseURL = baseURL
	}
	if baseURL := os.Getenv("CUSTOM_BASE_URL"); baseURL != "" {
		cfg.CustomBaseURL = baseURL
	}

	// Models
	if model := os.Getenv("CLASP_MODEL"); model != "" {
		cfg.DefaultModel = model
	}
	if model := os.Getenv("CLASP_MODEL_OPUS"); model != "" {
		cfg.ModelOpus = model
	}
	if model := os.Getenv("CLASP_MODEL_SONNET"); model != "" {
		cfg.ModelSonnet = model
	}
	if model := os.Getenv("CLASP_MODEL_HAIKU"); model != "" {
		cfg.ModelHaiku = model
	}

	// Server
	if port := os.Getenv("CLASP_PORT"); port != "" {
		if p, err := parseInt(port); err == nil {
			cfg.Port = p
		}
	}
	if logLevel := os.Getenv("CLASP_LOG_LEVEL"); logLevel != "" {
		cfg.LogLevel = logLevel
	}

	// Debug
	if os.Getenv("CLASP_DEBUG") == "true" || os.Getenv("CLASP_DEBUG") == "1" {
		cfg.Debug = true
	}
	if os.Getenv("CLASP_DEBUG_REQUESTS") == "true" {
		cfg.DebugRequests = true
	}
	if os.Getenv("CLASP_DEBUG_RESPONSES") == "true" {
		cfg.DebugResponses = true
	}

	// Rate limiting
	if os.Getenv("CLASP_RATE_LIMIT") == "true" || os.Getenv("CLASP_RATE_LIMIT") == "1" {
		cfg.RateLimitEnabled = true
	}
	if val := os.Getenv("CLASP_RATE_LIMIT_REQUESTS"); val != "" {
		if v, err := parseInt(val); err == nil {
			cfg.RateLimitRequests = v
		}
	}
	if val := os.Getenv("CLASP_RATE_LIMIT_WINDOW"); val != "" {
		if v, err := parseInt(val); err == nil {
			cfg.RateLimitWindow = v
		}
	}
	if val := os.Getenv("CLASP_RATE_LIMIT_BURST"); val != "" {
		if v, err := parseInt(val); err == nil {
			cfg.RateLimitBurst = v
		}
	}

	// Cache
	if os.Getenv("CLASP_CACHE") == "true" || os.Getenv("CLASP_CACHE") == "1" {
		cfg.CacheEnabled = true
	}
	if val := os.Getenv("CLASP_CACHE_MAX_SIZE"); val != "" {
		if v, err := parseInt(val); err == nil {
			cfg.CacheMaxSize = v
		}
	}
	if val := os.Getenv("CLASP_CACHE_TTL"); val != "" {
		if v, err := parseInt(val); err == nil {
			cfg.CacheTTL = v
		}
	}

	// Prompt cache
	if os.Getenv("CLASP_PROMPT_CACHE") == "true" || os.Getenv("CLASP_PROMPT_CACHE") == "1" {
		cfg.PromptCacheEnabled = true
	}
	if val := os.Getenv("CLASP_PROMPT_CACHE_MAX_SIZE"); val != "" {
		if v, err := parseInt(val); err == nil {
			cfg.PromptCacheMaxSize = v
		}
	}

	// Auth
	if os.Getenv("CLASP_AUTH") == "true" || os.Getenv("CLASP_AUTH") == "1" {
		cfg.AuthEnabled = true
	}
	if key := os.Getenv("CLASP_AUTH_API_KEY"); key != "" {
		cfg.AuthAPIKey = key
	}
	if os.Getenv("CLASP_AUTH_ALLOW_ANONYMOUS_HEALTH") == "false" || os.Getenv("CLASP_AUTH_ALLOW_ANONYMOUS_HEALTH") == "0" {
		cfg.AuthAllowAnonymousHealth = false
	}
	if os.Getenv("CLASP_AUTH_ALLOW_ANONYMOUS_METRICS") == "true" || os.Getenv("CLASP_AUTH_ALLOW_ANONYMOUS_METRICS") == "1" {
		cfg.AuthAllowAnonymousMetrics = true
	}

	// Queue
	if os.Getenv("CLASP_QUEUE") == "true" || os.Getenv("CLASP_QUEUE") == "1" {
		cfg.QueueEnabled = true
	}
	if val := os.Getenv("CLASP_QUEUE_MAX_SIZE"); val != "" {
		if v, err := parseInt(val); err == nil {
			cfg.QueueMaxSize = v
		}
	}
	if val := os.Getenv("CLASP_QUEUE_MAX_WAIT"); val != "" {
		if v, err := parseInt(val); err == nil {
			cfg.QueueMaxWaitSeconds = v
		}
	}
	if val := os.Getenv("CLASP_QUEUE_RETRY_DELAY"); val != "" {
		if v, err := parseInt(val); err == nil {
			cfg.QueueRetryDelayMs = v
		}
	}
	if val := os.Getenv("CLASP_QUEUE_MAX_RETRIES"); val != "" {
		if v, err := parseInt(val); err == nil {
			cfg.QueueMaxRetries = v
		}
	}

	// Circuit breaker
	if os.Getenv("CLASP_CIRCUIT_BREAKER") == "true" || os.Getenv("CLASP_CIRCUIT_BREAKER") == "1" {
		cfg.CircuitBreakerEnabled = true
	}
	if val := os.Getenv("CLASP_CIRCUIT_BREAKER_THRESHOLD"); val != "" {
		if v, err := parseInt(val); err == nil {
			cfg.CircuitBreakerThreshold = v
		}
	}
	if val := os.Getenv("CLASP_CIRCUIT_BREAKER_RECOVERY"); val != "" {
		if v, err := parseInt(val); err == nil {
			cfg.CircuitBreakerRecovery = v
		}
	}
	if val := os.Getenv("CLASP_CIRCUIT_BREAKER_TIMEOUT"); val != "" {
		if v, err := parseInt(val); err == nil {
			cfg.CircuitBreakerTimeoutSec = v
		}
	}

	// HTTP client
	if val := os.Getenv("CLASP_HTTP_TIMEOUT"); val != "" {
		if v, err := parseInt(val); err == nil {
			cfg.HTTPClientTimeoutSec = v
		}
	}

	// Multi-provider
	if os.Getenv("CLASP_MULTI_PROVIDER") == "true" || os.Getenv("CLASP_MULTI_PROVIDER") == "1" {
		cfg.MultiProviderEnabled = true
	}

	// Fallback
	if os.Getenv("CLASP_FALLBACK") == "true" || os.Getenv("CLASP_FALLBACK") == "1" {
		cfg.FallbackEnabled = true
	}
	if provider := os.Getenv("CLASP_FALLBACK_PROVIDER"); provider != "" {
		cfg.FallbackProvider = ProviderType(provider)
	}
	if model := os.Getenv("CLASP_FALLBACK_MODEL"); model != "" {
		cfg.FallbackModel = model
	}
	if key := os.Getenv("CLASP_FALLBACK_API_KEY"); key != "" {
		cfg.FallbackAPIKey = key
	}
	if baseURL := os.Getenv("CLASP_FALLBACK_BASE_URL"); baseURL != "" {
		cfg.FallbackBaseURL = baseURL
	}

	// Model aliases from env
	envAliases := loadModelAliases()
	for k, v := range envAliases {
		cfg.ModelAliases[k] = v
	}
}

// parseInt is a helper to parse integers.
func parseInt(s string) (int, error) {
	var result int
	_, err := fmt.Sscanf(s, "%d", &result)
	return result, err
}

// boolPtr returns a pointer to a bool.
func boolPtr(b bool) *bool {
	return &b
}

// LoadWithFile loads configuration from both file and environment variables.
// This is the main entry point for loading configuration.
// Precedence (highest to lowest): CLI flags, env vars, config file, defaults.
func LoadWithFile() (*Config, error) {
	// Load from file first (if exists)
	fileCfg, err := LoadFromFile("")
	if err != nil {
		return nil, err
	}

	// If no file config, just use env vars
	if fileCfg == nil {
		return LoadFromEnv()
	}

	// Create a base config from file and overlay env vars
	cfg := MergeWithEnv(fileCfg, DefaultConfig())

	// Validate
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}
