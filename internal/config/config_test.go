// Package config provides configuration management for CLASP.
package config

import (
	"os"
	"testing"
)

func clearEnv() {
	envVars := []string{
		"PROVIDER",
		"OPENAI_API_KEY", "OPENAI_BASE_URL",
		"AZURE_API_KEY", "AZURE_OPENAI_ENDPOINT", "AZURE_DEPLOYMENT_NAME", "AZURE_API_VERSION",
		"OPENROUTER_API_KEY", "OPENROUTER_BASE_URL",
		"ANTHROPIC_API_KEY",
		"CUSTOM_API_KEY", "CUSTOM_BASE_URL",
		"CLASP_MODEL", "CLASP_MODEL_OPUS", "CLASP_MODEL_SONNET", "CLASP_MODEL_HAIKU",
		"CLASP_PORT", "CLASP_LOG_LEVEL",
		"CLASP_DEBUG", "CLASP_DEBUG_REQUESTS", "CLASP_DEBUG_RESPONSES",
		"CLASP_RATE_LIMIT", "CLASP_RATE_LIMIT_REQUESTS", "CLASP_RATE_LIMIT_WINDOW", "CLASP_RATE_LIMIT_BURST",
		"CLASP_CACHE", "CLASP_CACHE_MAX_SIZE", "CLASP_CACHE_TTL",
		"CLASP_AUTH", "CLASP_AUTH_API_KEY",
		"CLASP_MULTI_PROVIDER",
		"CLASP_FALLBACK", "CLASP_FALLBACK_PROVIDER", "CLASP_FALLBACK_MODEL",
		"CLASP_CIRCUIT_BREAKER",
		"CLASP_QUEUE",
		"CLASP_MODEL_ALIASES",
	}
	for _, v := range envVars {
		os.Unsetenv(v)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Provider != ProviderOpenAI {
		t.Errorf("Provider = %q, want %q", cfg.Provider, ProviderOpenAI)
	}
	if cfg.Port != 8080 {
		t.Errorf("Port = %d, want %d", cfg.Port, 8080)
	}
	if cfg.DefaultModel != "gpt-4o" {
		t.Errorf("DefaultModel = %q, want %q", cfg.DefaultModel, "gpt-4o")
	}
	if cfg.OpenAIBaseURL != "https://api.openai.com/v1" {
		t.Errorf("OpenAIBaseURL = %q, want %q", cfg.OpenAIBaseURL, "https://api.openai.com/v1")
	}
	if cfg.RateLimitEnabled {
		t.Error("RateLimitEnabled should be false by default")
	}
	if cfg.CacheEnabled {
		t.Error("CacheEnabled should be false by default")
	}
	if cfg.AuthEnabled {
		t.Error("AuthEnabled should be false by default")
	}
}

func TestLoadFromEnv_OpenAI(t *testing.T) {
	clearEnv()
	os.Setenv("OPENAI_API_KEY", "sk-test-key")
	os.Setenv("CLASP_PORT", "9000")
	os.Setenv("CLASP_MODEL", "gpt-4-turbo")
	defer clearEnv()

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv failed: %v", err)
	}

	if cfg.Provider != ProviderOpenAI {
		t.Errorf("Provider = %q, want %q", cfg.Provider, ProviderOpenAI)
	}
	if cfg.OpenAIAPIKey != "sk-test-key" {
		t.Errorf("OpenAIAPIKey = %q, want %q", cfg.OpenAIAPIKey, "sk-test-key")
	}
	if cfg.Port != 9000 {
		t.Errorf("Port = %d, want %d", cfg.Port, 9000)
	}
	if cfg.DefaultModel != "gpt-4-turbo" {
		t.Errorf("DefaultModel = %q, want %q", cfg.DefaultModel, "gpt-4-turbo")
	}
}

func TestLoadFromEnv_Azure(t *testing.T) {
	clearEnv()
	os.Setenv("PROVIDER", "azure")
	os.Setenv("AZURE_API_KEY", "azure-key")
	os.Setenv("AZURE_OPENAI_ENDPOINT", "https://test.openai.azure.com")
	os.Setenv("AZURE_DEPLOYMENT_NAME", "gpt-4")
	defer clearEnv()

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv failed: %v", err)
	}

	if cfg.Provider != ProviderAzure {
		t.Errorf("Provider = %q, want %q", cfg.Provider, ProviderAzure)
	}
	if cfg.AzureAPIKey != "azure-key" {
		t.Errorf("AzureAPIKey = %q, want %q", cfg.AzureAPIKey, "azure-key")
	}
	if cfg.AzureEndpoint != "https://test.openai.azure.com" {
		t.Errorf("AzureEndpoint = %q", cfg.AzureEndpoint)
	}
	if cfg.AzureDeploymentName != "gpt-4" {
		t.Errorf("AzureDeploymentName = %q, want %q", cfg.AzureDeploymentName, "gpt-4")
	}
}

