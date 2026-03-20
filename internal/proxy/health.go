// Package proxy implements the HTTP proxy server.
package proxy

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/jedarden/clasp/internal/config"
	"github.com/jedarden/clasp/internal/provider"
)

// ProviderHealth represents the health status of a single provider.
type ProviderHealth struct {
	Name                  string        `json:"name"`
	Healthy               bool          `json:"healthy"`
	CircuitBreakerState   string        `json:"circuit_breaker_state,omitempty"`
	LastCheckTime         time.Time     `json:"last_check_time"`
	LastSuccessTime       *time.Time    `json:"last_success_time,omitempty"`
	LastFailureTime       *time.Time    `json:"last_failure_time,omitempty"`
	LastLatency           time.Duration `json:"last_latency,omitempty"`
	AvgLatencyMs          int64         `json:"avg_latency_ms"`
	ConsecutiveFailures   int           `json:"consecutive_failures"`
	TotalChecks           int64         `json:"total_checks"`
	SuccessfulChecks      int64         `json:"successful_checks"`
	FailedChecks          int64         `json:"failed_checks"`
	LastError             string        `json:"last_error,omitempty"`
	Endpoint              string        `json:"endpoint"`
	RequiresTransform     bool          `json:"requires_transformation"`
}

// HealthCheckerConfig holds configuration for the health checker.
type HealthCheckerConfig struct {
	Enabled       bool
	CheckInterval time.Duration
	Timeout       time.Duration
}

// DefaultHealthCheckerConfig returns the default health checker configuration.
func DefaultHealthCheckerConfig() *HealthCheckerConfig {
	return &HealthCheckerConfig{
		Enabled:       true,
		CheckInterval: 30 * time.Second,
		Timeout:       10 * time.Second,
	}
}

// HealthChecker performs periodic health checks on configured providers.
type HealthChecker struct {
	config     *HealthCheckerConfig
	cfg        *config.Config
	client     *http.Client
	shutdownCh chan struct{}

	// Provider registry
	mu         sync.RWMutex
	providers  map[string]*providerInfo
	health     map[string]*ProviderHealth
	circuitMap map[string]*CircuitBreaker // Maps provider name to circuit breaker

	// Aggregated metrics
	totalChecks      int64
	successfulChecks int64
	failedChecks     int64
}

type providerInfo struct {
	provider provider.Provider
	apiKey   string
	tier     string // Optional tier name for multi-provider routing
}

// NewHealthChecker creates a new health checker.
func NewHealthChecker(cfg *HealthCheckerConfig, appCfg *config.Config, client *http.Client) *HealthChecker {
	if cfg == nil {
		cfg = DefaultHealthCheckerConfig()
	}
	if client == nil {
		client = &http.Client{
			Timeout: cfg.Timeout,
		}
	}
	return &HealthChecker{
		config:     cfg,
		cfg:        appCfg,
		client:     client,
		shutdownCh: make(chan struct{}),
		providers:  make(map[string]*providerInfo),
		health:     make(map[string]*ProviderHealth),
		circuitMap: make(map[string]*CircuitBreaker),
	}
}

// RegisterProvider registers a provider for health checking.
func (hc *HealthChecker) RegisterProvider(name string, p provider.Provider, apiKey string, tier string) {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	hc.providers[name] = &providerInfo{
		provider: p,
		apiKey:   apiKey,
		tier:     tier,
	}

	// Initialize health status
	hc.health[name] = &ProviderHealth{
		Name:              name,
		Healthy:           true, // Assume healthy until first check
		Endpoint:          p.GetEndpointURL(),
		RequiresTransform: p.RequiresTransformation(),
	}
}

// RegisterCircuitBreaker registers a circuit breaker for a provider.
func (hc *HealthChecker) RegisterCircuitBreaker(providerName string, cb *CircuitBreaker) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.circuitMap[providerName] = cb
}

// Start begins the periodic health check goroutine.
func (hc *HealthChecker) Start() {
	if !hc.config.Enabled {
		return
	}

	go hc.run()
	log.Printf("[CLASP] Health checker started (interval: %v)", hc.config.CheckInterval)
}

// Stop stops the health checker.
func (hc *HealthChecker) Stop() {
	close(hc.shutdownCh)
}

// run is the main health check loop.
func (hc *HealthChecker) run() {
	// Perform initial check
	hc.checkAllProviders()

	ticker := time.NewTicker(hc.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-hc.shutdownCh:
			return
		case <-ticker.C:
			hc.checkAllProviders()
		}
	}
}

// checkAllProviders performs health checks on all registered providers.
func (hc *HealthChecker) checkAllProviders() {
	hc.mu.RLock()
	providers := make([]*providerInfo, 0, len(hc.providers))
	names := make([]string, 0, len(hc.providers))
	for name, info := range hc.providers {
		providers = append(providers, info)
		names = append(names, name)
	}
	hc.mu.RUnlock()

	var wg sync.WaitGroup
	for i, info := range providers {
		wg.Add(1)
		go func(name string, p *providerInfo) {
			defer wg.Done()
			hc.checkProvider(name, p)
		}(names[i], info)
	}
	wg.Wait()
}

