package tests

import (
	"os"
	"testing"

	"github.com/jedarden/clasp/internal/config"
)

func TestMultiProviderConfig_Disabled(t *testing.T) {
	// Clear any existing env vars
	os.Unsetenv("CLASP_MULTI_PROVIDER")
	os.Unsetenv("CLASP_OPUS_PROVIDER")
	os.Unsetenv("CLASP_SONNET_PROVIDER")
	os.Unsetenv("CLASP_HAIKU_PROVIDER")

	// Set minimal config
	os.Setenv("OPENAI_API_KEY", "test-key")
	defer os.Unsetenv("OPENAI_API_KEY")

	cfg, err := config.LoadFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.MultiProviderEnabled {
		t.Error("Expected multi-provider to be disabled by default")
	}

	// GetTierConfig should return nil when disabled
	tier := cfg.GetTierConfig("claude-3-opus-20240229")
	if tier != nil {
		t.Error("Expected GetTierConfig to return nil when multi-provider is disabled")
	}
}

func TestMultiProviderConfig_Enabled(t *testing.T) {
	// Set up multi-provider config
	os.Setenv("OPENAI_API_KEY", "test-openai-key")
	os.Setenv("OPENROUTER_API_KEY", "test-openrouter-key")
	os.Setenv("CLASP_MULTI_PROVIDER", "true")
	os.Setenv("CLASP_OPUS_PROVIDER", "openai")
	os.Setenv("CLASP_OPUS_MODEL", "gpt-4o")
	os.Setenv("CLASP_SONNET_PROVIDER", "openrouter")
	os.Setenv("CLASP_SONNET_MODEL", "anthropic/claude-3-sonnet")
	os.Setenv("CLASP_HAIKU_PROVIDER", "custom")
	os.Setenv("CLASP_HAIKU_MODEL", "llama3.1")
	os.Setenv("CLASP_HAIKU_BASE_URL", "http://localhost:11434/v1")

	defer func() {
		os.Unsetenv("OPENAI_API_KEY")
		os.Unsetenv("OPENROUTER_API_KEY")
		os.Unsetenv("CLASP_MULTI_PROVIDER")
		os.Unsetenv("CLASP_OPUS_PROVIDER")
		os.Unsetenv("CLASP_OPUS_MODEL")
		os.Unsetenv("CLASP_SONNET_PROVIDER")
		os.Unsetenv("CLASP_SONNET_MODEL")
		os.Unsetenv("CLASP_HAIKU_PROVIDER")
		os.Unsetenv("CLASP_HAIKU_MODEL")
		os.Unsetenv("CLASP_HAIKU_BASE_URL")
	}()

	cfg, err := config.LoadFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !cfg.MultiProviderEnabled {
		t.Error("Expected multi-provider to be enabled")
	}

	// Test Opus tier config
	opusTier := cfg.GetTierConfig("claude-3-opus-20240229")
	if opusTier == nil {
		t.Fatal("Expected Opus tier config to exist")
	}
	if opusTier.Provider != config.ProviderOpenAI {
		t.Errorf("Expected Opus provider 'openai', got '%s'", opusTier.Provider)
	}
	if opusTier.Model != "gpt-4o" {
		t.Errorf("Expected Opus model 'gpt-4o', got '%s'", opusTier.Model)
	}
	if opusTier.APIKey != "test-openai-key" {
		t.Error("Expected Opus API key to be inherited from main config")
	}

	// Test Sonnet tier config
	sonnetTier := cfg.GetTierConfig("claude-3-5-sonnet-20241022")
	if sonnetTier == nil {
		t.Fatal("Expected Sonnet tier config to exist")
	}
	if sonnetTier.Provider != config.ProviderOpenRouter {
		t.Errorf("Expected Sonnet provider 'openrouter', got '%s'", sonnetTier.Provider)
	}
	if sonnetTier.Model != "anthropic/claude-3-sonnet" {
		t.Errorf("Expected Sonnet model 'anthropic/claude-3-sonnet', got '%s'", sonnetTier.Model)
	}

	// Test Haiku tier config
	haikuTier := cfg.GetTierConfig("claude-3-haiku-20240307")
	if haikuTier == nil {
		t.Fatal("Expected Haiku tier config to exist")
	}
	if haikuTier.Provider != config.ProviderCustom {
		t.Errorf("Expected Haiku provider 'custom', got '%s'", haikuTier.Provider)
	}
	if haikuTier.Model != "llama3.1" {
		t.Errorf("Expected Haiku model 'llama3.1', got '%s'", haikuTier.Model)
	}
	if haikuTier.BaseURL != "http://localhost:11434/v1" {
		t.Errorf("Expected Haiku base URL 'http://localhost:11434/v1', got '%s'", haikuTier.BaseURL)
	}
}

