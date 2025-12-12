package tests

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/jedarden/clasp/internal/config"
	"github.com/jedarden/clasp/internal/provider"
)

func TestAnthropicProvider_Name(t *testing.T) {
	p := provider.NewAnthropicProvider("")
	if p.Name() != "anthropic" {
		t.Errorf("Expected name 'anthropic', got '%s'", p.Name())
	}
}

func TestAnthropicProvider_RequiresTransformation(t *testing.T) {
	p := provider.NewAnthropicProvider("")
	if p.RequiresTransformation() {
		t.Error("AnthropicProvider should NOT require transformation (passthrough mode)")
	}
}

func TestAnthropicProvider_SupportsStreaming(t *testing.T) {
	p := provider.NewAnthropicProvider("")
	if !p.SupportsStreaming() {
		t.Error("AnthropicProvider should support streaming")
	}
}

func TestAnthropicProvider_GetEndpointURL(t *testing.T) {
	tests := []struct {
		name     string
		baseURL  string
		expected string
	}{
		{
			name:     "default base URL",
			baseURL:  "",
			expected: "https://api.anthropic.com/v1/messages",
		},
		{
			name:     "custom base URL",
			baseURL:  "https://custom.anthropic.com",
			expected: "https://custom.anthropic.com/v1/messages",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := provider.NewAnthropicProvider(tt.baseURL)
			if p.GetEndpointURL() != tt.expected {
				t.Errorf("Expected endpoint '%s', got '%s'", tt.expected, p.GetEndpointURL())
			}
		})
	}
}

func TestAnthropicProvider_GetHeaders(t *testing.T) {
	p := provider.NewAnthropicProvider("")
	headers := p.GetHeaders("test-api-key")

	// Check x-api-key header
	if headers.Get("x-api-key") != "test-api-key" {
		t.Errorf("Expected x-api-key 'test-api-key', got '%s'", headers.Get("x-api-key"))
	}

	// Check Content-Type header
	if headers.Get("Content-Type") != "application/json" {
		t.Errorf("Expected Content-Type 'application/json', got '%s'", headers.Get("Content-Type"))
	}

	// Check anthropic-version header
	if headers.Get("anthropic-version") != "2023-06-01" {
		t.Errorf("Expected anthropic-version '2023-06-01', got '%s'", headers.Get("anthropic-version"))
	}
}

func TestAnthropicProvider_GetHeadersWithEmbeddedKey(t *testing.T) {
	p := provider.NewAnthropicProviderWithKey("", "embedded-key")
	headers := p.GetHeaders("fallback-key")

	// Should use embedded key, not the fallback
	if headers.Get("x-api-key") != "embedded-key" {
		t.Errorf("Expected embedded key 'embedded-key', got '%s'", headers.Get("x-api-key"))
	}
}

func TestAnthropicProvider_TransformModelID(t *testing.T) {
	p := provider.NewAnthropicProvider("")

	// Model IDs should pass through unchanged
	tests := []string{
		"claude-3-5-sonnet-20241022",
		"claude-3-opus-20240229",
		"claude-3-haiku-20240307",
	}

	for _, model := range tests {
		result := p.TransformModelID(model)
		if result != model {
			t.Errorf("Expected model '%s' unchanged, got '%s'", model, result)
		}
	}
}

func TestAnthropicConfig_Validation(t *testing.T) {
	// Set up anthropic config
	os.Setenv("PROVIDER", "anthropic")
	os.Setenv("ANTHROPIC_API_KEY", "test-anthropic-key")

	defer os.Unsetenv("PROVIDER")
	defer os.Unsetenv("ANTHROPIC_API_KEY")

	cfg, err := config.LoadFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Provider != config.ProviderAnthropic {
		t.Errorf("Expected provider 'anthropic', got '%s'", cfg.Provider)
	}

	if cfg.AnthropicAPIKey != "test-anthropic-key" {
		t.Errorf("Expected API key 'test-anthropic-key', got '%s'", cfg.AnthropicAPIKey)
	}

	// GetAPIKey should return anthropic key
	if cfg.GetAPIKey() != "test-anthropic-key" {
		t.Errorf("Expected GetAPIKey() to return 'test-anthropic-key', got '%s'", cfg.GetAPIKey())
	}
}

