# Anthropic Messages API Reference

The Anthropic Messages API is Claude's native API format. CLASP receives requests in this format from Claude Code and translates them to OpenAI-compatible formats.

## Endpoint

```
POST /v1/messages
```

## Request Format

### Top-Level Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `model` | string | Yes | Model identifier (e.g., `claude-sonnet-4-20250514`) |
| `max_tokens` | integer | Yes | Maximum tokens to generate |
| `messages` | array | Yes | Conversation messages |
| `system` | string/array | No | System prompt |
| `stream` | boolean | No | Enable streaming (default: false) |
| `temperature` | number | No | Sampling temperature (0-1) |
| `top_p` | number | No | Nucleus sampling parameter |
| `top_k` | integer | No | Top-k sampling parameter |
| `tools` | array | No | Available tools/functions |
| `tool_choice` | object | No | Tool selection behavior |
| `thinking` | object | No | Extended thinking parameters |

### Message Object

```json
{
  "role": "user" | "assistant",
  "content": "string" | ContentBlock[]
}
```

### Content Block Types

#### Text Block
```json
{
  "type": "text",
  "text": "Hello, how are you?"
}
```

#### Image Block
```json
{
  "type": "image",
  "source": {
    "type": "base64",
    "media_type": "image/png",
    "data": "base64-encoded-data"
  }
}
```

#### Tool Use Block (Assistant Response)
```json
{
  "type": "tool_use",
  "id": "toolu_01abc123",
  "name": "get_weather",
  "input": {
    "location": "San Francisco"
  }
}
```

#### Tool Result Block (User Message)
```json
{
  "type": "tool_result",
  "tool_use_id": "toolu_01abc123",
  "content": "72°F, sunny"
}
```

### System Message Formats

#### String Format
```json
{
  "system": "You are a helpful assistant."
}
```

#### Array Format (with cache control)
```json
{
  "system": [
    {
      "type": "text",
      "text": "You are a helpful assistant.",
      "cache_control": {"type": "ephemeral"}
    }
  ]
}
```

### Tool Definition

```json
{
  "name": "get_weather",
  "description": "Get current weather for a location",
  "input_schema": {
    "type": "object",
    "properties": {
      "location": {
        "type": "string",
        "description": "City name"
      }
    },
    "required": ["location"]
  }
}
```

### Tool Choice Options

| Type | Behavior |
|------|----------|
| `{"type": "auto"}` | Model decides whether to use tools |
| `{"type": "any"}` | Model must use at least one tool |
| `{"type": "none"}` | Model cannot use tools |
| `{"type": "tool", "name": "X"}` | Model must use specific tool |

### Thinking Parameters

```json
{
  "thinking": {
    "type": "enabled",
    "budget_tokens": 10000
  }
}
```

## Response Format

### Non-Streaming Response

```json
{
  "id": "msg_01XFDUDYJgAACzvnptvVoYEL",
  "type": "message",
  "role": "assistant",
  "content": [
    {
      "type": "text",
      "text": "Hello! I'm doing well, thank you for asking."
    }
  ],
  "model": "claude-sonnet-4-20250514",
  "stop_reason": "end_turn",
  "stop_sequence": null,
  "usage": {
    "input_tokens": 25,
    "output_tokens": 15
  }
}
```

### Stop Reasons

| Value | Description |
|-------|-------------|
| `end_turn` | Natural completion |
| `stop_sequence` | Hit a stop sequence |
| `max_tokens` | Reached max_tokens limit |
| `tool_use` | Model wants to use a tool |

## Streaming Format

Anthropic uses Server-Sent Events (SSE) with specific event types:

### Event Sequence

1. `message_start` - Initial message metadata
2. `content_block_start` - Start of content block
3. `content_block_delta` - Incremental content
4. `content_block_stop` - End of content block
5. `message_delta` - Final message updates
6. `message_stop` - Stream complete

### Event Examples

#### message_start
```
event: message_start
data: {"type":"message_start","message":{"id":"msg_01...","type":"message","role":"assistant","content":[],"model":"claude-sonnet-4-20250514","stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":25,"output_tokens":0}}}
```

#### content_block_start
```
event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}
```

#### content_block_delta
```
event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}
```

#### content_block_stop
```
event: content_block_stop
data: {"type":"content_block_stop","index":0}
```

#### message_delta
```
event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"output_tokens":15}}
```

#### message_stop
```
event: message_stop
data: {"type":"message_stop"}
```

## Error Format

```json
{
  "type": "error",
  "error": {
    "type": "invalid_request_error",
    "message": "max_tokens must be greater than 0"
  }
}
```

### Error Types

| Type | Description |
|------|-------------|
| `invalid_request_error` | Malformed request |
| `authentication_error` | Invalid API key |
| `permission_error` | Insufficient permissions |
| `not_found_error` | Resource not found |
| `rate_limit_error` | Too many requests |
| `api_error` | Internal server error |
| `overloaded_error` | Service overloaded |

## Headers

### Request Headers

| Header | Required | Description |
|--------|----------|-------------|
| `x-api-key` | Yes | Anthropic API key |
| `anthropic-version` | Yes | API version (e.g., `2023-06-01`) |
| `content-type` | Yes | `application/json` |
| `anthropic-beta` | No | Beta feature flags |

### Response Headers

| Header | Description |
|--------|-------------|
| `request-id` | Unique request identifier |
| `x-ratelimit-limit-requests` | Rate limit ceiling |
| `x-ratelimit-remaining-requests` | Remaining requests |
