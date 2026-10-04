package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jedarden/clasp/internal/proxy"
)

const integrationAPIKey = "integration-test-key"

func newProtectedAuthServer(t *testing.T, authConfig proxy.AuthConfig) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/messages", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("metrics"))
	})
	mux.HandleFunc("/metrics/prometheus", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("# HELP clasp_requests_total requests\n"))
	})

	return httptest.NewServer(proxy.AuthMiddleware(&authConfig)(mux))
}

func assertAuthFailure(t *testing.T, response *http.Response, wantMessage string) {
	t.Helper()
	defer response.Body.Close()

	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusUnauthorized)
	}
	if got := response.Header.Get("WWW-Authenticate"); got != "Bearer" {
		t.Errorf("WWW-Authenticate = %q, want Bearer", got)
	}

	var payload struct {
		Type  string `json:"type"`
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode authentication error: %v", err)
	}
	if payload.Type != "error" || payload.Error.Type != "authentication_error" {
		t.Fatalf("authentication error payload = %+v", payload)
	}
	if payload.Error.Message != wantMessage {
		t.Errorf("authentication error message = %q, want %q", payload.Error.Message, wantMessage)
	}
}

func TestAuthMiddleware_ProtectedProxyRequests(t *testing.T) {
	server := newProtectedAuthServer(t, proxy.AuthConfig{
		Enabled: true,
		APIKey:  integrationAPIKey,
	})
	defer server.Close()

	client := &http.Client{}
	tests := []struct {
		name       string
		header     string
		value      string
		wantStatus int
	}{
		{name: "missing key", wantStatus: http.StatusUnauthorized},
		{name: "invalid key", header: "x-api-key", value: "wrong-key", wantStatus: http.StatusUnauthorized},
		{name: "x-api-key", header: "x-api-key", value: integrationAPIKey, wantStatus: http.StatusOK},
		{name: "bearer", header: "Authorization", value: "Bearer " + integrationAPIKey, wantStatus: http.StatusOK},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request, err := http.NewRequest(http.MethodPost, server.URL+"/v1/messages", http.NoBody)
			if err != nil {
				t.Fatal(err)
			}
			if test.header != "" {
				request.Header.Set(test.header, test.value)
			}
			response, err := client.Do(request)
			if err != nil {
				t.Fatal(err)
			}
			if test.wantStatus == http.StatusUnauthorized {
				wantMessage := "Invalid API key"
				if test.header == "" {
					wantMessage = "Missing API key. Provide via x-api-key header or Authorization: Bearer <key>"
				}
				assertAuthFailure(t, response, wantMessage)
				return
			}
			defer response.Body.Close()
			if response.StatusCode != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.StatusCode, test.wantStatus)
			}
		})
	}
}

func TestAuthMiddleware_ProtectedMetricsRequests(t *testing.T) {
	server := newProtectedAuthServer(t, proxy.AuthConfig{
		Enabled: true,
		APIKey:  integrationAPIKey,
	})
	defer server.Close()

	for _, path := range []string{"/metrics", "/metrics/prometheus"} {
		t.Run(path+" rejects missing and invalid keys", func(t *testing.T) {
			for _, test := range []struct {
				name   string
				header string
				value  string
			}{
				{name: "missing"},
				{name: "invalid", header: "x-api-key", value: "wrong-key"},
			} {
				t.Run(test.name, func(t *testing.T) {
					request, err := http.NewRequest(http.MethodGet, server.URL+path, http.NoBody)
					if err != nil {
						t.Fatal(err)
					}
					if test.header != "" {
						request.Header.Set(test.header, test.value)
					}
					response, err := http.DefaultClient.Do(request)
					if err != nil {
						t.Fatal(err)
					}
					wantMessage := "Invalid API key"
					if test.header == "" {
						wantMessage = "Missing API key. Provide via x-api-key header or Authorization: Bearer <key>"
					}
					assertAuthFailure(t, response, wantMessage)
				})
			}
		})

		t.Run(path+" accepts a valid key", func(t *testing.T) {
			request, err := http.NewRequest(http.MethodGet, server.URL+path, http.NoBody)
			if err != nil {
				t.Fatal(err)
			}
			request.Header.Set("Authorization", "Bearer "+integrationAPIKey)
			response, err := http.DefaultClient.Do(request)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			if response.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusOK)
			}
		})
	}
}

func TestAuthMiddleware_DisabledAuthAllowsProxyAndMetrics(t *testing.T) {
	server := newProtectedAuthServer(t, proxy.AuthConfig{APIKey: integrationAPIKey})
	defer server.Close()

	for _, path := range []string{"/v1/messages", "/metrics", "/metrics/prometheus"} {
		method := http.MethodGet
		if path == "/v1/messages" {
			method = http.MethodPost
		}
		request, err := http.NewRequest(method, server.URL+path, http.NoBody)
		if err != nil {
			t.Fatal(err)
		}
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusOK {
			t.Errorf("disabled auth GET %s status = %d, want %d", path, response.StatusCode, http.StatusOK)
		}
	}
}

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

	req := httptest.NewRequest(http.MethodGet, "/v1/messages", http.NoBody)
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

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", http.NoBody)
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

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", http.NoBody)
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

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", http.NoBody)
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

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", http.NoBody)
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

	req := httptest.NewRequest(http.MethodGet, "/health", http.NoBody)
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

	req := httptest.NewRequest(http.MethodGet, "/health", http.NoBody)
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
	req := httptest.NewRequest(http.MethodGet, "/metrics", http.NoBody)
	rec := httptest.NewRecorder()
	middleware.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200 for anonymous /metrics, got %d", rec.Code)
	}

	// Test /metrics/prometheus endpoint
	req = httptest.NewRequest(http.MethodGet, "/metrics/prometheus", http.NoBody)
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

	req := httptest.NewRequest(http.MethodGet, "/metrics", http.NoBody)
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

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
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

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", http.NoBody)
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
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", http.NoBody)
	req.Header.Set("Authorization", "test-secret-key")
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
}
