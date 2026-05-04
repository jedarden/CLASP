# CLASP Gap Analysis

This document identifies functional gaps between CLASP and other Claude Code proxy solutions (like [claude-code-proxy](https://github.com/1rgs/claude-code-proxy), [CCProxy](https://ccproxy.orchestre.dev/), [anthropic-proxy](https://github.com/maxnowack/anthropic-proxy)), and analyzes gpt-5.1-codex model support.

## Executive Summary

CLASP has robust support for GPT-5 series models via the Responses API but has several gaps compared to competitor proxies. OpenRouter **does support** gpt-5.1-codex models with standard API access.

## gpt-5.1-codex Support Status

### OpenAI Direct API
- **Status**: Available (November 2025)
- **Endpoint**: `/v1/responses` (Responses API)
- **Models**: `gpt-5.1-codex`, `gpt-5.1-codex-mini`, `gpt-5.1-codex-max`
- **Pricing**: $1.25/M input, $10/M output tokens
- **Context**: 400K tokens

### OpenRouter Support
- **Status**: ✅ Fully Supported
- **Model IDs**:
  - `openai/gpt-5.1-codex`
  - `openai/gpt-5.1-codex-mini`
  - `openai/gpt-5.1-codex-max`
- **API**: OpenRouter translates to OpenAI's Responses API
- **Source**: [OpenRouter GPT-5.1-Codex](https://openrouter.ai/openai/gpt-5.1-codex)

### CLASP Support
- **Status**: ✅ Supported (via Responses API translation)
- **Implementation**: `internal/translator/endpoint.go` lines 32-37
- **Detection**: Models starting with `gpt-5`, `gpt-5.1`, or `codex` route to `/v1/responses`

```go
var responsesModels = []string{
    "gpt-5",
    "gpt-5.1",
    "codex",
}
```

## Newer Model Support

### OpenAI o3 Reasoning Models
- **Status**: ✅ Fully Supported
- **Models**: `o3`, `o3-mini`
- **Implementation**: `internal/translator/request.go` - `isO1OrO3Model()`, `applyThinkingParameters()`
- **Features**:
  - `reasoning_effort` parameter support ("minimal", "low", "medium", "high")
  - `max_completion_tokens` preferred over `max_tokens`
  - 200K context window
  - Extended timeout handling (recommended: 600s+)

### Google Gemini 2.5
- **Status**: ✅ Fully Supported
- **Models**: `gemini-2.5-pro`, `gemini-2.5-flash`, `google/gemini-2.5-pro`, `google/gemini-2.5-flash`
- **Implementation**: `internal/translator/request.go` - `isGemini25Model()`, `applyThinkingParameters()`
- **Features**:
  - `thinking_config` with budget tokens (cap at 24K)
  - 2M token context window (Pro), 1M (Flash)
  - Native Gemini provider support

### xAI Grok Models
- **Status**: ✅ Fully Supported
- **Models**: `grok-3-beta`, `grok-3-mini-beta`, `grok-2`, `x-ai/grok-3-beta`, `x-ai/grok-3-mini-beta`, `x-ai/grok-2`
- **Implementation**: `internal/provider/grok.go`, `internal/translator/request.go` - `isGrokModel()`
- **Features**:
  - Direct xAI API integration
  - OpenAI-compatible translation
  - `reasoning_effort` parameter support (low/high for Grok 3 Mini)
  - 131K context window
  - Native Grok provider support
  - XML tool call format handling

## Known Functional Gaps

### 1. Responses API Limitations

#### Feature: Compaction/Multi-Window Context
- **What**: GPT-5.1-Codex-Max supports "compaction" for multi-million token sessions
- **Status**: ✅ Supported
- **Implementation**: CLASP implements Responses API `previous_response_id` chaining for multi-turn conversations
- **Configuration**:
  - Enable: `CLASP_COMPACTION=true`
  - Session TTL: `CLASP_SESSION_TIMEOUT=3600` (seconds, default: 3600)
- **How it works**:
  1. Session tracker stores `response_id` from each completed Responses API request
  2. Session key is derived from SHA-256 hash of (model + first user message content)
  3. On subsequent requests with the same session key, only new messages are sent
  4. `previous_response_id` is injected into the Responses API request
  5. The provider maintains context across turns, enabling multi-million token sessions
- **Metrics**: Compaction hits/misses tracked in `/metrics` endpoint under `compaction` section

#### Gap: Codex Workspace Integration
- **What**: Native Codex IDE/CLI integration features
- **Status**: ❌ Not available via API
- **Impact**: No access to Codex-specific agentic features
- **Note**: These features are GitHub Copilot specific

### 2. Missing Features Compared to Competitors

| Feature | CLASP | CCProxy | claude-code-proxy | anthropic-proxy |
|---------|-------|---------|-------------------|-----------------|
| LiteLLM Backend | ✅ | ✅ | ✅ | ❌ |
| Google Gemini | ✅ | ✅ | ✅ | ❌ |
| DeepSeek | ✅ | ✅ | ❌ | ❌ |
| xAI Grok | ✅ | ✅ | ❌ | ❌ |
| Alibaba Qwen | ✅ | ✅ | ❌ | ❌ |
| MiniMax | ✅ | ✅ | ❌ | ❌ |
| Ollama Local | ✅ | ✅ | ✅ | ❌ |
| Provider Prefix | ✅ | ✅ | ✅ | ❌ |
| Multi-Model Routing | ✅ | ✅ | ✅ | ❌ |
| Response Caching | ✅ | ❌ | ❌ | ❌ |
| Prompt Caching Simulation | ✅ | ❌ | ❌ | ❌ |
| Prometheus Metrics | ✅ | ❌ | ❌ | ❌ |
| Cost Tracking | ✅ | ❌ | ❌ | ❌ |
| Docker Support | ✅ | ✅ | ✅ | ❌ |
| NPM Package | ✅ | ❌ | ✅ | ❌ |

### 3. Tool Schema Handling (FIXED in recent update)

#### Previously: Strict Mode Issues
- **Problem**: OpenAI `strict: true` rejected tool calls with missing optional params
- **Fix**: Now sets `strict: false` and filters `required` array
- **Status**: ✅ Fixed

### 4. Stream Timeout Issues (FIXED in v0.49.0)

#### Gap: Long-Running Requests
- **What**: Extended thinking/reasoning can take 5+ minutes
- **Current**: Default 5-minute HTTP timeout
- **Status**: ✅ Fixed with auto-warnings and setup wizard prompts
- **Implementation**:
  - Startup warnings for codex/reasoning models with short timeouts
  - Setup wizard recommends extended timeouts when selecting reasoning models
  - `CLASP_HTTP_TIMEOUT_SEC` exposed in `/metrics` and statusline
- **Recommendation**: Set to 600+ seconds for codex models (auto-suggested in wizard)

### 5. Model Picker Limitations

#### Dynamic Model Discovery
- **What**: Real-time model listing from providers
- **Status**: ✅ Implemented in v0.44.11
- **Current Version**: v0.49.0
- **Implementation**:
  - `OpenAIProvider.ListModels()` - Fetches from `/v1/models`, filters to chat models
  - `OpenRouterProvider.ListModels()` - Fetches all models
  - `OpenRouterProvider.ListModelsWithInfo()` - Includes pricing, context length, provider
  - `OpenRouterProvider.GetChatModels()` - Filters out embedding/audio/image models
- **Note**: Static `chatCompletionsModels` list still used for model picker fallback

### 6. Anthropic Beta Features

#### Gap: Prompt Caching (Simulation Implemented)
- **What**: Anthropic's `cache_control` for token savings
- **Status**: ✅ Implemented via simulation
- **Native cache_control**: ❌ Stripped in translation (OpenAI doesn't support)
- **Simulation**: ✅ Fully implemented via `cache.PromptCache` (v0.47.0+)
- **How it works**:
  - Full-response caching keyed by request hash
  - Prefix-based LRU with TTL
  - Enable with: `CLASP_PROMPT_CACHE=true` (env var) or `prompt_cache.enabled: true` (config file)
  - Metrics tracked in `/metrics` endpoint under `prompt_cache` section
- **Impact**: Cannot leverage native Anthropic prompt caching, but simulation provides token savings for repeated requests with identical prefixes

#### Gap: Computer Use Tools
- **What**: Anthropic's computer use API
- **Status**: ❌ Not applicable (OpenAI doesn't support)
- **Note**: Platform-specific feature

## Provider-Specific Gaps

### OpenRouter Translation

| Feature | Status | Notes |
|---------|--------|-------|
| Model Mapping | ✅ | Uses `openai/` prefix |
| API Key Header | ✅ | `Authorization: Bearer` |
| X-Title Header | ✅ | Attribution support |
| Rate Limits | ⚠️ | Different from OpenAI |
| Responses API | ✅ | Proxies to OpenAI |

### Azure OpenAI

| Feature | Status | Notes |
|---------|--------|-------|
| Deployment Names | ✅ | Custom deployment support |
| API Version | ✅ | Configurable |
| Responses API | ⚠️ | Graceful error: Azure + gpt-5/codex returns clear 400 error |
| gpt-5 models | ⚠️ | Graceful error: Not available on Azure (yet) |

## Recommendations for gpt-5.1-codex Users

### 1. Configuration
```bash
# For OpenRouter
OPENROUTER_API_KEY=sk-or-... clasp -provider openrouter -model openai/gpt-5.1-codex

# For OpenAI Direct
OPENAI_API_KEY=sk-... clasp -provider openai -model gpt-5.1-codex
```

### 2. Timeout Settings
```bash
# Extended timeout for codex's long reasoning
CLASP_HTTP_TIMEOUT_SEC=900 clasp -model gpt-5.1-codex
```

### 3. Debug Mode
```bash
# Enable debug to troubleshoot Responses API translation
CLASP_DEBUG=true clasp -model gpt-5.1-codex
```

## Comparison with Other Proxies

### claude-code-proxy (1rgs)
- **Backend**: LiteLLM
- **Advantage**: Multi-provider via LiteLLM abstraction
- **Disadvantage**: No caching, metrics, or cost tracking
- **gpt-5.1-codex**: ✅ Via LiteLLM/OpenAI

### CCProxy
- **Backend**: Native implementation
- **Advantage**: 100+ LLM support, web interface
- **Disadvantage**: Closed source, SaaS-dependent
- **gpt-5.1-codex**: ✅ Supported

### anthropic-proxy (maxnowack)
- **Backend**: Direct OpenRouter
- **Advantage**: Simple, lightweight
- **Disadvantage**: OpenRouter-only, limited features
- **gpt-5.1-codex**: ✅ Via OpenRouter

### CLASP
- **Backend**: Native Go implementation
- **Advantage**: Full Responses API, metrics, caching, cost tracking, LiteLLM support, 10+ providers
- **Version**: v0.49.0
- **gpt-5.1-codex**: ✅ Native Responses API support

## Future Improvements

1. ~~**Add LiteLLM integration** - Would enable 100+ providers~~ ✅ Added in v0.49.0
2. ~~**Implement prompt caching simulation** - Cache full responses by request hash~~ ✅ Added in v0.47.0
3. ~~**Dynamic model discovery** - Query providers for available models~~ ✅ Added in v0.44.11
4. ~~**DeepSeek provider** - Direct DeepSeek support~~ ✅ Added in v0.38.0
5. ~~**Local model support** - Ollama/LM Studio integration~~ ✅ Added in v0.36.0
6. ~~**Gemini provider** - Direct Google Gemini support~~ ✅ Added in v0.37.0
7. ~~**Compaction support** - Multi-window context management~~ ✅ Added in v0.48.0
8. ~~**MCP Server Mode** - Add MCP server for tool integration~~ ✅ Added in v0.47.0
9. ~~**Stream timeout UX** - Auto-warn and recommend settings for codex/reasoning models~~ ✅ Added in v0.49.0 (bf-5puj)
10. ~~**xAI Grok provider** - Direct Grok support~~ ✅ Added in v0.47.0
11. ~~**Alibaba Qwen provider** - Direct Qwen support~~ ✅ Added in v0.47.0
12. ~~**MiniMax provider** - Direct MiniMax support~~ ✅ Added in v0.47.0

## Sources

- [GPT-5.1-Codex API (OpenRouter)](https://openrouter.ai/openai/gpt-5.1-codex)
- [GPT-5.1-Codex-Max System Card](https://openai.com/index/gpt-5-1-codex-max-system-card/)
- [OpenAI Responses API Documentation](https://platform.openai.com/docs/api-reference/responses)
- [CCProxy Documentation](https://ccproxy.orchestre.dev/)
- [claude-code-proxy (GitHub)](https://github.com/1rgs/claude-code-proxy)
- [anthropic-proxy (GitHub)](https://github.com/maxnowack/anthropic-proxy)