func TestLoadFromEnv_OpenRouter(t *testing.T) {
	clearEnv()
	os.Setenv("OPENROUTER_API_KEY", "sk-or-test")
	defer clearEnv()

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv failed: %v", err)
	}

	if cfg.Provider != ProviderOpenRouter {
		t.Errorf("Provider = %q, want %q", cfg.Provider, ProviderOpenRouter)
	}
	if cfg.OpenRouterAPIKey != "sk-or-test" {
		t.Errorf("OpenRouterAPIKey = %q, want %q", cfg.OpenRouterAPIKey, "sk-or-test")
	}
}

func TestLoadFromEnv_Custom(t *testing.T) {
	clearEnv()
	os.Setenv("PROVIDER", "custom")
	os.Setenv("CUSTOM_BASE_URL", "http://localhost:11434/v1")
	defer clearEnv()

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv failed: %v", err)
	}

	if cfg.Provider != ProviderCustom {
		t.Errorf("Provider = %q, want %q", cfg.Provider, ProviderCustom)
	}
	if cfg.CustomBaseURL != "http://localhost:11434/v1" {
		t.Errorf("CustomBaseURL = %q", cfg.CustomBaseURL)
	}
}

func TestLoadFromEnv_Debug(t *testing.T) {
	clearEnv()
	os.Setenv("OPENAI_API_KEY", "sk-test")
	os.Setenv("CLASP_DEBUG", "true")
	defer clearEnv()

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv failed: %v", err)
	}

	if !cfg.Debug {
		t.Error("Debug should be true")
	}
	if !cfg.DebugRequests {
		t.Error("DebugRequests should be true when Debug is true")
	}
	if !cfg.DebugResponses {
		t.Error("DebugResponses should be true when Debug is true")
	}
}

func TestLoadFromEnv_RateLimiting(t *testing.T) {
	clearEnv()
	os.Setenv("OPENAI_API_KEY", "sk-test")
	os.Setenv("CLASP_RATE_LIMIT", "true")
	os.Setenv("CLASP_RATE_LIMIT_REQUESTS", "100")
	os.Setenv("CLASP_RATE_LIMIT_WINDOW", "120")
	os.Setenv("CLASP_RATE_LIMIT_BURST", "20")
	defer clearEnv()

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv failed: %v", err)
	}

	if !cfg.RateLimitEnabled {
		t.Error("RateLimitEnabled should be true")
	}
	if cfg.RateLimitRequests != 100 {
		t.Errorf("RateLimitRequests = %d, want %d", cfg.RateLimitRequests, 100)
	}
	if cfg.RateLimitWindow != 120 {
		t.Errorf("RateLimitWindow = %d, want %d", cfg.RateLimitWindow, 120)
	}
	if cfg.RateLimitBurst != 20 {
		t.Errorf("RateLimitBurst = %d, want %d", cfg.RateLimitBurst, 20)
	}
}

func TestLoadFromEnv_Cache(t *testing.T) {
	clearEnv()
	os.Setenv("OPENAI_API_KEY", "sk-test")
	os.Setenv("CLASP_CACHE", "1")
	os.Setenv("CLASP_CACHE_MAX_SIZE", "500")
	os.Setenv("CLASP_CACHE_TTL", "7200")
	defer clearEnv()

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv failed: %v", err)
	}

	if !cfg.CacheEnabled {
		t.Error("CacheEnabled should be true")
	}
	if cfg.CacheMaxSize != 500 {
		t.Errorf("CacheMaxSize = %d, want %d", cfg.CacheMaxSize, 500)
	}
	if cfg.CacheTTL != 7200 {
		t.Errorf("CacheTTL = %d, want %d", cfg.CacheTTL, 7200)
	}
}