func TestAnthropicConfig_ValidationError(t *testing.T) {
	// Set up anthropic config without API key
	os.Setenv("PROVIDER", "anthropic")
	os.Unsetenv("ANTHROPIC_API_KEY")

	defer os.Unsetenv("PROVIDER")

	_, err := config.LoadFromEnv()
	if err == nil {
		t.Error("Expected validation error for missing ANTHROPIC_API_KEY")
	}
}

func TestAnthropicConfig_AutoDetect(t *testing.T) {
	// Clear all other API keys and set only Anthropic
	os.Unsetenv("PROVIDER")
	os.Unsetenv("OPENAI_API_KEY")
	os.Unsetenv("OPENROUTER_API_KEY")
	os.Unsetenv("AZURE_API_KEY")
	os.Unsetenv("CUSTOM_API_KEY")
	os.Setenv("ANTHROPIC_API_KEY", "test-anthropic-key")

	defer os.Unsetenv("ANTHROPIC_API_KEY")

	cfg, err := config.LoadFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should auto-detect anthropic provider
	if cfg.Provider != config.ProviderAnthropic {
		t.Errorf("Expected auto-detected provider 'anthropic', got '%s'", cfg.Provider)
	}
}

func TestPassthroughModeHeader(t *testing.T) {
	// Create a mock Anthropic API server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify it's a passthrough request (not transformed)
		if r.Header.Get("x-api-key") == "" {
			t.Error("Expected x-api-key header in passthrough request")
		}
		if r.Header.Get("anthropic-version") == "" {
			t.Error("Expected anthropic-version header in passthrough request")
		}

		// Read request body to verify it's in Anthropic format
		body, _ := io.ReadAll(r.Body)
		var req map[string]interface{}
		if err := json.Unmarshal(body, &req); err != nil {
			t.Errorf("Failed to parse request body: %v", err)
		}

		// Verify Anthropic-format fields (not OpenAI format)
		if _, ok := req["max_tokens"]; !ok {
			t.Error("Expected max_tokens field in Anthropic request")
		}
		if _, ok := req["model"]; !ok {
			t.Error("Expected model field in Anthropic request")
		}

		// Return Anthropic-format response
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":   "msg_test123",
			"type": "message",
			"role": "assistant",
			"content": []map[string]interface{}{
				{"type": "text", "text": "Hello from passthrough!"},
			},
			"model":       "claude-3-5-sonnet-20241022",
			"stop_reason": "end_turn",
			"usage": map[string]int{
				"input_tokens":  10,
				"output_tokens": 5,
			},
		})
	}))
	defer mockServer.Close()

	// Create Anthropic provider pointing to mock server
	p := provider.NewAnthropicProviderWithKey(mockServer.URL, "test-key")

	// Verify passthrough mode
	if p.RequiresTransformation() {
		t.Error("Expected RequiresTransformation() to return false for passthrough")
	}

	// Verify endpoint URL
	expectedURL := mockServer.URL + "/v1/messages"
	if p.GetEndpointURL() != expectedURL {
		t.Errorf("Expected endpoint '%s', got '%s'", expectedURL, p.GetEndpointURL())
	}
}

func TestAnthropicPassthroughStreaming(t *testing.T) {
	// Create a mock streaming Anthropic API server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify streaming request
		body, _ := io.ReadAll(r.Body)
		var req map[string]interface{}
		json.Unmarshal(body, &req)

		if stream, ok := req["stream"].(bool); !ok || !stream {
			t.Error("Expected stream: true in request")
		}

		// Set SSE headers
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)

		// Write Anthropic SSE events
		events := []string{
			`event: message_start\ndata: {"type":"message_start","message":{"id":"msg_test","type":"message","role":"assistant","content":[],"model":"claude-3-5-sonnet-20241022","stop_reason":null,"usage":{"input_tokens":10,"output_tokens":0}}}`,
			`event: content_block_start\ndata: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
			`event: content_block_delta\ndata: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}`,
			`event: content_block_delta\ndata: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":" from"}}`,
			`event: content_block_delta\ndata: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":" passthrough!"}}`,
			`event: content_block_stop\ndata: {"type":"content_block_stop","index":0}`,
			`event: message_delta\ndata: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":3}}`,
			`event: message_stop\ndata: {"type":"message_stop"}`,
		}

		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Error("Expected ResponseWriter to support Flush")
			return
		}

		for _, event := range events {
			// Replace \n with actual newlines
			event = strings.ReplaceAll(event, "\\n", "\n")
			w.Write([]byte(event + "\n\n"))
			flusher.Flush()
		}
	}))
	defer mockServer.Close()

	// Create Anthropic provider
	p := provider.NewAnthropicProviderWithKey(mockServer.URL, "test-key")

	// Verify it's passthrough mode
	if p.RequiresTransformation() {
		t.Error("AnthropicProvider should not require transformation")
	}
	if !p.SupportsStreaming() {
		t.Error("AnthropicProvider should support streaming")
	}
}

