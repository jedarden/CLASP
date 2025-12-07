# OpenAI Chat Completions API Reference

The Chat Completions API is OpenAI's standard conversational API. CLASP translates Anthropic requests to this format for most OpenAI models.

## Endpoint

```
POST /v1/chat/completions
```

## Request Format

### Top-Level Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `model` | string | Yes | Model identifier (e.g., `gpt-4o`) |
| `messages` | array | Yes | Conversation messages |
| `max_tokens` | integer | No | Maximum tokens to generate |
| `max_completion_tokens` | integer | No | Alias for max_tokens (newer models) |
| `temperature` | number | No | Sampling temperature (0-2) |
| `top_p` | number | No | Nucleus sampling parameter |
| `stream` | boolean | No | Enable streaming (default: false) |
| `tools` | array | No | Available functions |
| `tool_choice` | string/object | No | Tool selection behavior |
| `response_format` | object | No | Output format constraints |
| `reasoning_effort` | string | No | For o1/reasoning models |

### Message Object

```json
{
  "role": "system" | "user" | "assistant" | "tool",
  "content": "string" | ContentPart[],
  "name": "optional_name",
  "tool_calls": [...],      // For assistant messages
  "tool_call_id": "..."     // For tool messages
}
```

### Content Part Types

#### Text Part
```json
{
  "type": "text",
  "text": "Hello, how are you?"
}
```

#### Image URL Part
```json
{
  "type": "image_url",
  "image_url": {
    "url": "https://example.com/image.png",
    "detail": "auto"
  }
}
```

#### Base64 Image
```json
{
  "type": "image_url",
  "image_url": {
    "url": "data:image/png;base64,iVBORw0KGgo..."
  }
}
```

### System Message

Unlike Anthropic, system messages are part of the messages array:

```json
{
  "messages": [
    {
      "role": "system",
      "content": "You are a helpful assistant."
    },
    {
      "role": "user",
      "content": "Hello!"
    }
  ]
}
```

### Tool/Function Definition

```json
{
  "type": "function",
  "function": {
    "name": "get_weather",
    "description": "Get current weather for a location",
    "parameters": {
      "type": "object",
      "properties": {
        "location": {
          "type": "string",
          "description": "City name"
        }
      },
      "required": ["location"]
    },
    "strict": false
  }
}
```

### Strict Mode

When `strict: true`, OpenAI validates that ALL parameters in the `required` array are provided. This can cause issues when:
- Anthropic schemas mark optional parameters as required
- Claude Code omits truly optional parameters

**CLASP sets `strict: false`** to allow lenient validation.

### Tool Choice Options

| Value | Behavior |
|-------|----------|
| `"auto"` | Model decides whether to use tools |
| `"required"` | Model must use at least one tool |
| `"none"` | Model cannot use tools |
| `{"type": "function", "function": {"name": "X"}}` | Use specific tool |

## Response Format

### Non-Streaming Response

```json
{
  "id": "chatcmpl-abc123",
  "object": "chat.completion",
  "created": 1699000000,
  "model": "gpt-4o-2024-08-06",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "Hello! I'm doing well, thank you.",
        "refusal": null
      },
      "logprobs": null,
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 25,
    "completion_tokens": 15,
    "total_tokens": 40
  }
}
```

### Tool Call Response

```json
{
  "choices": [
    {
      "message": {
        "role": "assistant",
        "content": null,
        "tool_calls": [
          {
            "id": "call_abc123",
            "type": "function",
            "function": {
              "name": "get_weather",
              "arguments": "{\"location\":\"San Francisco\"}"
            }
          }
        ]
      },
      "finish_reason": "tool_calls"
    }
  ]
}
```

### Finish Reasons

| Value | Description | Anthropic Equivalent |
|-------|-------------|---------------------|
| `stop` | Natural completion | `end_turn` |
| `length` | Hit max_tokens | `max_tokens` |
| `tool_calls` | Model wants to call tools | `tool_use` |
| `content_filter` | Content filtered | (no equivalent) |

## Streaming Format

OpenAI uses SSE with `data:` prefixed JSON chunks:

### Chunk Structure

```json
{
  "id": "chatcmpl-abc123",
  "object": "chat.completion.chunk",
  "created": 1699000000,
  "model": "gpt-4o",
  "choices": [
    {
      "index": 0,
      "delta": {
        "role": "assistant",
        "content": "Hello"
      },
      "finish_reason": null
    }
  ]
}
```

### Stream Events

1. First chunk includes `role: "assistant"` in delta
2. Subsequent chunks have `content` deltas
3. Tool calls stream in `tool_calls` delta array
4. Final chunk has `finish_reason` set
5. Stream ends with `data: [DONE]`

### Tool Call Streaming

```json
// First tool chunk
{
  "choices": [{
    "delta": {
      "tool_calls": [{
        "index": 0,
        "id": "call_abc123",
        "type": "function",
        "function": {"name": "get_weather", "arguments": ""}
      }]
    }
  }]
}

// Argument chunks
{
  "choices": [{
    "delta": {
      "tool_calls": [{
        "index": 0,
        "function": {"arguments": "{\"loc"}
      }]
    }
  }]
}
```

## Tool Results

Tool results are sent as messages with `role: "tool"`:

```json
{
  "role": "tool",
  "tool_call_id": "call_abc123",
  "content": "72°F, sunny"
}
```

## Error Format

```json
{
  "error": {
    "message": "Invalid API key",
    "type": "invalid_request_error",
    "param": null,
    "code": "invalid_api_key"
  }
}
```

## Headers

### Request Headers

| Header | Required | Description |
|--------|----------|-------------|
| `Authorization` | Yes | `Bearer sk-...` |
| `Content-Type` | Yes | `application/json` |
| `OpenAI-Organization` | No | Organization ID |

### Response Headers

| Header | Description |
|--------|-------------|
| `x-request-id` | Unique request identifier |
| `x-ratelimit-limit-requests` | Rate limit ceiling |
| `x-ratelimit-remaining-requests` | Remaining requests |

## Key Differences from Anthropic

| Feature | Anthropic | OpenAI Chat |
|---------|-----------|-------------|
| System message | Separate `system` field | First message in array |
| Tool results | `tool_result` content block | `tool` role message |
| Tool IDs | `toolu_` prefix | `call_` prefix |
| Max tokens | `max_tokens` (required) | `max_tokens` (optional) |
| Stop reason | `stop_reason` field | `finish_reason` field |
| Content blocks | Array of typed blocks | String or parts array |
