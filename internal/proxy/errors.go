package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
)

// Anthropic error types used by the Messages API.  Keep these values stable:
// clients use them to decide whether a request can be retried.
const (
	invalidRequestError  = "invalid_request_error"
	authenticationError  = "authentication_error"
	permissionError      = "permission_error"
	notFoundError        = "not_found_error"
	requestTooLargeError = "request_too_large"
	rateLimitError       = "rate_limit_error"
	apiError             = "api_error"
	overloadedError      = "overloaded_error"
)

// normalizedError is the wire-level error contract exposed by CLASP.
type normalizedError struct {
	status  int
	errType string
	message string
}

type anthropicErrorResponse struct {
	Type  string                `json:"type"`
	Error anthropicErrorDetails `json:"error"`
}

type anthropicErrorDetails struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// writeAnthropicError writes the same envelope for errors produced before and
// after an upstream request.  In particular, it never emits net/http's plain
// text error format to an Anthropic client.
func writeAnthropicError(w http.ResponseWriter, status int, errType, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(anthropicErrorResponse{
		Type: "error",
		Error: anthropicErrorDetails{
			Type:    errType,
			Message: message,
		},
	})
}

// writeAnthropicStreamError emits the Anthropic SSE error event. HTTP status
// codes can no longer be changed after a stream has started, so the error
// envelope travels in the event data instead.
func writeAnthropicStreamError(w io.Writer, errType, message string) error {
	payload, err := json.Marshal(anthropicErrorResponse{
		Type:  "error",
		Error: anthropicErrorDetails{Type: errType, Message: message},
	})
	if err != nil {
		return fmt.Errorf("marshal stream error: %w", err)
	}
	if _, err := fmt.Fprintf(w, "event: error\ndata: %s\n\n", payload); err != nil {
		return fmt.Errorf("write stream error: %w", err)
	}
	return nil
}

// normalizeUpstreamError converts provider-specific HTTP statuses into the
// stable status/type pairs documented in docs/api-reference/errors.md.
func normalizeUpstreamError(status int) normalizedError {
	switch status {
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return normalizedError{http.StatusBadRequest, invalidRequestError, "The upstream provider rejected the request."}
	case http.StatusUnauthorized:
		return normalizedError{http.StatusUnauthorized, authenticationError, "The upstream provider rejected the configured credentials."}
	case http.StatusForbidden:
		return normalizedError{http.StatusForbidden, permissionError, "The upstream provider denied access to this request."}
	case http.StatusNotFound:
		return normalizedError{http.StatusNotFound, notFoundError, "The requested model or upstream resource was not found."}
	case http.StatusRequestEntityTooLarge:
		return normalizedError{http.StatusRequestEntityTooLarge, requestTooLargeError, "The request is too large for the upstream provider."}
	case http.StatusRequestTimeout, http.StatusGatewayTimeout:
		return normalizedError{http.StatusGatewayTimeout, apiError, "The upstream provider request timed out."}
	case http.StatusTooManyRequests:
		return normalizedError{http.StatusTooManyRequests, rateLimitError, "The upstream provider rate limit was exceeded."}
	case http.StatusServiceUnavailable, 529:
		return normalizedError{529, overloadedError, "The upstream provider is temporarily overloaded."}
	default:
		if status >= 500 {
			return normalizedError{http.StatusBadGateway, apiError, "The upstream provider returned an error."}
		}
		if status >= 400 {
			return normalizedError{status, invalidRequestError, "The upstream provider rejected the request."}
		}
		return normalizedError{http.StatusBadGateway, apiError, "The upstream provider returned an unexpected response."}
	}
}

func normalizeUpstreamFailure(err error) normalizedError {
	if isTimeoutError(err) {
		return normalizedError{http.StatusGatewayTimeout, apiError, "The upstream provider request timed out."}
	}
	return normalizedError{http.StatusBadGateway, apiError, "Unable to reach the upstream provider."}
}

func malformedProviderResponseError() normalizedError {
	return normalizedError{http.StatusBadGateway, apiError, "The upstream provider returned a malformed response."}
}

func streamFailureError(err error) normalizedError {
	if isTimeoutError(err) {
		return normalizedError{http.StatusGatewayTimeout, apiError, "The upstream provider stream timed out."}
	}
	return normalizedError{http.StatusBadGateway, apiError, "The upstream provider stream failed."}
}

func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return errors.Is(err, context.DeadlineExceeded)
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

// providerErrorMessage deliberately does not copy arbitrary provider payloads
// into the public contract. It is kept for logging/tests that need to inspect
// whether a body was a provider error without exposing credentials or internals.
func providerErrorMessage(body []byte) string {
	var envelope struct {
		Error json.RawMessage `json:"error"`
	}
	if json.Unmarshal(body, &envelope) != nil || len(envelope.Error) == 0 {
		return ""
	}
	var detail struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(envelope.Error, &detail) == nil {
		return strings.TrimSpace(detail.Message)
	}
	return ""
}
