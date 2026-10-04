package proxy

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/jedarden/clasp/internal/logging"
	"github.com/jedarden/clasp/internal/provider"
)

const (
	defaultUpstreamTimeout = 5 * time.Minute
	upstreamDialTimeout    = 30 * time.Second
	upstreamKeepAlive      = 30 * time.Second
	upstreamIdleTimeout    = 90 * time.Second
	upstreamTLSHandshake   = 10 * time.Second
	upstreamMaxIdleConns   = 100

	// An attempt is the initial request plus any retry. Keeping this small limits
	// duplicate provider work when a network failure happens after request send.
	defaultRetryAttempts = 3
	defaultRetryDelay    = 500 * time.Millisecond
)

// retryableStatusCodes are transient responses for which replayable requests
// may be sent again. 4xx validation/authentication failures are never retried.
var retryableStatusCodes = map[int]bool{
	http.StatusRequestTimeout:      true, // 408
	http.StatusTooEarly:            true, // 425
	http.StatusTooManyRequests:     true, // 429
	http.StatusInternalServerError: true, // 500
	http.StatusBadGateway:          true, // 502
	http.StatusServiceUnavailable:  true, // 503
	http.StatusGatewayTimeout:      true, // 504
	529:                            true, // provider overload (Anthropic-style)
}

// retryConfig is injectable in tests so backoff behavior can be verified
// without making the test suite wait for production delays.
type retryConfig struct {
	attempts  int
	baseDelay time.Duration
}

var defaultUpstreamRetryConfig = retryConfig{
	attempts:  defaultRetryAttempts,
	baseDelay: defaultRetryDelay,
}

// newUpstreamHTTPTransport returns the single shared transport used by a
// Handler. Sharing it is what allows requests to the same provider host to
// reuse idle TCP/TLS connections safely.
func newUpstreamHTTPTransport() *http.Transport {
	return &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   upstreamDialTimeout,
			KeepAlive: upstreamKeepAlive,
		}).DialContext,
		MaxIdleConns:        upstreamMaxIdleConns,
		MaxIdleConnsPerHost: upstreamMaxIdleConns,
		IdleConnTimeout:     upstreamIdleTimeout,
		TLSHandshakeTimeout: upstreamTLSHandshake,
		DisableCompression:  false,
	}
}

func newUpstreamHTTPClient(timeoutSeconds int) *http.Client {
	timeout := defaultUpstreamTimeout
	if timeoutSeconds > 0 {
		timeout = time.Duration(timeoutSeconds) * time.Second
	}

	return &http.Client{
		Transport: newUpstreamHTTPTransport(),
		Timeout:   timeout,
	}
}

// doRequestWithRetry retains the replayable default for package-local callers
// that issue a request known to be safe to repeat. Request handlers use
// doRequestWithRetryPolicy so streaming requests can opt out explicitly.
func (h *Handler) doRequestWithRetry(ctx context.Context, reqBody []byte, p provider.Provider) (*http.Response, error) {
	return h.doRequestWithRetryPolicy(ctx, reqBody, p, true)
}

// doRequestWithRetryPolicy executes an upstream POST with bounded exponential
// backoff. Non-replayable work is sent exactly once, even for transient
// statuses or network errors. A response with a retryable status is returned
// after the final attempt so normal fallback/error handling can inspect it.
func (h *Handler) doRequestWithRetryPolicy(ctx context.Context, reqBody []byte, p provider.Provider, replayable bool) (*http.Response, error) {
	return h.doRequestWithRetryConfig(ctx, reqBody, p, replayable, defaultUpstreamRetryConfig)
}

func (h *Handler) doRequestWithRetryConfig(ctx context.Context, reqBody []byte, p provider.Provider, replayable bool, cfg retryConfig) (*http.Response, error) {
	if cfg.attempts < 1 {
		cfg.attempts = 1
	}
	if cfg.baseDelay < 0 {
		cfg.baseDelay = 0
	}

	var lastErr error
	for attempt := 1; attempt <= cfg.attempts; attempt++ {
		upstreamReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.GetEndpointURL(), bytes.NewReader(reqBody))
		if err != nil {
			return nil, fmt.Errorf("creating request: %w", err)
		}

		for key, values := range p.GetHeaders(h.cfg.GetAPIKey()) {
			for _, value := range values {
				upstreamReq.Header.Add(key, value)
			}
		}

		resp, err := h.client.Do(upstreamReq)
		if err == nil {
			if !replayable || !isRetryableStatus(resp.StatusCode) || attempt == cfg.attempts {
				return resp, nil
			}

			// Drain before closing so the shared transport can reuse the
			// connection for the next attempt when the body is small/finite.
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			lastErr = fmt.Errorf("upstream returned %d", resp.StatusCode)
		} else {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			lastErr = err
			if !replayable || !isRetryableNetworkError(err, ctx) || attempt == cfg.attempts {
				return nil, err
			}
		}

		delay := retryDelay(cfg.baseDelay, attempt)
		logging.Warn("Retry %d/%d after %v: %v", attempt, cfg.attempts, delay, lastErr)
		if err := waitForRetry(ctx, delay); err != nil {
			return nil, err
		}
	}

	return nil, lastErr
}

// retryDelay returns the delay before the given 1-based retry attempt. The
// first retry is baseDelay, the second is 2*baseDelay, and so on.
func retryDelay(baseDelay time.Duration, retryAttempt int) time.Duration {
	if retryAttempt < 1 || baseDelay <= 0 {
		return 0
	}
	return baseDelay * time.Duration(1<<(retryAttempt-1))
}

func isRetryableStatus(status int) bool {
	return retryableStatusCodes[status]
}

func isRetryableNetworkError(err error, ctx context.Context) bool {
	if err == nil || ctx.Err() != nil || errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}

	var netErr net.Error
	return errors.As(err, &netErr)
}

func waitForRetry(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			return nil
		}
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
