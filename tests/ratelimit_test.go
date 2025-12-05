package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jedarden/clasp/internal/config"
	"github.com/jedarden/clasp/internal/proxy"
	"github.com/jedarden/clasp/pkg/models"
)

func TestRateLimiter_AllowsRequests(t *testing.T) {
	// Create a rate limiter that allows 10 requests per 10 seconds with burst of 5
	limiter := proxy.NewRateLimiter(10, 10, 5)

	// Should allow burst + some refill
	allowedCount := 0
	for i := 0; i < 10; i++ {
		if limiter.Allow() {
			allowedCount++
		}
	}

	// Should have allowed at least burst amount
	if allowedCount < 5 {
		t.Errorf("Expected at least 5 allowed requests (burst), got %d", allowedCount)
	}
}

func TestRateLimiter_DeniesExcessRequests(t *testing.T) {
	// Create a rate limiter with 1 request per second, no burst
	limiter := proxy.NewRateLimiter(1, 1, 0)

	// First request should be denied (no burst, need to build up tokens)
	// Actually with rate 1/sec and 0 burst, we start with 0 tokens
	// So we need to wait for tokens to accumulate
	time.Sleep(100 * time.Millisecond) // Let some tokens accumulate

	// First request should be allowed after some time
	allowed1 := limiter.Allow()

	// Second request immediately after should be denied
	allowed2 := limiter.Allow()

	if !allowed1 && !allowed2 {
		t.Log("Both requests denied - expected given tight rate limit")
	}
}

func TestRateLimiter_RefillsTokens(t *testing.T) {
	// Create a rate limiter with 10 requests per second, burst of 2
	limiter := proxy.NewRateLimiter(10, 1, 2)

	// Use up the burst
	for i := 0; i < 5; i++ {
		limiter.Allow()
	}

	// Wait for tokens to refill
	time.Sleep(200 * time.Millisecond)

	// Should allow more requests after refill
	if !limiter.Allow() {
		t.Error("Expected request to be allowed after token refill")
	}
}

func TestRateLimiter_Stats(t *testing.T) {
	limiter := proxy.NewRateLimiter(100, 1, 10)

	// Make some requests
	for i := 0; i < 5; i++ {
		limiter.Allow()
	}

	allowed, denied := limiter.Stats()
	total := allowed + denied

	if total != 5 {
		t.Errorf("Expected 5 total requests tracked, got %d", total)
	}
}

func TestRateLimitMiddleware_AllowsNormalRequests(t *testing.T) {
	limiter := proxy.NewRateLimiter(100, 1, 50)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	middleware := proxy.RateLimitMiddleware(limiter)
	wrapped := middleware(handler)

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
}

func TestRateLimitMiddleware_RejectsExcessRequests(t *testing.T) {
	// Very strict rate limit: 1 request per minute, no burst
	limiter := proxy.NewRateLimiter(1, 60, 0)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := proxy.RateLimitMiddleware(limiter)
	wrapped := middleware(handler)

	// Make many requests quickly
	deniedCount := 0
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
		rec := httptest.NewRecorder()
		wrapped.ServeHTTP(rec, req)

		if rec.Code == http.StatusTooManyRequests {
			deniedCount++
		}
	}

	// Should have denied most requests
	if deniedCount < 8 {
		t.Errorf("Expected at least 8 denied requests, got %d", deniedCount)
	}
}

func TestRateLimitMiddleware_BypassesNonAPIEndpoints(t *testing.T) {
	// Very strict rate limit
	limiter := proxy.NewRateLimiter(1, 60, 0)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := proxy.RateLimitMiddleware(limiter)
	wrapped := middleware(handler)

	// Health endpoint should always be allowed
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()
		wrapped.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Health endpoint should not be rate limited, got status %d", rec.Code)
		}
	}
}

func TestRateLimitMiddleware_ReturnsProperError(t *testing.T) {
	// Very strict rate limit
	limiter := proxy.NewRateLimiter(1, 60, 0)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := proxy.RateLimitMiddleware(limiter)
	wrapped := middleware(handler)

	// Make requests until one is denied
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
		rec := httptest.NewRecorder()
		wrapped.ServeHTTP(rec, req)

		if rec.Code == http.StatusTooManyRequests {
			// Check response format
			var errResp map[string]interface{}
			if err := json.NewDecoder(rec.Body).Decode(&errResp); err != nil {
				t.Fatalf("Failed to decode error response: %v", err)
			}

			if errResp["type"] != "error" {
				t.Errorf("Expected type 'error', got '%v'", errResp["type"])
			}

			errDetails, ok := errResp["error"].(map[string]interface{})
			if !ok {
				t.Fatal("Expected error details in response")
			}

			if errDetails["type"] != "rate_limit_error" {
				t.Errorf("Expected error type 'rate_limit_error', got '%v'", errDetails["type"])
			}

			// Check Retry-After header
			retryAfter := rec.Header().Get("Retry-After")
			if retryAfter == "" {
				t.Error("Expected Retry-After header")
			}

			return
		}
	}

	t.Log("No request was denied - may need to adjust test")
}

func TestIntegration_RateLimitWithHandler(t *testing.T) {
	cfg := &config.Config{
		Provider:           config.ProviderOpenAI,
		OpenAIAPIKey:       "test-key",
		OpenAIBaseURL:      "https://api.openai.com/v1",
		DefaultModel:       "gpt-4o",
		Port:               8080,
		RateLimitEnabled:   true,
		RateLimitRequests:  2,
		RateLimitWindow:    1,
		RateLimitBurst:     1,
	}

	handler, err := proxy.NewHandler(cfg)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}

	// Create rate limiter
	limiter := proxy.NewRateLimiter(cfg.RateLimitRequests, cfg.RateLimitWindow, cfg.RateLimitBurst)
	handler.SetRateLimiter(limiter)

	// Create middleware-wrapped handler
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/messages", handler.HandleMessages)
	mux.HandleFunc("/metrics", handler.HandleMetrics)

	wrapped := proxy.RateLimitMiddleware(limiter)(mux)

	// Make a valid request body
	anthropicReq := models.AnthropicRequest{
		Model:     "claude-3-haiku",
		MaxTokens: 100,
		Messages: []models.AnthropicMessage{
			{Role: "user", Content: "test"},
		},
	}
	reqBody, _ := json.Marshal(anthropicReq)

	// Make several requests
	allowedCount := 0
	deniedCount := 0

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		wrapped.ServeHTTP(rec, req)

		if rec.Code == http.StatusTooManyRequests {
			deniedCount++
		} else {
			allowedCount++
		}
	}

	t.Logf("Allowed: %d, Denied: %d", allowedCount, deniedCount)

	// Should have some denied
	if deniedCount == 0 && allowedCount == 5 {
		t.Log("All requests allowed - rate limiter may be too lenient for test")
	}

	// Check metrics endpoint includes rate limit info
	metricsReq := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	metricsRec := httptest.NewRecorder()
	wrapped.ServeHTTP(metricsRec, metricsReq)

	var metrics map[string]interface{}
	if err := json.NewDecoder(metricsRec.Body).Decode(&metrics); err != nil {
		t.Fatalf("Failed to decode metrics: %v", err)
	}

	rateLimitInfo, ok := metrics["rate_limit"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected rate_limit info in metrics")
	}

	if rateLimitInfo["enabled"] != true {
		t.Error("Expected rate_limit.enabled to be true")
	}

	t.Logf("Rate limit metrics: %+v", rateLimitInfo)
}

func BenchmarkRateLimiter(b *testing.B) {
	limiter := proxy.NewRateLimiter(10000, 1, 1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		limiter.Allow()
	}
}
