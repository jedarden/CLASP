// Package proxy implements the HTTP proxy server.
package proxy

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/jedarden/clasp/internal/config"
	"github.com/jedarden/clasp/internal/logging"
	"github.com/jedarden/clasp/internal/provider"
	"github.com/jedarden/clasp/internal/secrets"
	"github.com/jedarden/clasp/internal/translator"
	"github.com/jedarden/clasp/pkg/models"
)

// Handler handles incoming Anthropic API requests.
type Handler struct {
	cfg              *config.Config
	provider         provider.Provider
	fallbackProvider provider.Provider
	client           *http.Client
	metrics          *Metrics
	rateLimiter      *RateLimiter
	cache            *RequestCache
	queue            *RequestQueue
	circuitBreaker   *CircuitBreaker
	costTracker      *CostTracker
	tierProviders    map[config.ModelTier]provider.Provider
	tierFallbacks    map[config.ModelTier]provider.Provider
	version          string
}

// Metrics tracks request statistics.
type Metrics struct {
	TotalRequests     int64
	SuccessRequests   int64
	ErrorRequests     int64
	StreamRequests    int64
	ToolCallRequests  int64
	TotalLatencyMs    int64
	FallbackAttempts  int64
	FallbackSuccesses int64
	StartTime         time.Time
}

// NewHandler creates a new request handler with optimized HTTP client.
func NewHandler(cfg *config.Config) (*Handler, error) {
	p, err := createProvider(cfg)
	if err != nil {
		return nil, err
	}

	// Create optimized HTTP transport with connection pooling
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
		DisableCompression:  false,
	}

	// Use configurable timeout (default 5 minutes for reasoning models)
	httpTimeout := time.Duration(cfg.HTTPClientTimeoutSec) * time.Second
	if httpTimeout == 0 {
		httpTimeout = 300 * time.Second // Fallback default
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   httpTimeout,
	}

	handler := &Handler{
		cfg:           cfg,
		provider:      p,
		client:        client,
		metrics:       &Metrics{StartTime: time.Now()},
		costTracker:   NewCostTracker(),
		tierProviders: make(map[config.ModelTier]provider.Provider),
		tierFallbacks: make(map[config.ModelTier]provider.Provider),
	}

	// Initialize global fallback provider if configured
	if cfg.HasGlobalFallback() {
		if fallbackCfg := cfg.GetGlobalFallbackConfig(); fallbackCfg != nil {
			if fallbackProvider, err := createTierProvider(fallbackCfg); err == nil {
				handler.fallbackProvider = fallbackProvider
				log.Printf("[CLASP] Global fallback: %s (%s)", cfg.FallbackProvider, cfg.FallbackModel)
			}
		}
	}

	// Initialize tier-specific providers if multi-provider routing is enabled
	if cfg.MultiProviderEnabled {
		handler.initializeTier(config.TierOpus, cfg.TierOpus)
		handler.initializeTier(config.TierSonnet, cfg.TierSonnet)
		handler.initializeTier(config.TierHaiku, cfg.TierHaiku)
	}

	return handler, nil
}

// initializeTier sets up a tier-specific provider and its fallback.
func (h *Handler) initializeTier(tier config.ModelTier, tierCfg *config.TierConfig) {
	if tierCfg == nil {
		return
	}

	// Initialize main tier provider
	if tierProvider, err := createTierProvider(tierCfg); err == nil {
		h.tierProviders[tier] = tierProvider
		log.Printf("[CLASP] Multi-provider: %s -> %s (%s)", tier, tierCfg.Provider, tierCfg.Model)
	}

	// Initialize tier-specific fallback
	if tierCfg.HasFallback() {
		if fb := tierCfg.GetFallbackConfig(); fb != nil {
			if fbProvider, err := createTierProvider(fb); err == nil {
				h.tierFallbacks[tier] = fbProvider
				log.Printf("[CLASP] Fallback: %s -> %s (%s)", tier, fb.Provider, fb.Model)
			}
		}
	}
}

// SetRateLimiter sets the rate limiter for metrics reporting.
func (h *Handler) SetRateLimiter(rl *RateLimiter) {
	h.rateLimiter = rl
}

// SetCache sets the request cache.
func (h *Handler) SetCache(cache *RequestCache) {
	h.cache = cache
}

// SetQueue sets the request queue.
func (h *Handler) SetQueue(queue *RequestQueue) {
	h.queue = queue
}

// SetCircuitBreaker sets the circuit breaker.
func (h *Handler) SetCircuitBreaker(cb *CircuitBreaker) {
	h.circuitBreaker = cb
}

// SetVersion sets the handler version (used for status endpoint).
func (h *Handler) SetVersion(version string) {
	h.version = version
}

// GetMetrics returns the current metrics.
func (h *Handler) GetMetrics() *Metrics {
	return h.metrics
}

// GetCostTracker returns the cost tracker.
func (h *Handler) GetCostTracker() *CostTracker {
	return h.costTracker
}

// createProvider creates the appropriate provider based on config.
func createProvider(cfg *config.Config) (provider.Provider, error) {
	switch cfg.Provider {
	case config.ProviderOpenAI:
		return provider.NewOpenAIProvider(cfg.OpenAIBaseURL), nil
	case config.ProviderOpenRouter:
		return provider.NewOpenRouterProvider(cfg.OpenRouterBaseURL), nil
	case config.ProviderAzure:
		return provider.NewAzureProvider(cfg.AzureEndpoint, cfg.AzureDeploymentName, cfg.AzureAPIVersion), nil
	case config.ProviderAnthropic:
		return provider.NewAnthropicProvider(""), nil
	case config.ProviderOllama:
		return provider.NewOllamaProvider(cfg.OllamaBaseURL), nil
	case config.ProviderGemini:
		return provider.NewGeminiProvider(cfg.GeminiAPIKey), nil
	case config.ProviderDeepSeek:
		return provider.NewDeepSeekProvider(cfg.DeepSeekAPIKey), nil
	case config.ProviderCustom:
		return provider.NewCustomProvider(cfg.CustomBaseURL), nil
	default:
		return nil, fmt.Errorf("unsupported provider: %s", cfg.Provider)
	}
}

// createTierProvider creates a provider from a tier configuration.
func createTierProvider(tierCfg *config.TierConfig) (provider.Provider, error) {
	baseURL := tierCfg.BaseURL
	switch tierCfg.Provider {
	case config.ProviderOpenAI:
		if baseURL == "" {
			baseURL = "https://api.openai.com/v1"
		}
		return provider.NewOpenAIProviderWithKey(baseURL, tierCfg.APIKey), nil
	case config.ProviderOpenRouter:
		if baseURL == "" {
			baseURL = "https://openrouter.ai/api/v1"
		}
		return provider.NewOpenRouterProviderWithKey(baseURL, tierCfg.APIKey), nil
	case config.ProviderAnthropic:
		if baseURL == "" {
			baseURL = "https://api.anthropic.com"
		}
		return provider.NewAnthropicProviderWithKey(baseURL, tierCfg.APIKey), nil
	case config.ProviderOllama:
		if baseURL == "" {
			baseURL = "http://localhost:11434"
		}
		return provider.NewOllamaProviderWithKey(baseURL, tierCfg.APIKey), nil
	case config.ProviderGemini:
		if baseURL == "" {
			baseURL = "https://generativelanguage.googleapis.com/v1beta"
		}
		return provider.NewGeminiProviderWithURL(baseURL, tierCfg.APIKey), nil
	case config.ProviderDeepSeek:
		if baseURL == "" {
			baseURL = "https://api.deepseek.com"
		}
		return provider.NewDeepSeekProviderWithURL(baseURL, tierCfg.APIKey), nil
	case config.ProviderCustom:
		return provider.NewCustomProviderWithKey(baseURL, tierCfg.APIKey), nil
	default:
		return nil, fmt.Errorf("unsupported tier provider: %s", tierCfg.Provider)
	}
}

