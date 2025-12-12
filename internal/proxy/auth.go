// Package proxy implements the HTTP proxy server.
package proxy

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"
)

// AuthConfig holds authentication configuration.
type AuthConfig struct {
	// Enabled indicates whether authentication is required.
	Enabled bool
	// APIKey is the required API key for authentication.
	// Clients must provide this key in the x-api-key header or Authorization header.
	APIKey string
	// AllowAnonymousHealth allows unauthenticated access to /health endpoint.
	AllowAnonymousHealth bool
	// AllowAnonymousMetrics allows unauthenticated access to /metrics endpoints.
	AllowAnonymousMetrics bool
}

// AuthMiddleware creates an authentication middleware.
// It validates the API key from the x-api-key header or Authorization header.
func AuthMiddleware(config *AuthConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip auth if not enabled
			if !config.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			// Allow anonymous access to specific endpoints
			path := r.URL.Path
			if config.AllowAnonymousHealth && path == "/health" {
				next.ServeHTTP(w, r)
				return
			}
			if config.AllowAnonymousMetrics && (path == "/metrics" || path == "/metrics/prometheus") {
				next.ServeHTTP(w, r)
				return
			}

			// Root endpoint is always accessible
			if path == "/" {
				next.ServeHTTP(w, r)
				return
			}

			// Extract API key from request
			apiKey := extractAPIKey(r)
			if apiKey == "" {
				writeAuthError(w, http.StatusUnauthorized, "authentication_error", "Missing API key. Provide via x-api-key header or Authorization: Bearer <key>")
				return
			}

			// Constant-time comparison to prevent timing attacks
			if subtle.ConstantTimeCompare([]byte(apiKey), []byte(config.APIKey)) != 1 {
				writeAuthError(w, http.StatusUnauthorized, "authentication_error", "Invalid API key")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// extractAPIKey extracts the API key from the request headers.
// It checks the following in order:
// 1. x-api-key header
// 2. Authorization header (Bearer token)
func extractAPIKey(r *http.Request) string {
	// Check x-api-key header first
	if key := r.Header.Get("x-api-key"); key != "" {
		return key
	}

	// Check Authorization header
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return ""
	}

	// Support "Bearer <key>" format
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}

	// Support raw API key in Authorization header
	return auth
}

// writeAuthError writes an Anthropic-formatted authentication error response.
func writeAuthError(w http.ResponseWriter, status int, errType, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("WWW-Authenticate", "Bearer")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"type": "error",
		"error": map[string]string{
			"type":    errType,
			"message": message,
		},
	})
}
