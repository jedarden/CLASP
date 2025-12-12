// Package proxy implements the HTTP proxy server.
package proxy

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// RateLimiter implements a token bucket rate limiter.
type RateLimiter struct {
	mu sync.Mutex

	// Configuration
	rate  float64 // tokens per second
	burst int     // maximum tokens

	// State
	tokens   float64
	lastTime time.Time

	// Metrics
	allowed int64
	denied  int64
}

// NewRateLimiter creates a new rate limiter.
// requests: number of requests allowed per window
// window: time window in seconds
// burst: additional burst capacity
func NewRateLimiter(requests, window, burst int) *RateLimiter {
	rate := float64(requests) / float64(window)
	return &RateLimiter{
		rate:     rate,
		burst:    burst,
		tokens:   float64(burst), // Start with full burst capacity
		lastTime: time.Now(),
	}
}

// Allow checks if a request should be allowed.
func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(rl.lastTime).Seconds()
	rl.lastTime = now

	// Add tokens based on elapsed time
	rl.tokens += elapsed * rl.rate

	// Cap at burst limit
	maxTokens := float64(rl.burst) + rl.rate // burst + 1 second worth
	if rl.tokens > maxTokens {
		rl.tokens = maxTokens
	}

	// Check if we have at least one token
	if rl.tokens >= 1.0 {
		rl.tokens -= 1.0
		rl.allowed++
		return true
	}

	rl.denied++
	return false
}

// Stats returns rate limiter statistics.
func (rl *RateLimiter) Stats() (allowed, denied int64) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	return rl.allowed, rl.denied
}

// WaitTime returns the duration until the next request would be allowed.
func (rl *RateLimiter) WaitTime() time.Duration {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if rl.tokens >= 1.0 {
		return 0
	}

	needed := 1.0 - rl.tokens
	return time.Duration(needed/rl.rate*1000) * time.Millisecond
}

// RateLimitMiddleware creates a middleware that enforces rate limiting.
func RateLimitMiddleware(limiter *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip rate limiting for non-API endpoints
			if r.URL.Path != "/v1/messages" {
				next.ServeHTTP(w, r)
				return
			}

			if !limiter.Allow() {
				writeRateLimitError(w, limiter.WaitTime())
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// writeRateLimitError writes an Anthropic-formatted rate limit error.
func writeRateLimitError(w http.ResponseWriter, retryAfter time.Duration) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Retry-After", retryAfter.String())
	w.WriteHeader(http.StatusTooManyRequests)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"type": "error",
		"error": map[string]string{
			"type":    "rate_limit_error",
			"message": "Request rate limit exceeded. Please slow down your requests.",
		},
	})
}
