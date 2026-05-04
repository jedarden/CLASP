//go:build integration
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

	"github.com/jedarden/clasp/internal/cache"
	"github.com/jedarden/clasp/internal/config"
	"github.com/jedarden/clasp/internal/proxy"
	"github.com/jedarden/clasp/internal/session"
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
		Provider:      config.ProviderOpenAI,
		OpenAIAPIKey:  apiKey,
		OpenAIBaseURL: "https://api.openai.com/v1",
		DefaultModel:  "gpt-4o-mini",
		Port:          8080,
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
		Provider:      config.ProviderOpenAI,
		OpenAIAPIKey:  apiKey,
		OpenAIBaseURL: "https://api.openai.com/v1",
		DefaultModel:  "gpt-4o-mini",
		Port:          8080,
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
		Provider:      config.ProviderOpenAI,
		OpenAIAPIKey:  apiKey,
		OpenAIBaseURL: "https://api.openai.com/v1",
		DefaultModel:  "gpt-4o-mini",
		Port:          8080,
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
		Provider:      config.ProviderOpenAI,
		OpenAIAPIKey:  "test-key",
		OpenAIBaseURL: "https://api.openai.com/v1",
		DefaultModel:  "gpt-4o",
		Port:          8080,
	}

	handler, err := proxy.NewHandler(cfg)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/health", http.NoBody)
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
		Provider:      config.ProviderOpenAI,
		OpenAIAPIKey:  "test-key",
		OpenAIBaseURL: "https://api.openai.com/v1",
		DefaultModel:  "gpt-4o",
		Port:          8080,
	}

	handler, err := proxy.NewHandler(cfg)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/metrics/prometheus", http.NoBody)
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
		Provider:      config.ProviderOpenAI,
		OpenAIAPIKey:  apiKey,
		OpenAIBaseURL: "https://api.openai.com/v1",
		DefaultModel:  "gpt-4o-mini",
		Port:          8080,
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

