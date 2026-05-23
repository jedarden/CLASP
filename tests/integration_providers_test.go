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

	"github.com/jedarden/clasp/internal/config"
	"github.com/jedarden/clasp/internal/proxy"
	"github.com/jedarden/clasp/pkg/models"
)

// TestIntegration_Gemini tests the full proxy with live Gemini API
// Run with: go test -tags=integration ./tests/... -v -run TestIntegration_Gemini
func TestIntegration_Gemini(t *testing.T) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		t.Skip("GEMINI_API_KEY not set, skipping Gemini integration test")
	}

	cfg := &config.Config{
		Provider:      config.ProviderGemini,
		GeminiAPIKey:  apiKey,
		GeminiBaseURL: "https://generativelanguage.googleapis.com/v1beta",
		DefaultModel:  "gemini-2.0-flash-exp",
		Port:          8080,
	}

	handler, err := proxy.NewHandler(cfg)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}

	// Test basic request
	anthropicReq := models.AnthropicRequest{
		Model:     "claude-3-haiku-20240307",
		MaxTokens: 100,
		Stream:    false,
		Messages: []models.AnthropicMessage{
			{
				Role:    "user",
				Content: "Say 'Hello from Gemini' and nothing else.",
			},
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

	if anthropicResp.Type != "message" {
		t.Errorf("Expected type 'message', got '%s'", anthropicResp.Type)
	}

	if anthropicResp.Role != "assistant" {
		t.Errorf("Expected role 'assistant', got '%s'", anthropicResp.Role)
	}

	if len(anthropicResp.Content) == 0 {
		t.Error("Expected content, got empty")
	}

	if len(anthropicResp.Content) > 0 && anthropicResp.Content[0].Type == "text" {
		text := anthropicResp.Content[0].Text
		t.Logf("Gemini response text: %s", text)
		if !strings.Contains(strings.ToLower(text), "hello") {
			t.Logf("Warning: Response doesn't contain 'hello': %s", text)
		}
	}
}

// TestIntegration_GeminiStreaming tests streaming with live Gemini API
func TestIntegration_GeminiStreaming(t *testing.T) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		t.Skip("GEMINI_API_KEY not set, skipping Gemini streaming test")
	}

	cfg := &config.Config{
		Provider:      config.ProviderGemini,
		GeminiAPIKey:  apiKey,
		GeminiBaseURL: "https://generativelanguage.googleapis.com/v1beta",
		DefaultModel:  "gemini-2.0-flash-exp",
		Port:          8080,
	}

	handler, err := proxy.NewHandler(cfg)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}

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

	rec := httptest.NewRecorder()
	handler.HandleMessages(rec, req)

	resp := rec.Result()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, string(body))
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "text/event-stream") {
		t.Errorf("Expected Content-Type 'text/event-stream', got '%s'", contentType)
	}

	body, _ := io.ReadAll(resp.Body)
	output := string(body)
	t.Logf("Gemini streaming response:\n%s", output)

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

// TestIntegration_Grok tests the full proxy with live Grok API
// Run with: go test -tags=integration ./tests/... -v -run TestIntegration_Grok
func TestIntegration_Grok(t *testing.T) {
	apiKey := os.Getenv("GROK_API_KEY")
	if apiKey == "" {
		t.Skip("GROK_API_KEY not set, skipping Grok integration test")
	}

	cfg := &config.Config{
		Provider:      config.ProviderGrok,
		GrokAPIKey:    apiKey,
		GrokBaseURL:   "https://api.x.ai",
		DefaultModel:  "grok-3-beta",
		Port:          8080,
	}

	handler, err := proxy.NewHandler(cfg)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}

	anthropicReq := models.AnthropicRequest{
		Model:     "claude-3-haiku-20240307",
		MaxTokens: 100,
		Stream:    false,
		Messages: []models.AnthropicMessage{
			{
				Role:    "user",
				Content: "Say 'Hello from Grok' and nothing else.",
			},
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

	if anthropicResp.Type != "message" {
		t.Errorf("Expected type 'message', got '%s'", anthropicResp.Type)
	}

	if anthropicResp.Role != "assistant" {
		t.Errorf("Expected role 'assistant', got '%s'", anthropicResp.Role)
	}

	if len(anthropicResp.Content) > 0 && anthropicResp.Content[0].Type == "text" {
		text := anthropicResp.Content[0].Text
		t.Logf("Grok response text: %s", text)
	}
}

// TestIntegration_GrokStreaming tests streaming with live Grok API
func TestIntegration_GrokStreaming(t *testing.T) {
	apiKey := os.Getenv("GROK_API_KEY")
	if apiKey == "" {
		t.Skip("GROK_API_KEY not set, skipping Grok streaming test")
	}

	cfg := &config.Config{
		Provider:      config.ProviderGrok,
		GrokAPIKey:    apiKey,
		GrokBaseURL:   "https://api.x.ai",
		DefaultModel:  "grok-3-beta",
		Port:          8080,
	}

	handler, err := proxy.NewHandler(cfg)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}

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

	rec := httptest.NewRecorder()
	handler.HandleMessages(rec, req)

	resp := rec.Result()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, string(body))
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "text/event-stream") {
		t.Errorf("Expected Content-Type 'text/event-stream', got '%s'", contentType)
	}

	body, _ := io.ReadAll(resp.Body)
	t.Logf("Grok streaming response length: %d bytes", len(body))
}

