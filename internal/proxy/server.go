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
	"github.com/jedarden/clasp/internal/logging"
	"github.com/jedarden/clasp/internal/statusline"
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
	statusManager  *statusline.Manager
	version        string
	shutdownCh     chan struct{} // Channel to signal goroutines to stop
}

// NewServer creates a new proxy server.
func NewServer(cfg *config.Config) (*Server, error) {
	return NewServerWithVersion(cfg, "unknown")
}

// NewServerWithVersion creates a new proxy server with version info for status line.
func NewServerWithVersion(cfg *config.Config, version string) (*Server, error) {
	handler, err := NewHandler(cfg)
	if err != nil {
		return nil, fmt.Errorf("creating handler: %w", err)
	}

	// Pass version to handler for status endpoint
	handler.SetVersion(version)

	// Initialize status line manager
	statusManager, err := statusline.NewManager()
	if err != nil {
		log.Printf("[CLASP] Warning: Could not initialize status line: %v", err)
		// Continue without status line support
	}

	s := &Server{
		cfg:           cfg,
		handler:       handler,
		statusManager: statusManager,
		version:       version,
		shutdownCh:    make(chan struct{}),
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
	} else {
		log.Printf("[CLASP] Warning: Rate limiting is disabled. Set RATE_LIMIT_ENABLED=true for production use.")
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
	} else {
		log.Printf("[CLASP] Warning: Authentication is disabled. Set AUTH_ENABLED=true for production use.")
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

	// Set session port for logging - this enables port-specific log files
	logging.SetSessionPort(port)

	// Create server
	s.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second, // Long timeout for streaming
		IdleTimeout:  120 * time.Second,
	}

	// Update status line with initial status
	if s.statusManager != nil {
		// Set the port for per-instance status files
		s.statusManager.SetPort(port)

		// Configure status line on first run
		if !s.statusManager.IsConfigured() {
			if err := s.statusManager.Setup(); err != nil {
				log.Printf("[CLASP] Warning: Could not configure status line: %v", err)
			} else {
				log.Printf("[CLASP] Status line configured for Claude Code")
			}
		}

		// Clean up stale status files from previous instances
		if cleaned, err := statusline.CleanupStaleInstances(); err == nil && cleaned > 0 {
			log.Printf("[CLASP] Cleaned up %d stale status file(s)", cleaned)
		}

		// Write initial status
		model := s.cfg.DefaultModel
		if model == "" {
			model = "auto"
		}
		status := statusline.Status{
			Running:      true,
			Port:         port,
			SessionID:    logging.GetSessionID(),
			Provider:     string(s.cfg.Provider),
			Model:        model,
			Requests:     0,
			Errors:       0,
			CostUSD:      0,
			StartTime:    time.Now(),
			Version:      s.version,
			CacheEnabled: s.cache != nil,
		}
		if s.cfg.FallbackProvider != "" {
			status.Fallback = string(s.cfg.FallbackProvider)
		}
		if err := s.statusManager.UpdateStatus(status); err != nil {
			log.Printf("[CLASP] Warning: Could not update status: %v", err)
		}

		// Start metrics update goroutine
		go s.updateStatusPeriodically()
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
	ln, err := net.Listen("tcp", ":0") //nolint:gosec // G102: binding to all interfaces is intentional for port discovery
	if err != nil {
		return 0, err
	}
	defer ln.Close()
	tcpAddr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		return 0, fmt.Errorf("unexpected address type")
	}
	return tcpAddr.Port, nil
}

// GetPort returns the actual port the server is running on.
// This is useful when auto-port selection is used.
func (s *Server) GetPort() int {
	return s.cfg.Port
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown() error {
	// Signal all goroutines to stop
	close(s.shutdownCh)

	// Mark status as stopped
	if s.statusManager != nil {
		if err := s.statusManager.ClearStatus(); err != nil {
			log.Printf("[CLASP] Warning: Could not clear status: %v", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := s.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown error: %w", err)
	}

	log.Printf("[CLASP] Server stopped")
	return nil
}

// updateStatusPeriodically updates the status file with current metrics every 5 seconds.
// It terminates gracefully when the shutdown channel is closed.
func (s *Server) updateStatusPeriodically() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.shutdownCh:
			return
		case <-ticker.C:
			if s.server == nil {
				return
			}

			// Get metrics from handler
			metrics := s.handler.GetMetrics()
			costs := s.handler.GetCostTracker()

			// Calculate average latency
			var avgLatency float64
			if metrics.TotalRequests > 0 {
				avgLatency = float64(metrics.TotalLatencyMs) / float64(metrics.TotalRequests)
			}

			// Get cache hit rate if available
			var cacheHitRate float64
			if s.cache != nil {
				_, _, hits, misses, _ := s.cache.Stats()
				total := hits + misses
				if total > 0 {
					cacheHitRate = float64(hits) / float64(total)
				}
			}

			// Update status
			if err := s.statusManager.UpdateMetrics(
				metrics.TotalRequests,
				metrics.ErrorRequests,
				costs.GetTotalCostUSD(),
				avgLatency,
			); err != nil {
				// Silently ignore update errors
				continue
			}

			// Update cache stats
			if s.cache != nil {
				_ = s.statusManager.UpdateCacheStats(true, cacheHitRate)
			}
		}
	}
}

// GetHandler returns the handler for testing and metrics access.
func (s *Server) GetHandler() *Handler {
	return s.handler
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