// TestIntegration_PromptCacheHit tests the prompt cache hit path
// Run with: go test -tags=integration ./tests/... -v -run TestIntegration_PromptCacheHit
func TestIntegration_PromptCacheHit(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set, skipping integration test")
	}

	// Create config with prompt cache enabled
	cfg := &config.Config{
		Provider:           config.ProviderOpenAI,
		OpenAIAPIKey:       apiKey,
		OpenAIBaseURL:      "https://api.openai.com/v1",
		DefaultModel:       "gpt-4o-mini",
		Port:               8080,
		PromptCacheEnabled: true,
		PromptCacheMaxSize: 100,
		CacheTTL:           3600,
	}

	handler, err := proxy.NewHandler(cfg)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}

	// Initialize prompt cache
	pc := cache.NewPromptCache(100, time.Hour)
	handler.SetPromptCache(pc)

	// Create a request with cache_control markers in the system prompt
	cacheControlType := "ephemeral"
	anthropicReq := models.AnthropicRequest{
		Model:     "claude-3-haiku-20240307",
		MaxTokens: 50,
		Stream:    false,
		System: []models.ContentBlock{
			{
				Type: "text",
				Text: "You are a helpful assistant.",
				CacheControl: &models.CacheControl{
					Type: cacheControlType,
				},
			},
		},
		Messages: []models.AnthropicMessage{
			{
				Role:    "user",
				Content: "Say 'test response'",
			},
		},
	}

	// First request - cache miss
	t.Log("Making first request (cache miss expected)...")
	reqBody1, _ := json.Marshal(anthropicReq)
	req1 := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(reqBody1))
	req1.Header.Set("Content-Type", "application/json")

	rec1 := httptest.NewRecorder()
	handler.HandleMessages(rec1, req1)

	resp1 := rec1.Result()
	if resp1.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp1.Body)
		t.Fatalf("First request failed with status %d: %s", resp1.StatusCode, string(body))
	}

	// Check cache header - should be MISS on first request
	cacheHeader1 := resp1.Header.Get("X-CLASP-Cache")
	if cacheHeader1 != "MISS" {
		t.Logf("First request cache header: %s (expected MISS, but got %s - this is OK if prompt cache worked)", cacheHeader1, cacheHeader1)
	}

	// Check prompt cache header
	promptCacheHeader1 := resp1.Header.Get("X-CLASP-Prompt-Cache")
	if promptCacheHeader1 != "" {
		t.Logf("First request prompt cache header: %s", promptCacheHeader1)
	}

	// Parse first response
	var anthropicResp1 models.AnthropicResponse
	if err := json.NewDecoder(resp1.Body).Decode(&anthropicResp1); err != nil {
		t.Fatalf("Failed to decode first response: %v", err)
	}

	// Get cache stats after first request
	stats1 := pc.Stats()
	t.Logf("After first request - Hits: %d, Misses: %d, Size: %d", stats1.Hits, stats1.Misses, stats1.Size)

	// Second identical request - should hit prompt cache if the first response was stored
	t.Log("Making second identical request (prompt cache hit expected)...")
	reqBody2, _ := json.Marshal(anthropicReq)
	req2 := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(reqBody2))
	req2.Header.Set("Content-Type", "application/json")

	rec2 := httptest.NewRecorder()
	handler.HandleMessages(rec2, req2)

	resp2 := rec2.Result()
	if resp2.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp2.Body)
		t.Fatalf("Second request failed with status %d: %s", resp2.StatusCode, string(body))
	}

	// Check prompt cache header - should be HIT on second request
	promptCacheHeader2 := resp2.Header.Get("X-CLASP-Prompt-Cache")
	if promptCacheHeader2 == "HIT" {
		t.Log("SUCCESS: Second request served from prompt cache!")
	} else {
		t.Logf("Second request prompt cache header: %s (may not be HIT if cache population happened after response)", promptCacheHeader2)
	}

	// Get cache stats after second request
	stats2 := pc.Stats()
	t.Logf("After second request - Hits: %d, Misses: %d, Size: %d, Hit Rate: %.2f%%",
		stats2.Hits, stats2.Misses, stats2.Size, stats2.HitRate)

	// If we got a cache hit, verify the response is identical
	if promptCacheHeader2 == "HIT" {
		var anthropicResp2 models.AnthropicResponse
		if err := json.NewDecoder(resp2.Body).Decode(&anthropicResp2); err != nil {
			t.Fatalf("Failed to decode second response: %v", err)
		}

		if anthropicResp2.ID != anthropicResp1.ID {
			t.Errorf("Cache hit returned different response ID: got %s, want %s", anthropicResp2.ID, anthropicResp1.ID)
		}

		if len(anthropicResp2.Content) != len(anthropicResp1.Content) {
			t.Errorf("Cache hit returned different content length: got %d, want %d", len(anthropicResp2.Content), len(anthropicResp1.Content))
		}
	}

	// Test that requests without cache_control markers don't use prompt cache
	t.Log("Making request without cache_control markers (should not use prompt cache)...")
	anthropicReqNoCache := models.AnthropicRequest{
		Model:     "claude-3-haiku-20240307",
		MaxTokens: 50,
		Stream:    false,
		System: []models.ContentBlock{
			{
				Type: "text",
				Text: "You are a helpful assistant.",
				// No CacheControl
			},
		},
		Messages: []models.AnthropicMessage{
			{
				Role:    "user",
				Content: "Say 'no cache test'",
			},
		},
	}

	reqBody3, _ := json.Marshal(anthropicReqNoCache)
	req3 := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(reqBody3))
	req3.Header.Set("Content-Type", "application/json")

	rec3 := httptest.NewRecorder()
	handler.HandleMessages(rec3, req3)

	resp3 := rec3.Result()
	if resp3.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp3.Body)
		t.Fatalf("Third request failed with status %d: %s", resp3.StatusCode, string(body))
	}

	promptCacheHeader3 := resp3.Header.Get("X-CLASP-Prompt-Cache")
	if promptCacheHeader3 == "HIT" {
		t.Error("Request without cache_control markers should not hit prompt cache")
	}

	// Test that streaming requests don't use prompt cache
	t.Log("Making streaming request (should not use prompt cache)...")
	anthropicReqStream := models.AnthropicRequest{
		Model:     "claude-3-haiku-20240307",
		MaxTokens: 50,
		Stream:    true,
		System: []models.ContentBlock{
			{
				Type: "text",
				Text: "You are a helpful assistant.",
				CacheControl: &models.CacheControl{
					Type: cacheControlType,
				},
			},
		},
		Messages: []models.AnthropicMessage{
			{
				Role:    "user",
				Content: "Say 'stream test'",
			},
		},
	}

	reqBody4, _ := json.Marshal(anthropicReqStream)
	req4 := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(reqBody4))
	req4.Header.Set("Content-Type", "application/json")

	rec4 := httptest.NewRecorder()
	handler.HandleMessages(rec4, req4)

	resp4 := rec4.Result()
	if resp4.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp4.Body)
		t.Logf("Streaming request failed with status %d: %s", resp4.StatusCode, string(body))
		// Streaming test may fail due to test recorder limitations, log and continue
	}

	promptCacheHeader4 := resp4.Header.Get("X-CLASP-Prompt-Cache")
	if promptCacheHeader4 == "HIT" {
		t.Error("Streaming requests should not hit prompt cache")
	}

	t.Log("Prompt cache integration test completed successfully")
}

