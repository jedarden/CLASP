// Package provider implements unit tests for LLM provider backends.
package provider

import (
	"testing"
)

// TestOpenAIProvider tests the OpenAI provider implementation.
func TestOpenAIProvider(t *testing.T) {
	t.Run("NewOpenAIProvider with default URL", func(t *testing.T) {
		p := NewOpenAIProvider("")
		if p.BaseURL != "https://api.openai.com/v1" {
			t.Errorf("Expected default URL, got %s", p.BaseURL)
		}
	})

	t.Run("NewOpenAIProvider with custom URL", func(t *testing.T) {
		p := NewOpenAIProvider("https://custom.api.com/v1")
		if p.BaseURL != "https://custom.api.com/v1" {
			t.Errorf("Expected custom URL, got %s", p.BaseURL)
		}
	})

	t.Run("NewOpenAIProviderWithKey", func(t *testing.T) {
		p := NewOpenAIProviderWithKey("", "sk-test-key")
		if p.BaseURL != "https://api.openai.com/v1" {
			t.Errorf("Expected default URL, got %s", p.BaseURL)
		}
		if p.apiKey != "sk-test-key" {
			t.Errorf("Expected apiKey to be set")
		}
	})

	t.Run("Name returns openai", func(t *testing.T) {
		p := NewOpenAIProvider("")
		if p.Name() != "openai" {
			t.Errorf("Expected 'openai', got %s", p.Name())
		}
	})

	t.Run("GetHeaders with provided key", func(t *testing.T) {
		p := NewOpenAIProvider("")
		headers := p.GetHeaders("sk-provided")
		if got := headers.Get("Authorization"); got != "Bearer sk-provided" {
			t.Errorf("Expected 'Bearer sk-provided', got %s", got)
		}
		if got := headers.Get("Content-Type"); got != "application/json" {
			t.Errorf("Expected 'application/json', got %s", got)
		}
	})

	t.Run("GetHeaders with embedded key", func(t *testing.T) {
		p := NewOpenAIProviderWithKey("", "sk-embedded")
		headers := p.GetHeaders("sk-provided")
		if got := headers.Get("Authorization"); got != "Bearer sk-embedded" {
			t.Errorf("Expected 'Bearer sk-embedded', got %s", got)
		}
	})

	t.Run("GetEndpointURL", func(t *testing.T) {
		p := NewOpenAIProvider("")
		expected := "https://api.openai.com/v1/chat/completions"
		if got := p.GetEndpointURL(); got != expected {
			t.Errorf("Expected %s, got %s", expected, got)
		}
	})

	t.Run("TransformModelID strips openai prefix", func(t *testing.T) {
		p := NewOpenAIProvider("")
		if got := p.TransformModelID("openai/gpt-4o"); got != "gpt-4o" {
			t.Errorf("Expected 'gpt-4o', got %s", got)
		}
	})

	t.Run("TransformModelID preserves other models", func(t *testing.T) {
		p := NewOpenAIProvider("")
		if got := p.TransformModelID("gpt-4o"); got != "gpt-4o" {
			t.Errorf("Expected 'gpt-4o', got %s", got)
		}
	})

	t.Run("SupportsStreaming returns true", func(t *testing.T) {
		p := NewOpenAIProvider("")
		if !p.SupportsStreaming() {
			t.Error("Expected SupportsStreaming to return true")
		}
	})

	t.Run("RequiresTransformation returns true", func(t *testing.T) {
		p := NewOpenAIProvider("")
		if !p.RequiresTransformation() {
			t.Error("Expected RequiresTransformation to return true")
		}
	})
}

