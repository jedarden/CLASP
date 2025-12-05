// Package proxy implements the HTTP proxy server.
package proxy

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jedarden/clasp/internal/config"
)

// Server represents the CLASP proxy server.
type Server struct {
	cfg         *config.Config
	handler     *Handler
	server      *http.Server
	rateLimiter *RateLimiter
	cache       *RequestCache
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

	// Apply logging middleware
	handler = loggingMiddleware(handler)

	// Create server
	s.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.cfg.Port),
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second, // Long timeout for streaming
		IdleTimeout:  120 * time.Second,
	}

	// Start server in goroutine
	errCh := make(chan error, 1)
	go func() {
		log.Printf("[CLASP] Starting proxy server on port %d", s.cfg.Port)
		log.Printf("[CLASP] Provider: %s", s.cfg.Provider)
		if s.cfg.DefaultModel != "" {
			log.Printf("[CLASP] Default model: %s", s.cfg.DefaultModel)
		}
		log.Printf("[CLASP] Set ANTHROPIC_BASE_URL=http://localhost:%d to use with Claude Code", s.cfg.Port)

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