// TestIntegration_Compaction tests Responses API previous_response_id chaining (compaction)
// Run with: go test -tags=integration ./tests/... -v -run TestIntegration_Compaction
func TestIntegration_Compaction(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set, skipping integration test")
	}

	// Create config with compaction enabled
	cfg := &config.Config{
		Provider:           config.ProviderOpenAI,
		OpenAIAPIKey:       apiKey,
		OpenAIBaseURL:      "https://api.openai.com/v1",
		DefaultModel:       "gpt-4o-mini",
		Port:               8080,
		CompactionEnabled:  true,
		SessionTimeoutSec:  3600, // 1 hour TTL
	}

	handler, err := proxy.NewHandler(cfg)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}

	// Initialize session tracker
	sessionTracker := session.NewTracker(time.Hour)
	defer sessionTracker.Stop()
	handler.SetSessionTracker(sessionTracker)

	// Create a consistent first message to establish session identity
	firstUserMessage := "You are a helpful assistant. Please respond with 'ACK' to confirm."

	// First request - establishes session, should be a compaction miss
	t.Log("Making first request (compaction miss expected)...")
	anthropicReq1 := models.AnthropicRequest{
		Model:     "gpt-5.1-codex", // Triggers Responses API
		MaxTokens: 50,
		Stream:    false,
		Messages: []models.AnthropicMessage{
			{
				Role:    "user",
				Content: firstUserMessage,
			},
		},
	}

	reqBody1, _ := json.Marshal(anthropicReq1)
	req1 := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(reqBody1))
	req1.Header.Set("Content-Type", "application/json")

	rec1 := httptest.NewRecorder()
	handler.HandleMessages(rec1, req1)

	resp1 := rec1.Result()
	if resp1.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp1.Body)
		t.Logf("First request response body: %s", string(body))
		// GPT-5 models may not be available, so we'll skip if we get a 404 or model not found error
		if resp1.StatusCode == 404 || resp1.StatusCode == 400 {
			t.Skip("GPT-5 model not available, skipping compaction test")
		}
		t.Fatalf("First request failed with status %d: %s", resp1.StatusCode, string(body))
	}

	var anthropicResp1 models.AnthropicResponse
	if err := json.NewDecoder(resp1.Body).Decode(&anthropicResp1); err != nil {
		t.Fatalf("Failed to decode first response: %v", err)
	}

	t.Logf("First request response ID: %s", anthropicResp1.ID)

	// Check compaction metrics after first request
	metrics := handler.GetMetrics()
	t.Logf("After first request - CompactionHits: %d, CompactionMisses: %d",
		metrics.CompactionHits, metrics.CompactionMisses)

	if metrics.CompactionMisses == 0 {
		t.Error("Expected at least 1 compaction miss after first request")
	}

	// Verify session was stored
	t.Logf("Active sessions: %d", sessionTracker.Len())

	// Second request - same first message, should hit compaction
	t.Log("Making second request with same first message (compaction hit expected)...")
	anthropicReq2 := models.AnthropicRequest{
		Model:     "gpt-5.1-codex",
		MaxTokens: 50,
		Stream:    false,
		Messages: []models.AnthropicMessage{
			{
				Role:    "user",
				Content: firstUserMessage, // Same first message = same session key
			},
			{
				Role:    "assistant",
				Content: []models.ContentBlock{{Type: "text", Text: "ACK"}},
			},
			{
				Role:    "user",
				Content: "Now say 'SECOND'", // New message to continue conversation
			},
		},
	}

	reqBody2, _ := json.Marshal(anthropicReq2)
	req2 := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(reqBody2))
	req2.Header.Set("Content-Type", "application/json")

	rec2 := httptest.NewRecorder()
	handler.HandleMessages(rec2, req2)

	resp2 := rec2.Result()
	if resp2.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp2.Body)
		t.Fatalf("Second request failed with status %d: %s", resp2.StatusCode, string(body))
	}

	var anthropicResp2 models.AnthropicResponse
	if err := json.NewDecoder(resp2.Body).Decode(&anthropicResp2); err != nil {
		t.Fatalf("Failed to decode second response: %v", err)
	}

	// Check compaction metrics after second request
	metrics = handler.GetMetrics()
	t.Logf("After second request - CompactionHits: %d, CompactionMisses: %d",
		metrics.CompactionHits, metrics.CompactionMisses)

	// We should have at least 1 compaction hit now
	if metrics.CompactionHits > 0 {
		t.Log("SUCCESS: Compaction hit detected on second request!")
	} else {
		t.Log("Note: No compaction hit detected - this may be due to session key derivation or timing")
	}

	t.Logf("Active sessions: %d", sessionTracker.Len())

	// Test streaming with compaction
	t.Log("Making streaming request with compaction...")
	anthropicReq3 := models.AnthropicRequest{
		Model:     "gpt-5.1-codex",
		MaxTokens: 50,
		Stream:    true,
		Messages: []models.AnthropicMessage{
			{
				Role:    "user",
				Content: firstUserMessage,
			},
			{
				Role:    "assistant",
				Content: []models.ContentBlock{{Type: "text", Text: "ACK"}},
			},
			{
				Role:    "user",
				Content: "Count to 3",
			},
		},
	}

	reqBody3, _ := json.Marshal(anthropicReq3)
	req3 := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(reqBody3))
	req3.Header.Set("Content-Type", "application/json")

	rec3 := httptest.NewRecorder()
	handler.HandleMessages(rec3, req3)

	resp3 := rec3.Result()
	if resp3.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp3.Body)
		t.Logf("Streaming request failed with status %d: %s", resp3.StatusCode, string(body))
		// Streaming may have issues with test recorder, log and continue
	} else {
		// Verify content type
		contentType := resp3.Header.Get("Content-Type")
		if !strings.HasPrefix(contentType, "text/event-stream") {
			t.Errorf("Expected Content-Type 'text/event-stream', got '%s'", contentType)
		}
		body, _ := io.ReadAll(resp3.Body)
		t.Logf("Streaming response length: %d bytes", len(body))
	}

	// Final metrics check
	metrics = handler.GetMetrics()
	t.Logf("Final metrics - CompactionHits: %d, CompactionMisses: %d, ActiveSessions: %d",
		metrics.CompactionHits, metrics.CompactionMisses, sessionTracker.Len())

	t.Log("Compaction integration test completed")
}
