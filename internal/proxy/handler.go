// Package proxy implements the HTTP proxy server.
package proxy

import (
	"bytes"
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
	"github.com/jedarden/clasp/internal/provider"
	"github.com/jedarden/clasp/internal/translator"
	"github.com/jedarden/clasp/pkg/models"
)

// Handler handles incoming Anthropic API requests.
type Handler struct {
	cfg      *config.Config
	provider provider.Provider
	client   *http.Client
	metrics  *Metrics
}

// Metrics tracks request statistics.
type Metrics struct {
	TotalRequests    int64
	SuccessRequests  int64
	ErrorRequests    int64
	StreamRequests   int64
	ToolCallRequests int64
	TotalLatencyMs   int64
	StartTime        time.Time
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

	client := &http.Client{
		Transport: transport,
		Timeout:   120 * time.Second, // Long timeout for streaming
	}

	return &Handler{
		cfg:      cfg,
		provider: p,
		client:   client,
		metrics:  &Metrics{StartTime: time.Now()},
	}, nil
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
	case config.ProviderCustom:
		return provider.NewCustomProvider(cfg.CustomBaseURL), nil
	default:
		return nil, fmt.Errorf("unsupported provider: %s", cfg.Provider)
	}
}

// HandleMessages handles POST /v1/messages requests.
func (h *Handler) HandleMessages(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	atomic.AddInt64(&h.metrics.TotalRequests, 1)

	// Only accept POST
	if r.Method != http.MethodPost {
		atomic.AddInt64(&h.metrics.ErrorRequests, 1)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body
	var anthropicReq models.AnthropicRequest
	if err := json.NewDecoder(r.Body).Decode(&anthropicReq); err != nil {
		atomic.AddInt64(&h.metrics.ErrorRequests, 1)
		log.Printf("[CLASP] Error parsing request: %v", err)
		h.writeErrorResponse(w, http.StatusBadRequest, "invalid_request_error", "Invalid request body")
		return
	}
	defer r.Body.Close()

	// Track request types
	if anthropicReq.Stream {
		atomic.AddInt64(&h.metrics.StreamRequests, 1)
	}
	if len(anthropicReq.Tools) > 0 {
		atomic.AddInt64(&h.metrics.ToolCallRequests, 1)
	}

	// Map the model
	targetModel := h.cfg.MapModel(anthropicReq.Model)
	targetModel = h.provider.TransformModelID(targetModel)

	log.Printf("[CLASP] Request: %s -> %s (streaming: %v)", anthropicReq.Model, targetModel, anthropicReq.Stream)

	// Transform request to OpenAI format
	openAIReq, err := translator.TransformRequest(&anthropicReq, targetModel)
	if err != nil {
		atomic.AddInt64(&h.metrics.ErrorRequests, 1)
		log.Printf("[CLASP] Error transforming request: %v", err)
		h.writeErrorResponse(w, http.StatusInternalServerError, "api_error", "Error transforming request")
		return
	}

	// Make upstream request
	reqBody, err := json.Marshal(openAIReq)
	if err != nil {
		atomic.AddInt64(&h.metrics.ErrorRequests, 1)
		log.Printf("[CLASP] Error marshaling request: %v", err)
		h.writeErrorResponse(w, http.StatusInternalServerError, "api_error", "Error preparing request")
		return
	}

	// Execute request with retry logic
	resp, err := h.doRequestWithRetry(r.Context(), reqBody)
	if err != nil {
		atomic.AddInt64(&h.metrics.ErrorRequests, 1)
		log.Printf("[CLASP] Error making upstream request: %v", err)
		h.writeErrorResponse(w, http.StatusBadGateway, "api_error", "Error connecting to upstream provider")
		return
	}
	defer resp.Body.Close()

	// Check for upstream errors
	if resp.StatusCode >= 400 {
		atomic.AddInt64(&h.metrics.ErrorRequests, 1)
		body, _ := io.ReadAll(resp.Body)
		log.Printf("[CLASP] Upstream error (%d): %s", resp.StatusCode, string(body))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		w.Write(body)
		return
	}

	atomic.AddInt64(&h.metrics.SuccessRequests, 1)
	atomic.AddInt64(&h.metrics.TotalLatencyMs, time.Since(start).Milliseconds())

	// Handle streaming vs non-streaming
	if anthropicReq.Stream {
		h.handleStreamingResponse(w, resp, targetModel)
	} else {
		h.handleNonStreamingResponse(w, resp, targetModel)
	}
}

// doRequestWithRetry executes the upstream request with exponential backoff retry.
func (h *Handler) doRequestWithRetry(ctx interface{ Done() <-chan struct{} }, reqBody []byte) (*http.Response, error) {
	maxRetries := 3
	baseDelay := 500 * time.Millisecond

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		// Create fresh request for each attempt
		upstreamReq, err := http.NewRequest(http.MethodPost, h.provider.GetEndpointURL(), bytes.NewReader(reqBody))
		if err != nil {
			return nil, fmt.Errorf("creating request: %w", err)
		}

		// Set headers
		for key, values := range h.provider.GetHeaders(h.cfg.GetAPIKey()) {
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

// writeErrorResponse writes an Anthropic-formatted error response.
func (h *Handler) writeErrorResponse(w http.ResponseWriter, status int, errType, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
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
	if err := processor.ProcessStream(resp.Body); err != nil {
		log.Printf("[CLASP] Error processing stream: %v", err)
	}
}

// handleNonStreamingResponse handles non-streaming responses.
func (h *Handler) handleNonStreamingResponse(w http.ResponseWriter, resp *http.Response, targetModel string) {
	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[CLASP] Error reading response: %v", err)
		http.Error(w, "Error reading upstream response", http.StatusBadGateway)
		return
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
			json.Unmarshal([]byte(tc.Function.Arguments), &input)

			anthropicResp.Content = append(anthropicResp.Content, models.AnthropicContentBlock{
				Type:  "tool_use",
				ID:    tc.ID,
				Name:  tc.Function.Name,
				Input: input,
			})
		}
	}

	// Write response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(anthropicResp)
}

// HandleHealth handles health check requests.
func (h *Handler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"requests": map[string]interface{}{
			"total":       total,
			"successful":  success,
			"errors":      errors,
			"streaming":   streams,
			"tool_calls":  toolCalls,
			"success_rate": fmt.Sprintf("%.2f%%", successRate),
		},
		"performance": map[string]interface{}{
			"avg_latency_ms":  fmt.Sprintf("%.2f", avgLatency),
			"requests_per_sec": fmt.Sprintf("%.2f", requestsPerSec),
		},
		"uptime": uptime.String(),
		"provider": h.provider.Name(),
	})
}

// HandleRoot handles root path requests.
func (h *Handler) HandleRoot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"name":     "CLASP",
		"version":  "0.2.1",
		"provider": h.provider.Name(),
		"status":   "running",
		"endpoints": map[string]string{
			"messages": "/v1/messages",
			"health":   "/health",
			"metrics":  "/metrics",
		},
	})
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