// HandleMessages handles POST /v1/messages requests.
func (h *Handler) HandleMessages(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	atomic.AddInt64(&h.metrics.TotalRequests, 1)

	// Parse and validate request
	anthropicReq, reqErr := h.parseAndValidateRequest(r)
	if reqErr != nil {
		atomic.AddInt64(&h.metrics.ErrorRequests, 1)
		h.writeErrorResponse(w, reqErr.statusCode, reqErr.errType, reqErr.message)
		return
	}
	defer r.Body.Close()

	// Check cache for non-streaming requests
	cacheKey, cacheable := h.checkCache(w, anthropicReq)
	if cacheKey == "HIT" {
		return // Response already sent from cache
	}

	// Select provider and resolve target model
	selectedProvider, targetModel := h.selectProviderAndModel(anthropicReq)

	// Check circuit breaker
	if h.circuitBreaker != nil && !h.circuitBreaker.Allow() {
		atomic.AddInt64(&h.metrics.ErrorRequests, 1)
		log.Printf("[CLASP] Circuit breaker open - rejecting request")
		w.Header().Set("X-CLASP-Circuit-Breaker", "open")
		h.writeErrorResponse(w, http.StatusServiceUnavailable, "overloaded_error", "Service temporarily unavailable - circuit breaker open")
		return
	}

	// Check if this provider requires transformation (passthrough mode for Anthropic)
	if !selectedProvider.RequiresTransformation() {
		h.handlePassthroughRequest(w, r, anthropicReq, selectedProvider, start, cacheKey, cacheable)
		return
	}

	// Transform and execute request
	resp, targetModel, useResponsesAPI, usedFallback, execErr := h.transformAndExecute(r.Context(), anthropicReq, selectedProvider, targetModel)
	if execErr != nil {
		atomic.AddInt64(&h.metrics.ErrorRequests, 1)
		if h.circuitBreaker != nil {
			h.circuitBreaker.RecordFailure()
		}
		log.Printf("[CLASP] Error making upstream request: %v", execErr)
		h.writeErrorResponse(w, http.StatusBadGateway, "api_error", "Error connecting to upstream provider")
		return
	}
	defer resp.Body.Close()

	// Handle upstream errors
	if resp.StatusCode >= 400 {
		h.handleUpstreamError(w, resp)
		return
	}

	// Record success
	if h.circuitBreaker != nil {
		h.circuitBreaker.RecordSuccess()
	}
	atomic.AddInt64(&h.metrics.SuccessRequests, 1)
	atomic.AddInt64(&h.metrics.TotalLatencyMs, time.Since(start).Milliseconds())

	// Set response headers
	if usedFallback {
		w.Header().Set("X-CLASP-Fallback", "true")
	}
	if useResponsesAPI {
		w.Header().Set("X-CLASP-Responses-API", "true")
	}

	// Handle streaming vs non-streaming response
	h.handleResponse(w, resp, anthropicReq.Stream, useResponsesAPI, targetModel, cacheKey, cacheable)
}

// requestError represents a request validation error with HTTP status info.
type requestError struct {
	statusCode int
	errType    string
	message    string
}

// parseAndValidateRequest parses and validates an incoming Anthropic request.
func (h *Handler) parseAndValidateRequest(r *http.Request) (*models.AnthropicRequest, *requestError) {
	// Only accept POST
	if r.Method != http.MethodPost {
		return nil, &requestError{
			statusCode: http.StatusMethodNotAllowed,
			errType:    "invalid_request_error",
			message:    "Method not allowed",
		}
	}

	// Parse request body
	var anthropicReq models.AnthropicRequest
	if err := json.NewDecoder(r.Body).Decode(&anthropicReq); err != nil {
		log.Printf("[CLASP] Error parsing request: %v", err)
		return nil, &requestError{
			statusCode: http.StatusBadRequest,
			errType:    "invalid_request_error",
			message:    fmt.Sprintf("Invalid request body: %v. Expected Anthropic Messages API format with 'model', 'messages', and optionally 'stream', 'tools', etc.", err),
		}
	}

	// Validate required fields
	if err := h.validateRequest(&anthropicReq); err != nil {
		return nil, err
	}

	// Track request types
	if anthropicReq.Stream {
		atomic.AddInt64(&h.metrics.StreamRequests, 1)
	}
	if len(anthropicReq.Tools) > 0 {
		atomic.AddInt64(&h.metrics.ToolCallRequests, 1)
	}

	// Resolve model alias if configured
	originalModel := anthropicReq.Model
	anthropicReq.Model = h.cfg.ResolveAlias(anthropicReq.Model)
	if anthropicReq.Model != originalModel {
		log.Printf("[CLASP] Resolved model alias: %s -> %s", originalModel, anthropicReq.Model)
	}

	// Debug logging for incoming request (secrets are masked)
	if h.cfg.DebugRequests {
		debugJSON, _ := json.MarshalIndent(anthropicReq, "", "  ")
		maskedJSON := secrets.MaskJSONSecrets(debugJSON)
		log.Printf("[CLASP DEBUG] Incoming Anthropic request:\n%s", string(maskedJSON))
		logging.LogDebugRequestRaw("INCOMING", "/v1/messages", maskedJSON)
	}

	return &anthropicReq, nil
}

// validateRequest validates required fields in the Anthropic request.
func (h *Handler) validateRequest(req *models.AnthropicRequest) *requestError {
	if req.Model == "" {
		return &requestError{
			statusCode: http.StatusBadRequest,
			errType:    "invalid_request_error",
			message:    "Missing required field: 'model'",
		}
	}
	if len(req.Messages) == 0 {
		return &requestError{
			statusCode: http.StatusBadRequest,
			errType:    "invalid_request_error",
			message:    "Missing required field: 'messages'",
		}
	}
	return nil
}

// checkCache checks if the request is in cache and returns cache key/status.
// Returns "HIT" as cacheKey if response was served from cache.
func (h *Handler) checkCache(w http.ResponseWriter, req *models.AnthropicRequest) (string, bool) {
	if h.cache == nil || req.Stream {
		return "", false
	}

	cacheKey, cacheable := GenerateCacheKey(req)
	if !cacheable {
		return "", false
	}

	if cachedResp, found := h.cache.Get(cacheKey); found {
		log.Printf("[CLASP] Cache HIT for request")
		atomic.AddInt64(&h.metrics.SuccessRequests, 1)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-CLASP-Cache", "HIT")
		_ = json.NewEncoder(w).Encode(cachedResp)
		return "HIT", true
	}

	log.Printf("[CLASP] Cache MISS for request")
	return cacheKey, cacheable
}

// selectProviderAndModel selects the appropriate provider and target model.
func (h *Handler) selectProviderAndModel(req *models.AnthropicRequest) (provider.Provider, string) {
	selectedProvider := h.provider
	tierCfg := h.cfg.GetTierConfig(req.Model)
	var targetModel string

	if tierCfg != nil {
		tier := config.GetModelTier(req.Model)
		if tierProvider, ok := h.tierProviders[tier]; ok {
			selectedProvider = tierProvider
			targetModel = tierCfg.Model
			if targetModel == "" {
				targetModel = h.cfg.MapModel(req.Model)
			}
			log.Printf("[CLASP] Multi-provider routing: %s -> %s via %s", req.Model, targetModel, tierCfg.Provider)
		} else {
			targetModel = h.cfg.MapModel(req.Model)
			targetModel = selectedProvider.TransformModelID(targetModel)
		}
	} else {
		targetModel = h.cfg.MapModel(req.Model)
		targetModel = selectedProvider.TransformModelID(targetModel)
	}

	log.Printf("[CLASP] Request: %s -> %s (streaming: %v, provider: %s, passthrough: %v)",
		req.Model, targetModel, req.Stream, selectedProvider.Name(), !selectedProvider.RequiresTransformation())

	return selectedProvider, targetModel
}

