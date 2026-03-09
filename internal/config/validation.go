// Package config provides configuration validation for CLASP.
package config

import (
	"fmt"
	"strings"
)

// ValidateFileConfig validates a FileConfig structure.
func ValidateFileConfig(cfg *FileConfig) error {
	var errors []string

	// Validate provider
	if cfg.Provider != "" {
		if err := validateProvider(cfg.Provider); err != nil {
			errors = append(errors, err.Error())
		}
	}

	// Validate server settings
	if err := validateServerConfig(&cfg.Server); err != nil {
		errors = append(errors, err.Error())
	}

	// Validate rate limit settings
	if err := validateRateLimitConfig(&cfg.RateLimit); err != nil {
		errors = append(errors, err.Error())
	}

	// Validate cache settings
	if err := validateCacheConfig(&cfg.Cache); err != nil {
		errors = append(errors, err.Error())
	}

	// Validate queue settings
	if err := validateQueueConfig(&cfg.Queue); err != nil {
		errors = append(errors, err.Error())
	}

	// Validate circuit breaker settings
	if err := validateCircuitBreakerConfig(&cfg.CircuitBreaker); err != nil {
		errors = append(errors, err.Error())
	}

	// Validate HTTP client settings
	if err := validateHTTPClientConfig(&cfg.HTTPClient); err != nil {
		errors = append(errors, err.Error())
	}

	// Validate multi-provider settings
	if err := validateMultiProviderConfig(&cfg.MultiProvider); err != nil {
		errors = append(errors, err.Error())
	}

	// Validate fallback settings
	if err := validateFallbackConfig(&cfg.Fallback); err != nil {
		errors = append(errors, err.Error())
	}

	// Validate auth settings
	if err := validateAuthConfig(&cfg.Auth); err != nil {
		errors = append(errors, err.Error())
	}

	// Validate tier configs
	if cfg.MultiProvider.Opus != nil {
		if err := validateTierFileConfig(cfg.MultiProvider.Opus, "opus"); err != nil {
			errors = append(errors, err.Error())
		}
	}
	if cfg.MultiProvider.Sonnet != nil {
		if err := validateTierFileConfig(cfg.MultiProvider.Sonnet, "sonnet"); err != nil {
			errors = append(errors, err.Error())
		}
	}
	if cfg.MultiProvider.Haiku != nil {
		if err := validateTierFileConfig(cfg.MultiProvider.Haiku, "haiku"); err != nil {
			errors = append(errors, err.Error())
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("validation errors:\n  - %s", strings.Join(errors, "\n  - "))
	}

	return nil
}

// validateProvider validates a provider string.
func validateProvider(provider string) error {
	validProviders := map[string]bool{
		"openai":     true,
		"azure":      true,
		"openrouter": true,
		"anthropic":  true,
		"ollama":     true,
		"gemini":     true,
		"deepseek":   true,
		"grok":       true,
		"qwen":       true,
		"minimax":    true,
		"custom":     true,
	}

	if !validProviders[strings.ToLower(provider)] {
		validList := make([]string, 0, len(validProviders))
		for p := range validProviders {
			validList = append(validList, p)
		}
		return fmt.Errorf("invalid provider '%s', must be one of: %s", provider, strings.Join(validList, ", "))
	}

	return nil
}

// validateServerConfig validates server configuration.
func validateServerConfig(cfg *ServerConfig) error {
	if cfg.Port < 0 || cfg.Port > 65535 {
		return fmt.Errorf("server.port must be between 0 and 65535, got %d", cfg.Port)
	}

	validLogLevels := map[string]bool{
		"":        true,
		"debug":   true,
		"info":    true,
		"warn":    true,
		"error":   true,
		"warning": true,
	}

	if !validLogLevels[strings.ToLower(cfg.LogLevel)] {
		return fmt.Errorf("server.log_level must be one of: debug, info, warn, error, got '%s'", cfg.LogLevel)
	}

	return nil
}

// validateRateLimitConfig validates rate limiting configuration.
func validateRateLimitConfig(cfg *RateLimitConfig) error {
	if cfg.Requests < 0 {
		return fmt.Errorf("rate_limit.requests must be non-negative, got %d", cfg.Requests)
	}
	if cfg.Window < 0 {
		return fmt.Errorf("rate_limit.window must be non-negative, got %d", cfg.Window)
	}
	if cfg.Burst < 0 {
		return fmt.Errorf("rate_limit.burst must be non-negative, got %d", cfg.Burst)
	}
	return nil
}

// validateCacheConfig validates cache configuration.
func validateCacheConfig(cfg *CacheConfig) error {
	if cfg.MaxSize < 0 {
		return fmt.Errorf("cache.max_size must be non-negative, got %d", cfg.MaxSize)
	}
	if cfg.TTL < 0 {
		return fmt.Errorf("cache.ttl must be non-negative, got %d", cfg.TTL)
	}
	return nil
}

// validateQueueConfig validates queue configuration.
func validateQueueConfig(cfg *QueueConfig) error {
	if cfg.MaxSize < 0 {
		return fmt.Errorf("queue.max_size must be non-negative, got %d", cfg.MaxSize)
	}
	if cfg.MaxWaitSeconds < 0 {
		return fmt.Errorf("queue.max_wait_seconds must be non-negative, got %d", cfg.MaxWaitSeconds)
	}
	if cfg.RetryDelayMs < 0 {
		return fmt.Errorf("queue.retry_delay_ms must be non-negative, got %d", cfg.RetryDelayMs)
	}
	if cfg.MaxRetries < 0 {
		return fmt.Errorf("queue.max_retries must be non-negative, got %d", cfg.MaxRetries)
	}
	return nil
}

// validateCircuitBreakerConfig validates circuit breaker configuration.
func validateCircuitBreakerConfig(cfg *CircuitBreakerConfig) error {
	if cfg.Threshold < 0 {
		return fmt.Errorf("circuit_breaker.threshold must be non-negative, got %d", cfg.Threshold)
	}
	if cfg.Recovery < 0 {
		return fmt.Errorf("circuit_breaker.recovery must be non-negative, got %d", cfg.Recovery)
	}
	if cfg.TimeoutSec < 0 {
		return fmt.Errorf("circuit_breaker.timeout_sec must be non-negative, got %d", cfg.TimeoutSec)
	}
	return nil
}

// validateHTTPClientConfig validates HTTP client configuration.
func validateHTTPClientConfig(cfg *HTTPClientConfig) error {
	if cfg.TimeoutSec < 0 {
		return fmt.Errorf("http_client.timeout_sec must be non-negative, got %d", cfg.TimeoutSec)
	}
	return nil
}

// validateMultiProviderConfig validates multi-provider configuration.
func validateMultiProviderConfig(cfg *MultiProviderConfig) error {
	// If multi-provider is disabled, no need to validate tier configs
	if !cfg.Enabled {
		return nil
	}

	// At least one tier should be configured if multi-provider is enabled
	if cfg.Opus == nil && cfg.Sonnet == nil && cfg.Haiku == nil {
		return fmt.Errorf("multi_provider is enabled but no tiers are configured")
	}

	return nil
}

// validateFallbackConfig validates fallback configuration.
func validateFallbackConfig(cfg *FallbackConfig) error {
	// If fallback is disabled, no need to validate
	if !cfg.Enabled {
		return nil
	}

	// Provider is required if fallback is enabled
	if cfg.Provider == "" {
		return fmt.Errorf("fallback.provider is required when fallback is enabled")
	}

	// Validate provider
	if err := validateProvider(cfg.Provider); err != nil {
		return fmt.Errorf("fallback.provider: %w", err)
	}

	return nil
}

// validateAuthConfig validates authentication configuration.
func validateAuthConfig(cfg *AuthConfig) error {
	// If auth is disabled, no need to validate
	if !cfg.Enabled {
		return nil
	}

	// API key is required if auth is enabled
	if cfg.APIKey == "" {
		return fmt.Errorf("auth.api_key is required when auth is enabled")
	}

	return nil
}

// validateTierFileConfig validates a tier configuration.
func validateTierFileConfig(cfg *TierFileConfig, tierName string) error {
	if cfg.Provider == "" && cfg.Model == "" {
		return fmt.Errorf("multi_provider.%s: at least provider or model must be specified", tierName)
	}

	// Validate provider if specified
	if cfg.Provider != "" {
		if err := validateProvider(cfg.Provider); err != nil {
			return fmt.Errorf("multi_provider.%s.provider: %w", tierName, err)
		}
	}

	// Validate fallback if configured
	if cfg.Fallback.Provider != "" {
		if err := validateProvider(cfg.Fallback.Provider); err != nil {
			return fmt.Errorf("multi_provider.%s.fallback.provider: %w", tierName, err)
		}
	}

	return nil
}

// ValidateConfigFile validates a YAML configuration file at the given path.
func ValidateConfigFile(path string) error {
	cfg, err := LoadFromFile(path)
	if err != nil {
		return err
	}

	if cfg == nil {
		return fmt.Errorf("no configuration file found at %s", path)
	}

	return ValidateFileConfig(cfg)
}
