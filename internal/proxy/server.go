// Package proxy implements the HTTP proxy server.
package proxy

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jedarden/clasp/internal/config"
)

// Server represents the CLASP proxy server.
type Server struct {
	cfg            *config.Config
	handler        *Handler
	server         *http.Server
	rateLimiter    *RateLimiter
	cache          *RequestCache
	authConfig     *AuthConfig
	queue          *RequestQueue
	circuitBreaker *CircuitBreaker
}

// NewServer creates a new proxy server.
func NewServer(cfg *config.Config) (*Server, error) {
	handler, err := NewHandler(cfg)
	if err != nil {
		return nil, fmt.Errorf("creating handler: %w", err)
	}

	s := &Server{
		cfg:     cfg,
		handler: handler,
	}

	// Initialize rate limiter if enabled
	if cfg.RateLimitEnabled {
		s.rateLimiter = NewRateLimiter(
			cfg.RateLimitRequests,
			cfg.RateLimitWindow,
			cfg.RateLimitBurst,
		)
		// Set rate limiter on handler for metrics
		s.handler.SetRateLimiter(s.rateLimiter)
	}

	// Initialize cache if enabled
	if cfg.CacheEnabled {
		s.cache = NewRequestCache(cfg.CacheMaxSize, time.Duration(cfg.CacheTTL)*time.Second)
		s.handler.SetCache(s.cache)
	}

	// Initialize authentication if enabled
	if cfg.AuthEnabled {
		s.authConfig = &AuthConfig{
			Enabled:               true,
			APIKey:                cfg.AuthAPIKey,
			AllowAnonymousHealth:  cfg.AuthAllowAnonymousHealth,
			AllowAnonymousMetrics: cfg.AuthAllowAnonymousMetrics,
		}
	}

	// Initialize request queue if enabled
	if cfg.QueueEnabled {
		queueConfig := &QueueConfig{
			Enabled:    true,
			MaxSize:    cfg.QueueMaxSize,
			MaxWait:    time.Duration(cfg.QueueMaxWaitSeconds) * time.Second,
			RetryDelay: time.Duration(cfg.QueueRetryDelayMs) * time.Millisecond,
			MaxRetries: cfg.QueueMaxRetries,
		}
		s.queue = NewRequestQueue(queueConfig)
		s.handler.SetQueue(s.queue)
	}

	// Initialize circuit breaker if enabled
	if cfg.CircuitBreakerEnabled {
		s.circuitBreaker = NewCircuitBreaker(
			cfg.CircuitBreakerThreshold,
			cfg.CircuitBreakerRecovery,
			time.Duration(cfg.CircuitBreakerTimeoutSec)*time.Second,
		)
		s.handler.SetCircuitBreaker(s.circuitBreaker)
	}

	return s, nil
}

// Start starts the proxy server.
func (s *Server) Start() error {
	// Create mux
	mux := http.NewServeMux()

	// Register routes
	mux.HandleFunc("/", s.handler.HandleRoot)
	mux.HandleFunc("/health", s.handler.HandleHealth)
	mux.HandleFunc("/metrics", s.handler.HandleMetrics)
	mux.HandleFunc("/metrics/prometheus", s.handler.HandleMetricsPrometheus)
	mux.HandleFunc("/costs", s.handler.HandleCosts)
	mux.HandleFunc("/v1/messages", s.handler.HandleMessages)

	// Build middleware chain
	var handler http.Handler = mux

	// Apply rate limiting middleware if enabled
	if s.rateLimiter != nil {
		handler = RateLimitMiddleware(s.rateLimiter)(handler)
		log.Printf("[CLASP] Rate limiting enabled: %d requests per %d seconds (burst: %d)",
			s.cfg.RateLimitRequests, s.cfg.RateLimitWindow, s.cfg.RateLimitBurst)
	}

	// Log cache status
	if s.cache != nil {
		log.Printf("[CLASP] Response caching enabled: max %d entries, TTL %d seconds",
			s.cfg.CacheMaxSize, s.cfg.CacheTTL)
	}

	// Log queue status
	if s.queue != nil {
		log.Printf("[CLASP] Request queue enabled: max %d requests, timeout %d seconds",
			s.cfg.QueueMaxSize, s.cfg.QueueMaxWaitSeconds)
	}

	// Log circuit breaker status
	if s.circuitBreaker != nil {
		log.Printf("[CLASP] Circuit breaker enabled: threshold %d failures, recovery %d successes, timeout %d seconds",
			s.cfg.CircuitBreakerThreshold, s.cfg.CircuitBreakerRecovery, s.cfg.CircuitBreakerTimeoutSec)
	}

	// Apply authentication middleware if enabled
	if s.authConfig != nil && s.authConfig.Enabled {
		handler = AuthMiddleware(s.authConfig)(handler)
		log.Printf("[CLASP] Authentication enabled (anonymous health: %v, anonymous metrics: %v)",
			s.authConfig.AllowAnonymousHealth, s.authConfig.AllowAnonymousMetrics)
	}

	// Apply logging middleware
	handler = loggingMiddleware(handler)

	// Auto-select port if default port is in use
	port := s.cfg.Port
	if !isPortAvailable(port) {
		log.Printf("[CLASP] Port %d is in use, finding available port...", port)
		newPort, err := findAvailablePort(port)
		if err != nil {
			return fmt.Errorf("failed to find available port: %w", err)
		}
		port = newPort
		s.cfg.Port = port
		log.Printf("[CLASP] Using port %d instead", port)
	}

	// Create server
	s.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second, // Long timeout for streaming
		IdleTimeout:  120 * time.Second,
	}

	// Start server in goroutine
	errCh := make(chan error, 1)
	go func() {
		log.Printf("[CLASP] Starting proxy server on port %d", port)
		log.Printf("[CLASP] Provider: %s", s.cfg.Provider)
		if s.cfg.DefaultModel != "" {
			log.Printf("[CLASP] Default model: %s", s.cfg.DefaultModel)
		}
		log.Printf("[CLASP] Set ANTHROPIC_BASE_URL=http://localhost:%d to use with Claude Code", port)

		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return fmt.Errorf("server error: %w", err)
	case sig := <-sigCh:
		log.Printf("[CLASP] Received signal %v, shutting down...", sig)
		return s.Shutdown()
	}
}

// isPortAvailable checks if a port is available for binding.
func isPortAvailable(port int) bool {
	addr := fmt.Sprintf(":%d", port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	ln.Close()
	return true
}

// findAvailablePort finds an available port starting from the given port.
// It tries the next 100 ports before giving up.
func findAvailablePort(startPort int) (int, error) {
	for port := startPort + 1; port <= startPort+100; port++ {
		if isPortAvailable(port) {
			return port, nil
		}
	}
	// If all nearby ports are taken, let the OS assign one
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, err
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port, nil
}

// GetPort returns the actual port the server is running on.
// This is useful when auto-port selection is used.
func (s *Server) GetPort() int {
	return s.cfg.Port
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := s.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown error: %w", err)
	}

	log.Printf("[CLASP] Server stopped")
	return nil
}

// loggingMiddleware logs incoming requests.
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap response writer to capture status
		lrw := &loggingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(lrw, r)

		duration := time.Since(start)
		log.Printf("[CLASP] %s %s %d %v", r.Method, r.URL.Path, lrw.statusCode, duration)
	})
}

// loggingResponseWriter wraps http.ResponseWriter to capture status code.
type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

// Flush implements http.Flusher.
func (lrw *loggingResponseWriter) Flush() {
	if f, ok := lrw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
