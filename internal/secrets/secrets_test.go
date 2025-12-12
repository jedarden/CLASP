package secrets

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMaskAPIKey(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		expected string
	}{
		{"empty string", "", ""},
		{"short key", "abc", "***"},
		{"8 char key", "12345678", "***"},
		{"normal openai key", "sk-1234567890abcdefghij", "sk-1...ghij"},
		{"openrouter key", "sk-or-v1-1234567890abcdefghij", "sk-o...ghij"},
		{"anthropic key", "sk-ant-api01-1234567890abcdefghij", "sk-a...ghij"},
		{"azure key", "abcdefghijklmnopqrstuvwxyz123456", "abcd...3456"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskAPIKey(tt.key)
			if result != tt.expected {
				t.Errorf("MaskAPIKey(%q) = %q, want %q", tt.key, result, tt.expected)
			}
		})
	}
}

func TestMaskBearer(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains string
	}{
		{"bearer token", "Bearer sk-1234567890abcdefghij", "Bearer sk-1...ghij"},
		{"no token", "Hello World", "Hello World"},
		{"mixed content", "Auth: Bearer mytoken1234567890abcdef after", "Bearer myto...cdef"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskBearer(tt.input)
			if !strings.Contains(result, tt.contains) {
				t.Errorf("MaskBearer(%q) = %q, should contain %q", tt.input, result, tt.contains)
			}
		})
	}
}

func TestMaskAllSecrets(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		shouldNotMatch string // Original secret should not appear in output
	}{
		{
			"openai key",
			"Using key sk-1234567890abcdefghijklmnop",
			"sk-1234567890abcdefghijklmnop",
		},
		{
			"openrouter key",
			"Key is sk-or-v1-1234567890abcdefghijklmnop",
			"sk-or-v1-1234567890abcdefghijklmnop",
		},
		{
			"bearer token",
			"Authorization: Bearer sk-1234567890abcdefghij",
			"sk-1234567890abcdefghij",
		},
		{
			"multiple secrets",
			"Key1: sk-aaaa1234567890abcdef Key2: sk-bbbb1234567890abcdef",
			"sk-aaaa1234567890abcdef",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskAllSecrets(tt.input)
			if strings.Contains(result, tt.shouldNotMatch) {
				t.Errorf("MaskAllSecrets(%q) = %q, should not contain %q", tt.input, result, tt.shouldNotMatch)
			}
		})
	}
}

func TestMaskJSONSecrets(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		shouldNotMatch string
	}{
		{
			"api_key field",
			`{"api_key": "sk-1234567890abcdefghij", "model": "gpt-4"}`,
			"sk-1234567890abcdefghij",
		},
		{
			"authorization field",
			`{"authorization": "Bearer sk-1234567890abcdefghij"}`,
			"sk-1234567890abcdefghij",
		},
		{
			"nested api_key",
			`{"config": {"api_key": "sk-1234567890abcdefghij"}}`,
			"sk-1234567890abcdefghij",
		},
		{
			"array with secrets",
			`{"keys": [{"api_key": "sk-1234567890abcdefghij"}]}`,
			"sk-1234567890abcdefghij",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskJSONSecrets([]byte(tt.input))
			if strings.Contains(string(result), tt.shouldNotMatch) {
				t.Errorf("MaskJSONSecrets(%q) = %q, should not contain %q", tt.input, string(result), tt.shouldNotMatch)
			}
		})
	}
}

func TestMaskJSONSecrets_PreservesStructure(t *testing.T) {
	input := `{"model": "gpt-4", "api_key": "sk-1234567890abcdefghij", "messages": []}`
	result := MaskJSONSecrets([]byte(input))

	// Should be valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Errorf("Result is not valid JSON: %v", err)
	}

	// Should still have the model field
	if model, ok := parsed["model"].(string); !ok || model != "gpt-4" {
		t.Errorf("Model field was lost or modified: %v", parsed["model"])
	}

	// api_key should be masked
	if apiKey, ok := parsed["api_key"].(string); ok {
		if strings.Contains(apiKey, "sk-1234567890abcdefghij") {
			t.Errorf("API key was not masked: %s", apiKey)
		}
	}
}

