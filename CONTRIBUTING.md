# Contributing to CLASP

Thank you for your interest in contributing to CLASP! This document provides guidelines and instructions for contributing.

## Code of Conduct

Please be respectful and constructive in all interactions. We welcome contributions from everyone.

## Getting Started

### Prerequisites

- Go 1.22 or later
- Node.js 20+ (for npm package)
- Docker (optional, for container builds)

### Local Development Setup

```bash
# Clone the repository
git clone https://github.com/jedarden/CLASP.git
cd CLASP

# Install Go dependencies
go mod download

# Build
go build ./cmd/clasp

# Run tests
go test -v ./...

# Run linter
golangci-lint run
```

## Development Workflow

### Branch Naming

- `feat/description` - New features
- `fix/description` - Bug fixes
- `docs/description` - Documentation updates
- `refactor/description` - Code refactoring
- `test/description` - Test additions/updates

### Commit Messages

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
type(scope): description

[optional body]

[optional footer]
```

Types:
- `feat` - New feature (triggers minor version bump)
- `fix` - Bug fix (triggers patch version bump)
- `docs` - Documentation only
- `style` - Code style (formatting, etc.)
- `refactor` - Code refactoring
- `test` - Adding/updating tests
- `chore` - Maintenance tasks

Examples:
```
feat(proxy): add support for Gemini provider
fix(stream): resolve race condition in XML buffer
docs(readme): update installation instructions
```

## Adding a New LLM Provider

CLASP is designed to make adding new providers straightforward. Follow these steps:

### 1. Create Provider Implementation

Create a new file in `internal/provider/`:

```go
// internal/provider/newprovider.go
package provider

type NewProvider struct {
    baseURL string
    apiKey  string
}

func NewNewProvider(baseURL string) *NewProvider {
    return &NewProvider{baseURL: baseURL}
}

func NewNewProviderWithKey(baseURL, apiKey string) *NewProvider {
    return &NewProvider{baseURL: baseURL, apiKey: apiKey}
}

func (p *NewProvider) Name() string {
    return "newprovider"
}

func (p *NewProvider) GetEndpointURL() string {
    return p.baseURL + "/v1/chat/completions"
}

func (p *NewProvider) GetHeaders(apiKey string) http.Header {
    key := apiKey
    if p.apiKey != "" {
        key = p.apiKey
    }
    return http.Header{
        "Content-Type":  []string{"application/json"},
        "Authorization": []string{"Bearer " + key},
    }
}

func (p *NewProvider) RequiresTransformation() bool {
    return true // Set to false if API is Anthropic-compatible
}

func (p *NewProvider) TransformModelID(model string) string {
    return model // Transform if needed
}
```

### 2. Register Provider in Config

Add to `internal/config/config.go`:

```go
const (
    // ... existing providers
    ProviderNewProvider ProviderType = "newprovider"
)
```

### 3. Add Provider Creation

Update `internal/proxy/handler.go`:

```go
case config.ProviderNewProvider:
    return provider.NewNewProvider(cfg.NewProviderBaseURL), nil
```

### 4. Add Configuration Options

Update `internal/config/config.go` with any provider-specific config:

```go
type Config struct {
    // ... existing fields
    NewProviderBaseURL string
    NewProviderAPIKey  string
}
```

### 5. Add Environment Variable Support

```go
if url := os.Getenv("NEWPROVIDER_BASE_URL"); url != "" {
    cfg.NewProviderBaseURL = url
}
```

### 6. Add Tests

Create `internal/provider/newprovider_test.go`:

```go
func TestNewProvider_Name(t *testing.T) {
    p := NewNewProvider("https://api.example.com")
    if p.Name() != "newprovider" {
        t.Errorf("expected 'newprovider', got %s", p.Name())
    }
}
```

### 7. Update Documentation

- Add provider to README.md
- Document any special configuration requirements

## API Translation Guidelines

When adding support for a new API format:

### Request Translation

1. Study the target API's request format
2. Implement translation in `internal/translator/request.go`
3. Handle all content block types (text, images, tool use)
4. Preserve tool definitions and tool choice settings

### Response Translation

1. Handle both streaming and non-streaming responses
2. Map finish/stop reasons correctly
3. Preserve usage/token information
4. Handle error responses gracefully

### Streaming Protocol

CLASP translates OpenAI-style SSE to Anthropic-style SSE:

```
OpenAI: data: {"choices":[{"delta":{"content":"Hi"}}]}
        ↓
Anthropic: event: content_block_delta
           data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"Hi"}}
```

## Testing

### Running Tests

```bash
# All tests
go test -v ./...

# With race detection
go test -race ./...

# With coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Specific package
go test -v ./internal/translator/...
```

### Writing Tests

- Use table-driven tests where appropriate
- Include edge cases and error scenarios
- Mock external dependencies

```go
func TestFunction(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
        wantErr  bool
    }{
        {"valid input", "test", "TEST", false},
        {"empty input", "", "", true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := Function(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("unexpected error: %v", err)
            }
            if result != tt.expected {
                t.Errorf("expected %s, got %s", tt.expected, result)
            }
        })
    }
}
```

## Pull Request Process

1. **Fork and Branch**: Create a feature branch from `main`
2. **Implement**: Make your changes following the guidelines
3. **Test**: Ensure all tests pass and add new tests
4. **Lint**: Run `golangci-lint run` and fix any issues
5. **Document**: Update documentation if needed
6. **PR**: Open a pull request with a clear description

### PR Checklist

- [ ] Tests pass locally (`go test ./...`)
- [ ] Linter passes (`golangci-lint run`)
- [ ] Code follows project style
- [ ] Documentation updated (if applicable)
- [ ] Commit messages follow conventional commits
- [ ] No secrets or sensitive data committed

## Release Process

Releases are automated via GitHub Actions:

1. Push to `main` triggers version bump based on commit messages
2. Binaries are built for all platforms
3. GitHub Release is created
4. npm package is published
5. Docker image is pushed to GHCR

## Getting Help

- Open an issue for bugs or feature requests
- Use discussions for questions
- Check existing issues before creating new ones

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
