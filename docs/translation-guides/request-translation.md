# Request Translation Guide

This document details how CLASP transforms incoming Anthropic Messages API requests into OpenAI-compatible formats.

## Translation Flow

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│  Claude Code    │────▶│     CLASP       │────▶│   OpenAI API    │
│  (Anthropic)    │     │  (Translator)   │     │ (Chat/Responses)│
└─────────────────┘     └─────────────────┘     └─────────────────┘
```

## Target API Selection

CLASP routes to different APIs based on model:

| Model Pattern | Target API | Endpoint |
|--------------|------------|----------|
| `gpt-5*` | Responses API | `/v1/responses` |
| `gpt-4*`, `gpt-3.5*` | Chat Completions | `/v1/chat/completions` |
| `o1*`, `o3*` | Chat Completions | `/v1/chat/completions` |
| Other | Chat Completions | `/v1/chat/completions` |

## Field Mapping

### Basic Fields

| Anthropic | Chat Completions | Responses API |
|-----------|-----------------|---------------|
| `model` | `model` (mapped) | `model` (mapped) |
| `max_tokens` | `max_tokens` | `max_output_tokens` (min 16) |
| `temperature` | `temperature` | `temperature` |
| `top_p` | `top_p` | `top_p` |
| `stream` | `stream` | `stream` |
| `system` | First message | `instructions` |

### Model Mapping

```go
var modelMap = map[string]string{
    "claude-opus-4-20250514":     "gpt-4o",
    "claude-sonnet-4-20250514":   "gpt-4o",
    "claude-3-5-sonnet-20241022": "gpt-4o",
    "claude-3-5-haiku-20241022":  "gpt-4o-mini",
    "claude-3-opus-20240229":     "gpt-4o",
    "claude-3-haiku-20240307":    "gpt-4o-mini",
}
```

## System Message Translation

### Anthropic Input

```json
{
  "system": "You are a helpful assistant named Claude.",
  "messages": [...]
}
```

### Chat Completions Output

```json
{
  "messages": [
    {
      "role": "system",
      "content": "You are a helpful assistant."
    },
    ...
  ]
}
```

### Responses API Output

```json
{
  "instructions": "You are a helpful assistant.",
  "input": [...]
}
```

### Identity Filtering

CLASP applies identity filtering to system messages, replacing Claude-specific references:

| Original | Replacement |
|----------|-------------|
| `Claude` | (model-appropriate name) |
| `Anthropic` | (provider name) |
| `I am Claude` | `I am an AI assistant` |

## Message Translation

### User Message with Text

**Anthropic:**
```json
{
  "role": "user",
  "content": "Hello, how are you?"
}
```

**Chat Completions:**
```json
{
  "role": "user",
  "content": "Hello, how are you?"
}
```

**Responses API:**
```json
{
  "type": "message",
  "role": "user",
  "content": "Hello, how are you?"
}
```

### User Message with Image

**Anthropic:**
```json
{
  "role": "user",
  "content": [
    {
      "type": "text",
      "text": "What's in this image?"
    },
    {
      "type": "image",
      "source": {
        "type": "base64",
        "media_type": "image/png",
        "data": "iVBORw0..."
      }
    }
  ]
}
```

**Chat Completions:**
```json
{
  "role": "user",
  "content": [
    {
      "type": "text",
      "text": "What's in this image?"
    },
    {
      "type": "image_url",
      "image_url": {
        "url": "data:image/png;base64,iVBORw0..."
      }
    }
  ]
}
```

**Responses API:**
```json
{
  "type": "message",
  "role": "user",
  "content": [
    {
      "type": "input_text",
      "text": "What's in this image?"
    },
    {
      "type": "input_image",
      "image_url": {
        "url": "data:image/png;base64,iVBORw0..."
      }
    }
  ]
}
```

### Assistant Message with Tool Use

**Anthropic:**
```json
{
  "role": "assistant",
  "content": [
    {
      "type": "text",
      "text": "Let me check the weather."
    },
    {
      "type": "tool_use",
      "id": "toolu_abc123",
      "name": "get_weather",
      "input": {"location": "San Francisco"}
    }
  ]
}
```

**Chat Completions:**
```json
{
  "role": "assistant",
  "content": "Let me check the weather.",
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
}
```

**Responses API:**
```json
[
  {
    "type": "message",
    "role": "assistant",
    "content": "Let me check the weather."
  },
  {
    "type": "function_call",
    "id": "fc_abc123",
    "call_id": "fc_abc123",
    "name": "get_weather",
    "arguments": "{\"location\":\"San Francisco\"}"
  }
]
```

### Tool Result

**Anthropic:**
```json
{
  "role": "user",
  "content": [
    {
      "type": "tool_result",
      "tool_use_id": "toolu_abc123",
      "content": "72°F, sunny"
    }
  ]
}
```

**Chat Completions:**
```json
{
  "role": "tool",
  "tool_call_id": "call_abc123",
  "content": "72°F, sunny"
}
```

**Responses API:**
```json
{
  "type": "function_call_output",
  "call_id": "fc_abc123",
  "output": "72°F, sunny"
}
```

## Thinking Parameters

### Anthropic Input

```json
{
  "thinking": {
    "type": "enabled",
    "budget_tokens": 10000
  }
}
```

### Chat Completions (o1 models)

```json
{
  "reasoning_effort": "medium"
}
```

### Responses API (GPT-5)

```json
{
  "reasoning": {
    "effort": "medium"
  }
}
```

### Budget to Effort Mapping

| budget_tokens | effort |
|---------------|--------|
| < 4000 | `low` |
| 4000 - 16000 | `medium` |
| > 16000 | `high` |

## Complete Example

### Anthropic Request

```json
{
  "model": "claude-sonnet-4-20250514",
  "max_tokens": 4096,
  "system": "You are a helpful assistant.",
  "messages": [
    {
      "role": "user",
      "content": "What's the weather in SF?"
    },
    {
      "role": "assistant",
      "content": [
        {
          "type": "tool_use",
          "id": "toolu_weather123",
          "name": "get_weather",
          "input": {"location": "San Francisco"}
        }
      ]
    },
    {
      "role": "user",
      "content": [
        {
          "type": "tool_result",
          "tool_use_id": "toolu_weather123",
          "content": "72°F, sunny"
        }
      ]
    }
  ],
  "tools": [
    {
      "name": "get_weather",
      "description": "Get weather",
      "input_schema": {
        "type": "object",
        "properties": {
          "location": {"type": "string"}
        },
        "required": ["location"]
      }
    }
  ],
  "stream": true
}
```

### Chat Completions Translation

```json
{
  "model": "gpt-4o",
  "max_tokens": 4096,
  "messages": [
    {
      "role": "system",
      "content": "You are a helpful assistant."
    },
    {
      "role": "user",
      "content": "What's the weather in SF?"
    },
    {
      "role": "assistant",
      "content": null,
      "tool_calls": [
        {
          "id": "call_weather123",
          "type": "function",
          "function": {
            "name": "get_weather",
            "arguments": "{\"location\":\"San Francisco\"}"
          }
        }
      ]
    },
    {
      "role": "tool",
      "tool_call_id": "call_weather123",
      "content": "72°F, sunny"
    }
  ],
  "tools": [
    {
      "type": "function",
      "function": {
        "name": "get_weather",
        "description": "Get weather",
        "parameters": {
          "type": "object",
          "properties": {
            "location": {"type": "string"}
          },
          "required": ["location"]
        },
        "strict": false
      }
    }
  ],
  "stream": true
}
```

### Responses API Translation

```json
{
  "model": "gpt-5",
  "max_output_tokens": 4096,
  "instructions": "You are a helpful assistant.",
  "input": [
    {
      "type": "message",
      "role": "user",
      "content": "What's the weather in SF?"
    },
    {
      "type": "function_call",
      "id": "fc_weather123",
      "call_id": "fc_weather123",
      "name": "get_weather",
      "arguments": "{\"location\":\"San Francisco\"}"
    },
    {
      "type": "function_call_output",
      "call_id": "fc_weather123",
      "output": "72°F, sunny"
    }
  ],
  "tools": [
    {
      "type": "function",
      "name": "get_weather",
      "description": "Get weather",
      "parameters": {
        "type": "object",
        "properties": {
          "location": {"type": "string"}
        },
        "required": ["location"]
      }
    }
  ],
  "stream": true
}
```

## Edge Cases

### Empty Content

If a message has empty content, CLASP preserves it as an empty string rather than omitting the field.

### Mixed Content Blocks

When user messages contain both text and tool results, CLASP:
1. Extracts text into a user message
2. Extracts tool results into separate items (Responses) or messages (Chat)

### Cache Control

Anthropic's `cache_control` fields are stripped as OpenAI doesn't support them.

### Beta Headers

Anthropic's `anthropic-beta` headers are not forwarded; equivalent OpenAI features are used where available.
