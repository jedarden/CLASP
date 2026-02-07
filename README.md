# CLASP - Claude Language Agent Super Proxy

A high-performance Go proxy that translates Claude/Anthropic API calls to OpenAI-compatible endpoints, enabling Claude Code to work with any LLM provider.

## Features

- **Bundled Claude Code**: Automatically includes Claude Code as a dependency - single `npx clasp-ai` installs everything
- **Multi-Provider Support**: OpenAI, Azure OpenAI, OpenRouter (200+ models), and custom endpoints (Ollama, vLLM, LM Studio)
- **Full Protocol Translation**: Anthropic Messages API ↔ OpenAI Chat Completions API
- **SSE Streaming**: Real-time token streaming with state machine processing
- **Tool Calls**: Complete translation of tool_use/tool_result between formats
- **Connection Pooling**: Optimized HTTP transport with persistent connections
- **Retry Logic**: Exponential backoff for transient failures
- **Metrics Endpoint**: Request statistics and performance monitoring
- **API Key Authentication**: Secure the proxy with optional API key validation

## Installation

### Via npm (recommended)

```bash
# Install globally
npm install -g clasp-ai

# Or run directly with npx
npx clasp-ai
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
# Run with Docker (from GitHub Container Registry)
docker run -d -p 8080:8080 \
  -e OPENAI_API_KEY=sk-... \
  ghcr.io/jedarden/clasp:latest

# With specific version
docker run -d -p 8080:8080 \
  -e OPENAI_API_KEY=sk-... \
  ghcr.io/jedarden/clasp:0.24.8

# Or with docker-compose
docker-compose up -d
```

**Available Docker tags:**
- `latest` - Latest stable release
- `0.24` - Latest 0.24.x release
- `0.24.8` - Specific version

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
  -port <port>           Port to listen on (default: 8080)
  -provider <name>       LLM provider: openai, azure, openrouter, custom
  -model <model>         Default model to use for all requests
  -debug                 Enable debug logging (full request/response)
  -rate-limit            Enable rate limiting
  -cache                 Enable response caching
  -cache-max-size <n>    Maximum cache entries (default: 1000)
  -cache-ttl <n>         Cache TTL in seconds (default: 3600)
  -multi-provider        Enable multi-provider tier routing
  -fallback              Enable fallback routing for auto-failover
  -auth                  Enable API key authentication
  -auth-api-key <key>    API key for authentication (required with -auth)
  -version               Show version information
  -help                  Show help message
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
| `CLASP_RATE_LIMIT` | Enable rate limiting | `false` |
| `CLASP_RATE_LIMIT_REQUESTS` | Requests per window | `60` |
| `CLASP_RATE_LIMIT_WINDOW` | Window in seconds | `60` |
| `CLASP_RATE_LIMIT_BURST` | Burst allowance | `10` |
| `CLASP_CACHE` | Enable response caching | `false` |
| `CLASP_CACHE_MAX_SIZE` | Maximum cache entries | `1000` |
| `CLASP_CACHE_TTL` | Cache TTL in seconds | `3600` |
| `CLASP_MULTI_PROVIDER` | Enable multi-provider routing | `false` |
| `CLASP_FALLBACK` | Enable fallback routing | `false` |
| `CLASP_AUTH` | Enable API key authentication | `false` |
| `CLASP_AUTH_API_KEY` | Required API key for access | - |
| `CLASP_AUTH_ALLOW_ANONYMOUS_HEALTH` | Allow /health without auth | `true` |
| `CLASP_AUTH_ALLOW_ANONYMOUS_METRICS` | Allow /metrics without auth | `false` |

### Model Mapping

CLASP can automatically map Claude model tiers to your provider's models:

```bash
# Map Claude tiers to specific models
export CLASP_MODEL_OPUS=gpt-4o
export CLASP_MODEL_SONNET=gpt-4o-mini
export CLASP_MODEL_HAIKU=gpt-3.5-turbo
```

### Multi-Provider Routing

Route different Claude model tiers to different LLM providers for cost optimization:

```bash
# Enable multi-provider routing
export CLASP_MULTI_PROVIDER=true

# Route Opus tier to OpenAI (premium)
export CLASP_OPUS_PROVIDER=openai
export CLASP_OPUS_MODEL=gpt-4o
export CLASP_OPUS_API_KEY=sk-...  # Optional, inherits from OPENAI_API_KEY

# Route Sonnet tier to OpenRouter (cost-effective)
export CLASP_SONNET_PROVIDER=openrouter
export CLASP_SONNET_MODEL=anthropic/claude-3-sonnet
export CLASP_SONNET_API_KEY=sk-or-...  # Optional, inherits from OPENROUTER_API_KEY

# Route Haiku tier to local Ollama (free)
export CLASP_HAIKU_PROVIDER=custom
export CLASP_HAIKU_MODEL=llama3.1
export CLASP_HAIKU_BASE_URL=http://localhost:11434/v1

clasp -multi-provider
```

**Multi-Provider Environment Variables:**

| Variable | Description |
|----------|-------------|
| `CLASP_MULTI_PROVIDER` | Enable multi-provider routing (`true`/`1`) |
| `CLASP_{TIER}_PROVIDER` | Provider for tier: `openai`, `openrouter`, `custom` |
| `CLASP_{TIER}_MODEL` | Model name for the tier |
| `CLASP_{TIER}_API_KEY` | API key (optional, inherits from main config) |
| `CLASP_{TIER}_BASE_URL` | Base URL (optional, uses provider default) |

Where `{TIER}` is `OPUS`, `SONNET`, or `HAIKU`.

**Benefits:**
- **Cost Optimization**: Use expensive providers only for complex tasks
- **Latency Reduction**: Route simple requests to faster local models
- **Redundancy**: Mix cloud and local providers for reliability
- **A/B Testing**: Compare different models across tiers

## API Endpoints

| Endpoint | Description |
|----------|-------------|
| `POST /v1/messages` | Anthropic Messages API (translated) |
| `GET /health` | Health check |
| `GET /metrics` | Request statistics (JSON) |
| `GET /metrics/prometheus` | Prometheus metrics |
| `GET /` | Server info |

## Supported Tools

CLASP supports all Claude Code 2.1.34+ tools with full parameter validation and OpenAI-compatible schema transformation. See [docs/api-reference/claude-code-tools.md](docs/api-reference/claude-code-tools.md) for the complete tool reference including:

- **File Operations**: Read, Write, Edit, Glob, Grep
- **Command Execution**: Bash with background support
- **Web Operations**: WebSearch, WebFetch
- **Agent Orchestration**: Task, TaskOutput, TaskStop (with model/resume/max_turns parameters)
- **Interactive Features**: AskUserQuestion, ExitPlanMode
- **Specialized Tools**: NotebookEdit (Jupyter), LSP (code intelligence), Skill invocation
- **Task Management**: TaskCreate, TaskGet, TaskUpdate, TaskList (CLI-only)

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

## Response Caching

CLASP can cache responses to reduce API costs and improve latency for repeated requests:

```bash
# Enable caching with defaults (1000 entries, 1 hour TTL)
clasp -cache

# Custom cache settings
clasp -cache -cache-max-size 500 -cache-ttl 1800

# Via environment
CLASP_CACHE=true CLASP_CACHE_MAX_SIZE=500 clasp
```

