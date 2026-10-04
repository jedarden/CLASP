package proxy

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jedarden/clasp/internal/config"
)

type transportTestProvider struct {
	endpoint string
}

func (p *transportTestProvider) Name() string { return "transport-test" }

func (p *transportTestProvider) GetHeaders(string) http.Header {
	return http.Header{"Content-Type": []string{"application/json"}}
}

func (p *transportTestProvider) GetEndpointURL() string { return p.endpoint }

func (p *transportTestProvider) TransformModelID(modelID string) string { return modelID }

func (p *transportTestProvider) SupportsStreaming() bool { return true }

func (p *transportTestProvider) RequiresTransformation() bool { return true }

type transportRoundTripper func(*http.Request) (*http.Response, error)

func (f transportRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

type temporaryTransportError struct{}

func (temporaryTransportError) Error() string   { return "temporary network failure" }
func (temporaryTransportError) Timeout() bool   { return false }
func (temporaryTransportError) Temporary() bool { return true }

func newTransportTestHandler(client *http.Client) *Handler {
	return &Handler{
		cfg:    config.DefaultConfig(),
		client: client,
	}
}

func transportResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

func TestRetryableStatusCodes(t *testing.T) {
	for _, status := range []int{408, 425, 429, 500, 502, 503, 504, 529} {
		if !isRetryableStatus(status) {
			t.Errorf("status %d should be retryable", status)
		}
	}

	for _, status := range []int{400, 401, 403, 404, 409, 413, 422, 501} {
		if isRetryableStatus(status) {
			t.Errorf("status %d should not be retryable", status)
		}
	}
}

func TestRetryDelayUsesExponentialBackoff(t *testing.T) {
	base := 500 * time.Millisecond
	for attempt, want := range map[int]time.Duration{
		1: 500 * time.Millisecond,
		2: time.Second,
		3: 2 * time.Second,
	} {
		if got := retryDelay(base, attempt); got != want {
			t.Errorf("retryDelay(%d) = %v, want %v", attempt, got, want)
		}
	}
}

func TestDoRequestWithRetry_RetriesTransientStatusWithExponentialBackoff(t *testing.T) {
	var attempts int32
	client := &http.Client{Transport: transportRoundTripper(func(*http.Request) (*http.Response, error) {
		attempt := atomic.AddInt32(&attempts, 1)
		if attempt == 1 {
			return transportResponse(http.StatusServiceUnavailable, "try again"), nil
		}
		return transportResponse(http.StatusOK, "ok"), nil
	})}
	h := newTransportTestHandler(client)

	resp, err := h.doRequestWithRetryConfig(
		context.Background(), []byte(`{"work":"replayable"}`),
		&transportTestProvider{endpoint: "http://provider.invalid/v1/messages"}, true,
		retryConfig{attempts: 3, baseDelay: 0},
	)
	if err != nil {
		t.Fatalf("retry returned error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if got := atomic.LoadInt32(&attempts); got != 2 {
		t.Fatalf("attempts = %d, want 2", got)
	}
}

func TestDoRequestWithRetry_RetriesNetworkError(t *testing.T) {
	var attempts int32
	client := &http.Client{Transport: transportRoundTripper(func(*http.Request) (*http.Response, error) {
		if atomic.AddInt32(&attempts, 1) == 1 {
			return nil, temporaryTransportError{}
		}
		return transportResponse(http.StatusOK, "ok"), nil
	})}
	h := newTransportTestHandler(client)

	resp, err := h.doRequestWithRetryConfig(
		context.Background(), []byte(`{"work":"replayable"}`),
		&transportTestProvider{endpoint: "http://provider.invalid/v1/messages"}, true,
		retryConfig{attempts: 3, baseDelay: 0},
	)
	if err != nil {
		t.Fatalf("retry returned error: %v", err)
	}
	defer resp.Body.Close()
	if got := atomic.LoadInt32(&attempts); got != 2 {
		t.Fatalf("attempts = %d, want 2", got)
	}
}

func TestDoRequestWithRetry_DoesNotDuplicateNonReplayableWork(t *testing.T) {
	var attempts int32
	client := &http.Client{Transport: transportRoundTripper(func(*http.Request) (*http.Response, error) {
		atomic.AddInt32(&attempts, 1)
		return transportResponse(http.StatusServiceUnavailable, "overloaded"), nil
	})}
	h := newTransportTestHandler(client)

	resp, err := h.doRequestWithRetryConfig(
		context.Background(), []byte(`{"stream":true}`),
		&transportTestProvider{endpoint: "http://provider.invalid/v1/messages"}, false,
		retryConfig{attempts: 3, baseDelay: 0},
	)
	if err != nil {
		t.Fatalf("non-replayable request returned error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want original transient status", resp.StatusCode)
	}
	if got := atomic.LoadInt32(&attempts); got != 1 {
		t.Fatalf("attempts = %d, want exactly 1", got)
	}
}

func TestDoRequestWithRetry_StopsAtAttemptLimitAndReturnsFinalResponse(t *testing.T) {
	var attempts int32
	client := &http.Client{Transport: transportRoundTripper(func(*http.Request) (*http.Response, error) {
		atomic.AddInt32(&attempts, 1)
		return transportResponse(http.StatusBadGateway, "still unavailable"), nil
	})}
	h := newTransportTestHandler(client)

	resp, err := h.doRequestWithRetryConfig(
		context.Background(), []byte(`{"work":"replayable"}`),
		&transportTestProvider{endpoint: "http://provider.invalid/v1/messages"}, true,
		retryConfig{attempts: 3, baseDelay: 0},
	)
	if err != nil {
		t.Fatalf("retry returned error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("status = %d, want final upstream status", resp.StatusCode)
	}
	if got := atomic.LoadInt32(&attempts); got != 3 {
		t.Fatalf("attempts = %d, want configured limit 3", got)
	}
}

func TestDoRequestWithRetry_CancellationStopsBackoff(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client := &http.Client{Transport: transportRoundTripper(func(*http.Request) (*http.Response, error) {
		cancel()
		return transportResponse(http.StatusServiceUnavailable, "overloaded"), nil
	})}
	h := newTransportTestHandler(client)

	resp, err := h.doRequestWithRetryConfig(
		ctx, []byte(`{"work":"replayable"}`),
		&transportTestProvider{endpoint: "http://provider.invalid/v1/messages"}, true,
		retryConfig{attempts: 3, baseDelay: time.Hour},
	)
	if resp != nil {
		resp.Body.Close()
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func TestUpstreamTransportReusesPooledConnection(t *testing.T) {
	var requests int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}
		if string(body) != `{"work":"replayable"}` {
			t.Errorf("body = %q", body)
		}
		atomic.AddInt32(&requests, 1)
		w.Header().Set("Content-Length", "2")
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	transport := newUpstreamHTTPTransport()
	var dials int32
	dialer := &net.Dialer{}
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		atomic.AddInt32(&dials, 1)
		return dialer.DialContext(ctx, network, address)
	}
	h := newTransportTestHandler(&http.Client{Transport: transport})
	p := &transportTestProvider{endpoint: server.URL}

	for i := 0; i < 2; i++ {
		resp, err := h.doRequestWithRetryConfig(
			context.Background(), []byte(`{"work":"replayable"}`), p, true,
			retryConfig{attempts: 1},
		)
		if err != nil {
			t.Fatalf("request %d: %v", i+1, err)
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}

	if got := atomic.LoadInt32(&requests); got != 2 {
		t.Fatalf("requests = %d, want 2", got)
	}
	if got := atomic.LoadInt32(&dials); got != 1 {
		t.Fatalf("TCP dials = %d, want 1 pooled connection", got)
	}
}

func TestNewUpstreamHTTPClientUsesConfiguredTimeout(t *testing.T) {
	client := newUpstreamHTTPClient(7)
	if client.Timeout != 7*time.Second {
		t.Fatalf("timeout = %v, want 7s", client.Timeout)
	}

	defaultClient := newUpstreamHTTPClient(0)
	if defaultClient.Timeout != defaultUpstreamTimeout {
		t.Fatalf("default timeout = %v, want %v", defaultClient.Timeout, defaultUpstreamTimeout)
	}
}