func TestMultiProviderWithAnthropicTier(t *testing.T) {
	// Set up multi-provider with Anthropic as one tier
	os.Setenv("OPENAI_API_KEY", "test-openai-key")
	os.Setenv("ANTHROPIC_API_KEY", "test-anthropic-key")
	os.Setenv("CLASP_MULTI_PROVIDER", "true")
	os.Setenv("CLASP_OPUS_PROVIDER", "anthropic")
	os.Setenv("CLASP_OPUS_MODEL", "claude-3-opus-20240229")
	os.Setenv("CLASP_SONNET_PROVIDER", "openai")
	os.Setenv("CLASP_SONNET_MODEL", "gpt-4o")

	defer os.Unsetenv("OPENAI_API_KEY")
	defer os.Unsetenv("ANTHROPIC_API_KEY")
	defer os.Unsetenv("CLASP_MULTI_PROVIDER")
	defer os.Unsetenv("CLASP_OPUS_PROVIDER")
	defer os.Unsetenv("CLASP_OPUS_MODEL")
	defer os.Unsetenv("CLASP_SONNET_PROVIDER")
	defer os.Unsetenv("CLASP_SONNET_MODEL")

	cfg, err := config.LoadFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !cfg.MultiProviderEnabled {
		t.Error("Expected multi-provider to be enabled")
	}

	// Verify Opus tier is configured with Anthropic
	opusTier := cfg.GetTierConfig("claude-3-opus-20240229")
	if opusTier == nil {
		t.Fatal("Expected Opus tier config to exist")
	}
	if opusTier.Provider != config.ProviderAnthropic {
		t.Errorf("Expected Opus provider 'anthropic', got '%s'", opusTier.Provider)
	}
	if opusTier.Model != "claude-3-opus-20240229" {
		t.Errorf("Expected Opus model 'claude-3-opus-20240229', got '%s'", opusTier.Model)
	}
	// API key should be inherited from main anthropic config
	if opusTier.APIKey != "test-anthropic-key" {
		t.Errorf("Expected Opus API key to be inherited, got '%s'", opusTier.APIKey)
	}

	// Verify Sonnet tier is configured with OpenAI (still requires transformation)
	sonnetTier := cfg.GetTierConfig("claude-3-5-sonnet-20241022")
	if sonnetTier == nil {
		t.Fatal("Expected Sonnet tier config to exist")
	}
	if sonnetTier.Provider != config.ProviderOpenAI {
		t.Errorf("Expected Sonnet provider 'openai', got '%s'", sonnetTier.Provider)
	}
}

func TestProviderRequiresTransformation(t *testing.T) {
	tests := []struct {
		name           string
		provider       provider.Provider
		needsTransform bool
	}{
		{
			name:           "OpenAI requires transformation",
			provider:       provider.NewOpenAIProvider(""),
			needsTransform: true,
		},
		{
			name:           "OpenRouter requires transformation",
			provider:       provider.NewOpenRouterProvider(""),
			needsTransform: true,
		},
		{
			name:           "Custom requires transformation",
			provider:       provider.NewCustomProvider("http://localhost:11434/v1"),
			needsTransform: true,
		},
		{
			name:           "Anthropic does NOT require transformation",
			provider:       provider.NewAnthropicProvider(""),
			needsTransform: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.provider.RequiresTransformation()
			if result != tt.needsTransform {
				t.Errorf("Expected RequiresTransformation() = %v, got %v", tt.needsTransform, result)
			}
		})
	}
}