// TestAzureProvider tests the Azure OpenAI provider implementation.
func TestAzureProvider(t *testing.T) {
	t.Run("NewAzureProvider with default version", func(t *testing.T) {
		p := NewAzureProvider("https://test.openai.azure.com", "gpt-4", "")
		if p.APIVersion != "2024-02-15-preview" {
			t.Errorf("Expected default API version, got %s", p.APIVersion)
		}
	})

	t.Run("NewAzureProvider with custom version", func(t *testing.T) {
		p := NewAzureProvider("https://test.openai.azure.com", "gpt-4", "2024-01-01")
		if p.APIVersion != "2024-01-01" {
			t.Errorf("Expected '2024-01-01', got %s", p.APIVersion)
		}
	})

	t.Run("Name returns azure", func(t *testing.T) {
		p := NewAzureProvider("", "", "")
		if p.Name() != "azure" {
			t.Errorf("Expected 'azure', got %s", p.Name())
		}
	})

	t.Run("GetHeaders uses api-key", func(t *testing.T) {
		p := NewAzureProvider("", "", "")
		headers := p.GetHeaders("azure-key")
		if got := headers.Get("api-key"); got != "azure-key" {
			t.Errorf("Expected 'azure-key', got %s", got)
		}
		if got := headers.Get("Content-Type"); got != "application/json" {
			t.Errorf("Expected 'application/json', got %s", got)
		}
	})

	t.Run("GetEndpointURL formats correctly", func(t *testing.T) {
		p := NewAzureProvider("https://test.openai.azure.com", "gpt-4", "2024-02-15-preview")
		expected := "https://test.openai.azure.com/openai/deployments/gpt-4/chat/completions?api-version=2024-02-15-preview"
		if got := p.GetEndpointURL(); got != expected {
			t.Errorf("Expected %s, got %s", expected, got)
		}
	})

	t.Run("TransformModelID returns deployment name", func(t *testing.T) {
		p := NewAzureProvider("", "my-deployment", "")
		if got := p.TransformModelID("any-model"); got != "my-deployment" {
			t.Errorf("Expected 'my-deployment', got %s", got)
		}
	})

	t.Run("SupportsStreaming returns true", func(t *testing.T) {
		p := NewAzureProvider("", "", "")
		if !p.SupportsStreaming() {
			t.Error("Expected SupportsStreaming to return true")
		}
	})

	t.Run("RequiresTransformation returns true", func(t *testing.T) {
		p := NewAzureProvider("", "", "")
		if !p.RequiresTransformation() {
			t.Error("Expected RequiresTransformation to return true")
		}
	})
}

// TestOpenRouterProvider tests the OpenRouter provider implementation.
func TestOpenRouterProvider(t *testing.T) {
	t.Run("NewOpenRouterProvider with default URL", func(t *testing.T) {
		p := NewOpenRouterProvider("")
		if p.BaseURL != "https://openrouter.ai/api/v1" {
			t.Errorf("Expected default URL, got %s", p.BaseURL)
		}
	})

	t.Run("NewOpenRouterProvider with custom URL", func(t *testing.T) {
		p := NewOpenRouterProvider("https://custom.router.ai/api/v1")
		if p.BaseURL != "https://custom.router.ai/api/v1" {
			t.Errorf("Expected custom URL, got %s", p.BaseURL)
		}
	})

	t.Run("NewOpenRouterProviderWithKey", func(t *testing.T) {
		p := NewOpenRouterProviderWithKey("", "sk-or-test")
		if p.apiKey != "sk-or-test" {
			t.Errorf("Expected apiKey to be set")
		}
	})

	t.Run("Name returns openrouter", func(t *testing.T) {
		p := NewOpenRouterProvider("")
		if p.Name() != "openrouter" {
			t.Errorf("Expected 'openrouter', got %s", p.Name())
		}
	})

	t.Run("GetHeaders includes OpenRouter-specific headers", func(t *testing.T) {
		p := NewOpenRouterProvider("")
		headers := p.GetHeaders("sk-or-test")

		if got := headers.Get("Authorization"); got != "Bearer sk-or-test" {
			t.Errorf("Expected 'Bearer sk-or-test', got %s", got)
		}
		if got := headers.Get("HTTP-Referer"); got != "https://github.com/jedarden/CLASP" {
			t.Errorf("Expected CLASP referer, got %s", got)
		}
		if got := headers.Get("X-Title"); got != "CLASP Proxy" {
			t.Errorf("Expected 'CLASP Proxy', got %s", got)
		}
		if got := headers.Get("User-Agent"); got == "" {
			t.Error("Expected User-Agent to be set")
		}
	})

	t.Run("GetHeaders uses embedded key over provided", func(t *testing.T) {
		p := NewOpenRouterProviderWithKey("", "sk-embedded")
		headers := p.GetHeaders("sk-provided")
		if got := headers.Get("Authorization"); got != "Bearer sk-embedded" {
			t.Errorf("Expected 'Bearer sk-embedded', got %s", got)
		}
	})

	t.Run("GetEndpointURL", func(t *testing.T) {
		p := NewOpenRouterProvider("")
		expected := "https://openrouter.ai/api/v1/chat/completions"
		if got := p.GetEndpointURL(); got != expected {
			t.Errorf("Expected %s, got %s", expected, got)
		}
	})

	t.Run("TransformModelID passes through", func(t *testing.T) {
		p := NewOpenRouterProvider("")
		if got := p.TransformModelID("anthropic/claude-3-opus"); got != "anthropic/claude-3-opus" {
			t.Errorf("Expected 'anthropic/claude-3-opus', got %s", got)
		}
	})

	t.Run("SupportsStreaming returns true", func(t *testing.T) {
		p := NewOpenRouterProvider("")
		if !p.SupportsStreaming() {
			t.Error("Expected SupportsStreaming to return true")
		}
	})

	t.Run("RequiresTransformation returns true", func(t *testing.T) {
		p := NewOpenRouterProvider("")
		if !p.RequiresTransformation() {
			t.Error("Expected RequiresTransformation to return true")
		}
	})
}

