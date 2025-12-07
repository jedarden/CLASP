# Response Translation Guide

This document details how CLASP transforms OpenAI responses back into Anthropic Messages API format.

## Translation Flow

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│   OpenAI API    │────▶│     CLASP       │────▶│  Claude Code    │
│ (Chat/Responses)│     │  (Translator)   │     │  (Anthropic)    │
└─────────────────┘     └─────────────────┘     └─────────────────┘
```

## Non-Streaming Response Translation

### Chat Completions → Anthropic

**OpenAI Response:**
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
        "content": "Hello! How can I help you today?"
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 25,
    "completion_tokens": 12,
    "total_tokens": 37
  }
}
```

**Anthropic Response:**
```json
{
  "id": "msg_chatcmpl-abc123",
  "type": "message",
  "role": "assistant",
  "content": [
    {
      "type": "text",
      "text": "Hello! How can I help you today?"
    }
  ],
  "model": "claude-sonnet-4-20250514",
  "stop_reason": "end_turn",
  "stop_sequence": null,
  "usage": {
    "input_tokens": 25,
    "output_tokens": 12
  }
}
```

### Responses API → Anthropic

**OpenAI Responses Output:**
```json
{
  "id": "resp_abc123",
  "object": "response",
  "model": "gpt-5",
  "output": [
    {
      "type": "message",
      "role": "assistant",
      "content": [
        {
          "type": "output_text",
          "text": "Hello! How can I help?"
        }
      ]
    }
  ],
  "usage": {
    "input_tokens": 25,
    "output_tokens": 10
  },
  "status": "completed"
}
```

**Anthropic Response:**
```json
{
  "id": "msg_resp_abc123",
  "type": "message",
  "role": "assistant",
  "content": [
    {
      "type": "text",
      "text": "Hello! How can I help?"
    }
  ],
  "model": "claude-sonnet-4-20250514",
  "stop_reason": "end_turn",
  "usage": {
    "input_tokens": 25,
    "output_tokens": 10
  }
}
```

## Field Mapping

### ID Translation

| OpenAI ID | Anthropic ID |
|-----------|-------------|
| `chatcmpl-abc123` | `msg_chatcmpl-abc123` |
| `resp_abc123` | `msg_resp_abc123` |

### Stop Reason Mapping

| OpenAI finish_reason | Anthropic stop_reason |
|---------------------|----------------------|
| `stop` | `end_turn` |
| `length` | `max_tokens` |
| `tool_calls` | `tool_use` |
| `content_filter` | `end_turn` |
| `null` (streaming) | `null` |

### Usage Mapping

| OpenAI | Anthropic |
|--------|-----------|
| `prompt_tokens` | `input_tokens` |
| `completion_tokens` | `output_tokens` |
| `total_tokens` | (not included) |

## Tool Call Response Translation

### Chat Completions Tool Calls → Anthropic

**OpenAI Response:**
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

**Anthropic Response:**
```json
{
  "content": [
    {
      "type": "tool_use",
      "id": "toolu_abc123",
      "name": "get_weather",
      "input": {
        "location": "San Francisco"
      }
    }
  ],
  "stop_reason": "tool_use"
}
```

### Responses API Function Calls → Anthropic

**OpenAI Responses Output:**
```json
{
  "output": [
    {
      "type": "function_call",
      "id": "fc_abc123",
      "call_id": "fc_abc123",
      "name": "get_weather",
      "arguments": "{\"location\":\"San Francisco\"}"
    }
  ]
}
```

**Anthropic Response:**
```json
{
  "content": [
    {
      "type": "tool_use",
      "id": "toolu_abc123",
      "name": "get_weather",
      "input": {
        "location": "San Francisco"
      }
    }
  ],
  "stop_reason": "tool_use"
}
```

### Tool ID Translation

| OpenAI Format | Anthropic Format |
|--------------|-----------------|
| `call_abc123` | `toolu_abc123` |
| `fc_abc123` | `toolu_abc123` |

## Streaming Response Translation

### SSE Event Mapping

CLASP transforms OpenAI streaming chunks into Anthropic's structured SSE events:

| OpenAI Event | Anthropic Event(s) |
|--------------|-------------------|
| First chunk | `message_start` |
| Content delta | `content_block_start` (first), `content_block_delta` |
| Tool call start | `content_block_start` (tool_use) |
| Tool call delta | `content_block_delta` (input_json_delta) |
| Finish chunk | `content_block_stop`, `message_delta`, `message_stop` |
| `[DONE]` | (already emitted message_stop) |

### Streaming Event Sequence

**OpenAI Chat Completions Stream:**
```
data: {"id":"chatcmpl-abc","choices":[{"delta":{"role":"assistant","content":""},"finish_reason":null}]}
data: {"id":"chatcmpl-abc","choices":[{"delta":{"content":"Hello"},"finish_reason":null}]}
data: {"id":"chatcmpl-abc","choices":[{"delta":{"content":"!"},"finish_reason":null}]}
data: {"id":"chatcmpl-abc","choices":[{"delta":{},"finish_reason":"stop"}]}
data: [DONE]
```

**CLASP Anthropic Stream:**
```
event: message_start
data: {"type":"message_start","message":{"id":"msg_chatcmpl-abc","type":"message","role":"assistant","content":[],"model":"claude-sonnet-4-20250514","stop_reason":null,"usage":{"input_tokens":0,"output_tokens":0}}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"!"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"output_tokens":2}}

event: message_stop
data: {"type":"message_stop"}
```

### Tool Call Streaming

**OpenAI Tool Call Stream:**
```
data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_abc","function":{"name":"get_weather","arguments":""}}]}}]}
data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"loc"}}]}}]}
data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"ation\":"}}]}}]}
data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"SF\"}"}}]}}]}
data: {"choices":[{"delta":{},"finish_reason":"tool_calls"}]}
data: [DONE]
```

**CLASP Anthropic Tool Stream:**
```
event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"toolu_abc","name":"get_weather","input":{}}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"location\":"}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"\"SF\"}"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":15}}

event: message_stop
data: {"type":"message_stop"}
```

## Error Response Translation

### OpenAI Error → Anthropic Error

**OpenAI Error:**
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

**Anthropic Error:**
```json
{
  "type": "error",
  "error": {
    "type": "authentication_error",
    "message": "Invalid API key"
  }
}
```

### Error Type Mapping

| OpenAI Type | Anthropic Type |
|-------------|---------------|
| `invalid_request_error` | `invalid_request_error` |
| `authentication_error` | `authentication_error` |
| `rate_limit_error` | `rate_limit_error` |
| `server_error` | `api_error` |
| `insufficient_quota` | `permission_error` |

## Model Name Translation

When returning responses, CLASP maps back to the original Anthropic model name:

| Target Model | Response Model |
|-------------|---------------|
| `gpt-4o` | Original request model |
| `gpt-5` | Original request model |

The `model` field in responses reflects the model that was requested, not the underlying provider model.

## Edge Cases

### Empty Content

If OpenAI returns `content: null` with tool calls, CLASP omits the text content block entirely.

### Multiple Tool Calls

Multiple tool calls are translated to multiple `tool_use` content blocks with sequential indices.

### Partial Responses

If streaming is interrupted, CLASP ensures proper event sequence termination with `content_block_stop` and `message_stop`.

### Unicode and Escaping

CLASP preserves Unicode characters and proper JSON escaping through translation.
