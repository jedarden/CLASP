package tests

import (
	"os"
	"testing"

	"github.com/jedarden/clasp/internal/config"
)

func TestModelAlias_ResolveAlias(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.ModelAliases = map[string]string{
		"fast":  "gpt-4o-mini",
		"smart": "gpt-4o",
		"best":  "o1-preview",
	}

	tests := []struct {
		input    string
		expected string
	}{
		{"fast", "gpt-4o-mini"},
		{"FAST", "gpt-4o-mini"},   // Case insensitive
		{"Fast", "gpt-4o-mini"},   // Case insensitive
		{"smart", "gpt-4o"},
		{"best", "o1-preview"},
		{"gpt-4o", "gpt-4o"},      // Non-alias passes through
		{"claude-3-opus", "claude-3-opus"}, // Non-alias passes through
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := cfg.ResolveAlias(tt.input)
			if result != tt.expected {
				t.Errorf("ResolveAlias(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestModelAlias_AddAlias(t *testing.T) {
	cfg := config.DefaultConfig()

	// Add aliases at runtime
	cfg.AddAlias("custom", "custom-model-v1")
	cfg.AddAlias("UPPERCASE", "lowercase-model")

	if result := cfg.ResolveAlias("custom"); result != "custom-model-v1" {
		t.Errorf("expected custom-model-v1, got %s", result)
	}

	if result := cfg.ResolveAlias("uppercase"); result != "lowercase-model" {
		t.Errorf("expected lowercase-model, got %s", result)
	}
}

func TestModelAlias_GetAliases(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.ModelAliases = map[string]string{
		"fast":  "gpt-4o-mini",
		"smart": "gpt-4o",
	}

	aliases := cfg.GetAliases()

	if len(aliases) != 2 {
		t.Errorf("expected 2 aliases, got %d", len(aliases))
	}

	if aliases["fast"] != "gpt-4o-mini" {
		t.Errorf("expected gpt-4o-mini, got %s", aliases["fast"])
	}

	// Verify it's a copy (modifying returned map doesn't affect original)
	aliases["fast"] = "modified"
	if cfg.ModelAliases["fast"] != "gpt-4o-mini" {
		t.Error("GetAliases should return a copy, not the original map")
	}
}

func TestModelAlias_EmptyAliases(t *testing.T) {
	cfg := config.DefaultConfig()

	// With no aliases configured, model should pass through unchanged
	result := cfg.ResolveAlias("any-model")
	if result != "any-model" {
		t.Errorf("expected any-model, got %s", result)
	}

	aliases := cfg.GetAliases()
	if len(aliases) != 0 {
		t.Errorf("expected 0 aliases, got %d", len(aliases))
	}
}

func TestModelAlias_LoadFromEnv_IndividualVars(t *testing.T) {
	// Set up test environment variables
	os.Setenv("OPENAI_API_KEY", "test-key")
	os.Setenv("CLASP_ALIAS_FAST", "gpt-4o-mini")
	os.Setenv("CLASP_ALIAS_SMART", "gpt-4o")
	defer os.Unsetenv("OPENAI_API_KEY")
	defer os.Unsetenv("CLASP_ALIAS_FAST")
	defer os.Unsetenv("CLASP_ALIAS_SMART")

	cfg, err := config.LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv error: %v", err)
	}

	if cfg.ResolveAlias("fast") != "gpt-4o-mini" {
		t.Errorf("expected gpt-4o-mini, got %s", cfg.ResolveAlias("fast"))
	}

	if cfg.ResolveAlias("smart") != "gpt-4o" {
		t.Errorf("expected gpt-4o, got %s", cfg.ResolveAlias("smart"))
	}
}

func TestModelAlias_LoadFromEnv_CommaList(t *testing.T) {
	// Set up test environment variables
	os.Setenv("OPENAI_API_KEY", "test-key")
	os.Setenv("CLASP_MODEL_ALIASES", "quick:gpt-4o-mini,powerful:gpt-4o,reasoning:o1-preview")
	defer os.Unsetenv("OPENAI_API_KEY")
	defer os.Unsetenv("CLASP_MODEL_ALIASES")

	cfg, err := config.LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv error: %v", err)
	}

	tests := []struct {
		alias    string
		expected string
	}{
		{"quick", "gpt-4o-mini"},
		{"powerful", "gpt-4o"},
		{"reasoning", "o1-preview"},
	}

	for _, tt := range tests {
		result := cfg.ResolveAlias(tt.alias)
		if result != tt.expected {
			t.Errorf("ResolveAlias(%q) = %q, want %q", tt.alias, result, tt.expected)
		}
	}
}

func TestModelAlias_LoadFromEnv_Mixed(t *testing.T) {
	// Test that both formats work together
	os.Setenv("OPENAI_API_KEY", "test-key")
	os.Setenv("CLASP_ALIAS_INDIVIDUAL", "model-from-env-var")
	os.Setenv("CLASP_MODEL_ALIASES", "fromlist:model-from-list")
	defer os.Unsetenv("OPENAI_API_KEY")
	defer os.Unsetenv("CLASP_ALIAS_INDIVIDUAL")
	defer os.Unsetenv("CLASP_MODEL_ALIASES")

	cfg, err := config.LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv error: %v", err)
	}

	if cfg.ResolveAlias("individual") != "model-from-env-var" {
		t.Errorf("expected model-from-env-var, got %s", cfg.ResolveAlias("individual"))
	}

	if cfg.ResolveAlias("fromlist") != "model-from-list" {
		t.Errorf("expected model-from-list, got %s", cfg.ResolveAlias("fromlist"))
	}
}

func TestModelAlias_WithSpaces(t *testing.T) {
	os.Setenv("OPENAI_API_KEY", "test-key")
	os.Setenv("CLASP_MODEL_ALIASES", " spaced : model-with-spaces , another : value ")
	defer os.Unsetenv("OPENAI_API_KEY")
	defer os.Unsetenv("CLASP_MODEL_ALIASES")

	cfg, err := config.LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv error: %v", err)
	}

	// Spaces should be trimmed
	if cfg.ResolveAlias("spaced") != "model-with-spaces" {
		t.Errorf("expected model-with-spaces, got %s", cfg.ResolveAlias("spaced"))
	}

	if cfg.ResolveAlias("another") != "value" {
		t.Errorf("expected value, got %s", cfg.ResolveAlias("another"))
	}
}

func TestModelAlias_EmptyValues(t *testing.T) {
	os.Setenv("OPENAI_API_KEY", "test-key")
	os.Setenv("CLASP_MODEL_ALIASES", "valid:model,,empty:,:novalue")
	defer os.Unsetenv("OPENAI_API_KEY")
	defer os.Unsetenv("CLASP_MODEL_ALIASES")

	cfg, err := config.LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv error: %v", err)
	}

	// Valid alias should work
	if cfg.ResolveAlias("valid") != "model" {
		t.Errorf("expected model, got %s", cfg.ResolveAlias("valid"))
	}

	// Empty/malformed entries should be ignored, not cause errors
	aliases := cfg.GetAliases()
	if _, ok := aliases["empty"]; ok {
		t.Error("empty value should not create alias")
	}
	if _, ok := aliases[""]; ok {
		t.Error("empty key should not create alias")
	}
}