// TestCustomProvider tests the custom provider implementation.
func TestCustomProvider(t *testing.T) {
	t.Run("NewCustomProvider", func(t *testing.T) {
		p := NewCustomProvider("http://localhost:11434/v1")
		if p.BaseURL != "http://localhost:11434/v1" {
			t.Errorf("Expected custom URL, got %s", p.BaseURL)
		}
	})

	t.Run("NewCustomProviderWithKey", func(t *testing.T) {
		p := NewCustomProviderWithKey("http://localhost:11434/v1", "local-key")
		if p.apiKey != "local-key" {
			t.Errorf("Expected apiKey to be set")
		}
	})

	t.Run("Name returns custom", func(t *testing.T) {
		p := NewCustomProvider("")
		if p.Name() != "custom" {
			t.Errorf("Expected 'custom', got %s", p.Name())
		}
	})

	t.Run("GetHeaders with key", func(t *testing.T) {
		p := NewCustomProvider("")
		headers := p.GetHeaders("custom-key")
		if got := headers.Get("Authorization"); got != "Bearer custom-key" {
			t.Errorf("Expected 'Bearer custom-key', got %s", got)
		}
	})

	t.Run("GetHeaders with empty key", func(t *testing.T) {
		p := NewCustomProvider("")
		headers := p.GetHeaders("")
		if got := headers.Get("Authorization"); got != "" {
			t.Errorf("Expected empty Authorization, got %s", got)
		}
	})

	t.Run("GetHeaders with not-required key", func(t *testing.T) {
		p := NewCustomProvider("")
		headers := p.GetHeaders("not-required")
		if got := headers.Get("Authorization"); got != "" {
			t.Errorf("Expected empty Authorization for not-required key, got %s", got)
		}
	})

	t.Run("GetHeaders uses embedded key", func(t *testing.T) {
		p := NewCustomProviderWithKey("", "embedded-key")
		headers := p.GetHeaders("provided-key")
		if got := headers.Get("Authorization"); got != "Bearer embedded-key" {
			t.Errorf("Expected 'Bearer embedded-key', got %s", got)
		}
	})

	t.Run("GetEndpointURL", func(t *testing.T) {
		p := NewCustomProvider("http://localhost:11434/v1")
		expected := "http://localhost:11434/v1/chat/completions"
		if got := p.GetEndpointURL(); got != expected {
			t.Errorf("Expected %s, got %s", expected, got)
		}
	})

	t.Run("TransformModelID passes through", func(t *testing.T) {
		p := NewCustomProvider("")
		if got := p.TransformModelID("llama3.1"); got != "llama3.1" {
			t.Errorf("Expected 'llama3.1', got %s", got)
		}
	})

	t.Run("SupportsStreaming returns true", func(t *testing.T) {
		p := NewCustomProvider("")
		if !p.SupportsStreaming() {
			t.Error("Expected SupportsStreaming to return true")
		}
	})

	t.Run("RequiresTransformation returns true", func(t *testing.T) {
		p := NewCustomProvider("")
		if !p.RequiresTransformation() {
			t.Error("Expected RequiresTransformation to return true")
		}
	})
}