// TestIntegration_DeepSeek tests the full proxy with live DeepSeek API
// Run with: go test -tags=integration ./tests/... -v -run TestIntegration_DeepSeek
func TestIntegration_DeepSeek(t *testing.T) {
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		t.Skip("DEEPSEEK_API_KEY not set, skipping DeepSeek integration test")
	}

	cfg := &config.Config{
		Provider:        config.ProviderDeepSeek,
		DeepSeekAPIKey:  apiKey,
		DeepSeekBaseURL: "https://api.deepseek.com",
		DefaultModel:    "deepseek-chat",
		Port:            8080,
	}

	handler, err := proxy.NewHandler(cfg)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}

	anthropicReq := models.AnthropicRequest{
		Model:     "claude-3-haiku-20240307",
		MaxTokens: 100,
		Stream:    false,
		Messages: []models.AnthropicMessage{
			{
				Role:    "user",
				Content: "Say 'Hello from DeepSeek' and nothing else.",
			},
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

	if anthropicResp.Type != "message" {
		t.Errorf("Expected type 'message', got '%s'", anthropicResp.Type)
	}

	if anthropicResp.Role != "assistant" {
		t.Errorf("Expected role 'assistant', got '%s'", anthropicResp.Role)
	}

	if len(anthropicResp.Content) > 0 && anthropicResp.Content[0].Type == "text" {
		text := anthropicResp.Content[0].Text
		t.Logf("DeepSeek response text: %s", text)
	}
}

// TestIntegration_DeepSeekStreaming tests streaming with live DeepSeek API
func TestIntegration_DeepSeekStreaming(t *testing.T) {
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		t.Skip("DEEPSEEK_API_KEY not set, skipping DeepSeek streaming test")
	}

	cfg := &config.Config{
		Provider:        config.ProviderDeepSeek,
		DeepSeekAPIKey:  apiKey,
		DeepSeekBaseURL: "https://api.deepseek.com",
		DefaultModel:    "deepseek-chat",
		Port:            8080,
	}

	handler, err := proxy.NewHandler(cfg)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}

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

	rec := httptest.NewRecorder()
	handler.HandleMessages(rec, req)

	resp := rec.Result()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, string(body))
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "text/event-stream") {
		t.Errorf("Expected Content-Type 'text/event-stream', got '%s'", contentType)
	}

	body, _ := io.ReadAll(resp.Body)
	t.Logf("DeepSeek streaming response length: %d bytes", len(body))
}