**Caching behavior:**
- Only non-streaming requests are cached
- Requests with `temperature > 0` are not cached (non-deterministic)
- Cache uses LRU (Least Recently Used) eviction when full
- Cache entries expire after TTL (time-to-live)
- Response headers include `X-CLASP-Cache: HIT` or `X-CLASP-Cache: MISS`

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
  "cache": {
    "enabled": true,
    "size": 42,
    "max_size": 1000,
    "hits": 156,
    "misses": 44,
    "hit_rate": "78.00%"
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

## Authentication

Secure your CLASP proxy with API key authentication to control access:

```bash
# Enable authentication with CLI flags
clasp -auth -auth-api-key "my-secret-key"

# Or via environment variables
CLASP_AUTH=true CLASP_AUTH_API_KEY="my-secret-key" clasp
```

### Providing the API Key

Clients can provide the API key in two ways:

```bash
# Via x-api-key header
curl http://localhost:8080/v1/messages \
  -H "x-api-key: my-secret-key" \
  -H "Content-Type: application/json" \
  -d '{"model": "claude-3-5-sonnet-20241022", ...}'

# Via Authorization header (Bearer token)
curl http://localhost:8080/v1/messages \
  -H "Authorization: Bearer my-secret-key" \
  -H "Content-Type: application/json" \
  -d '{"model": "claude-3-5-sonnet-20241022", ...}'
```

### Authentication Options

| Variable | Description | Default |
|----------|-------------|---------|
| `CLASP_AUTH` | Enable authentication | `false` |
| `CLASP_AUTH_API_KEY` | Required API key | - |
| `CLASP_AUTH_ALLOW_ANONYMOUS_HEALTH` | Allow /health without auth | `true` |
| `CLASP_AUTH_ALLOW_ANONYMOUS_METRICS` | Allow /metrics without auth | `false` |

### Endpoint Access with Authentication Enabled

| Endpoint | Default Access |
|----------|----------------|
| `/` | Always accessible |
| `/health` | Anonymous by default |
| `/metrics` | Requires auth by default |
| `/metrics/prometheus` | Requires auth by default |
| `/v1/messages` | Requires auth |

### Using with Claude Code

When authentication is enabled, set both the base URL and API key:

```bash
# Start CLASP with auth
OPENAI_API_KEY=sk-... clasp -auth -auth-api-key "proxy-key"

# Use with Claude Code (the proxy key is passed as the Anthropic key)
ANTHROPIC_BASE_URL=http://localhost:8080 ANTHROPIC_API_KEY=proxy-key claude
```

## Request Queuing

Queue requests during provider outages for automatic retry:

```bash
# Enable request queuing
clasp -queue

# Custom queue settings
clasp -queue -queue-max-size 200 -queue-max-wait 60

# Via environment
CLASP_QUEUE=true CLASP_QUEUE_MAX_SIZE=200 clasp
```

### Queue Options

| Variable | Description | Default |
|----------|-------------|---------|
| `CLASP_QUEUE` | Enable request queuing | `false` |
| `CLASP_QUEUE_MAX_SIZE` | Maximum queued requests | `100` |
| `CLASP_QUEUE_MAX_WAIT` | Queue timeout in seconds | `30` |
| `CLASP_QUEUE_RETRY_DELAY` | Retry delay in milliseconds | `1000` |
| `CLASP_QUEUE_MAX_RETRIES` | Maximum retries per request | `3` |

## Circuit Breaker

Prevent cascade failures with circuit breaker pattern:

```bash
# Enable circuit breaker
clasp -circuit-breaker

# Custom circuit breaker settings
clasp -circuit-breaker -cb-threshold 10 -cb-recovery 3 -cb-timeout 60

# Via environment
CLASP_CIRCUIT_BREAKER=true clasp
```

### Circuit Breaker Options

| Variable | Description | Default |
|----------|-------------|---------|
| `CLASP_CIRCUIT_BREAKER` | Enable circuit breaker | `false` |
| `CLASP_CIRCUIT_BREAKER_THRESHOLD` | Failures before opening circuit | `5` |
| `CLASP_CIRCUIT_BREAKER_RECOVERY` | Successes to close circuit | `2` |
| `CLASP_CIRCUIT_BREAKER_TIMEOUT` | Timeout in seconds before retry | `30` |

### Circuit Breaker States

- **Closed**: Normal operation, requests pass through
- **Open**: Circuit tripped, requests fail fast with 503
- **Half-Open**: Testing if service recovered, limited requests allowed

### Maximum Resilience Configuration

For production deployments requiring maximum resilience:

```bash
# Enable queue + circuit breaker + fallback
OPENAI_API_KEY=sk-xxx OPENROUTER_API_KEY=sk-or-xxx \
  clasp -queue -circuit-breaker -fallback \
    -queue-max-size 200 \
    -cb-threshold 5 \
    -cb-timeout 30
```

## License

MIT License - see [LICENSE](LICENSE) for details.

## Contributing

Contributions are welcome! Please open an issue or submit a pull request on [GitHub](https://github.com/jedarden/CLASP).