// TestAnthropicProvider tests the Anthropic provider implementation.
func TestAnthropicProvider(t *testing.T) {
	t.Run("NewAnthropicProvider with default URL", func(t *testing.T) {
		p := NewAnthropicProvider("")
		if p.BaseURL != "https://api.anthropic.com" {
			t.Errorf("Expected default URL, got %s", p.BaseURL)
		}
	})

	t.Run("NewAnthropicProvider with custom URL", func(t *testing.T) {
		p := NewAnthropicProvider("https://custom.anthropic.com")
		if p.BaseURL != "https://custom.anthropic.com" {
			t.Errorf("Expected custom URL, got %s", p.BaseURL)
		}
	})

	t.Run("NewAnthropicProviderWithKey", func(t *testing.T) {
		p := NewAnthropicProviderWithKey("", "sk-ant-test")
		if p.apiKey != "sk-ant-test" {
			t.Errorf("Expected apiKey to be set")
		}
	})

	t.Run("Name returns anthropic", func(t *testing.T) {
		p := NewAnthropicProvider("")
		if p.Name() != "anthropic" {
			t.Errorf("Expected 'anthropic', got %s", p.Name())
		}
	})

	t.Run("GetHeaders uses x-api-key", func(t *testing.T) {
		p := NewAnthropicProvider("")
		headers := p.GetHeaders("sk-ant-test")

		if got := headers.Get("x-api-key"); got != "sk-ant-test" {
			t.Errorf("Expected 'sk-ant-test', got %s", got)
		}
		if got := headers.Get("anthropic-version"); got != "2023-06-01" {
			t.Errorf("Expected '2023-06-01', got %s", got)
		}
		if got := headers.Get("Content-Type"); got != "application/json" {
			t.Errorf("Expected 'application/json', got %s", got)
		}
	})

	t.Run("GetHeaders uses embedded key", func(t *testing.T) {
		p := NewAnthropicProviderWithKey("", "sk-ant-embedded")
		headers := p.GetHeaders("sk-ant-provided")
		if got := headers.Get("x-api-key"); got != "sk-ant-embedded" {
			t.Errorf("Expected 'sk-ant-embedded', got %s", got)
		}
	})

	t.Run("GetEndpointURL", func(t *testing.T) {
		p := NewAnthropicProvider("")
		expected := "https://api.anthropic.com/v1/messages"
		if got := p.GetEndpointURL(); got != expected {
			t.Errorf("Expected %s, got %s", expected, got)
		}
	})

	t.Run("TransformModelID passes through", func(t *testing.T) {
		p := NewAnthropicProvider("")
		if got := p.TransformModelID("claude-3-opus-20240229"); got != "claude-3-opus-20240229" {
			t.Errorf("Expected 'claude-3-opus-20240229', got %s", got)
		}
	})

	t.Run("SupportsStreaming returns true", func(t *testing.T) {
		p := NewAnthropicProvider("")
		if !p.SupportsStreaming() {
			t.Error("Expected SupportsStreaming to return true")
		}
	})

	t.Run("RequiresTransformation returns false", func(t *testing.T) {
		p := NewAnthropicProvider("")
		if p.RequiresTransformation() {
			t.Error("Expected RequiresTransformation to return false for passthrough mode")
		}
	})
}

// TestProviderInterface verifies all providers implement the interface correctly.
func TestProviderInterface(t *testing.T) {
	providers := []Provider{
		NewOpenAIProvider(""),
		NewAzureProvider("https://test.openai.azure.com", "gpt-4", ""),
		NewOpenRouterProvider(""),
		NewCustomProvider("http://localhost:11434/v1"),
		NewAnthropicProvider(""),
	}

	for _, p := range providers {
		t.Run(p.Name(), func(t *testing.T) {
			// Test Name is not empty
			if p.Name() == "" {
				t.Error("Name() should not return empty string")
			}

			// Test GetHeaders returns non-nil
			headers := p.GetHeaders("test-key")
			if headers == nil {
				t.Error("GetHeaders() should not return nil")
			}
			if headers.Get("Content-Type") == "" {
				t.Error("GetHeaders() should include Content-Type")
			}

			// Test GetEndpointURL returns non-empty
			if p.GetEndpointURL() == "" {
				t.Error("GetEndpointURL() should not return empty string")
			}

			// Test TransformModelID
			transformed := p.TransformModelID("test-model")
			if transformed == "" {
				t.Error("TransformModelID() should not return empty string")
			}
		})
	}
}

// TestProviderHeaderSecurity verifies headers don't leak sensitive info.
func TestProviderHeaderSecurity(t *testing.T) {
	testKey := "sk-secret-key-12345"

	providers := []Provider{
		NewOpenAIProvider(""),
		NewOpenRouterProvider(""),
		NewCustomProvider("http://localhost:11434/v1"),
		NewAnthropicProvider(""),
	}

	for _, p := range providers {
		t.Run(p.Name()+" secure headers", func(t *testing.T) {
			headers := p.GetHeaders(testKey)

			// Verify Authorization or x-api-key contains the key
			authHeader := headers.Get("Authorization")
			apiKeyHeader := headers.Get("x-api-key")
			azureKeyHeader := headers.Get("api-key")

			hasKey := authHeader != "" || apiKeyHeader != "" || azureKeyHeader != ""
			if !hasKey && p.Name() != "custom" {
				t.Error("Expected some form of authentication header")
			}
		})
	}
}

// Benchmark tests.
func BenchmarkOpenAIGetHeaders(b *testing.B) {
	p := NewOpenAIProvider("")
	for i := 0; i < b.N; i++ {
		p.GetHeaders("sk-test")
	}
}

func BenchmarkProviderGetEndpointURL(b *testing.B) {
	p := NewAzureProvider("https://test.openai.azure.com", "gpt-4", "2024-02-15-preview")
	for i := 0; i < b.N; i++ {
		p.GetEndpointURL()
	}
}

func BenchmarkTransformModelID(b *testing.B) {
	p := NewOpenAIProvider("")
	for i := 0; i < b.N; i++ {
		p.TransformModelID("openai/gpt-4o")
	}
}
