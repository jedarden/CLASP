// Package provider defines the provider abstraction for different LLM backends.
package provider

import (
	"net/http"
)

// Provider defines the interface for LLM provider backends.
type Provider interface {
	// Name returns the provider name.
	Name() string

	// GetHeaders returns the HTTP headers for API requests.
	GetHeaders(apiKey string) http.Header

	// GetEndpointURL returns the full URL for chat completions.
	GetEndpointURL() string

	// TransformModelID transforms a model ID for the provider.
	TransformModelID(modelID string) string

	// SupportsStreaming indicates if the provider supports SSE streaming.
	SupportsStreaming() bool

	// RequiresTransformation indicates if the provider needs Anthropic->OpenAI translation.
	RequiresTransformation() bool
}
