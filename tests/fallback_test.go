package tests

import (
	"os"
	"testing"

	"github.com/jedarden/clasp/internal/config"
)

func TestFallbackConfig_Disabled(t *testing.T) {
	// Clear any existing env vars
	os.Unsetenv("CLASP_FALLBACK")
	os.Unsetenv("CLASP_FALLBACK_PROVIDER")
	os.Unsetenv("CLASP_FALLBACK_MODEL")

	// Set minimal config
	os.Setenv("OPENAI_API_KEY", "test-key")
	defer os.Unsetenv("OPENAI_API_KEY")

	cfg, err := config.LoadFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.FallbackEnabled {
		t.Error("Expected fallback to be disabled by default")
	}

	if cfg.HasGlobalFallback() {
		t.Error("Expected HasGlobalFallback to return false when disabled")
	}
}

func TestFallbackConfig_GlobalEnabled(t *testing.T) {
	// Set up global fallback config
	os.Setenv("OPENAI_API_KEY", "test-openai-key")
	os.Setenv("OPENROUTER_API_KEY", "test-openrouter-key")
	os.Setenv("CLASP_FALLBACK", "true")
	os.Setenv("CLASP_FALLBACK_PROVIDER", "openrouter")
	os.Setenv("CLASP_FALLBACK_MODEL", "openai/gpt-4o")

	defer func() {
		os.Unsetenv("OPENAI_API_KEY")
		os.Unsetenv("OPENROUTER_API_KEY")
		os.Unsetenv("CLASP_FALLBACK")
		os.Unsetenv("CLASP_FALLBACK_PROVIDER")
		os.Unsetenv("CLASP_FALLBACK_MODEL")
	}()

	cfg, err := config.LoadFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !cfg.FallbackEnabled {
		t.Error("Expected fallback to be enabled")
	}

	if !cfg.HasGlobalFallback() {
		t.Error("Expected HasGlobalFallback to return true")
	}

	if cfg.FallbackProvider != config.ProviderOpenRouter {
		t.Errorf("Expected fallback provider 'openrouter', got '%s'", cfg.FallbackProvider)
	}

	if cfg.FallbackModel != "openai/gpt-4o" {
		t.Errorf("Expected fallback model 'openai/gpt-4o', got '%s'", cfg.FallbackModel)
	}

	// API key should be inherited
	if cfg.FallbackAPIKey != "test-openrouter-key" {
		t.Errorf("Expected fallback API key to be inherited, got '%s'", cfg.FallbackAPIKey)
	}
}

func TestFallbackConfig_GlobalFallbackConfig(t *testing.T) {
	os.Setenv("OPENAI_API_KEY", "test-openai-key")
	os.Setenv("OPENROUTER_API_KEY", "test-openrouter-key")
	os.Setenv("CLASP_FALLBACK", "true")
	os.Setenv("CLASP_FALLBACK_PROVIDER", "openrouter")
	os.Setenv("CLASP_FALLBACK_MODEL", "openai/gpt-4o")

	defer func() {
		os.Unsetenv("OPENAI_API_KEY")
		os.Unsetenv("OPENROUTER_API_KEY")
		os.Unsetenv("CLASP_FALLBACK")
		os.Unsetenv("CLASP_FALLBACK_PROVIDER")
		os.Unsetenv("CLASP_FALLBACK_MODEL")
	}()

	cfg, err := config.LoadFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	fbConfig := cfg.GetGlobalFallbackConfig()
	if fbConfig == nil {
		t.Fatal("Expected GetGlobalFallbackConfig to return non-nil config")
	}

	if fbConfig.Provider != config.ProviderOpenRouter {
		t.Errorf("Expected fallback provider 'openrouter', got '%s'", fbConfig.Provider)
	}

	if fbConfig.Model != "openai/gpt-4o" {
		t.Errorf("Expected fallback model 'openai/gpt-4o', got '%s'", fbConfig.Model)
	}

	// Base URL should be set to default for OpenRouter
	expectedBaseURL := "https://openrouter.ai/api/v1"
	if fbConfig.BaseURL != expectedBaseURL {
		t.Errorf("Expected fallback base URL '%s', got '%s'", expectedBaseURL, fbConfig.BaseURL)
	}
}

