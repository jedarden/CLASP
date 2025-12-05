# CLASP - Claude Language Agent Super Proxy

A high-performance Go proxy that translates Claude/Anthropic API calls to OpenAI-compatible endpoints, enabling Claude Code to work with any LLM provider.

## Features

- **Multi-Provider Support**: OpenAI, Azure OpenAI, OpenRouter (200+ models), and custom endpoints (Ollama, vLLM, LM Studio)
- **Full Protocol Translation**: Anthropic Messages API ↔ OpenAI Chat Completions API
- **SSE Streaming**: Real-time token streaming with state machine processing
- **Tool Calls**: Complete translation of tool_use/tool_result between formats
- **Connection Pooling**: Optimized HTTP transport with persistent connections
- **Retry Logic**: Exponential backoff for transient failures
- **Metrics Endpoint**: Request statistics and performance monitoring

## Installation

### Via npm (recommended)

```bash
# Install globally
npm install -g clasp-proxy

# Or run directly with npx
npx clasp-proxy
```

### Via Go

```bash
go install github.com/jedarden/clasp/cmd/clasp@latest
```

### From Source

```bash
git clone https://github.com/jedarden/CLASP.git
cd CLASP
make build
```

### Via Docker

```bash
# Run with Docker
docker run -d -p 8080:8080 \
  -e OPENAI_API_KEY=sk-... \
  ghcr.io/jedarden/clasp-proxy:latest

# Or with docker-compose
docker-compose up -d
```

## Quick Start

### Using with OpenAI

```bash
# Set your API key
export OPENAI_API_KEY=sk-...

# Start the proxy
clasp -model gpt-4o

# In another terminal, use Claude Code
ANTHROPIC_BASE_URL=http://localhost:8080 claude
```

### Using with Azure OpenAI

```bash
export AZURE_API_KEY=your-key
export AZURE_OPENAI_ENDPOINT=https://your-resource.openai.azure.com
export AZURE_DEPLOYMENT_NAME=gpt-4

clasp -provider azure
```

### Using with OpenRouter

```bash
export OPENROUTER_API_KEY=sk-or-...

clasp -provider openrouter -model anthropic/claude-3-sonnet
```

### Using with Local Models (Ollama)

```bash
export CUSTOM_BASE_URL=http://localhost:11434/v1

clasp -provider custom -model llama3.1
```

## Configuration

### Command Line Options

```
clasp [options]

Options:
  -port <port>       Port to listen on (default: 8080)
  -provider <name>   LLM provider: openai, azure, openrouter, custom
  -model <model>     Default model to use for all requests
  -debug             Enable debug logging (full request/response)
  -version           Show version information
  -help              Show help message
```

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `PROVIDER` | LLM provider type | `openai` |
| `CLASP_PORT` | Proxy server port | `8080` |
| `CLASP_MODEL` | Default model | - |
| `CLASP_MODEL_OPUS` | Model for Opus tier | - |
| `CLASP_MODEL_SONNET` | Model for Sonnet tier | - |
| `CLASP_MODEL_HAIKU` | Model for Haiku tier | - |
| `OPENAI_API_KEY` | OpenAI API key | - |
| `OPENAI_BASE_URL` | Custom OpenAI base URL | `https://api.openai.com/v1` |
| `AZURE_API_KEY` | Azure OpenAI API key | - |
| `AZURE_OPENAI_ENDPOINT` | Azure endpoint URL | - |
| `AZURE_DEPLOYMENT_NAME` | Azure deployment name | - |
| `AZURE_API_VERSION` | Azure API version | `2024-02-15-preview` |
| `OPENROUTER_API_KEY` | OpenRouter API key | - |
| `CUSTOM_BASE_URL` | Custom endpoint base URL | - |
| `CUSTOM_API_KEY` | Custom endpoint API key | - |
| `CLASP_DEBUG` | Enable all debug logging | `false` |
| `CLASP_DEBUG_REQUESTS` | Log requests only | `false` |
| `CLASP_DEBUG_RESPONSES` | Log responses only | `false` |

### Model Mapping

CLASP can automatically map Claude model tiers to your provider's models:

```bash
# Map Claude tiers to specific models
export CLASP_MODEL_OPUS=gpt-4o
export CLASP_MODEL_SONNET=gpt-4o-mini
export CLASP_MODEL_HAIKU=gpt-3.5-turbo
```

## API Endpoints

| Endpoint | Description |
|----------|-------------|
| `POST /v1/messages` | Anthropic Messages API (translated) |
| `GET /health` | Health check |
| `GET /metrics` | Request statistics |
| `GET /` | Server info |

## Example Usage

### With curl

```bash
curl http://localhost:8080/v1/messages \
  -H "Content-Type: application/json" \
  -H "x-api-key: any-key" \
  -d '{
    "model": "claude-3-5-sonnet-20241022",
    "max_tokens": 1024,
    "messages": [
      {"role": "user", "content": "Hello!"}
    ]
  }'
```

### Streaming

```bash
curl http://localhost:8080/v1/messages \
  -H "Content-Type: application/json" \
  -H "x-api-key: any-key" \
  -d '{
    "model": "claude-3-5-sonnet-20241022",
    "max_tokens": 1024,
    "stream": true,
    "messages": [
      {"role": "user", "content": "Count to 5"}
    ]
  }'
```

## Metrics

Access `/metrics` for request statistics:

```json
{
  "requests": {
    "total": 100,
    "successful": 98,
    "errors": 2,
    "streaming": 75,
    "tool_calls": 15,
    "success_rate": "98.00%"
  },
  "performance": {
    "avg_latency_ms": "523.50",
    "requests_per_sec": "2.34"
  },
  "uptime": "5m30s"
}
```

## Docker

### Build and Run

```bash
# Build Docker image
make docker

# Run container
make docker-run

# Stop container
make docker-stop
```

### Docker Compose

Create a `.env` file with your configuration:

```bash
PROVIDER=openai
OPENAI_API_KEY=sk-...
CLASP_DEFAULT_MODEL=gpt-4o
```

Then start the service:

```bash
docker-compose up -d
```

### Docker Environment Variables

All configuration is done through environment variables. See the Environment Variables section above.

## Development

```bash
# Build
make build

# Run tests
make test

# Build for all platforms
make build-all

# Build Docker image
make docker

# Format code
make fmt
```

## Debugging

Enable debug logging to troubleshoot issues:

```bash
# Via CLI flag
clasp -debug

# Via environment variable
CLASP_DEBUG=true clasp

# Log only requests or responses
CLASP_DEBUG_REQUESTS=true clasp
CLASP_DEBUG_RESPONSES=true clasp
```

Debug output includes:
- Incoming Anthropic requests
- Outgoing OpenAI requests
- Raw OpenAI responses
- Transformed Anthropic responses

## License

MIT License - see [LICENSE](LICENSE) for details.

## Contributing

Contributions are welcome! Please open an issue or submit a pull request on [GitHub](https://github.com/jedarden/CLASP).