// checkProvider performs a health check on a single provider.
func (hc *HealthChecker) checkProvider(name string, info *providerInfo) {
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), hc.config.Timeout)
	defer cancel()

	healthy, err := hc.doHealthCheck(ctx, info)
	latency := time.Since(start)

	hc.mu.Lock()
	defer hc.mu.Unlock()

	health, ok := hc.health[name]
	if !ok {
		return
	}

	hc.totalChecks++
	health.TotalChecks++
	health.LastCheckTime = time.Now()
	health.LastLatency = latency

	// Update latency average (simple moving average)
	if health.TotalChecks > 1 {
		health.AvgLatencyMs = (health.AvgLatencyMs*(health.TotalChecks-1) + latency.Milliseconds()) / health.TotalChecks
	} else {
		health.AvgLatencyMs = latency.Milliseconds()
	}

	if healthy {
		health.Healthy = true
		health.SuccessfulChecks++
		health.ConsecutiveFailures = 0
		health.LastSuccessTime = &health.LastCheckTime
		health.LastError = ""
		hc.successfulChecks++
	} else {
		health.Healthy = false
		health.FailedChecks++
		health.ConsecutiveFailures++
		health.LastFailureTime = &health.LastCheckTime
		if err != nil {
			health.LastError = err.Error()
		}
		hc.failedChecks++
	}

	// Update circuit breaker state if available
	if cb, ok := hc.circuitMap[name]; ok {
		health.CircuitBreakerState = cb.State()
	}
}

// doHealthCheck performs the actual HTTP health check.
func (hc *HealthChecker) doHealthCheck(ctx context.Context, info *providerInfo) (bool, error) {
	// For most providers, we check if we can reach the models endpoint
	// This is a lightweight check that doesn't consume tokens
	endpoint := info.provider.GetEndpointURL()

	// Try to hit a lightweight endpoint
	// For OpenAI-compatible providers, use /models endpoint
	checkURL := endpoint
	if info.provider.RequiresTransformation() {
		// OpenAI-compatible: check /models endpoint
		checkURL = endpoint + "/models"
	} else {
		// Anthropic passthrough: just check if we can reach the base
		// Use a HEAD request to minimize overhead
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, checkURL, nil)
	if err != nil {
		return false, err
	}

	// Set headers
	headers := info.provider.GetHeaders(info.apiKey)
	for key, values := range headers {
		for _, v := range values {
			req.Header.Add(key, v)
		}
	}

	resp, err := hc.client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	// Consider healthy if we get any response (even 401 means the server is up)
	// Only consider unhealthy on 5xx errors or connection failures
	if resp.StatusCode >= 500 {
		return false, nil
	}

	return true, nil
}

// GetHealth returns the health status of all providers.
func (hc *HealthChecker) GetHealth() map[string]*ProviderHealth {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	// Return a copy to avoid race conditions
	result := make(map[string]*ProviderHealth, len(hc.health))
	for name, health := range hc.health {
		healthCopy := *health
		// Update circuit breaker state if available
		if cb, ok := hc.circuitMap[name]; ok {
			healthCopy.CircuitBreakerState = cb.State()
		}
		result[name] = &healthCopy
	}
	return result
}

// GetProviderHealth returns the health status of a specific provider.
func (hc *HealthChecker) GetProviderHealth(name string) *ProviderHealth {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	if health, ok := hc.health[name]; ok {
		healthCopy := *health
		// Update circuit breaker state if available
		if cb, ok := hc.circuitMap[name]; ok {
			healthCopy.CircuitBreakerState = cb.State()
		}
		return &healthCopy
	}
	return nil
}

// IsHealthy returns true if all providers are healthy.
func (hc *HealthChecker) IsHealthy() bool {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	for _, health := range hc.health {
		if !health.Healthy {
			return false
		}
		// Also check circuit breaker
		if cb, ok := hc.circuitMap[health.Name]; ok {
			if cb.IsOpen() {
				return false
			}
		}
	}
	return true
}

// GetStats returns aggregated health check statistics.
func (hc *HealthChecker) GetStats() map[string]interface{} {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	healthyCount := 0
	unhealthyCount := 0
	for _, health := range hc.health {
		if health.Healthy {
			healthyCount++
		} else {
			unhealthyCount++
		}
	}

	return map[string]interface{}{
		"enabled":            hc.config.Enabled,
		"check_interval_sec": hc.config.CheckInterval.Seconds(),
		"total_providers":    len(hc.providers),
		"healthy_count":      healthyCount,
		"unhealthy_count":    unhealthyCount,
		"total_checks":       hc.totalChecks,
		"successful_checks":  hc.successfulChecks,
		"failed_checks":      hc.failedChecks,
	}
}

// HealthResponse represents the response from the /health endpoint.
type HealthResponse struct {
	Status    string                     `json:"status"`
	Uptime    string                     `json:"uptime"`
	Providers map[string]*ProviderHealth `json:"providers,omitempty"`
	Summary   *HealthSummary             `json:"summary,omitempty"`
}

// HealthSummary provides an overview of all provider health.
type HealthSummary struct {
	TotalProviders   int   `json:"total_providers"`
	HealthyProviders int   `json:"healthy_providers"`
	TotalChecks      int64 `json:"total_checks"`
	SuccessfulChecks int64 `json:"successful_checks"`
	FailedChecks     int64 `json:"failed_checks"`
}

// HandleProvidersHealth handles the /providers/health endpoint.
func (h *Handler) HandleProvidersHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if h.healthChecker == nil {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "disabled",
			"message": "Health checker is not enabled",
		})
		return
	}

	health := h.healthChecker.GetHealth()
	stats := h.healthChecker.GetStats()

	response := map[string]interface{}{
		"status":    "ok",
		"providers": health,
		"summary":   stats,
	}

	_ = json.NewEncoder(w).Encode(response)
}