// transformAndExecute transforms the request and executes it against the provider.
func (h *Handler) transformAndExecute(ctx interface{ Done() <-chan struct{} }, req *models.AnthropicRequest, selectedProvider provider.Provider, targetModel string) (*http.Response, string, bool, bool, error) {
	endpointType := translator.GetEndpointType(targetModel)
	useResponsesAPI := endpointType == translator.EndpointResponses

	// Set target model on provider for endpoint URL selection
	if openaiProvider, ok := selectedProvider.(*provider.OpenAIProvider); ok {
		openaiProvider.SetTargetModel(targetModel)
	}

	// Transform request
	reqBody, err := h.transformRequest(req, targetModel, useResponsesAPI)
	if err != nil {
		return nil, targetModel, useResponsesAPI, false, err
	}

	// Execute request
	resp, err := h.doRequestWithRetry(ctx, reqBody, selectedProvider)
	usedFallback := false

	// Check if we should try fallback
	if err != nil || (resp != nil && resp.StatusCode >= 500) {
		resp, targetModel, useResponsesAPI, usedFallback, err = h.tryFallback(ctx, req, resp, targetModel, err)
	}

	return resp, targetModel, useResponsesAPI, usedFallback, err
}

// transformRequest transforms an Anthropic request to the appropriate format.
func (h *Handler) transformRequest(req *models.AnthropicRequest, targetModel string, useResponsesAPI bool) ([]byte, error) {
	if useResponsesAPI {
		responsesReq, err := translator.TransformRequestToResponses(req, targetModel, "")
		if err != nil {
			log.Printf("[CLASP] Error transforming request to Responses API: %v", err)
			return nil, err
		}

		reqBody, err := json.Marshal(responsesReq)
		if err != nil {
			log.Printf("[CLASP] Error marshaling Responses request: %v", err)
			return nil, err
		}

		if h.cfg.DebugRequests {
			debugJSON, _ := json.MarshalIndent(responsesReq, "", "  ")
			maskedJSON := secrets.MaskJSONSecrets(debugJSON)
			log.Printf("[CLASP DEBUG] Outgoing OpenAI Responses API request:\n%s", string(maskedJSON))
			logging.LogDebugRequestRaw("OUTGOING", "/v1/responses", maskedJSON)
		}

		log.Printf("[CLASP] Using Responses API for model: %s", targetModel)
		return reqBody, nil
	}

	openAIReq, err := translator.TransformRequest(req, targetModel)
	if err != nil {
		log.Printf("[CLASP] Error transforming request: %v", err)
		return nil, err
	}

	reqBody, err := json.Marshal(openAIReq)
	if err != nil {
		log.Printf("[CLASP] Error marshaling request: %v", err)
		return nil, err
	}

	if h.cfg.DebugRequests {
		debugJSON, _ := json.MarshalIndent(openAIReq, "", "  ")
		maskedJSON := secrets.MaskJSONSecrets(debugJSON)
		log.Printf("[CLASP DEBUG] Outgoing OpenAI Chat Completions request:\n%s", string(maskedJSON))
		logging.LogDebugRequestRaw("OUTGOING", "/v1/chat/completions", maskedJSON)
	}

	return reqBody, nil
}

// tryFallback attempts to use a fallback provider if the primary fails.
func (h *Handler) tryFallback(ctx interface{ Done() <-chan struct{} }, req *models.AnthropicRequest, resp *http.Response, targetModel string, originalErr error) (*http.Response, string, bool, bool, error) {
	fallbackProvider, fallbackModel := h.getFallbackProvider(req.Model)
	if fallbackProvider == nil {
		return resp, targetModel, false, false, originalErr
	}

	// Close original response if it exists
	if resp != nil {
		resp.Body.Close()
	}

	atomic.AddInt64(&h.metrics.FallbackAttempts, 1)
	log.Printf("[CLASP] Primary provider failed, attempting fallback to %s", fallbackProvider.Name())

	// Re-transform request with fallback model if specified
	useResponsesAPI := false
	var reqBody []byte
	if fallbackModel != "" {
		targetModel = fallbackModel
		fallbackEndpointType := translator.GetEndpointType(fallbackModel)
		useResponsesAPI = fallbackEndpointType == translator.EndpointResponses

		if openaiProvider, ok := fallbackProvider.(*provider.OpenAIProvider); ok {
			openaiProvider.SetTargetModel(targetModel)
		}
	}

	var err error
	reqBody, err = h.transformRequest(req, targetModel, useResponsesAPI)
	if err != nil {
		return nil, targetModel, useResponsesAPI, false, err
	}

	// Try fallback provider
	resp, err = h.doRequestWithRetry(ctx, reqBody, fallbackProvider)
	if err == nil && resp.StatusCode < 500 {
		atomic.AddInt64(&h.metrics.FallbackSuccesses, 1)
		log.Printf("[CLASP] Fallback to %s succeeded", fallbackProvider.Name())
		return resp, targetModel, useResponsesAPI, true, nil
	}

	return resp, targetModel, useResponsesAPI, false, err
}

// handleUpstreamError handles error responses from the upstream provider.
func (h *Handler) handleUpstreamError(w http.ResponseWriter, resp *http.Response) {
	atomic.AddInt64(&h.metrics.ErrorRequests, 1)
	if h.circuitBreaker != nil && resp.StatusCode >= 500 {
		h.circuitBreaker.RecordFailure()
	}
	body, _ := io.ReadAll(resp.Body)
	maskedBody := secrets.MaskAllSecrets(string(body))
	log.Printf("[CLASP] Upstream error (%d): %s", resp.StatusCode, maskedBody)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(body)
}

// handleResponse routes the response to the appropriate handler.
func (h *Handler) handleResponse(w http.ResponseWriter, resp *http.Response, isStreaming, useResponsesAPI bool, targetModel, cacheKey string, cacheable bool) {
	if isStreaming {
		if useResponsesAPI {
			h.handleResponsesStreamingResponse(w, resp, targetModel)
		} else {
			h.handleStreamingResponse(w, resp, targetModel)
		}
	} else {
		if useResponsesAPI {
			h.handleResponsesNonStreamingResponse(w, resp, targetModel, cacheKey, cacheable)
		} else {
			h.handleNonStreamingResponse(w, resp, targetModel, cacheKey, cacheable)
		}
	}
}