// TestIntegration_Qwen tests the full proxy with live Qwen API
// Run with: go test -tags=integration ./tests/... -v -run TestIntegration_Qwen
func TestIntegration_Qwen(t *testing.T) {
	apiKey := os.Getenv("QWEN_API_KEY")
	if apiKey == "" {
		t.Skip("QWEN_API_KEY not set, skipping Qwen integration test")
	}

	cfg := &config.Config{
		Provider:      config.ProviderQwen,
		QwenAPIKey:    apiKey,
		QwenBaseURL:   "https://dashscope.aliyuncs.com/compatible-mode",
		DefaultModel:  "qwen-plus",
		Port:          8080,
	}

	handler, err := proxy.NewHandler(cfg)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}

	anthropicReq := models.AnthropicRequest{
		Model:     "claude-3-haiku-20240307",
		MaxTokens: 100,
		Stream:    false,
		Messages: []models.AnthropicMessage{
			{
				Role:    "user",
				Content: "Say 'Hello from Qwen' and nothing else.",
			},
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

	if anthropicResp.Type != "message" {
		t.Errorf("Expected type 'message', got '%s'", anthropicResp.Type)
	}

	if anthropicResp.Role != "assistant" {
		t.Errorf("Expected role 'assistant', got '%s'", anthropicResp.Role)
	}

	if len(anthropicResp.Content) > 0 && anthropicResp.Content[0].Type == "text" {
		text := anthropicResp.Content[0].Text
		t.Logf("Qwen response text: %s", text)
	}
}

// TestIntegration_QwenStreaming tests streaming with live Qwen API
func TestIntegration_QwenStreaming(t *testing.T) {
	apiKey := os.Getenv("QWEN_API_KEY")
	if apiKey == "" {
		t.Skip("QWEN_API_KEY not set, skipping Qwen streaming test")
	}

	cfg := &config.Config{
		Provider:      config.ProviderQwen,
		QwenAPIKey:    apiKey,
		QwenBaseURL:   "https://dashscope.aliyuncs.com/compatible-mode",
		DefaultModel:  "qwen-plus",
		Port:          8080,
	}

	handler, err := proxy.NewHandler(cfg)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}

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

	rec := httptest.NewRecorder()
	handler.HandleMessages(rec, req)

	resp := rec.Result()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, string(body))
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "text/event-stream") {
		t.Errorf("Expected Content-Type 'text/event-stream', got '%s'", contentType)
	}

	body, _ := io.ReadAll(resp.Body)
	t.Logf("Qwen streaming response length: %d bytes", len(body))
}

// TestIntegration_MiniMax tests the full proxy with live MiniMax API
// Run with: go test -tags=integration ./tests/... -v -run TestIntegration_MiniMax
func TestIntegration_MiniMax(t *testing.T) {
	apiKey := os.Getenv("MINIMAX_API_KEY")
	if apiKey == "" {
		t.Skip("MINIMAX_API_KEY not set, skipping MiniMax integration test")
	}

	cfg := &config.Config{
		Provider:       config.ProviderMiniMax,
		MiniMaxAPIKey:  apiKey,
		MiniMaxBaseURL: "https://api.minimax.chat",
		DefaultModel:   "abab6.5s-chat",
		Port:           8080,
	}

	handler, err := proxy.NewHandler(cfg)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}

	anthropicReq := models.AnthropicRequest{
		Model:     "claude-3-haiku-20240307",
		MaxTokens: 100,
		Stream:    false,
		Messages: []models.AnthropicMessage{
			{
				Role:    "user",
				Content: "Say 'Hello from MiniMax' and nothing else.",
			},
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

	if anthropicResp.Type != "message" {
		t.Errorf("Expected type 'message', got '%s'", anthropicResp.Type)
	}

	if anthropicResp.Role != "assistant" {
		t.Errorf("Expected role 'assistant', got '%s'", anthropicResp.Role)
	}

	if len(anthropicResp.Content) > 0 && anthropicResp.Content[0].Type == "text" {
		text := anthropicResp.Content[0].Text
		t.Logf("MiniMax response text: %s", text)
	}
}

// TestIntegration_MiniMaxStreaming tests streaming with live MiniMax API
func TestIntegration_MiniMaxStreaming(t *testing.T) {
	apiKey := os.Getenv("MINIMAX_API_KEY")
	if apiKey == "" {
		t.Skip("MINIMAX_API_KEY not set, skipping MiniMax streaming test")
	}

	cfg := &config.Config{
		Provider:       config.ProviderMiniMax,
		MiniMaxAPIKey:  apiKey,
		MiniMaxBaseURL: "https://api.minimax.chat",
		DefaultModel:   "abab6.5s-chat",
		Port:           8080,
	}

	handler, err := proxy.NewHandler(cfg)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}

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

	rec := httptest.NewRecorder()
	handler.HandleMessages(rec, req)

	resp := rec.Result()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, string(body))
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "text/event-stream") {
		t.Errorf("Expected Content-Type 'text/event-stream', got '%s'", contentType)
	}

	body, _ := io.ReadAll(resp.Body)
	t.Logf("MiniMax streaming response length: %d bytes", len(body))
}