func TestGetModelTier(t *testing.T) {
	tests := []struct {
		model    string
		expected config.ModelTier
	}{
		{"claude-3-opus-20240229", config.TierOpus},
		{"claude-4-opus-20250101", config.TierOpus},
		{"claude-3-5-sonnet-20241022", config.TierSonnet},
		{"claude-3-sonnet-20240229", config.TierSonnet},
		{"claude-3-haiku-20240307", config.TierHaiku},
		{"claude-3-5-haiku-20241022", config.TierHaiku},
		{"unknown-model", config.TierSonnet}, // Default to sonnet
		{"gpt-4o", config.TierSonnet},        // Default to sonnet for non-Claude models
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			tier := config.GetModelTier(tt.model)
			if tier != tt.expected {
				t.Errorf("GetModelTier(%s) = %s, expected %s", tt.model, tier, tt.expected)
			}
		})
	}
}

func TestMultiProviderConfig_PartialTiers(t *testing.T) {
	// Set up partial config (only opus tier)
	os.Setenv("OPENAI_API_KEY", "test-key")
	os.Setenv("CLASP_MULTI_PROVIDER", "true")
	os.Setenv("CLASP_OPUS_PROVIDER", "openai")
	os.Setenv("CLASP_OPUS_MODEL", "gpt-4o")

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

	// Opus should exist
	opusTier := cfg.GetTierConfig("claude-3-opus-20240229")
	if opusTier == nil {
		t.Fatal("Expected Opus tier config to exist")
	}

	// Sonnet and Haiku should be nil
	sonnetTier := cfg.GetTierConfig("claude-3-5-sonnet-20241022")
	if sonnetTier != nil {
		t.Error("Expected Sonnet tier config to be nil when not configured")
	}

	haikuTier := cfg.GetTierConfig("claude-3-haiku-20240307")
	if haikuTier != nil {
		t.Error("Expected Haiku tier config to be nil when not configured")
	}
}

func TestMultiProviderConfig_TierSpecificAPIKey(t *testing.T) {
	// Set up with tier-specific API key
	os.Setenv("OPENAI_API_KEY", "main-openai-key")
	os.Setenv("CLASP_MULTI_PROVIDER", "true")
	os.Setenv("CLASP_OPUS_PROVIDER", "openai")
	os.Setenv("CLASP_OPUS_MODEL", "gpt-4o")
	os.Setenv("CLASP_OPUS_API_KEY", "tier-specific-key")

	defer func() {
		os.Unsetenv("OPENAI_API_KEY")
		os.Unsetenv("CLASP_MULTI_PROVIDER")
		os.Unsetenv("CLASP_OPUS_PROVIDER")
		os.Unsetenv("CLASP_OPUS_MODEL")
		os.Unsetenv("CLASP_OPUS_API_KEY")
	}()

	cfg, err := config.LoadFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	opusTier := cfg.GetTierConfig("claude-3-opus-20240229")
	if opusTier == nil {
		t.Fatal("Expected Opus tier config to exist")
	}

	// Should use tier-specific API key, not inherited
	if opusTier.APIKey != "tier-specific-key" {
		t.Errorf("Expected tier-specific API key 'tier-specific-key', got '%s'", opusTier.APIKey)
	}
}