func TestSanitizeHeaders(t *testing.T) {
	headers := map[string][]string{
		"Content-Type":  {"application/json"},
		"Authorization": {"Bearer sk-1234567890abcdefghij"},
		"X-Api-Key":     {"sk-another1234567890abcd"},
		"User-Agent":    {"CLASP/1.0"},
	}

	result := SanitizeHeaders(headers)

	// Content-Type and User-Agent should be unchanged
	if result["Content-Type"][0] != "application/json" {
		t.Errorf("Content-Type was modified: %v", result["Content-Type"])
	}
	if result["User-Agent"][0] != "CLASP/1.0" {
		t.Errorf("User-Agent was modified: %v", result["User-Agent"])
	}

	// Authorization should be masked
	if strings.Contains(result["Authorization"][0], "sk-1234567890abcdefghij") {
		t.Errorf("Authorization was not masked: %v", result["Authorization"])
	}

	// X-Api-Key should be masked
	if strings.Contains(result["X-Api-Key"][0], "sk-another1234567890abcd") {
		t.Errorf("X-Api-Key was not masked: %v", result["X-Api-Key"])
	}
}

func TestIsPotentialSecret(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"openai key", "sk-1234567890abcdefghij", true},
		{"short string", "hello", false},
		{"normal text", "This is a normal sentence", false},
		{"high entropy", "aB3dE5gH7jK9mN1pQ3sT5vX7", true},
		{"low entropy", "aaaaaaaaaaaaaaaaaaaaaa", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsPotentialSecret(tt.input)
			if result != tt.expected {
				t.Errorf("IsPotentialSecret(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFormatKeySource(t *testing.T) {
	tests := []struct {
		name         string
		envVarName   string
		hasDirectKey bool
		expected     string
	}{
		{"from env var", "OPENAI_API_KEY", false, "${OPENAI_API_KEY}"},
		{"from profile", "", true, "(stored in profile)"},
		{"not configured", "", false, "(not configured)"},
		{"env var takes precedence", "MY_KEY", true, "${MY_KEY}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatKeySource(tt.envVarName, tt.hasDirectKey)
			if result != tt.expected {
				t.Errorf("FormatKeySource(%q, %v) = %q, want %q", tt.envVarName, tt.hasDirectKey, result, tt.expected)
			}
		})
	}
}

func TestIsSensitiveField(t *testing.T) {
	sensitiveNames := []string{
		"api_key", "apiKey", "API_KEY",
		"authorization", "Authorization",
		"x-api-key", "X-Api-Key",
		"secret", "SECRET",
		"password", "PASSWORD",
		"token", "TOKEN",
		"credential", "credentials",
	}

	for _, name := range sensitiveNames {
		if !isSensitiveField(name) {
			t.Errorf("isSensitiveField(%q) = false, want true", name)
		}
	}

	nonSensitiveNames := []string{
		"model", "temperature", "max_tokens",
		"content", "role", "messages",
	}

	for _, name := range nonSensitiveNames {
		if isSensitiveField(name) {
			t.Errorf("isSensitiveField(%q) = true, want false", name)
		}
	}
}

// Benchmark tests
func BenchmarkMaskAPIKey(b *testing.B) {
	key := "sk-1234567890abcdefghijklmnopqrstuvwxyz"
	for i := 0; i < b.N; i++ {
		MaskAPIKey(key)
	}
}

func BenchmarkMaskAllSecrets(b *testing.B) {
	text := "Using API key sk-1234567890abcdefghij and Bearer sk-another1234567890"
	for i := 0; i < b.N; i++ {
		MaskAllSecrets(text)
	}
}

func BenchmarkMaskJSONSecrets(b *testing.B) {
	jsonData := []byte(`{"api_key": "sk-1234567890abcdefghij", "model": "gpt-4", "config": {"secret": "mysecret123456"}}`)
	for i := 0; i < b.N; i++ {
		MaskJSONSecrets(jsonData)
	}
}