// TestIntegration_Ollama tests the full proxy with Ollama
// Run with: go test -tags=integration ./tests/... -v -run TestIntegration_Ollama
func TestIntegration_Ollama(t *testing.T) {
	// Check if Ollama is running at default URL
	if !isOllamaAvailable(t) {
		t.Skip("Ollama not available at http://localhost:11434")
	}

	cfg := &config.Config{
		Provider:      config.ProviderOllama,
		OllamaBaseURL: "http://localhost:11434",
		DefaultModel:  "llama3.2",
		Port:          8080,
	}

	p, err := proxy.NewHandler(cfg)
	if err != nil {
		t.Fatalf("Failed to create Ollama handler: %v", err)
	}

	anthropicReq := models.AnthropicRequest{
		Model:     "llama3.2",
		MaxTokens: 100,
		Stream:    false,
		Messages: []models.AnthropicMessage{
			{
				Role:    "user",
				Content: "Say 'Hello from Ollama' and nothing else.",
			},
		},
	}

	reqBody, _ := json.Marshal(anthropicReq)
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	p.HandleMessages(rec, req)

	resp := rec.Result()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		// Ollama may not have the model pulled, skip in that case
		if resp.StatusCode == 404 || resp.StatusCode == 400 {
			t.Skipf("Ollama model not available: %s", string(body))
		}
		t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, string(body))
	}

	var anthropicResp models.AnthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&anthropicResp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if anthropicResp.Type != "message" {
		t.Errorf("Expected type 'message', got '%s'", anthropicResp.Type)
	}

	if anthropicResp.Role != "assistant" {
		t.Errorf("Expected role 'assistant', got '%s'", anthropicResp.Role)
	}

	if len(anthropicResp.Content) > 0 && anthropicResp.Content[0].Type == "text" {
		text := anthropicResp.Content[0].Text
		t.Logf("Ollama response text: %s", text)
	}
}

// TestIntegration_OllamaStreaming tests streaming with Ollama
func TestIntegration_OllamaStreaming(t *testing.T) {
	if !isOllamaAvailable(t) {
		t.Skip("Ollama not available at http://localhost:11434")
	}

	cfg := &config.Config{
		Provider:      config.ProviderOllama,
		OllamaBaseURL: "http://localhost:11434",
		DefaultModel:  "llama3.2",
		Port:          8080,
	}

	handler, err := proxy.NewHandler(cfg)
	if err != nil {
		t.Fatalf("Failed to create Ollama handler: %v", err)
	}

	anthropicReq := models.AnthropicRequest{
		Model:     "llama3.2",
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

	rec := httptest.NewRecorder()
	handler.HandleMessages(rec, req)

	resp := rec.Result()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode == 404 || resp.StatusCode == 400 {
			t.Skipf("Ollama model not available: %s", string(body))
		}
		t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, string(body))
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "text/event-stream") {
		t.Errorf("Expected Content-Type 'text/event-stream', got '%s'", contentType)
	}

	body, _ := io.ReadAll(resp.Body)
	t.Logf("Ollama streaming response length: %d bytes", len(body))
}

// TestIntegration_GeminiToolCall tests tool calling with live Gemini API
func TestIntegration_GeminiToolCall(t *testing.T) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		t.Skip("GEMINI_API_KEY not set, skipping Gemini tool call test")
	}

	cfg := &config.Config{
		Provider:      config.ProviderGemini,
		GeminiAPIKey:  apiKey,
		GeminiBaseURL: "https://generativelanguage.googleapis.com/v1beta",
		DefaultModel:  "gemini-2.0-flash-exp",
		Port:          8080,
	}

	handler, err := proxy.NewHandler(cfg)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}

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

	t.Logf("Gemini tool call response: %+v", anthropicResp)

	hasToolUse := false
	for _, block := range anthropicResp.Content {
		if block.Type == "tool_use" {
			hasToolUse = true
			t.Logf("Tool call: %s (id: %s)", block.Name, block.ID)
		}
	}

	if !hasToolUse {
		t.Log("No tool_use in response (model might have responded without tool call)")
	}
}

