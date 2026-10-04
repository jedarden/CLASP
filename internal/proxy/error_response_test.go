package proxy

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jedarden/clasp/internal/config"
)

func decodeAnthropicError(t *testing.T, body []byte) anthropicErrorResponse {
	t.Helper()
	var response anthropicErrorResponse
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("decode error response: %v; body=%q", err, body)
	}
	if response.Type != "error" {
		t.Fatalf("response type = %q, want error", response.Type)
	}
	return response
}

func TestNormalizeUpstreamError(t *testing.T) {
	tests := []struct {
		status   int
		wantHTTP int
		wantType string
	}{
		{http.StatusBadRequest, http.StatusBadRequest, invalidRequestError},
		{http.StatusUnauthorized, http.StatusUnauthorized, authenticationError},
		{http.StatusForbidden, http.StatusForbidden, permissionError},
		{http.StatusNotFound, http.StatusNotFound, notFoundError},
		{http.StatusRequestEntityTooLarge, http.StatusRequestEntityTooLarge, requestTooLargeError},
		{http.StatusTooManyRequests, http.StatusTooManyRequests, rateLimitError},
		{http.StatusInternalServerError, http.StatusBadGateway, apiError},
		{http.StatusServiceUnavailable, 529, overloadedError},
		{529, 529, overloadedError},
	}

	for _, tt := range tests {
		t.Run(http.StatusText(tt.status), func(t *testing.T) {
			got := normalizeUpstreamError(tt.status)
			if got.status != tt.wantHTTP || got.errType != tt.wantType {
				t.Fatalf("normalizeUpstreamError(%d) = (%d, %q), want (%d, %q)", tt.status, got.status, got.errType, tt.wantHTTP, tt.wantType)
			}
		})
	}
}

func TestWriteAnthropicError(t *testing.T) {
	recorder := httptest.NewRecorder()
	writeAnthropicError(recorder, http.StatusBadRequest, invalidRequestError, "bad request")

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", recorder.Code)
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content type = %q, want application/json", got)
	}
	response := decodeAnthropicError(t, recorder.Body.Bytes())
	if response.Error.Type != invalidRequestError || response.Error.Message != "bad request" {
		t.Fatalf("error = %+v", response.Error)
	}
}

func TestInvalidRequestUsesAnthropicErrorEnvelope(t *testing.T) {
	handler, err := NewHandler(config.DefaultConfig())
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"messages":[]}`))
	recorder := httptest.NewRecorder()
	handler.HandleMessages(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", recorder.Code)
	}
	response := decodeAnthropicError(t, recorder.Body.Bytes())
	if response.Error.Type != invalidRequestError || response.Error.Message != "Missing required field: 'model'" {
		t.Fatalf("error = %+v", response.Error)
	}
}

func TestLocalAuthenticationAndRateLimitErrorsUseAnthropicEnvelope(t *testing.T) {
	tests := []struct {
		name       string
		write      func(http.ResponseWriter)
		wantStatus int
		wantType   string
	}{
		{
			name: "authentication",
			write: func(w http.ResponseWriter) {
				writeAuthError(w, http.StatusUnauthorized, authenticationError, "Invalid API key")
			},
			wantStatus: http.StatusUnauthorized,
			wantType:   authenticationError,
		},
		{
			name: "rate limit",
			write: func(w http.ResponseWriter) {
				writeRateLimitError(w, time.Second)
			},
			wantStatus: http.StatusTooManyRequests,
			wantType:   rateLimitError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			tt.write(recorder)
			if recorder.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, tt.wantStatus)
			}
			response := decodeAnthropicError(t, recorder.Body.Bytes())
			if response.Error.Type != tt.wantType {
				t.Fatalf("error type = %q, want %q", response.Error.Type, tt.wantType)
			}
		})
	}
}

func TestHandleUpstreamErrorNormalizesProviderPayload(t *testing.T) {
	handler := &Handler{metrics: &Metrics{}}
	upstream := &http.Response{
		StatusCode: http.StatusTooManyRequests,
		Body:       io.NopCloser(strings.NewReader(`{"error":{"type":"rate_limit_error","message":"provider internals"}}`)),
	}
	recorder := httptest.NewRecorder()
	handler.handleUpstreamError(recorder, upstream)

	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", recorder.Code)
	}
	response := decodeAnthropicError(t, recorder.Body.Bytes())
	if response.Error.Type != rateLimitError {
		t.Fatalf("error type = %q, want %q", response.Error.Type, rateLimitError)
	}
	if strings.Contains(recorder.Body.String(), "provider internals") {
		t.Fatal("provider-specific message leaked into normalized response")
	}
}

func TestCustomProviderChatErrorIsNormalized(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, `{"error":{"message":"custom provider failure"}}`)
	}))
	defer upstream.Close()

	cfg := config.DefaultConfig()
	cfg.Provider = config.ProviderCustom
	cfg.CustomBaseURL = upstream.URL
	cfg.CustomAPIKey = "test-key"
	cfg.DefaultModel = "custom-model"
	handler, err := NewHandler(cfg)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"custom-model","messages":[{"role":"user","content":"hello"}]}`))
	recorder := httptest.NewRecorder()
	handler.HandleMessages(recorder, req)

	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", recorder.Code)
	}
	response := decodeAnthropicError(t, recorder.Body.Bytes())
	if response.Error.Type != apiError {
		t.Fatalf("error type = %q, want %q", response.Error.Type, apiError)
	}
}