func TestFallbackConfig_TierSpecificFallback(t *testing.T) {
	// Set up tier-specific fallback
	os.Setenv("OPENAI_API_KEY", "test-openai-key")
	os.Setenv("OPENROUTER_API_KEY", "test-openrouter-key")
	os.Setenv("CLASP_MULTI_PROVIDER", "true")
	os.Setenv("CLASP_OPUS_PROVIDER", "openai")
	os.Setenv("CLASP_OPUS_MODEL", "gpt-4o")
	os.Setenv("CLASP_OPUS_FALLBACK_PROVIDER", "openrouter")
	os.Setenv("CLASP_OPUS_FALLBACK_MODEL", "openai/gpt-4-turbo")

	defer func() {
		os.Unsetenv("OPENAI_API_KEY")
		os.Unsetenv("OPENROUTER_API_KEY")
		os.Unsetenv("CLASP_MULTI_PROVIDER")
		os.Unsetenv("CLASP_OPUS_PROVIDER")
		os.Unsetenv("CLASP_OPUS_MODEL")
		os.Unsetenv("CLASP_OPUS_FALLBACK_PROVIDER")
		os.Unsetenv("CLASP_OPUS_FALLBACK_MODEL")
	}()

	cfg, err := config.LoadFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	opusTier := cfg.GetTierConfig("claude-3-opus-20240229")
	if opusTier == nil {
		t.Fatal("Expected Opus tier config to exist")
	}

	if !opusTier.HasFallback() {
		t.Error("Expected Opus tier to have fallback configured")
	}

	if opusTier.FallbackProvider != config.ProviderOpenRouter {
		t.Errorf("Expected Opus fallback provider 'openrouter', got '%s'", opusTier.FallbackProvider)
	}

	if opusTier.FallbackModel != "openai/gpt-4-turbo" {
		t.Errorf("Expected Opus fallback model 'openai/gpt-4-turbo', got '%s'", opusTier.FallbackModel)
	}

	// Fallback API key should be inherited
	if opusTier.FallbackAPIKey != "test-openrouter-key" {
		t.Errorf("Expected Opus fallback API key to be inherited, got '%s'", opusTier.FallbackAPIKey)
	}
}

func TestFallbackConfig_TierGetFallbackConfig(t *testing.T) {
	os.Setenv("OPENAI_API_KEY", "test-openai-key")
	os.Setenv("OPENROUTER_API_KEY", "test-openrouter-key")
	os.Setenv("CLASP_MULTI_PROVIDER", "true")
	os.Setenv("CLASP_OPUS_PROVIDER", "openai")
	os.Setenv("CLASP_OPUS_MODEL", "gpt-4o")
	os.Setenv("CLASP_OPUS_FALLBACK_PROVIDER", "openrouter")
	os.Setenv("CLASP_OPUS_FALLBACK_MODEL", "openai/gpt-4-turbo")

	defer func() {
		os.Unsetenv("OPENAI_API_KEY")
		os.Unsetenv("OPENROUTER_API_KEY")
		os.Unsetenv("CLASP_MULTI_PROVIDER")
		os.Unsetenv("CLASP_OPUS_PROVIDER")
		os.Unsetenv("CLASP_OPUS_MODEL")
		os.Unsetenv("CLASP_OPUS_FALLBACK_PROVIDER")
		os.Unsetenv("CLASP_OPUS_FALLBACK_MODEL")
	}()

	cfg, err := config.LoadFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	opusTier := cfg.GetTierConfig("claude-3-opus-20240229")
	if opusTier == nil {
		t.Fatal("Expected Opus tier config to exist")
	}

	fbConfig := opusTier.GetFallbackConfig()
	if fbConfig == nil {
		t.Fatal("Expected GetFallbackConfig to return non-nil config")
	}

	if fbConfig.Provider != config.ProviderOpenRouter {
		t.Errorf("Expected fallback provider 'openrouter', got '%s'", fbConfig.Provider)
	}

	if fbConfig.Model != "openai/gpt-4-turbo" {
		t.Errorf("Expected fallback model 'openai/gpt-4-turbo', got '%s'", fbConfig.Model)
	}
}