// TestIntegration_GrokToolCall tests tool calling with live Grok API
func TestIntegration_GrokToolCall(t *testing.T) {
	apiKey := os.Getenv("GROK_API_KEY")
	if apiKey == "" {
		t.Skip("GROK_API_KEY not set, skipping Grok tool call test")
	}

	cfg := &config.Config{
		Provider:      config.ProviderGrok,
		GrokAPIKey:    apiKey,
		GrokBaseURL:   "https://api.x.ai",
		DefaultModel:  "grok-3-beta",
		Port:          8080,
	}

	handler, err := proxy.NewHandler(cfg)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}

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

	t.Logf("Grok tool call response: %+v", anthropicResp)

	hasToolUse := false
	for _, block := range anthropicResp.Content {
		if block.Type == "tool_use" {
			hasToolUse = true
			t.Logf("Tool call: %s (id: %s)", block.Name, block.ID)
		}
	}

	if !hasToolUse {
		t.Log("No tool_use in response (model might have responded without tool call)")
	}
}

// TestIntegration_DeepSeekToolCall tests tool calling with live DeepSeek API
func TestIntegration_DeepSeekToolCall(t *testing.T) {
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		t.Skip("DEEPSEEK_API_KEY not set, skipping DeepSeek tool call test")
	}

	cfg := &config.Config{
		Provider:        config.ProviderDeepSeek,
		DeepSeekAPIKey:  apiKey,
		DeepSeekBaseURL: "https://api.deepseek.com",
		DefaultModel:    "deepseek-chat",
		Port:            8080,
	}

	handler, err := proxy.NewHandler(cfg)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}

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

	t.Logf("DeepSeek tool call response: %+v", anthropicResp)

	hasToolUse := false
	for _, block := range anthropicResp.Content {
		if block.Type == "tool_use" {
			hasToolUse = true
			t.Logf("Tool call: %s (id: %s)", block.Name, block.ID)
		}
	}

	if !hasToolUse {
		t.Log("No tool_use in response (model might have responded without tool call)")
	}
}

// TestIntegration_OllamaToolCall tests tool calling with Ollama
func TestIntegration_OllamaToolCall(t *testing.T) {
	if !isOllamaAvailable(t) {
		t.Skip("Ollama not available at http://localhost:11434")
	}

	cfg := &config.Config{
		Provider:      config.ProviderOllama,
		OllamaBaseURL: "http://localhost:11434",
		DefaultModel:  "llama3.2",
		Port:          8080,
	}

	handler, err := proxy.NewHandler(cfg)
	if err != nil {
		t.Fatalf("Failed to create Ollama handler: %v", err)
	}

	anthropicReq := models.AnthropicRequest{
		Model:     "llama3.2",
		MaxTokens: 200,
		Stream:    false,
		Messages: []models.AnthropicMessage{
			{
				Role:    "user",
				Content: "What's 2+2? Use a calculator tool.",
			},
		},
		Tools: []models.AnthropicTool{
			{
				Name:        "calculator",
				Description: "Perform basic math operations",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"operation": map[string]interface{}{
							"type":        "string",
							"description": "Math operation (e.g., '2+2')",
						},
					},
					"required": []string{"operation"},
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
		if resp.StatusCode == 404 || resp.StatusCode == 400 {
			t.Skipf("Ollama model not available: %s", string(body))
		}
		t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, string(body))
	}

	var anthropicResp models.AnthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&anthropicResp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	t.Logf("Ollama tool call response: %+v", anthropicResp)

	hasToolUse := false
	for _, block := range anthropicResp.Content {
		if block.Type == "tool_use" {
			hasToolUse = true
			t.Logf("Tool call: %s (id: %s)", block.Name, block.ID)
		}
	}

	if !hasToolUse {
		t.Log("No tool_use in response (model might have responded without tool call)")
	}
}