// handlePassthroughRequest handles requests that don't require transformation.
// This is used for direct Anthropic API passthrough where the request is already
// in the correct format.
func (h *Handler) handlePassthroughRequest(w http.ResponseWriter, r *http.Request, anthropicReq *models.AnthropicRequest, p provider.Provider, start time.Time, cacheKey string, cacheable bool) {
	// Marshal the original Anthropic request
	reqBody, err := json.Marshal(anthropicReq)
	if err != nil {
		atomic.AddInt64(&h.metrics.ErrorRequests, 1)
		log.Printf("[CLASP] Error marshaling passthrough request: %v", err)
		h.writeErrorResponse(w, http.StatusInternalServerError, "api_error", "Error preparing request")
		return
	}

	// Debug logging for passthrough request (secrets are masked)
	if h.cfg.DebugRequests {
		maskedJSON := secrets.MaskJSONSecrets(reqBody)
		log.Printf("[CLASP DEBUG] Passthrough to Anthropic API:\n%s", string(maskedJSON))
		// Also log to dedicated debug file
		logging.LogDebugRequestRaw("PASSTHROUGH", "/v1/messages", maskedJSON)
	}

	// Execute request with retry logic
	resp, err := h.doRequestWithRetry(r.Context(), reqBody, p)
	if err != nil {
		atomic.AddInt64(&h.metrics.ErrorRequests, 1)
		if h.circuitBreaker != nil {
			h.circuitBreaker.RecordFailure()
		}
		log.Printf("[CLASP] Error in passthrough request: %v", err)
		h.writeErrorResponse(w, http.StatusBadGateway, "api_error", "Error connecting to Anthropic API")
		return
	}
	defer resp.Body.Close()

	// Check for upstream errors
	if resp.StatusCode >= 400 {
		atomic.AddInt64(&h.metrics.ErrorRequests, 1)
		if h.circuitBreaker != nil && resp.StatusCode >= 500 {
			h.circuitBreaker.RecordFailure()
		}
		body, _ := io.ReadAll(resp.Body)
		// Mask any secrets in error response before logging
		maskedBody := secrets.MaskAllSecrets(string(body))
		log.Printf("[CLASP] Anthropic API error (%d): %s", resp.StatusCode, maskedBody)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		_, _ = w.Write(body) // Send original response to client
		return
	}

	// Record success for circuit breaker
	if h.circuitBreaker != nil {
		h.circuitBreaker.RecordSuccess()
	}

	atomic.AddInt64(&h.metrics.SuccessRequests, 1)
	atomic.AddInt64(&h.metrics.TotalLatencyMs, time.Since(start).Milliseconds())

	// Add passthrough indicator header
	w.Header().Set("X-CLASP-Passthrough", "true")

	// Handle streaming vs non-streaming passthrough
	if anthropicReq.Stream {
		h.handlePassthroughStreaming(w, resp)
	} else {
		h.handlePassthroughNonStreaming(w, resp, cacheKey, cacheable)
	}
}

// handlePassthroughStreaming streams the Anthropic response directly.
func (h *Handler) handlePassthroughStreaming(w http.ResponseWriter, resp *http.Response) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	// Flush headers
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	// Stream response directly
	buf := make([]byte, 4096)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := w.Write(buf[:n]); writeErr != nil {
				log.Printf("[CLASP] Error writing passthrough stream: %v", writeErr)
				return
			}
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
		if err != nil {
			if err != io.EOF {
				log.Printf("[CLASP] Error reading passthrough stream: %v", err)
			}
			return
		}
	}
}

// handlePassthroughNonStreaming handles non-streaming passthrough responses.
func (h *Handler) handlePassthroughNonStreaming(w http.ResponseWriter, resp *http.Response, cacheKey string, cacheable bool) {
	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[CLASP] Error reading passthrough response: %v", err)
		h.writeErrorResponse(w, http.StatusBadGateway, "api_error", "Error reading upstream response")
		return
	}

	// Debug logging (secrets are masked)
	if h.cfg.DebugResponses {
		maskedBody := secrets.MaskJSONSecrets(body)
		log.Printf("[CLASP DEBUG] Passthrough response:\n%s", string(maskedBody))
		// Also log to dedicated debug file
		logging.LogDebugRequestRaw("RESPONSE", "/v1/messages (passthrough)", maskedBody)
	}

	// Parse response for caching and cost tracking
	var anthropicResp models.AnthropicResponse
	if err := json.Unmarshal(body, &anthropicResp); err == nil {
		// Track costs for passthrough
		if h.costTracker != nil && anthropicResp.Usage != nil {
			h.costTracker.RecordUsage(
				"anthropic",
				anthropicResp.Model,
				anthropicResp.Usage.InputTokens,
				anthropicResp.Usage.OutputTokens,
			)
		}

		// Cache if enabled
		if h.cache != nil && cacheable && cacheKey != "" {
			h.cache.Set(cacheKey, &anthropicResp)
			log.Printf("[CLASP] Passthrough response cached (key: %s...)", cacheKey[:16])
		}
	}

	// Write response
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-CLASP-Cache", "MISS")
	_, _ = w.Write(body)
}

// doRequestWithRetry executes the upstream request with exponential backoff retry.
func (h *Handler) doRequestWithRetry(ctx interface{ Done() <-chan struct{} }, reqBody []byte, p provider.Provider) (*http.Response, error) {
	maxRetries := 3
	baseDelay := 500 * time.Millisecond

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		// Create fresh request for each attempt with context
		upstreamReq, err := http.NewRequestWithContext(context.Background(), http.MethodPost, p.GetEndpointURL(), bytes.NewReader(reqBody))
		if err != nil {
			return nil, fmt.Errorf("creating request: %w", err)
		}

		// Set headers (API key may be embedded in provider for tier routing)
		for key, values := range p.GetHeaders(h.cfg.GetAPIKey()) {
			for _, v := range values {
				upstreamReq.Header.Add(key, v)
			}
		}

		resp, err := h.client.Do(upstreamReq)
		if err == nil {
			// Check if we should retry based on status code
			if resp.StatusCode < 500 || resp.StatusCode == 529 { // Don't retry 5xx except overload
				return resp, nil
			}
			// Close response for retry
			resp.Body.Close()
			lastErr = fmt.Errorf("upstream returned %d", resp.StatusCode)
		} else {
			lastErr = err
		}

		// Don't retry on last attempt
		if attempt < maxRetries-1 {
			delay := baseDelay * time.Duration(1<<attempt) // Exponential backoff
			log.Printf("[CLASP] Retry %d/%d after %v: %v", attempt+1, maxRetries, delay, lastErr)

			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("context cancelled")
			case <-time.After(delay):
			}
		}
	}

	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}

// getFallbackProvider returns the appropriate fallback provider and model for the given request model.
// It checks tier-specific fallbacks first, then global fallback.
func (h *Handler) getFallbackProvider(requestModel string) (provider.Provider, string) {
	// First check for tier-specific fallback
	tier := config.GetModelTier(requestModel)
	if fbProvider, ok := h.tierFallbacks[tier]; ok {
		// Get fallback model from tier config
		tierCfg := h.cfg.GetTierConfig(requestModel)
		if tierCfg != nil && tierCfg.FallbackModel != "" {
			return fbProvider, tierCfg.FallbackModel
		}
		return fbProvider, ""
	}

	// Fall back to global fallback provider
	if h.fallbackProvider != nil {
		return h.fallbackProvider, h.cfg.FallbackModel
	}

	return nil, ""
}

// writeErrorResponse writes an Anthropic-formatted error response.
func (h *Handler) writeErrorResponse(w http.ResponseWriter, status int, errType, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"type": "error",
		"error": map[string]string{
			"type":    errType,
			"message": message,
		},
	})
}

// handleStreamingResponse handles SSE streaming responses.
func (h *Handler) handleStreamingResponse(w http.ResponseWriter, resp *http.Response, targetModel string) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	// Flush headers
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	// Create flush writer
	fw := &flushWriter{w: w}
	if f, ok := w.(http.Flusher); ok {
		fw.flusher = f
	}

	// Generate message ID
	messageID := generateMessageID()

	// Process stream
	processor := translator.NewStreamProcessor(fw, messageID, targetModel)

	// Set up cost tracking callback if cost tracker is available
	if h.costTracker != nil {
		processor.SetUsageCallback(func(inputTokens, outputTokens int) {
			h.costTracker.RecordUsage(
				h.provider.Name(),
				targetModel,
				inputTokens,
				outputTokens,
			)
			log.Printf("[CLASP] Streaming cost tracked: %d input tokens, %d output tokens", inputTokens, outputTokens)
		})
	}

	if err := processor.ProcessStream(resp.Body); err != nil {
		log.Printf("[CLASP] Error processing stream: %v", err)
	}
}

