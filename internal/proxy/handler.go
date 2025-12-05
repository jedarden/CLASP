// Package proxy implements the HTTP proxy server.
package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

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
}

// NewHandler creates a new request handler.
func NewHandler(cfg *config.Config) (*Handler, error) {
	p, err := createProvider(cfg)
	if err != nil {
		return nil, err
	}

	return &Handler{
		cfg:      cfg,
		provider: p,
		client:   &http.Client{},
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
	// Only accept POST
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body
	var anthropicReq models.AnthropicRequest
	if err := json.NewDecoder(r.Body).Decode(&anthropicReq); err != nil {
		log.Printf("[CLASP] Error parsing request: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Map the model
	targetModel := h.cfg.MapModel(anthropicReq.Model)
	targetModel = h.provider.TransformModelID(targetModel)

	log.Printf("[CLASP] Request: %s -> %s (streaming: %v)", anthropicReq.Model, targetModel, anthropicReq.Stream)

	// Transform request to OpenAI format
	openAIReq, err := translator.TransformRequest(&anthropicReq, targetModel)
	if err != nil {
		log.Printf("[CLASP] Error transforming request: %v", err)
		http.Error(w, "Error transforming request", http.StatusInternalServerError)
		return
	}

	// Make upstream request
	reqBody, err := json.Marshal(openAIReq)
	if err != nil {
		log.Printf("[CLASP] Error marshaling request: %v", err)
		http.Error(w, "Error preparing request", http.StatusInternalServerError)
		return
	}

	upstreamReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, h.provider.GetEndpointURL(), bytes.NewReader(reqBody))
	if err != nil {
		log.Printf("[CLASP] Error creating upstream request: %v", err)
		http.Error(w, "Error creating upstream request", http.StatusInternalServerError)
		return
	}

	// Set headers
	for key, values := range h.provider.GetHeaders(h.cfg.GetAPIKey()) {
		for _, v := range values {
			upstreamReq.Header.Add(key, v)
		}
	}

	// Make request
	resp, err := h.client.Do(upstreamReq)
	if err != nil {
		log.Printf("[CLASP] Error making upstream request: %v", err)
		http.Error(w, "Error connecting to upstream", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Check for upstream errors
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("[CLASP] Upstream error (%d): %s", resp.StatusCode, string(body))
		w.WriteHeader(resp.StatusCode)
		w.Write(body)
		return
	}

	// Handle streaming vs non-streaming
	if anthropicReq.Stream {
		h.handleStreamingResponse(w, resp, targetModel)
	} else {
		h.handleNonStreamingResponse(w, resp, targetModel)
	}
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
	json.NewEncoder(w).Encode(map[string]string{
		"status":   "healthy",
		"provider": h.provider.Name(),
	})
}

// HandleRoot handles root path requests.
func (h *Handler) HandleRoot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"name":     "CLASP",
		"version":  "0.1.0",
		"provider": h.provider.Name(),
		"status":   "running",
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
	return fmt.Sprintf("msg_%d", randomID())
}

// randomID generates a random ID component.
func randomID() int64 {
	// Simple incrementing ID for now
	return int64(1234567890)
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
