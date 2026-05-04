package tests

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jedarden/clasp/internal/config"
	"github.com/jedarden/clasp/internal/proxy"
	"github.com/jedarden/clasp/pkg/models"
)

// TestAzurePlusGPT5Model_ReturnsError tests that Azure provider + gpt-5 models
// return a clear error instead of failing silently or with an opaque upstream error.
func TestAzurePlusGPT5Model_ReturnsError(t *testing.T) {
	cfg := &config.Config{
		Provider:           config.ProviderAzure,
		AzureAPIKey:        "test-key",
		AzureEndpoint:      "https://test.openai.azure.com",
		AzureDeploymentName: "gpt-4",
		DefaultModel:       "gpt-4",
		Port:               8080,
	}

	handler, err := proxy.NewHandler(cfg)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}

	// Test with gpt-5 model (requires Responses API)
	testCases := []struct {
		name          string
		model         string
		expectError   bool
		errorContains string
	}{
		{
			name:          "gpt-5 model",
			model:         "gpt-5",
			expectError:   true,
			errorContains: "Azure OpenAI does not support the Responses API",
		},
		{
			name:          "gpt-5.1 model",
			model:         "gpt-5.1",
			expectError:   true,
			errorContains: "Azure OpenAI does not support the Responses API",
		},
		{
			name:          "gpt-5.1-codex model",
			model:         "gpt-5.1-codex",
			expectError:   true,
			errorContains: "Azure OpenAI does not support the Responses API",
		},
		{
			name:          "codex model",
			model:         "codex",
			expectError:   true,
			errorContains: "Azure OpenAI does not support the Responses API",
		},
		{
			name:        "gpt-4o model (should work)",
			model:       "gpt-4o",
			expectError: false,
		},
		{
			name:        "gpt-4o-mini model (should work)",
			model:       "gpt-4o-mini",
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			anthropicReq := models.AnthropicRequest{
				Model:     tc.model,
				MaxTokens: 100,
				Stream:    false,
				Messages: []models.AnthropicMessage{
					{
						Role:    "user",
						Content: "Hello",
					},
				},
			}

			reqBody, _ := json.Marshal(anthropicReq)
			req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")

			rec := httptest.NewRecorder()
			handler.HandleMessages(rec, req)

			resp := rec.Result()

			if tc.expectError {
				if resp.StatusCode != http.StatusBadRequest {
					body, _ := readBody(resp)
					t.Fatalf("Expected status 400, got %d: %s", resp.StatusCode, string(body))
				}

				// Check error response format
				var errResp map[string]interface{}
				if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
					t.Fatalf("Failed to parse error response: %v", err)
				}

				if errResp["type"] != "error" {
					t.Errorf("Expected type 'error', got %v", errResp["type"])
				}

				errDetails, ok := errResp["error"].(map[string]interface{})
				if !ok {
					t.Fatal("Expected error details in response")
				}

				if errDetails["type"] != "invalid_request_error" {
					t.Errorf("Expected error type 'invalid_request_error', got %v", errDetails["type"])
				}

				errMessage, ok := errDetails["message"].(string)
				if !ok {
					t.Fatal("Expected message string in error details")
				}

				if !containsString(errMessage, tc.errorContains) {
					t.Errorf("Expected error message to contain '%s', got '%s'", tc.errorContains, errMessage)
				}

				// Verify the error message mentions alternative providers
				if !containsString(errMessage, "openai") && !containsString(errMessage, "openrouter") {
					t.Errorf("Expected error message to suggest alternative providers (openai/openrouter), got '%s'", errMessage)
				}
			} else {
				// For valid models, we expect either:
				// 1. A non-400 status (request passed validation)
				// 2. Or a different error (auth, network, etc.) but NOT 400 with our validation error
				if resp.StatusCode == http.StatusBadRequest {
					body, _ := readBody(resp)
					var errResp map[string]interface{}
					if err := json.Unmarshal(body, &errResp); err == nil {
						if errDetails, ok := errResp["error"].(map[string]interface{}); ok {
							if msg, ok := errDetails["message"].(string); ok {
								if containsString(msg, "Azure OpenAI does not support the Responses API") {
									t.Errorf("Model '%s' should not trigger Responses API validation error, but got: %s", tc.model, msg)
								}
							}
						}
					}
				}
				// We don't assert success because we're not making real API calls
				// Just that validation passed
			}
		})
	}
}

// TestAzureTierProviderPlusGPT5Model_ReturnsError tests that tier-specific
// Azure providers also return a clear error for gpt-5 models.
func TestAzureTierProviderPlusGPT5Model_ReturnsError(t *testing.T) {
	cfg := &config.Config{
		Provider:           config.ProviderOpenAI,
		OpenAIAPIKey:       "test-key",
		OpenAIBaseURL:      "https://api.openai.com/v1",
		DefaultModel:       "gpt-4o",
		Port:               8080,
		MultiProviderEnabled: true,
		TierOpus: &config.TierConfig{
			Provider: "azure",
			Model:    "gpt-5", // Azure tier with gpt-5 model
			BaseURL:  "https://test.openai.azure.com",
			APIKey:   "test-key",
		},
	}

	handler, err := proxy.NewHandler(cfg)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}

	// Request that will route to Azure tier (claude-3-opus-20240229 maps to Opus tier)
	anthropicReq := models.AnthropicRequest{
		Model:     "claude-3-opus-20240229",
		MaxTokens: 100,
		Stream:    false,
		Messages: []models.AnthropicMessage{
			{
				Role:    "user",
				Content: "Hello",
			},
		},
	}

	reqBody, _ := json.Marshal(anthropicReq)
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.HandleMessages(rec, req)

	resp := rec.Result()

	// Should return 400 for Azure + gpt-5 combination
	if resp.StatusCode != http.StatusBadRequest {
		body, _ := readBody(resp)
		t.Fatalf("Expected status 400, got %d: %s", resp.StatusCode, string(body))
	}

	// Check error response format
	var errResp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("Failed to parse error response: %v", err)
	}

	errDetails, ok := errResp["error"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected error details in response")
	}

	errMessage, ok := errDetails["message"].(string)
	if !ok {
		t.Fatal("Expected message string in error details")
	}

	if !containsString(errMessage, "Azure OpenAI does not support the Responses API") {
		t.Errorf("Expected error message to mention Azure Responses API limitation, got '%s'", errMessage)
	}
}

// Helper function to read response body
func readBody(resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// Helper function to check if string contains substring
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && contains(s, substr))
}

// Helper function for substring search
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
