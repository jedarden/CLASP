// Package proxy implements unit tests for the health checker.
package proxy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jedarden/clasp/internal/config"
	"github.com/jedarden/clasp/internal/provider"
)

// ===== Health Checker Tests =====

func TestHealthCheckerConfig(t *testing.T) {
	t.Run("DefaultHealthCheckerConfig returns correct defaults", func(t *testing.T) {
		cfg := DefaultHealthCheckerConfig()

		if !cfg.Enabled {
			t.Error("Expected Enabled to be true by default")
		}
		if cfg.CheckInterval != 30*time.Second {
			t.Errorf("Expected CheckInterval 30s, got %v", cfg.CheckInterval)
		}
		if cfg.Timeout != 10*time.Second {
			t.Errorf("Expected Timeout 10s, got %v", cfg.Timeout)
		}
	})
}

func TestNewHealthChecker(t *testing.T) {
	t.Run("creates health checker with nil config", func(t *testing.T) {
		appCfg := &config.Config{}
		hc := NewHealthChecker(nil, appCfg, nil)

		if hc == nil {
			t.Error("Expected non-nil health checker")
		}
		if hc.config.Enabled != true {
			t.Error("Expected default config to be used")
		}
	})

	t.Run("creates health checker with provided config", func(t *testing.T) {
		appCfg := &config.Config{}
		cfg := &HealthCheckerConfig{
			Enabled:       true,
			CheckInterval: 60 * time.Second,
			Timeout:       5 * time.Second,
		}
		hc := NewHealthChecker(cfg, appCfg, nil)

		if hc.config.CheckInterval != 60*time.Second {
			t.Errorf("Expected CheckInterval 60s, got %v", hc.config.CheckInterval)
		}
		if hc.config.Timeout != 5*time.Second {
			t.Errorf("Expected Timeout 5s, got %v", hc.config.Timeout)
		}
	})
}

func TestHealthChecker_RegisterProvider(t *testing.T) {
	t.Run("registers provider correctly", func(t *testing.T) {
		hc := NewHealthChecker(nil, &config.Config{}, nil)
		p := provider.NewOpenAIProvider("https://api.openai.com/v1")

		hc.RegisterProvider("openai", p, "test-key", "primary")

		health := hc.GetProviderHealth("openai")
		if health == nil {
			t.Error("Expected provider health to be registered")
		}
		if health.Name != "openai" {
			t.Errorf("Expected name 'openai', got %s", health.Name)
		}
		if !health.Healthy {
			t.Error("Expected initial healthy state to be true")
		}
		if health.Endpoint != "https://api.openai.com/v1/chat/completions" {
			t.Errorf("Unexpected endpoint: %s", health.Endpoint)
		}
	})
}

func TestHealthChecker_RegisterCircuitBreaker(t *testing.T) {
	t.Run("registers circuit breaker correctly", func(t *testing.T) {
		hc := NewHealthChecker(nil, &config.Config{}, nil)
		p := provider.NewOpenAIProvider("https://api.openai.com/v1")
		cb := NewCircuitBreaker(5, 2, 30*time.Second)

		hc.RegisterProvider("openai", p, "test-key", "primary")
		hc.RegisterCircuitBreaker("openai", cb)

		health := hc.GetProviderHealth("openai")
		if health.CircuitBreakerState != "closed" {
			t.Errorf("Expected circuit breaker state 'closed', got %s", health.CircuitBreakerState)
		}
	})
}

func TestHealthChecker_GetHealth(t *testing.T) {
	t.Run("returns copy of health data", func(t *testing.T) {
		hc := NewHealthChecker(nil, &config.Config{}, nil)
		p := provider.NewOpenAIProvider("https://api.openai.com/v1")

		hc.RegisterProvider("openai", p, "test-key", "primary")

		health1 := hc.GetHealth()
		health2 := hc.GetHealth()

		// Modify one copy
		health1["openai"].Healthy = false

		// Other copy should not be affected
		if !health2["openai"].Healthy {
			t.Error("Expected GetHealth to return a copy")
		}
	})
}

func TestHealthChecker_IsHealthy(t *testing.T) {
	t.Run("returns true when all providers healthy", func(t *testing.T) {
		hc := NewHealthChecker(nil, &config.Config{}, nil)
		p := provider.NewOpenAIProvider("https://api.openai.com/v1")

		hc.RegisterProvider("openai", p, "test-key", "primary")

		if !hc.IsHealthy() {
			t.Error("Expected IsHealthy to be true for healthy providers")
		}
	})

	t.Run("returns false when circuit breaker is open", func(t *testing.T) {
		hc := NewHealthChecker(nil, &config.Config{}, nil)
		p := provider.NewOpenAIProvider("https://api.openai.com/v1")
		cb := NewCircuitBreaker(1, 2, 30*time.Second)

		hc.RegisterProvider("openai", p, "test-key", "primary")
		hc.RegisterCircuitBreaker("openai", cb)

		// Open the circuit breaker
		cb.RecordFailure()

		if hc.IsHealthy() {
			t.Error("Expected IsHealthy to be false when circuit breaker is open")
		}
	})
}

