// +build integration

package tests

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jedarden/clasp/internal/config"
	"github.com/jedarden/clasp/internal/proxy"
	"github.com/jedarden/clasp/pkg/models"
)

// TestIntegration_LiveOpenAI tests the full proxy with live OpenAI API
// Run with: go test -tags=integration ./tests/... -v
func TestIntegration_LiveOpenAI(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set, skipping integration test")
	}

	// Create config
	cfg := &config.Config{
		Provider:     config.ProviderOpenAI,
		OpenAIAPIKey: apiKey,
		OpenAIBaseURL: "https://api.openai.com/v1",
		DefaultModel: "gpt-4o-mini",
		Port:        8080,
	}

	// Create server
	server, err := proxy.NewServer(cfg)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Create test server
	handler, err := proxy.NewHandler(cfg)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}
	_ = server // suppress unused warning

	// Test request
	anthropicReq := models.AnthropicRequest{
		Model:     "claude-3-haiku-20240307",
		MaxTokens: 100,
		Stream:    false,
		Messages: []models.AnthropicMessage{
			{
				Role:    "user",
				Content: "Say 'Hello from CLASP' and nothing else.",
			},
		},
	}

	reqBody, _ := json.Marshal(anthropicReq)

	// Create test HTTP request
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", "test-key") // Anthropic format header

	// Record response
	rec := httptest.NewRecorder()
	handler.HandleMessages(rec, req)

	// Check response
	resp := rec.Result()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var anthropicResp models.AnthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&anthropicResp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Verify response structure
	if anthropicResp.Type != "message" {
		t.Errorf("Expected type 'message', got '%s'", anthropicResp.Type)
	}

	if anthropicResp.Role != "assistant" {
		t.Errorf("Expected role 'assistant', got '%s'", anthropicResp.Role)
	}

	if len(anthropicResp.Content) == 0 {
		t.Error("Expected content, got empty")
	}

	// Check content contains expected text
	if len(anthropicResp.Content) > 0 && anthropicResp.Content[0].Type == "text" {
		text := anthropicResp.Content[0].Text
		t.Logf("Response text: %s", text)
		if !strings.Contains(strings.ToLower(text), "hello") {
			t.Logf("Warning: Response doesn't contain 'hello': %s", text)
		}
	}
}

// TestIntegration_StreamingOpenAI tests streaming with live OpenAI API
func TestIntegration_StreamingOpenAI(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set, skipping integration test")
	}

	// Create config
	cfg := &config.Config{
		Provider:     config.ProviderOpenAI,
		OpenAIAPIKey: apiKey,
		OpenAIBaseURL: "https://api.openai.com/v1",
		DefaultModel: "gpt-4o-mini",
		Port:        8080,
	}

	handler, err := proxy.NewHandler(cfg)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}

	// Streaming request
	anthropicReq := models.AnthropicRequest{
		Model:     "claude-3-haiku-20240307",
		MaxTokens: 50,
		Stream:    true,
		Messages: []models.AnthropicMessage{
			{
				Role:    "user",
				Content: "Count to 3.",
			},
		},
	}

	reqBody, _ := json.Marshal(anthropicReq)
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	// Record streaming response
	rec := httptest.NewRecorder()
	handler.HandleMessages(rec, req)

	resp := rec.Result()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, string(body))
	}

	// Verify content type
	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "text/event-stream") {
		t.Errorf("Expected Content-Type 'text/event-stream', got '%s'", contentType)
	}

	// Read and verify stream
	body, _ := io.ReadAll(resp.Body)
	output := string(body)

	t.Logf("Streaming response:\n%s", output)

	// Verify SSE events
	requiredEvents := []string{
		"event: message_start",
		"event: content_block_start",
		"event: content_block_delta",
		"event: message_stop",
		"data: [DONE]",
	}

	for _, event := range requiredEvents {
		if !strings.Contains(output, event) {
			t.Errorf("Missing required event: %s", event)
		}
	}
}