// handleNonStreamingResponse handles non-streaming responses.
func (h *Handler) handleNonStreamingResponse(w http.ResponseWriter, resp *http.Response, targetModel, cacheKey string, cacheable bool) {
	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[CLASP] Error reading response: %v", err)
		http.Error(w, "Error reading upstream response", http.StatusBadGateway)
		return
	}

	// Debug logging for raw response (secrets are masked)
	if h.cfg.DebugResponses {
		maskedBody := secrets.MaskJSONSecrets(body)
		log.Printf("[CLASP DEBUG] Raw OpenAI response:\n%s", string(maskedBody))
		// Also log to dedicated debug file
		logging.LogDebugRequestRaw("RESPONSE", "/v1/chat/completions (raw)", maskedBody)
	}

	// Parse OpenAI response
	var openAIResp struct {
		ID      string `json:"id"`
		Choices []struct {
			Message struct {
				Role      string `json:"role"`
				Content   string `json:"content"`
				ToolCalls []struct {
					ID       string `json:"id"`
					Type     string `json:"type"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(body, &openAIResp); err != nil {
		log.Printf("[CLASP] Error parsing response: %v", err)
		http.Error(w, "Error parsing upstream response", http.StatusBadGateway)
		return
	}

	// Build Anthropic response
	anthropicResp := models.AnthropicResponse{
		ID:    openAIResp.ID,
		Type:  "message",
		Role:  "assistant",
		Model: targetModel,
		Usage: &models.AnthropicUsage{
			InputTokens:  openAIResp.Usage.PromptTokens,
			OutputTokens: openAIResp.Usage.CompletionTokens,
		},
	}

	if len(openAIResp.Choices) > 0 {
		choice := openAIResp.Choices[0]
		anthropicResp.StopReason = mapFinishReason(choice.FinishReason)

		// Add text content
		if choice.Message.Content != "" {
			anthropicResp.Content = append(anthropicResp.Content, models.AnthropicContentBlock{
				Type: "text",
				Text: choice.Message.Content,
			})
		}

		// Add tool calls
		for _, tc := range choice.Message.ToolCalls {
			var input interface{}
			_ = json.Unmarshal([]byte(tc.Function.Arguments), &input)

			anthropicResp.Content = append(anthropicResp.Content, models.AnthropicContentBlock{
				Type:  "tool_use",
				ID:    tc.ID,
				Name:  tc.Function.Name,
				Input: input,
			})
		}
	}

	// Debug logging for Anthropic response (secrets are masked)
	if h.cfg.DebugResponses {
		debugJSON, _ := json.MarshalIndent(anthropicResp, "", "  ")
		maskedJSON := secrets.MaskJSONSecrets(debugJSON)
		log.Printf("[CLASP DEBUG] Transformed Anthropic response:\n%s", string(maskedJSON))
		// Also log to dedicated debug file
		logging.LogDebugRequestRaw("RESPONSE", "/v1/messages (transformed)", maskedJSON)
	}

	// Track costs
	if h.costTracker != nil && anthropicResp.Usage != nil {
		h.costTracker.RecordUsage(
			h.provider.Name(),
			targetModel,
			anthropicResp.Usage.InputTokens,
			anthropicResp.Usage.OutputTokens,
		)
	}

	// Store in cache if cacheable
	if h.cache != nil && cacheable && cacheKey != "" {
		h.cache.Set(cacheKey, &anthropicResp)
		log.Printf("[CLASP] Response cached (key: %s...)", cacheKey[:16])
	}

	// Write response
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-CLASP-Cache", "MISS")
	_ = json.NewEncoder(w).Encode(anthropicResp)
}

// handleResponsesStreamingResponse handles SSE streaming responses from Responses API.
func (h *Handler) handleResponsesStreamingResponse(w http.ResponseWriter, resp *http.Response, targetModel string) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	// Flush headers
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	// Create flush writer
	fw := &flushWriter{w: w}
	if f, ok := w.(http.Flusher); ok {
		fw.flusher = f
	}

	// Generate message ID
	messageID := generateMessageID()

	// Process stream using Responses API processor
	processor := translator.NewResponsesStreamProcessor(fw, messageID, targetModel)

	// Set up cost tracking callback if cost tracker is available
	if h.costTracker != nil {
		processor.SetUsageCallback(func(inputTokens, outputTokens int) {
			h.costTracker.RecordUsage(
				h.provider.Name(),
				targetModel,
				inputTokens,
				outputTokens,
			)
			log.Printf("[CLASP] Responses API streaming cost tracked: %d input tokens, %d output tokens", inputTokens, outputTokens)
		})
	}

	if err := processor.ProcessStream(resp.Body); err != nil {
		log.Printf("[CLASP] Error processing Responses API stream: %v", err)
	}

	// Log response ID for conversation continuation
	if responseID := processor.GetResponseID(); responseID != "" {
		log.Printf("[CLASP] Responses API response ID: %s", responseID)
	}
}

// handleResponsesNonStreamingResponse handles non-streaming responses from Responses API.
func (h *Handler) handleResponsesNonStreamingResponse(w http.ResponseWriter, resp *http.Response, targetModel, cacheKey string, cacheable bool) {
	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[CLASP] Error reading Responses API response: %v", err)
		http.Error(w, "Error reading upstream response", http.StatusBadGateway)
		return
	}

	// Debug logging for raw response (secrets are masked)
	if h.cfg.DebugResponses {
		maskedBody := secrets.MaskJSONSecrets(body)
		log.Printf("[CLASP DEBUG] Raw OpenAI Responses API response:\n%s", string(maskedBody))
		// Also log to dedicated debug file
		logging.LogDebugRequestRaw("RESPONSE", "/v1/responses (raw)", maskedBody)
	}

	// Parse Responses API response
	var responsesResp models.ResponsesResponse
	if err := json.Unmarshal(body, &responsesResp); err != nil {
		log.Printf("[CLASP] Error parsing Responses API response: %v", err)
		http.Error(w, "Error parsing upstream response", http.StatusBadGateway)
		return
	}

	// Build Anthropic response from Responses API format
	anthropicResp := models.AnthropicResponse{
		ID:    responsesResp.ID,
		Type:  "message",
		Role:  "assistant",
		Model: targetModel,
	}

	// Map usage
	if responsesResp.Usage != nil {
		anthropicResp.Usage = &models.AnthropicUsage{
			InputTokens:  responsesResp.Usage.InputTokens,
			OutputTokens: responsesResp.Usage.OutputTokens,
		}
	}

	// Process output items
	hasToolCalls := false
	for _, item := range responsesResp.Output {
		switch item.Type {
		case "message":
			// Extract text content from message
			if content, ok := item.Content.(string); ok && content != "" {
				anthropicResp.Content = append(anthropicResp.Content, models.AnthropicContentBlock{
					Type: "text",
					Text: content,
				})
			} else if contentParts, ok := item.Content.([]interface{}); ok {
				// Handle array format content
				for _, part := range contentParts {
					if partMap, ok := part.(map[string]interface{}); ok {
						if partType, ok := partMap["type"].(string); ok {
							switch partType {
							case "text":
								if text, ok := partMap["text"].(string); ok && text != "" {
									anthropicResp.Content = append(anthropicResp.Content, models.AnthropicContentBlock{
										Type: "text",
										Text: text,
									})
								}
							case "refusal":
								if refusal, ok := partMap["refusal"].(string); ok && refusal != "" {
									anthropicResp.Content = append(anthropicResp.Content, models.AnthropicContentBlock{
										Type: "text",
										Text: "[Refused] " + refusal,
									})
								}
							}
						}
					}
				}
			}
		case "function_call":
			hasToolCalls = true
			var input interface{}
			if err := json.Unmarshal([]byte(item.Arguments), &input); err != nil {
				input = map[string]interface{}{}
			}
			// Convert Responses API "fc_xxx" ID back to Anthropic "call_xxx" format
			anthropicID := translator.TranslateResponsesIDToAnthropic(item.CallID)
			anthropicResp.Content = append(anthropicResp.Content, models.AnthropicContentBlock{
				Type:  "tool_use",
				ID:    anthropicID,
				Name:  item.Name,
				Input: input,
			})
		case "reasoning":
			// Include reasoning summary if available (thinking block)
			// Summary is an array of items with .Text fields
			if len(item.Summary) > 0 {
				// Concatenate all summary texts
				var summaryText string
				for _, summaryItem := range item.Summary {
					if summaryItem.Text != "" {
						if summaryText != "" {
							summaryText += "\n"
						}
						summaryText += summaryItem.Text
					}
				}
				if summaryText != "" {
					anthropicResp.Content = append(anthropicResp.Content, models.AnthropicContentBlock{
						Type: "thinking",
						Text: summaryText,
					})
				}
			}
		}
	}

	// Determine stop reason
	if responsesResp.Status == "completed" {
		if hasToolCalls {
			anthropicResp.StopReason = "tool_use"
		} else {
			anthropicResp.StopReason = "end_turn"
		}
	} else if responsesResp.Status == "failed" {
		anthropicResp.StopReason = "end_turn"
	}

	// Debug logging for Anthropic response (secrets are masked)
	if h.cfg.DebugResponses {
		debugJSON, _ := json.MarshalIndent(anthropicResp, "", "  ")
		maskedJSON := secrets.MaskJSONSecrets(debugJSON)
		log.Printf("[CLASP DEBUG] Transformed Anthropic response from Responses API:\n%s", string(maskedJSON))
		// Also log to dedicated debug file
		logging.LogDebugRequestRaw("RESPONSE", "/v1/messages (from responses)", maskedJSON)
	}

	// Track costs
	if h.costTracker != nil && anthropicResp.Usage != nil {
		h.costTracker.RecordUsage(
			h.provider.Name(),
			targetModel,
			anthropicResp.Usage.InputTokens,
			anthropicResp.Usage.OutputTokens,
		)
	}

	// Store in cache if cacheable
	if h.cache != nil && cacheable && cacheKey != "" {
		h.cache.Set(cacheKey, &anthropicResp)
		log.Printf("[CLASP] Responses API response cached (key: %s...)", cacheKey[:16])
	}

	// Write response
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-CLASP-Cache", "MISS")
	_ = json.NewEncoder(w).Encode(anthropicResp)
}

// HandleHealth handles health check requests.
func (h *Handler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "healthy",
		"provider": h.provider.Name(),
		"uptime":   time.Since(h.metrics.StartTime).String(),
	})
}

// HandleMetrics handles metrics endpoint requests.
func (h *Handler) HandleMetrics(w http.ResponseWriter, r *http.Request) {
	total := atomic.LoadInt64(&h.metrics.TotalRequests)
	success := atomic.LoadInt64(&h.metrics.SuccessRequests)
	errors := atomic.LoadInt64(&h.metrics.ErrorRequests)
	streams := atomic.LoadInt64(&h.metrics.StreamRequests)
	toolCalls := atomic.LoadInt64(&h.metrics.ToolCallRequests)
	totalLatency := atomic.LoadInt64(&h.metrics.TotalLatencyMs)

	var avgLatency float64
	if success > 0 {
		avgLatency = float64(totalLatency) / float64(success)
	}

	uptime := time.Since(h.metrics.StartTime)
	var requestsPerSec float64
	if uptime.Seconds() > 0 {
		requestsPerSec = float64(total) / uptime.Seconds()
	}

	var successRate float64
	if total > 0 {
		successRate = float64(success) / float64(total) * 100
	}

	response := map[string]interface{}{
		"requests": map[string]interface{}{
			"total":        total,
			"successful":   success,
			"errors":       errors,
			"streaming":    streams,
			"tool_calls":   toolCalls,
			"success_rate": fmt.Sprintf("%.2f%%", successRate),
		},
		"performance": map[string]interface{}{
			"avg_latency_ms":   fmt.Sprintf("%.2f", avgLatency),
			"requests_per_sec": fmt.Sprintf("%.2f", requestsPerSec),
		},
		"uptime":   uptime.String(),
		"provider": h.provider.Name(),
	}

	// Add rate limit stats if enabled
	if h.rateLimiter != nil {
		allowed, denied := h.rateLimiter.Stats()
		response["rate_limit"] = map[string]interface{}{
			"enabled":  true,
			"allowed":  allowed,
			"denied":   denied,
			"requests": h.cfg.RateLimitRequests,
			"window":   h.cfg.RateLimitWindow,
			"burst":    h.cfg.RateLimitBurst,
		}
	}

	// Add cache stats if enabled
	if h.cache != nil {
		size, maxSize, hits, misses, hitRate := h.cache.Stats()
		response["cache"] = map[string]interface{}{
			"enabled":  true,
			"size":     size,
			"max_size": maxSize,
			"hits":     hits,
			"misses":   misses,
			"hit_rate": fmt.Sprintf("%.2f%%", hitRate),
		}
	}

	// Add fallback stats if fallback is configured
	if h.fallbackProvider != nil || len(h.tierFallbacks) > 0 {
		fbAttempts := atomic.LoadInt64(&h.metrics.FallbackAttempts)
		fbSuccesses := atomic.LoadInt64(&h.metrics.FallbackSuccesses)
		var fbSuccessRate float64
		if fbAttempts > 0 {
			fbSuccessRate = float64(fbSuccesses) / float64(fbAttempts) * 100
		}
		response["fallback"] = map[string]interface{}{
			"enabled":      true,
			"attempts":     fbAttempts,
			"successes":    fbSuccesses,
			"success_rate": fmt.Sprintf("%.2f%%", fbSuccessRate),
		}
	}

	// Add queue stats if enabled
	if h.queue != nil {
		queued, dequeued, dropped, retried, expired, length, paused := h.queue.Stats()
		response["queue"] = map[string]interface{}{
			"enabled":  true,
			"queued":   queued,
			"dequeued": dequeued,
			"dropped":  dropped,
			"retried":  retried,
			"expired":  expired,
			"length":   length,
			"paused":   paused,
		}
	}

	// Add circuit breaker stats if enabled
	if h.circuitBreaker != nil {
		response["circuit_breaker"] = map[string]interface{}{
			"enabled": true,
			"state":   h.circuitBreaker.State(),
		}
	}

	// Add cost tracking stats
	if h.costTracker != nil {
		summary := h.costTracker.GetSummary()
		response["costs"] = map[string]interface{}{
			"enabled":             true,
			"total_cost_usd":      fmt.Sprintf("%.6f", summary.TotalCostUSD),
			"input_cost_usd":      fmt.Sprintf("%.6f", summary.InputCostUSD),
			"output_cost_usd":     fmt.Sprintf("%.6f", summary.OutputCostUSD),
			"total_input_tokens":  summary.TotalInputTokens,
			"total_output_tokens": summary.TotalOutputTokens,
			"cost_per_request":    fmt.Sprintf("%.6f", summary.CostPerRequest),
			"cost_per_hour":       fmt.Sprintf("%.6f", summary.CostPerHour),
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

// HandleMetricsPrometheus handles Prometheus metrics endpoint requests.
func (h *Handler) HandleMetricsPrometheus(w http.ResponseWriter, r *http.Request) {
	total := atomic.LoadInt64(&h.metrics.TotalRequests)
	success := atomic.LoadInt64(&h.metrics.SuccessRequests)
	errors := atomic.LoadInt64(&h.metrics.ErrorRequests)
	streams := atomic.LoadInt64(&h.metrics.StreamRequests)
	toolCalls := atomic.LoadInt64(&h.metrics.ToolCallRequests)
	totalLatency := atomic.LoadInt64(&h.metrics.TotalLatencyMs)

	uptime := time.Since(h.metrics.StartTime)
	providerName := h.provider.Name()

	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

	// Write Prometheus format metrics
	fmt.Fprintf(w, "# HELP clasp_requests_total Total number of requests handled by CLASP\n")
	fmt.Fprintf(w, "# TYPE clasp_requests_total counter\n")
	fmt.Fprintf(w, "clasp_requests_total{provider=\"%s\"} %d\n", providerName, total)

	fmt.Fprintf(w, "# HELP clasp_requests_successful Total number of successful requests\n")
	fmt.Fprintf(w, "# TYPE clasp_requests_successful counter\n")
	fmt.Fprintf(w, "clasp_requests_successful{provider=\"%s\"} %d\n", providerName, success)

	fmt.Fprintf(w, "# HELP clasp_requests_errors Total number of failed requests\n")
	fmt.Fprintf(w, "# TYPE clasp_requests_errors counter\n")
	fmt.Fprintf(w, "clasp_requests_errors{provider=\"%s\"} %d\n", providerName, errors)

	fmt.Fprintf(w, "# HELP clasp_requests_streaming Total number of streaming requests\n")
	fmt.Fprintf(w, "# TYPE clasp_requests_streaming counter\n")
	fmt.Fprintf(w, "clasp_requests_streaming{provider=\"%s\"} %d\n", providerName, streams)

	fmt.Fprintf(w, "# HELP clasp_requests_tool_calls Total number of requests with tool calls\n")
	fmt.Fprintf(w, "# TYPE clasp_requests_tool_calls counter\n")
	fmt.Fprintf(w, "clasp_requests_tool_calls{provider=\"%s\"} %d\n", providerName, toolCalls)

	fmt.Fprintf(w, "# HELP clasp_latency_total_ms Total latency of all successful requests in milliseconds\n")
	fmt.Fprintf(w, "# TYPE clasp_latency_total_ms counter\n")
	fmt.Fprintf(w, "clasp_latency_total_ms{provider=\"%s\"} %d\n", providerName, totalLatency)

	fmt.Fprintf(w, "# HELP clasp_uptime_seconds Time since CLASP started in seconds\n")
	fmt.Fprintf(w, "# TYPE clasp_uptime_seconds gauge\n")
	fmt.Fprintf(w, "clasp_uptime_seconds{provider=\"%s\"} %.2f\n", providerName, uptime.Seconds())

	// Derived metrics
	var avgLatency float64
	if success > 0 {
		avgLatency = float64(totalLatency) / float64(success)
	}
	fmt.Fprintf(w, "# HELP clasp_latency_avg_ms Average latency per successful request in milliseconds\n")
	fmt.Fprintf(w, "# TYPE clasp_latency_avg_ms gauge\n")
	fmt.Fprintf(w, "clasp_latency_avg_ms{provider=\"%s\"} %.2f\n", providerName, avgLatency)

	var requestsPerSec float64
	if uptime.Seconds() > 0 {
		requestsPerSec = float64(total) / uptime.Seconds()
	}
	fmt.Fprintf(w, "# HELP clasp_requests_per_second Current request rate per second\n")
	fmt.Fprintf(w, "# TYPE clasp_requests_per_second gauge\n")
	fmt.Fprintf(w, "clasp_requests_per_second{provider=\"%s\"} %.4f\n", providerName, requestsPerSec)

	// Rate limit metrics
	if h.rateLimiter != nil {
		allowed, denied := h.rateLimiter.Stats()
		fmt.Fprintf(w, "# HELP clasp_rate_limit_allowed Total requests allowed by rate limiter\n")
		fmt.Fprintf(w, "# TYPE clasp_rate_limit_allowed counter\n")
		fmt.Fprintf(w, "clasp_rate_limit_allowed{provider=\"%s\"} %d\n", providerName, allowed)

		fmt.Fprintf(w, "# HELP clasp_rate_limit_denied Total requests denied by rate limiter\n")
		fmt.Fprintf(w, "# TYPE clasp_rate_limit_denied counter\n")
		fmt.Fprintf(w, "clasp_rate_limit_denied{provider=\"%s\"} %d\n", providerName, denied)
	}

	// Cache metrics
	if h.cache != nil {
		size, maxSize, hits, misses, _ := h.cache.Stats()
		fmt.Fprintf(w, "# HELP clasp_cache_size Current number of entries in cache\n")
		fmt.Fprintf(w, "# TYPE clasp_cache_size gauge\n")
		fmt.Fprintf(w, "clasp_cache_size{provider=\"%s\"} %d\n", providerName, size)

		fmt.Fprintf(w, "# HELP clasp_cache_max_size Maximum cache size\n")
		fmt.Fprintf(w, "# TYPE clasp_cache_max_size gauge\n")
		fmt.Fprintf(w, "clasp_cache_max_size{provider=\"%s\"} %d\n", providerName, maxSize)

		fmt.Fprintf(w, "# HELP clasp_cache_hits Total cache hits\n")
		fmt.Fprintf(w, "# TYPE clasp_cache_hits counter\n")
		fmt.Fprintf(w, "clasp_cache_hits{provider=\"%s\"} %d\n", providerName, hits)

		fmt.Fprintf(w, "# HELP clasp_cache_misses Total cache misses\n")
		fmt.Fprintf(w, "# TYPE clasp_cache_misses counter\n")
		fmt.Fprintf(w, "clasp_cache_misses{provider=\"%s\"} %d\n", providerName, misses)
	}

	// Fallback metrics
	if h.fallbackProvider != nil || len(h.tierFallbacks) > 0 {
		fbAttempts := atomic.LoadInt64(&h.metrics.FallbackAttempts)
		fbSuccesses := atomic.LoadInt64(&h.metrics.FallbackSuccesses)

		fmt.Fprintf(w, "# HELP clasp_fallback_attempts Total fallback attempts\n")
		fmt.Fprintf(w, "# TYPE clasp_fallback_attempts counter\n")
		fmt.Fprintf(w, "clasp_fallback_attempts{provider=\"%s\"} %d\n", providerName, fbAttempts)

		fmt.Fprintf(w, "# HELP clasp_fallback_successes Total successful fallback attempts\n")
		fmt.Fprintf(w, "# TYPE clasp_fallback_successes counter\n")
		fmt.Fprintf(w, "clasp_fallback_successes{provider=\"%s\"} %d\n", providerName, fbSuccesses)
	}

	// Queue metrics
	if h.queue != nil {
		queued, dequeued, dropped, retried, expired, length, _ := h.queue.Stats()

		fmt.Fprintf(w, "# HELP clasp_queue_total Total requests queued\n")
		fmt.Fprintf(w, "# TYPE clasp_queue_total counter\n")
		fmt.Fprintf(w, "clasp_queue_total{provider=\"%s\"} %d\n", providerName, queued)

		fmt.Fprintf(w, "# HELP clasp_queue_dequeued Total requests dequeued\n")
		fmt.Fprintf(w, "# TYPE clasp_queue_dequeued counter\n")
		fmt.Fprintf(w, "clasp_queue_dequeued{provider=\"%s\"} %d\n", providerName, dequeued)

		fmt.Fprintf(w, "# HELP clasp_queue_dropped Total requests dropped (queue full)\n")
		fmt.Fprintf(w, "# TYPE clasp_queue_dropped counter\n")
		fmt.Fprintf(w, "clasp_queue_dropped{provider=\"%s\"} %d\n", providerName, dropped)

		fmt.Fprintf(w, "# HELP clasp_queue_retried Total requests retried\n")
		fmt.Fprintf(w, "# TYPE clasp_queue_retried counter\n")
		fmt.Fprintf(w, "clasp_queue_retried{provider=\"%s\"} %d\n", providerName, retried)

		fmt.Fprintf(w, "# HELP clasp_queue_expired Total requests expired in queue\n")
		fmt.Fprintf(w, "# TYPE clasp_queue_expired counter\n")
		fmt.Fprintf(w, "clasp_queue_expired{provider=\"%s\"} %d\n", providerName, expired)

		fmt.Fprintf(w, "# HELP clasp_queue_length Current queue length\n")
		fmt.Fprintf(w, "# TYPE clasp_queue_length gauge\n")
		fmt.Fprintf(w, "clasp_queue_length{provider=\"%s\"} %d\n", providerName, length)
	}

	// Circuit breaker metrics
	if h.circuitBreaker != nil {
		state := h.circuitBreaker.State()
		var stateValue int
		switch state {
		case "closed":
			stateValue = 0
		case "half-open":
			stateValue = 1
		case "open":
			stateValue = 2
		}

		fmt.Fprintf(w, "# HELP clasp_circuit_breaker_state Circuit breaker state (0=closed, 1=half-open, 2=open)\n")
		fmt.Fprintf(w, "# TYPE clasp_circuit_breaker_state gauge\n")
		fmt.Fprintf(w, "clasp_circuit_breaker_state{provider=\"%s\"} %d\n", providerName, stateValue)

		fmt.Fprintf(w, "# HELP clasp_circuit_breaker_open Whether circuit breaker is open (1) or not (0)\n")
		fmt.Fprintf(w, "# TYPE clasp_circuit_breaker_open gauge\n")
		isOpen := 0
		if h.circuitBreaker.IsOpen() {
			isOpen = 1
		}
		fmt.Fprintf(w, "clasp_circuit_breaker_open{provider=\"%s\"} %d\n", providerName, isOpen)
	}

	// Cost tracking metrics
	if h.costTracker != nil {
		summary := h.costTracker.GetSummary()

		fmt.Fprintf(w, "# HELP clasp_cost_total_usd Total cost in USD\n")
		fmt.Fprintf(w, "# TYPE clasp_cost_total_usd counter\n")
		fmt.Fprintf(w, "clasp_cost_total_usd{provider=\"%s\"} %.8f\n", providerName, summary.TotalCostUSD)

		fmt.Fprintf(w, "# HELP clasp_cost_input_usd Total input token cost in USD\n")
		fmt.Fprintf(w, "# TYPE clasp_cost_input_usd counter\n")
		fmt.Fprintf(w, "clasp_cost_input_usd{provider=\"%s\"} %.8f\n", providerName, summary.InputCostUSD)

		fmt.Fprintf(w, "# HELP clasp_cost_output_usd Total output token cost in USD\n")
		fmt.Fprintf(w, "# TYPE clasp_cost_output_usd counter\n")
		fmt.Fprintf(w, "clasp_cost_output_usd{provider=\"%s\"} %.8f\n", providerName, summary.OutputCostUSD)

		fmt.Fprintf(w, "# HELP clasp_tokens_input_total Total input tokens processed\n")
		fmt.Fprintf(w, "# TYPE clasp_tokens_input_total counter\n")
		fmt.Fprintf(w, "clasp_tokens_input_total{provider=\"%s\"} %d\n", providerName, summary.TotalInputTokens)

		fmt.Fprintf(w, "# HELP clasp_tokens_output_total Total output tokens generated\n")
		fmt.Fprintf(w, "# TYPE clasp_tokens_output_total counter\n")
		fmt.Fprintf(w, "clasp_tokens_output_total{provider=\"%s\"} %d\n", providerName, summary.TotalOutputTokens)

		fmt.Fprintf(w, "# HELP clasp_cost_per_request_usd Average cost per request in USD\n")
		fmt.Fprintf(w, "# TYPE clasp_cost_per_request_usd gauge\n")
		fmt.Fprintf(w, "clasp_cost_per_request_usd{provider=\"%s\"} %.8f\n", providerName, summary.CostPerRequest)

		fmt.Fprintf(w, "# HELP clasp_cost_per_hour_usd Cost rate per hour in USD\n")
		fmt.Fprintf(w, "# TYPE clasp_cost_per_hour_usd gauge\n")
		fmt.Fprintf(w, "clasp_cost_per_hour_usd{provider=\"%s\"} %.8f\n", providerName, summary.CostPerHour)

		// Per-model costs
		for model, mc := range summary.ByModel {
			fmt.Fprintf(w, "clasp_cost_by_model_usd{provider=\"%s\",model=\"%s\"} %.8f\n", providerName, model, mc.TotalCostUSD)
			fmt.Fprintf(w, "clasp_tokens_by_model{provider=\"%s\",model=\"%s\",type=\"input\"} %d\n", providerName, model, mc.InputTokens)
			fmt.Fprintf(w, "clasp_tokens_by_model{provider=\"%s\",model=\"%s\",type=\"output\"} %d\n", providerName, model, mc.OutputTokens)
		}
	}
}

// HandleCosts handles cost tracking endpoint requests.
func (h *Handler) HandleCosts(w http.ResponseWriter, r *http.Request) {
	if h.costTracker == nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"enabled": false,
			"message": "Cost tracking is not enabled",
		})
		return
	}

	// Handle POST to reset costs
	if r.Method == http.MethodPost {
		action := r.URL.Query().Get("action")
		if action == "reset" {
			h.costTracker.Reset()
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "ok",
				"message": "Cost tracking data has been reset",
			})
			return
		}
	}

	summary := h.costTracker.GetSummary()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(summary)
}

// HandleRoot handles root path requests.
func (h *Handler) HandleRoot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	version := h.version
	if version == "" {
		version = "unknown"
	}
	response := map[string]interface{}{
		"name":     "CLASP",
		"version":  version,
		"provider": h.provider.Name(),
		"status":   "running",
		"endpoints": map[string]string{
			"messages":   "/v1/messages",
			"health":     "/health",
			"metrics":    "/metrics",
			"prometheus": "/metrics/prometheus",
			"costs":      "/costs",
		},
	}

	// Add model aliases if configured
	if aliases := h.cfg.GetAliases(); len(aliases) > 0 {
		response["model_aliases"] = aliases
	}

	// Add multi-provider routing info if enabled
	if h.cfg.MultiProviderEnabled && len(h.tierProviders) > 0 {
		routing := make(map[string]string)
		for tier, p := range h.tierProviders {
			routing[string(tier)] = p.Name()
		}
		response["multi_provider_routing"] = routing
	}

	// Add fallback info if configured
	if h.fallbackProvider != nil {
		response["fallback_provider"] = h.fallbackProvider.Name()
	}
	if len(h.tierFallbacks) > 0 {
		fallbacks := make(map[string]string)
		for tier, p := range h.tierFallbacks {
			fallbacks[string(tier)] = p.Name()
		}
		response["tier_fallbacks"] = fallbacks
	}

	json.NewEncoder(w).Encode(response)
}

// flushWriter wraps http.ResponseWriter to auto-flush after each write.
type flushWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
}

func (fw *flushWriter) Write(p []byte) (int, error) {
	n, err := fw.w.Write(p)
	if fw.flusher != nil {
		fw.flusher.Flush()
	}
	return n, err
}

// generateMessageID generates a unique message ID.
func generateMessageID() string {
	return fmt.Sprintf("msg_%s", randomHex(12))
}

// randomHex generates a random hex string of the specified length.
func randomHex(n int) string {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp-based ID
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(bytes)
}

// mapFinishReason maps OpenAI finish_reason to Anthropic stop_reason.
func mapFinishReason(reason string) string {
	switch strings.ToLower(reason) {
	case "stop":
		return "end_turn"
	case "tool_calls":
		return "tool_use"
	case "length":
		return "max_tokens"
	default:
		return "end_turn"
	}
}