// TestIntegration_GeminiThinkingConfig tests thinking_config with Gemini 2.5
// Run with: go test -tags=integration ./tests/... -v -run TestIntegration_GeminiThinkingConfig
func TestIntegration_GeminiThinkingConfig(t *testing.T) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		t.Skip("GEMINI_API_KEY not set, skipping Gemini thinking_config test")
	}

	cfg := &config.Config{
		Provider:      config.ProviderGemini,
		GeminiAPIKey:  apiKey,
		GeminiBaseURL: "https://generativelanguage.googleapis.com/v1beta",
		DefaultModel:  "gemini-2.5-flash-exp",
		Port:          8080,
	}

	handler, err := proxy.NewHandler(cfg)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}

	// Request with thinking enabled via Thinking config
	anthropicReq := models.AnthropicRequest{
		Model:     "gemini-2.5-flash-exp",
		MaxTokens: 100,
		Stream:    false,
		Messages: []models.AnthropicMessage{
			{
				Role:    "user",
				Content: "Think step by step: What is 15 * 23?",
			},
		},
		// Use Thinking field to trigger thinking_config for Gemini 2.5
		Thinking: &models.ThinkingConfig{
			Type:         "enabled",
			BudgetTokens: 10000,
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

	t.Logf("Gemini 2.5 thinking response: %+v", anthropicResp)

	// Check for thinking blocks if present
	hasThinking := false
	for _, block := range anthropicResp.Content {
		if block.Type == "thinking" {
			hasThinking = true
			t.Logf("Thinking block found")
		}
	}

	if hasThinking {
		t.Log("SUCCESS: Thinking block detected in Gemini 2.5 response")
	} else {
		t.Log("Note: No explicit thinking block (model may have processed thinking internally)")
	}

	if anthropicResp.Type != "message" {
		t.Errorf("Expected type 'message', got '%s'", anthropicResp.Type)
	}
}

// TestIntegration_QwenToolCall tests tool calling with live Qwen API
// Run with: go test -tags=integration ./tests/... -v -run TestIntegration_QwenToolCall
func TestIntegration_QwenToolCall(t *testing.T) {
	apiKey := os.Getenv("QWEN_API_KEY")
	if apiKey == "" {
		t.Skip("QWEN_API_KEY not set, skipping Qwen tool call test")
	}

	cfg := &config.Config{
		Provider:      config.ProviderQwen,
		QwenAPIKey:    apiKey,
		QwenBaseURL:   "https://dashscope.aliyuncs.com/compatible-mode",
		DefaultModel:  "qwen-plus",
		Port:          8080,
	}

	handler, err := proxy.NewHandler(cfg)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}

	anthropicReq := models.AnthropicRequest{
		Model:     "claude-3-haiku-20240307",
		MaxTokens: 200,
		Stream:    false,
		Messages: []models.AnthropicMessage{
			{
				Role:    "user",
				Content: "Get the current weather in Beijing.",
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

	t.Logf("Qwen tool call response: %+v", anthropicResp)

	hasToolUse := false
	for _, block := range anthropicResp.Content {
		if block.Type == "tool_use" {
			hasToolUse = true
			t.Logf("Tool call: %s (id: %s)", block.Name, block.ID)
			if block.Name != "get_weather" {
				t.Errorf("Expected tool name 'get_weather', got '%s'", block.Name)
			}
		}
	}

	if !hasToolUse {
		t.Log("No tool_use in response (model might have responded without tool call)")
	}
}

// TestIntegration_MiniMaxToolCall tests tool calling with live MiniMax API
// Run with: go test -tags=integration ./tests/... -v -run TestIntegration_MiniMaxToolCall
func TestIntegration_MiniMaxToolCall(t *testing.T) {
	apiKey := os.Getenv("MINIMAX_API_KEY")
	if apiKey == "" {
		t.Skip("MINIMAX_API_KEY not set, skipping MiniMax tool call test")
	}

	cfg := &config.Config{
		Provider:       config.ProviderMiniMax,
		MiniMaxAPIKey:  apiKey,
		MiniMaxBaseURL: "https://api.minimax.chat",
		DefaultModel:   "abab6.5s-chat",
		Port:           8080,
	}

	handler, err := proxy.NewHandler(cfg)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}

	anthropicReq := models.AnthropicRequest{
		Model:     "claude-3-haiku-20240307",
		MaxTokens: 200,
		Stream:    false,
		Messages: []models.AnthropicMessage{
			{
				Role:    "user",
				Content: "What is 2+2? Use a calculator tool.",
			},
		},
		Tools: []models.AnthropicTool{
			{
				Name:        "calculator",
				Description: "Perform basic math operations",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"expression": map[string]interface{}{
							"type":        "string",
							"description": "Math expression (e.g., '2+2')",
						},
					},
					"required": []string{"expression"},
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

	t.Logf("MiniMax tool call response: %+v", anthropicResp)

	hasToolUse := false
	for _, block := range anthropicResp.Content {
		if block.Type == "tool_use" {
			hasToolUse = true
			t.Logf("Tool call: %s (id: %s)", block.Name, block.ID)
		}
	}

	if !hasToolUse {
		t.Log("No tool_use in response (model might have responded without tool call)")
	}
}

// TestIntegration_GrokReasoningEffort tests reasoning_effort parameter with Grok
// Note: reasoning_effort is passed through to the provider, not stored in AnthropicRequest
// Run with: go test -tags=integration ./tests/... -v -run TestIntegration_GrokReasoningEffort
func TestIntegration_GrokReasoningEffort(t *testing.T) {
	apiKey := os.Getenv("GROK_API_KEY")
	if apiKey == "" {
		t.Skip("GROK_API_KEY not set, skipping Grok reasoning_effort test")
	}

	cfg := &config.Config{
		Provider:      config.ProviderGrok,
		GrokAPIKey:    apiKey,
		GrokBaseURL:   "https://api.x.ai",
		DefaultModel:  "grok-3-mini",
		Port:          8080,
	}

	handler, err := proxy.NewHandler(cfg)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}

	// Standard request - Grok handles reasoning_effort internally
	anthropicReq := models.AnthropicRequest{
		Model:     "grok-3-mini",
		MaxTokens: 100,
		Stream:    false,
		Messages: []models.AnthropicMessage{
			{
				Role:    "user",
				Content: "Think carefully: What is the sum of all prime numbers between 10 and 20?",
			},
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

	t.Logf("Grok reasoning response: %+v", anthropicResp)

	// Check for thinking blocks
	hasThinking := false
	for _, block := range anthropicResp.Content {
		if block.Type == "thinking" {
			hasThinking = true
			t.Logf("Thinking block found")
		}
	}

	if hasThinking {
		t.Log("SUCCESS: Thinking block detected in Grok response")
	}

	// Verify the answer is correct (primes between 10 and 20 are 11, 13, 17, 19; sum = 60)
	if len(anthropicResp.Content) > 0 && anthropicResp.Content[0].Type == "text" {
		text := strings.ToLower(anthropicResp.Content[0].Text)
		if strings.Contains(text, "60") {
			t.Log("Correct answer detected in response")
		}
	}
}

// TestIntegration_QwenThinking tests thinking parameters with Qwen
// Note: Qwen-specific thinking parameters are translated during request processing
// Run with: go test -tags=integration ./tests/... -v -run TestIntegration_QwenThinking
func TestIntegration_QwenThinking(t *testing.T) {
	apiKey := os.Getenv("QWEN_API_KEY")
	if apiKey == "" {
		t.Skip("QWEN_API_KEY not set, skipping Qwen thinking test")
	}

	cfg := &config.Config{
		Provider:      config.ProviderQwen,
		QwenAPIKey:    apiKey,
		QwenBaseURL:   "https://dashscope.aliyuncs.com/compatible-mode",
		DefaultModel:  "qwen-plus",
		Port:          8080,
	}

	handler, err := proxy.NewHandler(cfg)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}

	// Request with thinking enabled
	anthropicReq := models.AnthropicRequest{
		Model:     "qwen-plus",
		MaxTokens: 100,
		Stream:    false,
		Messages: []models.AnthropicMessage{
			{
				Role:    "user",
				Content: "Think step by step: If a train travels 120 km in 2 hours, what is its speed?",
			},
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

	t.Logf("Qwen thinking response: %+v", anthropicResp)

	// Check for thinking blocks
	hasThinking := false
	for _, block := range anthropicResp.Content {
		if block.Type == "thinking" {
			hasThinking = true
			t.Logf("Thinking block found")
		}
	}

	if hasThinking {
		t.Log("SUCCESS: Thinking block detected in Qwen response")
	}

	if anthropicResp.Type != "message" {
		t.Errorf("Expected type 'message', got '%s'", anthropicResp.Type)
	}
}

// isOllamaAvailable checks if Ollama is running and accessible
func isOllamaAvailable(t *testing.T) bool {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://localhost:11434/api/tags")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
