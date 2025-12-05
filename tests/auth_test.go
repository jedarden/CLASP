package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jedarden/clasp/internal/proxy"
)

func TestAuthMiddleware_DisabledAuth(t *testing.T) {
	config := &proxy.AuthConfig{
		Enabled: false,
		APIKey:  "test-key",
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	middleware := proxy.AuthMiddleware(config)(handler)

	req := httptest.NewRequest(http.MethodGet, "/v1/messages", nil)
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
}

func TestAuthMiddleware_ValidAPIKey_XAPIKeyHeader(t *testing.T) {
	config := &proxy.AuthConfig{
		Enabled: true,
		APIKey:  "test-secret-key",
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	middleware := proxy.AuthMiddleware(config)(handler)

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	req.Header.Set("x-api-key", "test-secret-key")
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
}

func TestAuthMiddleware_ValidAPIKey_BearerToken(t *testing.T) {
	config := &proxy.AuthConfig{
		Enabled: true,
		APIKey:  "test-secret-key",
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	middleware := proxy.AuthMiddleware(config)(handler)

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	req.Header.Set("Authorization", "Bearer test-secret-key")
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
}

func TestAuthMiddleware_MissingAPIKey(t *testing.T) {
	config := &proxy.AuthConfig{
		Enabled: true,
		APIKey:  "test-secret-key",
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	middleware := proxy.AuthMiddleware(config)(handler)

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rec.Code)
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

	if errDetails["type"] != "authentication_error" {
		t.Errorf("Expected error type 'authentication_error', got %v", errDetails["type"])
	}
}

func TestAuthMiddleware_InvalidAPIKey(t *testing.T) {
	config := &proxy.AuthConfig{
		Enabled: true,
		APIKey:  "correct-key",
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	middleware := proxy.AuthMiddleware(config)(handler)

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	req.Header.Set("x-api-key", "wrong-key")
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rec.Code)
	}
}

func TestAuthMiddleware_AnonymousHealthAllowed(t *testing.T) {
	config := &proxy.AuthConfig{
		Enabled:              true,
		APIKey:               "test-key",
		AllowAnonymousHealth: true,
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	middleware := proxy.AuthMiddleware(config)(handler)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200 for anonymous health check, got %d", rec.Code)
	}
}

func TestAuthMiddleware_AnonymousHealthDenied(t *testing.T) {
	config := &proxy.AuthConfig{
		Enabled:              true,
		APIKey:               "test-key",
		AllowAnonymousHealth: false,
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	middleware := proxy.AuthMiddleware(config)(handler)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401 for denied health check, got %d", rec.Code)
	}
}

func TestAuthMiddleware_AnonymousMetricsAllowed(t *testing.T) {
	config := &proxy.AuthConfig{
		Enabled:               true,
		APIKey:                "test-key",
		AllowAnonymousMetrics: true,
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	middleware := proxy.AuthMiddleware(config)(handler)

	// Test /metrics endpoint
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	middleware.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200 for anonymous /metrics, got %d", rec.Code)
	}

	// Test /metrics/prometheus endpoint
	req = httptest.NewRequest(http.MethodGet, "/metrics/prometheus", nil)
	rec = httptest.NewRecorder()
	middleware.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200 for anonymous /metrics/prometheus, got %d", rec.Code)
	}
}

func TestAuthMiddleware_AnonymousMetricsDenied(t *testing.T) {
	config := &proxy.AuthConfig{
		Enabled:               true,
		APIKey:                "test-key",
		AllowAnonymousMetrics: false,
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	middleware := proxy.AuthMiddleware(config)(handler)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	middleware.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401 for denied /metrics, got %d", rec.Code)
	}
}

func TestAuthMiddleware_RootEndpointAlwaysAccessible(t *testing.T) {
	config := &proxy.AuthConfig{
		Enabled: true,
		APIKey:  "test-key",
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	middleware := proxy.AuthMiddleware(config)(handler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	middleware.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200 for root endpoint, got %d", rec.Code)
	}
}

func TestAuthMiddleware_WWWAuthenticateHeader(t *testing.T) {
	config := &proxy.AuthConfig{
		Enabled: true,
		APIKey:  "test-key",
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := proxy.AuthMiddleware(config)(handler)

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	rec := httptest.NewRecorder()
	middleware.ServeHTTP(rec, req)

	if rec.Header().Get("WWW-Authenticate") != "Bearer" {
		t.Errorf("Expected WWW-Authenticate: Bearer header, got %s", rec.Header().Get("WWW-Authenticate"))
	}
}

func TestAuthMiddleware_RawAuthorizationHeader(t *testing.T) {
	config := &proxy.AuthConfig{
		Enabled: true,
		APIKey:  "test-secret-key",
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	middleware := proxy.AuthMiddleware(config)(handler)

	// Test with raw Authorization header (no Bearer prefix)
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	req.Header.Set("Authorization", "test-secret-key")
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
}
