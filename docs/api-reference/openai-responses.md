# OpenAI Responses API Reference

The Responses API is OpenAI's newer stateful conversation API, designed for GPT-5 and future models. It differs significantly from Chat Completions in structure and capabilities.

## Endpoint

```
POST /v1/responses
```

## When to Use Responses API

CLASP uses the Responses API for:
- GPT-5 models (`gpt-5`, `gpt-5-mini`)
- Models requiring the `/v1/responses` endpoint
- Stateful conversation management with `previous_response_id`

## Request Format

### Top-Level Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `model` | string | Yes | Model identifier |
| `input` | array | Yes | Input items (messages, tool calls, etc.) |
| `instructions` | string | No | System instructions (replaces system message) |
| `max_output_tokens` | integer | No | Maximum tokens (minimum: 16) |
| `temperature` | number | No | Sampling temperature |
| `top_p` | number | No | Nucleus sampling |
| `stream` | boolean | No | Enable streaming |
| `tools` | array | No | Available functions |
| `tool_choice` | string/object | No | Tool selection behavior |
| `previous_response_id` | string | No | Continue from previous response |
| `reasoning` | object | No | Reasoning/thinking parameters |

### Input Item Types

The `input` array contains different item types:

#### Message Item
```json
{
  "type": "message",
  "role": "user" | "assistant",
  "content": "string" | ContentPart[]
}
```

#### Function Call Item (Assistant's tool use)
```json
{
  "type": "function_call",
  "id": "fc_abc123",
  "call_id": "fc_abc123",
  "name": "get_weather",
  "arguments": "{\"location\":\"San Francisco\"}"
}
```

#### Function Call Output Item (Tool result)
```json
{
  "type": "function_call_output",
  "call_id": "fc_abc123",
  "output": "72°F, sunny"
}
```

### Content Part Types

#### Input Text (User content)
```json
{
  "type": "input_text",
  "text": "Hello, how are you?"
}
```

**Note:** User content uses `input_text`, NOT `text`. This differs from Chat Completions.

#### Input Image
```json
{
  "type": "input_image",
  "image_url": {
    "url": "data:image/png;base64,..."
  }
}
```

#### Output Text (Assistant content in history)
```json
{
  "type": "output_text",
  "text": "I'm doing well!"
}
```

### Instructions (System Message)

Unlike Chat Completions, system instructions are a top-level field:

```json
{
  "model": "gpt-5",
  "instructions": "You are a helpful assistant.",
  "input": [
    {
      "type": "message",
      "role": "user",
      "content": "Hello!"
    }
  ]
}
```

### Tool Definition

Responses API uses a flattened structure with fields at top level:

```json
{
  "type": "function",
  "name": "get_weather",
  "description": "Get current weather",
  "parameters": {
    "type": "object",
    "properties": {
      "location": {"type": "string"}
    },
    "required": ["location"]
  }
}
```

**Important:** The `function` wrapper is optional but supported for compatibility:

```json
{
  "type": "function",
  "name": "get_weather",
  "description": "Get current weather",
  "parameters": {...},
  "function": {
    "name": "get_weather",
    "description": "Get current weather",
    "parameters": {...},
    "strict": false
  }
}
```

### Tool ID Format

**Critical:** Responses API requires `fc_` prefix for tool call IDs:

| Format | Prefix | Example |
|--------|--------|---------|
| Anthropic | `toolu_` | `toolu_01abc123` |
| Chat Completions | `call_` | `call_abc123` |
| Responses API | `fc_` | `fc_abc123` |

CLASP automatically translates IDs:
- `toolu_xyz` → `fc_xyz`
- `call_xyz` → `fc_xyz`

### Reasoning Parameters

For extended thinking/reasoning models:

```json
{
  "reasoning": {
    "effort": "low" | "medium" | "high"
  }
}
```

CLASP maps Anthropic's `budget_tokens`:
- `< 4000` → `"low"`
- `4000-16000` → `"medium"`
- `> 16000` → `"high"`

## Response Format

### Non-Streaming Response

```json
{
  "id": "resp_abc123",
  "object": "response",
  "created_at": 1699000000,
  "model": "gpt-5",
  "output": [
    {
      "type": "message",
      "role": "assistant",
      "content": [
        {
          "type": "output_text",
          "text": "Hello! I'm doing well."
        }
      ]
    }
  ],
  "usage": {
    "input_tokens": 25,
    "output_tokens": 15,
    "total_tokens": 40
  },
  "status": "completed"
}
```

### Tool Call Response

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
  ],
  "status": "completed"
}
```

### Response Status

| Status | Description |
|--------|-------------|
| `completed` | Successfully finished |
| `failed` | Error occurred |
| `cancelled` | Request was cancelled |
| `incomplete` | Partial completion |

## Streaming Format

Responses API streaming uses different event structure:

### Stream Events

```
event: response.created
data: {"type":"response.created","response":{...}}

event: response.output_item.added
data: {"type":"response.output_item.added","item":{...}}

event: response.content_part.added
data: {"type":"response.content_part.added","part":{...}}

event: response.output_text.delta
data: {"type":"response.output_text.delta","delta":"Hello"}

event: response.output_text.done
data: {"type":"response.output_text.done","text":"Hello! How can I help?"}

event: response.output_item.done
data: {"type":"response.output_item.done","item":{...}}

event: response.completed
data: {"type":"response.completed","response":{...}}
```

## Stateful Conversations

The Responses API supports conversation continuity:

```json
{
  "model": "gpt-5",
  "previous_response_id": "resp_abc123",
  "input": [
    {
      "type": "message",
      "role": "user",
      "content": "What about tomorrow?"
    }
  ]
}
```

## Key Differences from Chat Completions

| Feature | Chat Completions | Responses API |
|---------|-----------------|---------------|
| Endpoint | `/v1/chat/completions` | `/v1/responses` |
| System message | `messages[0].role: "system"` | `instructions` field |
| User content type | `text` | `input_text` |
| Tool calls | In message `tool_calls` array | Separate `function_call` items |
| Tool results | `tool` role message | `function_call_output` item |
| Tool IDs | `call_` prefix | `fc_` prefix |
| Min tokens | No minimum | 16 minimum |
| Reasoning | `reasoning_effort` top-level | `reasoning.effort` nested |
| State | Stateless | `previous_response_id` |

## CLASP Translation Notes

### Request Translation

1. `system` → `instructions` (with identity filtering)
2. Messages → Input items with proper types
3. `tool_use` blocks → `function_call` items
4. `tool_result` blocks → `function_call_output` items
5. Tool IDs: `toolu_`/`call_` → `fc_`
6. `max_tokens` → `max_output_tokens` (min 16)
7. `thinking.budget_tokens` → `reasoning.effort`

### Response Translation

1. Output items → Content blocks
2. `function_call` → `tool_use` blocks
3. Tool IDs: `fc_` → `toolu_` (for Anthropic format)
4. `status: "completed"` → `stop_reason: "end_turn"`

## Error Handling

Responses API errors follow similar format to Chat Completions:

```json
{
  "error": {
    "message": "Invalid parameter",
    "type": "invalid_request_error",
    "code": "invalid_parameter"
  }
}
```

## Headers

Same as Chat Completions API:

| Header | Required | Description |
|--------|----------|-------------|
| `Authorization` | Yes | `Bearer sk-...` |
| `Content-Type` | Yes | `application/json` |