func TestHealthChecker_GetStats(t *testing.T) {
	t.Run("returns correct stats", func(t *testing.T) {
		hc := NewHealthChecker(nil, &config.Config{}, nil)
		p := provider.NewOpenAIProvider("https://api.openai.com/v1")

		hc.RegisterProvider("openai", p, "test-key", "primary")

		stats := hc.GetStats()

		if stats["total_providers"].(int) != 1 {
			t.Errorf("Expected 1 provider, got %d", stats["total_providers"])
		}
		if stats["healthy_count"].(int) != 1 {
			t.Errorf("Expected 1 healthy provider, got %d", stats["healthy_count"])
		}
		if !stats["enabled"].(bool) {
			t.Error("Expected enabled to be true")
		}
	})
}

func TestHealthChecker_doHealthCheck(t *testing.T) {
	t.Run("returns healthy for 200 response", func(t *testing.T) {
		// Create a mock server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		hc := NewHealthChecker(&HealthCheckerConfig{
			Timeout: 5 * time.Second,
		}, &config.Config{}, http.DefaultClient)

		p := provider.NewOpenAIProvider(server.URL)
		info := &providerInfo{
			provider: p,
			apiKey:   "test-key",
		}

		healthy, err := hc.doHealthCheck(context.Background(), info)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !healthy {
			t.Error("Expected healthy to be true")
		}
	})

	t.Run("returns healthy for 401 response (server is up)", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))
		defer server.Close()

		hc := NewHealthChecker(&HealthCheckerConfig{
			Timeout: 5 * time.Second,
		}, &config.Config{}, http.DefaultClient)

		p := provider.NewOpenAIProvider(server.URL)
		info := &providerInfo{
			provider: p,
			apiKey:   "test-key",
		}

		healthy, err := hc.doHealthCheck(context.Background(), info)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !healthy {
			t.Error("Expected healthy to be true (401 means server is responding)")
		}
	})

	t.Run("returns unhealthy for 500 response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		hc := NewHealthChecker(&HealthCheckerConfig{
			Timeout: 5 * time.Second,
		}, &config.Config{}, http.DefaultClient)

		p := provider.NewOpenAIProvider(server.URL)
		info := &providerInfo{
			provider: p,
			apiKey:   "test-key",
		}

		healthy, _ := hc.doHealthCheck(context.Background(), info)
		if healthy {
			t.Error("Expected healthy to be false for 500 response")
		}
	})

	t.Run("returns unhealthy for connection failure", func(t *testing.T) {
		hc := NewHealthChecker(&HealthCheckerConfig{
			Timeout: 1 * time.Second,
		}, &config.Config{}, http.DefaultClient)

		// Use an invalid URL that will fail to connect
		p := provider.NewOpenAIProvider("http://127.0.0.1:1")
		info := &providerInfo{
			provider: p,
			apiKey:   "test-key",
		}

		healthy, err := hc.doHealthCheck(context.Background(), info)
		if healthy {
			t.Error("Expected healthy to be false for connection failure")
		}
		if err == nil {
			t.Error("Expected error for connection failure")
		}
	})
}

func TestHealthChecker_StartStop(t *testing.T) {
	t.Run("does not start when disabled", func(t *testing.T) {
		hc := NewHealthChecker(&HealthCheckerConfig{
			Enabled:       false,
			CheckInterval: 100 * time.Millisecond,
		}, &config.Config{}, nil)

		// Start should return immediately when disabled
		hc.Start()

		// No panic means success
		hc.Stop()
	})

	t.Run("starts and stops cleanly", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		hc := NewHealthChecker(&HealthCheckerConfig{
			Enabled:       true,
			CheckInterval: 100 * time.Millisecond,
			Timeout:       50 * time.Millisecond,
		}, &config.Config{}, http.DefaultClient)

		p := provider.NewOpenAIProvider(server.URL)
		hc.RegisterProvider("openai", p, "test-key", "primary")

		hc.Start()

		// Wait for at least one check
		time.Sleep(150 * time.Millisecond)

		// Verify check was performed
		health := hc.GetProviderHealth("openai")
		if health.TotalChecks == 0 {
			t.Error("Expected at least one health check to be performed")
		}

		hc.Stop()
	})
}

func TestHandleProvidersHealth(t *testing.T) {
	t.Run("returns disabled when health checker is nil", func(t *testing.T) {
		h := &Handler{
			healthChecker: nil,
		}

		req := httptest.NewRequest("GET", "/providers/health", http.NoBody)
		rr := httptest.NewRecorder()

		h.HandleProvidersHealth(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected 200, got %d", rr.Code)
		}
		// Check body contains "disabled"
		if !containsStr(rr.Body.String(), "disabled") {
			t.Error("Expected response to indicate health checker is disabled")
		}
	})

	t.Run("returns health data when enabled", func(t *testing.T) {
		hc := NewHealthChecker(nil, &config.Config{}, nil)
		p := provider.NewOpenAIProvider("https://api.openai.com/v1")
		hc.RegisterProvider("openai", p, "test-key", "primary")

		h := &Handler{
			healthChecker: hc,
		}

		req := httptest.NewRequest("GET", "/providers/health", http.NoBody)
		rr := httptest.NewRecorder()

		h.HandleProvidersHealth(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected 200, got %d", rr.Code)
		}
		// Check body contains provider name
		if !containsStr(rr.Body.String(), "openai") {
			t.Error("Expected response to contain provider name")
		}
	})
}

// Helper function to check if a string contains a substring
func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsMiddle(s, substr)))
}