func TestResponsesMalformedProviderResponseIsNormalized(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{}`)
	}))
	defer upstream.Close()

	cfg := config.DefaultConfig()
	cfg.Provider = config.ProviderOpenAI
	cfg.OpenAIBaseURL = upstream.URL
	cfg.OpenAIAPIKey = "test-key"
	cfg.DefaultModel = "gpt-5"
	handler, err := NewHandler(cfg)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"gpt-5","messages":[{"role":"user","content":"hello"}]}`))
	recorder := httptest.NewRecorder()
	handler.HandleMessages(recorder, req)

	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", recorder.Code)
	}
	response := decodeAnthropicError(t, recorder.Body.Bytes())
	if response.Error.Type != apiError || response.Error.Message != malformedProviderResponseError().message {
		t.Fatalf("error = %+v", response.Error)
	}
}

func TestResponsesProviderHTTPErrorIsNormalized(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/responses" {
			t.Fatalf("upstream path = %q, want /responses", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = io.WriteString(w, `{"error":{"message":"provider-specific quota details"}}`)
	}))
	defer upstream.Close()

	cfg := config.DefaultConfig()
	cfg.Provider = config.ProviderOpenAI
	cfg.OpenAIBaseURL = upstream.URL
	cfg.OpenAIAPIKey = "test-key"
	cfg.DefaultModel = "gpt-5"
	handler, err := NewHandler(cfg)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"gpt-5","messages":[{"role":"user","content":"hello"}]}`))
	recorder := httptest.NewRecorder()
	handler.HandleMessages(recorder, req)

	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", recorder.Code)
	}
	response := decodeAnthropicError(t, recorder.Body.Bytes())
	if response.Error.Type != rateLimitError {
		t.Fatalf("error type = %q, want %q", response.Error.Type, rateLimitError)
	}
	if strings.Contains(recorder.Body.String(), "provider-specific") {
		t.Fatal("provider-specific message leaked into normalized response")
	}
}

func TestUpstreamTimeoutIsNormalized(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		_, _ = io.WriteString(w, `{}`)
	}))
	defer upstream.Close()

	cfg := config.DefaultConfig()
	cfg.Provider = config.ProviderCustom
	cfg.CustomBaseURL = upstream.URL
	cfg.CustomAPIKey = "test-key"
	cfg.DefaultModel = "custom-model"
	handler, err := NewHandler(cfg)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"custom-model","messages":[{"role":"user","content":"hello"}]}`)).WithContext(ctx)
	recorder := httptest.NewRecorder()
	handler.HandleMessages(recorder, req)

	if recorder.Code != http.StatusGatewayTimeout {
		t.Fatalf("status = %d, want 504", recorder.Code)
	}
	response := decodeAnthropicError(t, recorder.Body.Bytes())
	if response.Error.Type != apiError || response.Error.Message != "The upstream provider request timed out." {
		t.Fatalf("error = %+v", response.Error)
	}
}

func TestStreamingMalformedChatResponseEmitsAnthropicErrorEvent(t *testing.T) {
	handler := &Handler{}
	upstream := &http.Response{Body: io.NopCloser(strings.NewReader("data: {not-json}\n\n"))}
	recorder := httptest.NewRecorder()
	handler.handleStreamingResponse(recorder, upstream, "custom-model")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 after stream starts", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), `event: error`) || !strings.Contains(recorder.Body.String(), `"type":"api_error"`) {
		t.Fatalf("stream did not contain Anthropic error event: %q", recorder.Body.String())
	}
}

func TestStreamingMalformedResponsesResponseEmitsAnthropicErrorEvent(t *testing.T) {
	handler := &Handler{}
	upstream := &http.Response{Body: io.NopCloser(strings.NewReader("data: {not-json}\n\n"))}
	recorder := httptest.NewRecorder()
	handler.handleResponsesStreamingResponse(recorder, upstream, "gpt-5", "", 0)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 after stream starts", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), `event: error`) || !strings.Contains(recorder.Body.String(), `"type":"api_error"`) {
		t.Fatalf("stream did not contain Anthropic error event: %q", recorder.Body.String())
	}
}

type failingStreamBody struct {
	sent bool
}

func (b *failingStreamBody) Read(p []byte) (int, error) {
	if b.sent {
		return 0, io.ErrUnexpectedEOF
	}
	b.sent = true
	return copy(p, "event: message_start\ndata: {}\n\n"), nil
}

func (b *failingStreamBody) Close() error { return nil }

func TestPassthroughStreamingReadErrorEmitsAnthropicErrorEvent(t *testing.T) {
	handler := &Handler{}
	recorder := httptest.NewRecorder()
	handler.handlePassthroughStreaming(recorder, &http.Response{Body: &failingStreamBody{}}, "claude")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 after stream starts", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), `event: error`) || !strings.Contains(recorder.Body.String(), `"type":"api_error"`) {
		t.Fatalf("stream did not contain Anthropic error event: %q", recorder.Body.String())
	}
}
