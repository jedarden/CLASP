// Package provider implements LLM provider backends.
package provider

import (
	"fmt"
	"net/http"
)

// AzureProvider implements the Provider interface for Azure OpenAI.
type AzureProvider struct {
	Endpoint       string
	DeploymentName string
	APIVersion     string
}

// NewAzureProvider creates a new Azure OpenAI provider.
func NewAzureProvider(endpoint, deploymentName, apiVersion string) *AzureProvider {
	if apiVersion == "" {
		apiVersion = "2024-02-15-preview"
	}
	return &AzureProvider{
		Endpoint:       endpoint,
		DeploymentName: deploymentName,
		APIVersion:     apiVersion,
	}
}

// Name returns the provider name.
func (p *AzureProvider) Name() string {
	return "azure"
}

// GetHeaders returns the HTTP headers for Azure OpenAI API requests.
func (p *AzureProvider) GetHeaders(apiKey string) http.Header {
	headers := http.Header{}
	headers.Set("api-key", apiKey)
	headers.Set("Content-Type", "application/json")
	return headers
}

// GetEndpointURL returns the chat completions endpoint URL for Azure.
func (p *AzureProvider) GetEndpointURL() string {
	return fmt.Sprintf("%s/openai/deployments/%s/chat/completions?api-version=%s",
		p.Endpoint, p.DeploymentName, p.APIVersion)
}

// TransformModelID returns the deployment name for Azure.
func (p *AzureProvider) TransformModelID(modelID string) string {
	// Azure uses deployment names, not model IDs
	return p.DeploymentName
}

// SupportsStreaming indicates that Azure supports SSE streaming.
func (p *AzureProvider) SupportsStreaming() bool {
	return true
}

// RequiresTransformation indicates that Azure needs Anthropic->OpenAI translation.
func (p *AzureProvider) RequiresTransformation() bool {
	return true
}