func TestFallbackConfig_NoFallback(t *testing.T) {
	os.Setenv("OPENAI_API_KEY", "test-key")
	os.Setenv("CLASP_MULTI_PROVIDER", "true")
	os.Setenv("CLASP_OPUS_PROVIDER", "openai")
	os.Setenv("CLASP_OPUS_MODEL", "gpt-4o")
	// No fallback configured

	defer func() {
		os.Unsetenv("OPENAI_API_KEY")
		os.Unsetenv("CLASP_MULTI_PROVIDER")
		os.Unsetenv("CLASP_OPUS_PROVIDER")
		os.Unsetenv("CLASP_OPUS_MODEL")
	}()

	cfg, err := config.LoadFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	opusTier := cfg.GetTierConfig("claude-3-opus-20240229")
	if opusTier == nil {
		t.Fatal("Expected Opus tier config to exist")
	}

	if opusTier.HasFallback() {
		t.Error("Expected Opus tier to NOT have fallback configured")
	}

	fbConfig := opusTier.GetFallbackConfig()
	if fbConfig != nil {
		t.Error("Expected GetFallbackConfig to return nil when no fallback configured")
	}
}

func TestFallbackConfig_CustomAPIKey(t *testing.T) {
	// Test that explicit fallback API key takes precedence
	os.Setenv("OPENAI_API_KEY", "test-openai-key")
	os.Setenv("OPENROUTER_API_KEY", "test-openrouter-key")
	os.Setenv("CLASP_FALLBACK", "true")
	os.Setenv("CLASP_FALLBACK_PROVIDER", "openrouter")
	os.Setenv("CLASP_FALLBACK_MODEL", "openai/gpt-4o")
	os.Setenv("CLASP_FALLBACK_API_KEY", "custom-fallback-key")

	defer func() {
		os.Unsetenv("OPENAI_API_KEY")
		os.Unsetenv("OPENROUTER_API_KEY")
		os.Unsetenv("CLASP_FALLBACK")
		os.Unsetenv("CLASP_FALLBACK_PROVIDER")
		os.Unsetenv("CLASP_FALLBACK_MODEL")
		os.Unsetenv("CLASP_FALLBACK_API_KEY")
	}()

	cfg, err := config.LoadFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Explicit API key should be used
	if cfg.FallbackAPIKey != "custom-fallback-key" {
		t.Errorf("Expected custom fallback API key, got '%s'", cfg.FallbackAPIKey)
	}
}

func TestFallbackConfig_CustomBaseURL(t *testing.T) {
	// Test custom base URL for fallback
	os.Setenv("OPENAI_API_KEY", "test-openai-key")
	os.Setenv("CLASP_FALLBACK", "true")
	os.Setenv("CLASP_FALLBACK_PROVIDER", "custom")
	os.Setenv("CLASP_FALLBACK_MODEL", "llama3.1")
	os.Setenv("CLASP_FALLBACK_BASE_URL", "http://localhost:11434/v1")

	defer func() {
		os.Unsetenv("OPENAI_API_KEY")
		os.Unsetenv("CLASP_FALLBACK")
		os.Unsetenv("CLASP_FALLBACK_PROVIDER")
		os.Unsetenv("CLASP_FALLBACK_MODEL")
		os.Unsetenv("CLASP_FALLBACK_BASE_URL")
	}()

	cfg, err := config.LoadFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	fbConfig := cfg.GetGlobalFallbackConfig()
	if fbConfig == nil {
		t.Fatal("Expected GetGlobalFallbackConfig to return non-nil config")
	}

	if fbConfig.BaseURL != "http://localhost:11434/v1" {
		t.Errorf("Expected custom fallback base URL, got '%s'", fbConfig.BaseURL)
	}
}

func TestFallbackConfig_NilTierConfig(t *testing.T) {
	// Test that HasFallback returns false for nil TierConfig
	var nilTier *config.TierConfig = nil
	if nilTier.HasFallback() {
		t.Error("Expected HasFallback to return false for nil TierConfig")
	}

	fbConfig := nilTier.GetFallbackConfig()
	if fbConfig != nil {
		t.Error("Expected GetFallbackConfig to return nil for nil TierConfig")
	}
}