// TestIntegration_ToolCall tests tool calling with live OpenAI API
func TestIntegration_ToolCallOpenAI(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set, skipping integration test")
	}

	cfg := &config.Config{
		Provider:     config.ProviderOpenAI,
		OpenAIAPIKey: apiKey,
		OpenAIBaseURL: "https://api.openai.com/v1",
		DefaultModel: "gpt-4o-mini",
		Port:        8080,
	}

	handler, err := proxy.NewHandler(cfg)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}

	// Request with tools
	anthropicReq := models.AnthropicRequest{
		Model:     "claude-3-haiku-20240307",
		MaxTokens: 200,
		Stream:    false,
		Messages: []models.AnthropicMessage{
			{
				Role:    "user",
				Content: "Get the current weather in San Francisco.",
			},
		},
		Tools: []models.AnthropicTool{
			{
				Name:        "get_weather",
				Description: "Get the current weather for a location",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"location": map[string]interface{}{
							"type":        "string",
							"description": "City name",
						},
					},
					"required": []string{"location"},
				},
			},
		},
		ToolChoice: map[string]interface{}{
			"type": "any",
		},
	}

	reqBody, _ := json.Marshal(anthropicReq)
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.HandleMessages(rec, req)

	resp := rec.Result()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, string(body))
	}

	var anthropicResp models.AnthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&anthropicResp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	t.Logf("Response: %+v", anthropicResp)

	// Check for tool_use in response
	hasToolUse := false
	for _, block := range anthropicResp.Content {
		if block.Type == "tool_use" {
			hasToolUse = true
			t.Logf("Tool call: %s (id: %s)", block.Name, block.ID)
			if block.Name != "get_weather" {
				t.Errorf("Expected tool name 'get_weather', got '%s'", block.Name)
			}
			if block.ID == "" {
				t.Error("Tool call ID should not be empty")
			}
		}
	}

	if !hasToolUse {
		t.Log("No tool_use in response (model might have responded without tool call)")
	}
}

// TestIntegration_HealthCheck tests the health endpoint
func TestIntegration_HealthCheck(t *testing.T) {
	cfg := &config.Config{
		Provider:     config.ProviderOpenAI,
		OpenAIAPIKey: "test-key",
		OpenAIBaseURL: "https://api.openai.com/v1",
		DefaultModel: "gpt-4o",
		Port:        8080,
	}

	handler, err := proxy.NewHandler(cfg)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	handler.HandleHealth(rec, req)

	resp := rec.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", resp.StatusCode)
	}

	var health map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if health["status"] != "healthy" {
		t.Errorf("Expected status 'healthy', got '%s'", health["status"])
	}

	if health["provider"] != "openai" {
		t.Errorf("Expected provider 'openai', got '%s'", health["provider"])
	}
}

// TestIntegration_PrometheusMetrics tests the Prometheus metrics endpoint
func TestIntegration_PrometheusMetrics(t *testing.T) {
	cfg := &config.Config{
		Provider:     config.ProviderOpenAI,
		OpenAIAPIKey: "test-key",
		OpenAIBaseURL: "https://api.openai.com/v1",
		DefaultModel: "gpt-4o",
		Port:        8080,
	}

	handler, err := proxy.NewHandler(cfg)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/metrics/prometheus", nil)
	rec := httptest.NewRecorder()
	handler.HandleMetricsPrometheus(rec, req)

	resp := rec.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", resp.StatusCode)
	}

	// Verify content type
	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "text/plain") {
		t.Errorf("Expected Content-Type 'text/plain', got '%s'", contentType)
	}

	// Read body
	body, _ := io.ReadAll(resp.Body)
	output := string(body)

	t.Logf("Prometheus metrics:\n%s", output)

	// Verify required metrics
	requiredMetrics := []string{
		"clasp_requests_total",
		"clasp_requests_successful",
		"clasp_requests_errors",
		"clasp_requests_streaming",
		"clasp_requests_tool_calls",
		"clasp_latency_total_ms",
		"clasp_uptime_seconds",
		"clasp_latency_avg_ms",
		"clasp_requests_per_second",
	}

	for _, metric := range requiredMetrics {
		if !strings.Contains(output, metric) {
			t.Errorf("Missing required metric: %s", metric)
		}
	}

	// Verify HELP and TYPE comments
	if !strings.Contains(output, "# HELP") {
		t.Error("Missing HELP comments in Prometheus output")
	}
	if !strings.Contains(output, "# TYPE") {
		t.Error("Missing TYPE comments in Prometheus output")
	}

	// Verify provider label
	if !strings.Contains(output, `provider="openai"`) {
		t.Error("Missing provider label in metrics")
	}
}

// TestIntegration_Timeout tests request timeout handling
func TestIntegration_Timeout(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set, skipping integration test")
	}

	cfg := &config.Config{
		Provider:     config.ProviderOpenAI,
		OpenAIAPIKey: apiKey,
		OpenAIBaseURL: "https://api.openai.com/v1",
		DefaultModel: "gpt-4o-mini",
		Port:        8080,
	}

	handler, err := proxy.NewHandler(cfg)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}

	anthropicReq := models.AnthropicRequest{
		Model:     "claude-3-haiku-20240307",
		MaxTokens: 10,
		Stream:    false,
		Messages: []models.AnthropicMessage{
			{
				Role:    "user",
				Content: "Hi",
			},
		},
	}

	reqBody, _ := json.Marshal(anthropicReq)
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	// Set a reasonable timeout
	start := time.Now()
	rec := httptest.NewRecorder()
	handler.HandleMessages(rec, req)
	duration := time.Since(start)

	resp := rec.Result()

	// Should complete within reasonable time (30 seconds)
	if duration > 30*time.Second {
		t.Errorf("Request took too long: %v", duration)
	}

	t.Logf("Request completed in %v with status %d", duration, resp.StatusCode)
}
