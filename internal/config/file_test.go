// Package config provides tests for configuration file loading.
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFromFile(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "clasp.yaml")

	configContent := `
provider: openai

api_keys:
  openai: test-api-key-12345

models:
  default: gpt-4o
  opus: gpt-4o
  sonnet: gpt-4o-mini

server:
  port: 9090
  log_level: debug

cache:
  enabled: true
  max_size: 500
  ttl: 1800

rate_limit:
  enabled: false
  requests: 100
  window: 60
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}

	if cfg == nil {
		t.Fatal("Expected non-nil config")
	}

	// Check provider
	if cfg.Provider != "openai" {
		t.Errorf("Expected provider 'openai', got '%s'", cfg.Provider)
	}

	// Check API key
	if cfg.APIKeys.OpenAI != "test-api-key-12345" {
		t.Errorf("Expected OpenAI API key 'test-api-key-12345', got '%s'", cfg.APIKeys.OpenAI)
	}

	// Check models
	if cfg.Models.Default != "gpt-4o" {
		t.Errorf("Expected default model 'gpt-4o', got '%s'", cfg.Models.Default)
	}
	if cfg.Models.Opus != "gpt-4o" {
		t.Errorf("Expected opus model 'gpt-4o', got '%s'", cfg.Models.Opus)
	}
	if cfg.Models.Sonnet != "gpt-4o-mini" {
		t.Errorf("Expected sonnet model 'gpt-4o-mini', got '%s'", cfg.Models.Sonnet)
	}

	// Check server settings
	if cfg.Server.Port != 9090 {
		t.Errorf("Expected port 9090, got %d", cfg.Server.Port)
	}
	if cfg.Server.LogLevel != "debug" {
		t.Errorf("Expected log level 'debug', got '%s'", cfg.Server.LogLevel)
	}

	// Check cache settings
	if !cfg.Cache.Enabled {
		t.Error("Expected cache to be enabled")
	}
	if cfg.Cache.MaxSize != 500 {
		t.Errorf("Expected cache max size 500, got %d", cfg.Cache.MaxSize)
	}
	if cfg.Cache.TTL != 1800 {
		t.Errorf("Expected cache TTL 1800, got %d", cfg.Cache.TTL)
	}
}

func TestLoadFromFileWithEnvExpansion(t *testing.T) {
	// Set environment variables
	os.Setenv("TEST_OPENAI_KEY", "sk-test-key-123")
	os.Setenv("TEST_OPENROUTER_KEY", "sk-or-test-456")
	defer os.Unsetenv("TEST_OPENAI_KEY")
	defer os.Unsetenv("TEST_OPENROUTER_KEY")

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "clasp.yaml")

	configContent := `
provider: openai

api_keys:
  openai: ${TEST_OPENAI_KEY}
  openrouter: ${TEST_OPENROUTER_KEY}
  anthropic: ${TEST_MISSING_KEY:-default-anthropic-key}

endpoints:
  openai: ${OPENAI_BASE_URL:-https://api.openai.com/v1}
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}

	// Check expanded env vars
	if cfg.APIKeys.OpenAI != "sk-test-key-123" {
		t.Errorf("Expected OpenAI key 'sk-test-key-123', got '%s'", cfg.APIKeys.OpenAI)
	}
	if cfg.APIKeys.OpenRouter != "sk-or-test-456" {
		t.Errorf("Expected OpenRouter key 'sk-or-test-456', got '%s'", cfg.APIKeys.OpenRouter)
	}

	// Check default value for missing env var
	if cfg.APIKeys.Anthropic != "default-anthropic-key" {
		t.Errorf("Expected Anthropic key 'default-anthropic-key', got '%s'", cfg.APIKeys.Anthropic)
	}

	// Check default value for endpoint
	if cfg.Endpoints.OpenAI != "https://api.openai.com/v1" {
		t.Errorf("Expected OpenAI base URL 'https://api.openai.com/v1', got '%s'", cfg.Endpoints.OpenAI)
	}
}

func TestLoadFromFileNotFound(t *testing.T) {
	cfg, err := LoadFromFile("/nonexistent/path/clasp.yaml")
	if err != nil {
		t.Errorf("Expected no error for missing config file, got: %v", err)
	}
	if cfg != nil {
		t.Error("Expected nil config for missing file")
	}
}

func TestLoadFromFileInvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "clasp.yaml")

	invalidYAML := `
provider: openai
api_keys:
  openai: [this is not a string
`
	if err := os.WriteFile(configPath, []byte(invalidYAML), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	_, err := LoadFromFile(configPath)
	if err == nil {
		t.Error("Expected error for invalid YAML")
	}
}

func TestMergeWithEnv(t *testing.T) {
	// Set environment variables (these should override file config)
	os.Setenv("OPENAI_API_KEY", "env-override-key")
	os.Setenv("CLASP_PORT", "7777")
	defer os.Unsetenv("OPENAI_API_KEY")
	defer os.Unsetenv("CLASP_PORT")

	fileCfg := &FileConfig{
		Provider: "openai",
		APIKeys: APIKeysConfig{
			OpenAI: "file-api-key",
		},
		Server: ServerConfig{
			Port:     9090,
			LogLevel: "info",
		},
		Models: ModelsConfig{
			Default: "gpt-4o",
		},
		Aliases: make(map[string]string),
	}

	cfg := MergeWithEnv(fileCfg, DefaultConfig())

	// Environment variables should override file config
	if cfg.OpenAIAPIKey != "env-override-key" {
		t.Errorf("Expected API key from env 'env-override-key', got '%s'", cfg.OpenAIAPIKey)
	}
	if cfg.Port != 7777 {
		t.Errorf("Expected port from env 7777, got %d", cfg.Port)
	}

	// File config values should be preserved where env vars aren't set
	if cfg.DefaultModel != "gpt-4o" {
		t.Errorf("Expected default model from file 'gpt-4o', got '%s'", cfg.DefaultModel)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("Expected log level from file 'info', got '%s'", cfg.LogLevel)
	}
}

func TestValidateFileConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  *FileConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: &FileConfig{
				Provider: "openai",
				Server: ServerConfig{
					Port:     8080,
					LogLevel: "info",
				},
			},
			wantErr: false,
		},
		{
			name: "invalid provider",
			config: &FileConfig{
				Provider: "invalid-provider",
			},
			wantErr: true,
		},
		{
			name: "invalid port",
			config: &FileConfig{
				Server: ServerConfig{
					Port: -1,
				},
			},
			wantErr: true,
		},
		{
			name: "port too high",
			config: &FileConfig{
				Server: ServerConfig{
					Port: 70000,
				},
			},
			wantErr: true,
		},
		{
			name: "invalid log level",
			config: &FileConfig{
				Server: ServerConfig{
					LogLevel: "invalid",
				},
			},
			wantErr: true,
		},
		{
			name: "negative cache size",
			config: &FileConfig{
				Cache: CacheConfig{
					MaxSize: -1,
				},
			},
			wantErr: true,
		},
		{
			name: "auth enabled without api key",
			config: &FileConfig{
				Auth: AuthConfig{
					Enabled: true,
					APIKey:  "",
				},
			},
			wantErr: true,
		},
		{
			name: "multi-provider enabled without tiers",
			config: &FileConfig{
				MultiProvider: MultiProviderConfig{
					Enabled: true,
				},
			},
			wantErr: true,
		},
		{
			name: "fallback enabled without provider",
			config: &FileConfig{
				Fallback: FallbackConfig{
					Enabled: true,
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFileConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateFileConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMultiProviderConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "clasp.yaml")

	configContent := `
provider: openai

api_keys:
  openai: sk-test-key
  openrouter: sk-or-test-key

multi_provider:
  enabled: true
  opus:
    provider: openai
    model: gpt-4o
  sonnet:
    provider: openrouter
    model: anthropic/claude-3-sonnet
  haiku:
    provider: openai
    model: gpt-4o-mini
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}

	if !cfg.MultiProvider.Enabled {
		t.Error("Expected multi-provider to be enabled")
	}

	if cfg.MultiProvider.Opus == nil {
		t.Fatal("Expected opus tier config")
	}
	if cfg.MultiProvider.Opus.Provider != "openai" {
		t.Errorf("Expected opus provider 'openai', got '%s'", cfg.MultiProvider.Opus.Provider)
	}
	if cfg.MultiProvider.Opus.Model != "gpt-4o" {
		t.Errorf("Expected opus model 'gpt-4o', got '%s'", cfg.MultiProvider.Opus.Model)
	}

	if cfg.MultiProvider.Sonnet == nil {
		t.Fatal("Expected sonnet tier config")
	}
	if cfg.MultiProvider.Sonnet.Provider != "openrouter" {
		t.Errorf("Expected sonnet provider 'openrouter', got '%s'", cfg.MultiProvider.Sonnet.Provider)
	}

	if cfg.MultiProvider.Haiku == nil {
		t.Fatal("Expected haiku tier config")
	}
	if cfg.MultiProvider.Haiku.Model != "gpt-4o-mini" {
		t.Errorf("Expected haiku model 'gpt-4o-mini', got '%s'", cfg.MultiProvider.Haiku.Model)
	}
}

func TestTierFallbackConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "clasp.yaml")

	configContent := `
provider: openai

api_keys:
  openai: sk-test-key
  openrouter: sk-or-test-key

multi_provider:
  enabled: true
  opus:
    provider: openai
    model: gpt-4o
    fallback:
      provider: openrouter
      model: anthropic/claude-opus-4-20250514
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}

	if cfg.MultiProvider.Opus.Fallback.Provider != "openrouter" {
		t.Errorf("Expected fallback provider 'openrouter', got '%s'", cfg.MultiProvider.Opus.Fallback.Provider)
	}
	if cfg.MultiProvider.Opus.Fallback.Model != "anthropic/claude-opus-4-20250514" {
		t.Errorf("Expected fallback model 'anthropic/claude-opus-4-20250514', got '%s'", cfg.MultiProvider.Opus.Fallback.Model)
	}
}

func TestModelAliasesFromFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "clasp.yaml")

	configContent := `
provider: openai

api_keys:
  openai: sk-test-key

aliases:
  fast: gpt-4o-mini
  smart: gpt-4o
  cheap: gpt-3.5-turbo
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}

	if cfg.Aliases["fast"] != "gpt-4o-mini" {
		t.Errorf("Expected alias 'fast' -> 'gpt-4o-mini', got '%s'", cfg.Aliases["fast"])
	}
	if cfg.Aliases["smart"] != "gpt-4o" {
		t.Errorf("Expected alias 'smart' -> 'gpt-4o', got '%s'", cfg.Aliases["smart"])
	}
	if cfg.Aliases["cheap"] != "gpt-3.5-turbo" {
		t.Errorf("Expected alias 'cheap' -> 'gpt-3.5-turbo', got '%s'", cfg.Aliases["cheap"])
	}
}

func TestAzureConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "clasp.yaml")

	configContent := `
provider: azure

api_keys:
  azure: azure-test-key

endpoints:
  azure:
    endpoint: https://my-resource.openai.azure.com
    deployment_name: gpt-4-deployment
    api_version: "2024-06-01"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}

	if cfg.Provider != "azure" {
		t.Errorf("Expected provider 'azure', got '%s'", cfg.Provider)
	}
	if cfg.Endpoints.Azure.Endpoint != "https://my-resource.openai.azure.com" {
		t.Errorf("Expected Azure endpoint, got '%s'", cfg.Endpoints.Azure.Endpoint)
	}
	if cfg.Endpoints.Azure.DeploymentName != "gpt-4-deployment" {
		t.Errorf("Expected deployment name 'gpt-4-deployment', got '%s'", cfg.Endpoints.Azure.DeploymentName)
	}
	if cfg.Endpoints.Azure.APIVersion != "2024-06-01" {
		t.Errorf("Expected API version '2024-06-01', got '%s'", cfg.Endpoints.Azure.APIVersion)
	}
}

func TestDefaultFileConfig(t *testing.T) {
	cfg := DefaultFileConfig()

	if cfg.Provider != "openai" {
		t.Errorf("Expected default provider 'openai', got '%s'", cfg.Provider)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("Expected default port 8080, got %d", cfg.Server.Port)
	}
	if cfg.Server.LogLevel != "info" {
		t.Errorf("Expected default log level 'info', got '%s'", cfg.Server.LogLevel)
	}
	if cfg.Models.Default != "gpt-4o" {
		t.Errorf("Expected default model 'gpt-4o', got '%s'", cfg.Models.Default)
	}
	if cfg.HTTPClient.TimeoutSec != 300 {
		t.Errorf("Expected HTTP timeout 300, got %d", cfg.HTTPClient.TimeoutSec)
	}
}