func TestLoadFromEnv_InvalidPort(t *testing.T) {
	clearEnv()
	os.Setenv("OPENAI_API_KEY", "sk-test")
	os.Setenv("CLASP_PORT", "invalid")
	defer clearEnv()

	_, err := LoadFromEnv()
	if err == nil {
		t.Error("Expected error for invalid port")
	}
}

func TestValidate_MissingOpenAIKey(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Provider = ProviderOpenAI
	cfg.OpenAIAPIKey = ""

	err := cfg.Validate()
	if err == nil {
		t.Error("Expected validation error for missing OpenAI API key")
	}
}

func TestValidate_MissingAzureEndpoint(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Provider = ProviderAzure
	cfg.AzureAPIKey = "test-key"
	cfg.AzureEndpoint = ""
	cfg.AzureDeploymentName = "gpt-4"

	err := cfg.Validate()
	if err == nil {
		t.Error("Expected validation error for missing Azure endpoint")
	}
}

func TestValidate_MissingCustomBaseURL(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Provider = ProviderCustom
	cfg.CustomBaseURL = ""

	err := cfg.Validate()
	if err == nil {
		t.Error("Expected validation error for missing custom base URL")
	}
}

func TestGetAPIKey(t *testing.T) {
	tests := []struct {
		provider ProviderType
		expected string
	}{
		{ProviderOpenAI, "openai-key"},
		{ProviderAzure, "azure-key"},
		{ProviderOpenRouter, "openrouter-key"},
		{ProviderAnthropic, "anthropic-key"},
		{ProviderCustom, "custom-key"},
	}

	for _, tt := range tests {
		t.Run(string(tt.provider), func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.Provider = tt.provider
			cfg.OpenAIAPIKey = "openai-key"
			cfg.AzureAPIKey = "azure-key"
			cfg.OpenRouterAPIKey = "openrouter-key"
			cfg.AnthropicAPIKey = "anthropic-key"
			cfg.CustomAPIKey = "custom-key"

			result := cfg.GetAPIKey()
			if result != tt.expected {
				t.Errorf("GetAPIKey() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestGetBaseURL(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Provider = ProviderOpenAI
	cfg.OpenAIBaseURL = "https://api.openai.com/v1"

	result := cfg.GetBaseURL()
	if result != "https://api.openai.com/v1" {
		t.Errorf("GetBaseURL() = %q", result)
	}
}

func TestGetBaseURL_Azure(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Provider = ProviderAzure
	cfg.AzureEndpoint = "https://test.openai.azure.com"
	cfg.AzureDeploymentName = "gpt-4"

	result := cfg.GetBaseURL()
	expected := "https://test.openai.azure.com/openai/deployments/gpt-4"
	if result != expected {
		t.Errorf("GetBaseURL() = %q, want %q", result, expected)
	}
}

func TestGetChatCompletionsURL(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Provider = ProviderOpenAI
	cfg.OpenAIBaseURL = "https://api.openai.com/v1"

	result := cfg.GetChatCompletionsURL()
	expected := "https://api.openai.com/v1/chat/completions"
	if result != expected {
		t.Errorf("GetChatCompletionsURL() = %q, want %q", result, expected)
	}
}

func TestGetChatCompletionsURL_Azure(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Provider = ProviderAzure
	cfg.AzureEndpoint = "https://test.openai.azure.com"
	cfg.AzureDeploymentName = "gpt-4"
	cfg.AzureAPIVersion = "2024-02-15-preview"

	result := cfg.GetChatCompletionsURL()
	expected := "https://test.openai.azure.com/openai/deployments/gpt-4/chat/completions?api-version=2024-02-15-preview"
	if result != expected {
		t.Errorf("GetChatCompletionsURL() = %q, want %q", result, expected)
	}
}

func TestMapModel(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ModelOpus = "gpt-4-turbo"
	cfg.ModelSonnet = "gpt-4o"
	cfg.ModelHaiku = "gpt-4o-mini"
	cfg.DefaultModel = "gpt-4o"

	tests := []struct {
		requested string
		expected  string
	}{
		{"claude-3-opus-20240229", "gpt-4-turbo"},
		{"claude-3-sonnet-20240229", "gpt-4o"},
		{"claude-3-haiku-20240307", "gpt-4o-mini"},
		{"unknown-model", "gpt-4o"},
	}

	for _, tt := range tests {
		t.Run(tt.requested, func(t *testing.T) {
			result := cfg.MapModel(tt.requested)
			if result != tt.expected {
				t.Errorf("MapModel(%q) = %q, want %q", tt.requested, result, tt.expected)
			}
		})
	}
}

func TestGetModelTier(t *testing.T) {
	tests := []struct {
		model    string
		expected ModelTier
	}{
		{"claude-3-opus-20240229", TierOpus},
		{"claude-3-sonnet-20240229", TierSonnet},
		{"claude-3-haiku-20240307", TierHaiku},
		{"unknown-model", TierSonnet},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			result := GetModelTier(tt.model)
			if result != tt.expected {
				t.Errorf("GetModelTier(%q) = %q, want %q", tt.model, result, tt.expected)
			}
		})
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		s        string
		substr   string
		expected bool
	}{
		{"claude-3-opus-20240229", "opus", true},
		{"claude-3-sonnet-20240229", "sonnet", true},
		{"claude-3-haiku-20240307", "haiku", true},
		{"gpt-4o", "opus", false},
		{"opus", "opus", true},
		{"", "opus", false},
	}

	for _, tt := range tests {
		t.Run(tt.s+"_"+tt.substr, func(t *testing.T) {
			result := contains(tt.s, tt.substr)
			if result != tt.expected {
				t.Errorf("contains(%q, %q) = %v, want %v", tt.s, tt.substr, result, tt.expected)
			}
		})
	}
}

func TestModelAliases(t *testing.T) {
	clearEnv()
	os.Setenv("OPENAI_API_KEY", "sk-test")
	os.Setenv("CLASP_MODEL_ALIASES", "fast:gpt-4o-mini,smart:gpt-4o")
	defer clearEnv()

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv failed: %v", err)
	}

	if cfg.ResolveAlias("fast") != "gpt-4o-mini" {
		t.Errorf("ResolveAlias('fast') = %q, want %q", cfg.ResolveAlias("fast"), "gpt-4o-mini")
	}
	if cfg.ResolveAlias("smart") != "gpt-4o" {
		t.Errorf("ResolveAlias('smart') = %q, want %q", cfg.ResolveAlias("smart"), "gpt-4o")
	}
	if cfg.ResolveAlias("unknown") != "unknown" {
		t.Errorf("ResolveAlias('unknown') = %q, want %q", cfg.ResolveAlias("unknown"), "unknown")
	}
}

func TestAddAlias(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AddAlias("custom", "gpt-4-turbo")

	if cfg.ResolveAlias("custom") != "gpt-4-turbo" {
		t.Errorf("ResolveAlias('custom') = %q, want %q", cfg.ResolveAlias("custom"), "gpt-4-turbo")
	}
}

func TestGetAliases(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AddAlias("fast", "gpt-4o-mini")
	cfg.AddAlias("smart", "gpt-4o")

	aliases := cfg.GetAliases()
	if len(aliases) != 2 {
		t.Errorf("len(GetAliases()) = %d, want 2", len(aliases))
	}
	if aliases["fast"] != "gpt-4o-mini" {
		t.Errorf("aliases['fast'] = %q, want %q", aliases["fast"], "gpt-4o-mini")
	}
}

func TestTierConfig_HasFallback(t *testing.T) {
	tc := &TierConfig{
		Provider:         ProviderOpenAI,
		FallbackProvider: ProviderOpenRouter,
	}
	if !tc.HasFallback() {
		t.Error("HasFallback() should be true")
	}

	tcNoFallback := &TierConfig{Provider: ProviderOpenAI}
	if tcNoFallback.HasFallback() {
		t.Error("HasFallback() should be false without fallback provider")
	}
}

func TestTierConfig_GetFallbackConfig(t *testing.T) {
	tc := &TierConfig{
		Provider:         ProviderOpenAI,
		FallbackProvider: ProviderOpenRouter,
		FallbackModel:    "gpt-4o",
		FallbackAPIKey:   "sk-fallback",
	}

	fallback := tc.GetFallbackConfig()
	if fallback == nil {
		t.Fatal("GetFallbackConfig() should not return nil")
	}
	if fallback.Provider != ProviderOpenRouter {
		t.Errorf("fallback.Provider = %q, want %q", fallback.Provider, ProviderOpenRouter)
	}
	if fallback.Model != "gpt-4o" {
		t.Errorf("fallback.Model = %q, want %q", fallback.Model, "gpt-4o")
	}
}

func TestConfig_HasGlobalFallback(t *testing.T) {
	cfg := DefaultConfig()
	cfg.FallbackEnabled = true
	cfg.FallbackProvider = ProviderOpenRouter

	if !cfg.HasGlobalFallback() {
		t.Error("HasGlobalFallback() should be true")
	}

	cfg2 := DefaultConfig()
	if cfg2.HasGlobalFallback() {
		t.Error("HasGlobalFallback() should be false by default")
	}
}

func TestConfig_GetGlobalFallbackConfig(t *testing.T) {
	cfg := DefaultConfig()
	cfg.FallbackEnabled = true
	cfg.FallbackProvider = ProviderOpenRouter
	cfg.FallbackModel = "gpt-4o"
	cfg.OpenRouterBaseURL = "https://openrouter.ai/api/v1"

	fallback := cfg.GetGlobalFallbackConfig()
	if fallback == nil {
		t.Fatal("GetGlobalFallbackConfig() should not return nil")
	}
	if fallback.Provider != ProviderOpenRouter {
		t.Errorf("fallback.Provider = %q, want %q", fallback.Provider, ProviderOpenRouter)
	}
	if fallback.BaseURL != "https://openrouter.ai/api/v1" {
		t.Errorf("fallback.BaseURL = %q", fallback.BaseURL)
	}
}

func TestGetTierConfig(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MultiProviderEnabled = true
	cfg.TierOpus = &TierConfig{Provider: ProviderOpenAI, Model: "gpt-4-turbo"}
	cfg.TierSonnet = &TierConfig{Provider: ProviderOpenRouter, Model: "gpt-4o"}
	cfg.TierHaiku = &TierConfig{Provider: ProviderCustom, Model: "llama3.1"}

	opus := cfg.GetTierConfig("claude-3-opus-20240229")
	if opus == nil || opus.Model != "gpt-4-turbo" {
		t.Error("GetTierConfig for opus failed")
	}

	sonnet := cfg.GetTierConfig("claude-3-sonnet-20240229")
	if sonnet == nil || sonnet.Model != "gpt-4o" {
		t.Error("GetTierConfig for sonnet failed")
	}

	haiku := cfg.GetTierConfig("claude-3-haiku-20240307")
	if haiku == nil || haiku.Model != "llama3.1" {
		t.Error("GetTierConfig for haiku failed")
	}
}

func TestGetTierConfig_Disabled(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MultiProviderEnabled = false
	cfg.TierOpus = &TierConfig{Provider: ProviderOpenAI, Model: "gpt-4-turbo"}

	result := cfg.GetTierConfig("claude-3-opus-20240229")
	if result != nil {
		t.Error("GetTierConfig should return nil when multi-provider is disabled")
	}
}

func TestDetectProvider(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*Config)
		expected ProviderType
	}{
		{
			name:     "OpenAI key",
			setup:    func(c *Config) { c.OpenAIAPIKey = "sk-test" },
			expected: ProviderOpenAI,
		},
		{
			name:     "OpenRouter key",
			setup:    func(c *Config) { c.OpenRouterAPIKey = "sk-or-test" },
			expected: ProviderOpenRouter,
		},
		{
			name: "Azure",
			setup: func(c *Config) {
				c.AzureAPIKey = "azure-key"
				c.AzureEndpoint = "https://test.azure.com"
			},
			expected: ProviderAzure,
		},
		{
			name:     "Anthropic key",
			setup:    func(c *Config) { c.AnthropicAPIKey = "sk-ant-test" },
			expected: ProviderAnthropic,
		},
		{
			name: "Custom",
			setup: func(c *Config) {
				c.CustomAPIKey = "custom-key"
				c.CustomBaseURL = "http://localhost:8000"
			},
			expected: ProviderCustom,
		},
		{
			name:     "Default",
			setup:    func(c *Config) {},
			expected: ProviderOpenAI,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			// Clear all keys first
			cfg.OpenAIAPIKey = ""
			cfg.OpenRouterAPIKey = ""
			cfg.AzureAPIKey = ""
			cfg.AnthropicAPIKey = ""
			cfg.CustomAPIKey = ""
			cfg.CustomBaseURL = ""
			cfg.AzureEndpoint = ""

			tt.setup(cfg)
			result := detectProvider(cfg)
			if result != tt.expected {
				t.Errorf("detectProvider() = %q, want %q", result, tt.expected)
			}
		})
	}
}
