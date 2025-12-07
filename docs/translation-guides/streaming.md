# Streaming Translation Guide

This document details how CLASP handles SSE (Server-Sent Events) streaming translation between Anthropic and OpenAI formats.

## Streaming Architecture

```
┌─────────────┐     ┌──────────────────┐     ┌─────────────┐
│   OpenAI    │────▶│  CLASP Stream    │────▶│   Claude    │
│   Chunks    │     │  Processor       │     │   Events    │
└─────────────┘     └──────────────────┘     └─────────────┘
                           │
                    ┌──────┴──────┐
                    │ State       │
                    │ Machine     │
                    └─────────────┘
```

## State Machine

CLASP uses a state machine to track streaming progress:

```
                    ┌─────────┐
                    │  IDLE   │
                    └────┬────┘
                         │ first chunk
                         ▼
                    ┌─────────┐
            ┌──────▶│ CONTENT │◀──────┐
            │       └────┬────┘       │
            │            │            │
   text delta│      tool_call   tool_call delta
            │            │            │
            │       ┌────▼────┐       │
            │       │TOOL_CALL│───────┘
            │       └────┬────┘
            │            │
            └────────────┤ finish_reason
                         ▼
                    ┌─────────┐
                    │  DONE   │
                    └─────────┘
```

### States

| State | Description |
|-------|-------------|
| `IDLE` | Initial state, waiting for first chunk |
| `CONTENT` | Processing text content deltas |
| `TOOL_CALL` | Processing tool call chunks |
| `DONE` | Stream complete |

## Event Translation

### Anthropic Event Types

| Event | Purpose |
|-------|---------|
| `message_start` | Initial message metadata |
| `ping` | Keep-alive (optional) |
| `content_block_start` | New content block begins |
| `content_block_delta` | Incremental content |
| `content_block_stop` | Content block complete |
| `message_delta` | Final message updates |
| `message_stop` | Stream complete |

### OpenAI Chunk Structure

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

## Translation Examples

### Text Content Streaming

**OpenAI Chunks:**
```
data: {"choices":[{"delta":{"role":"assistant"},"finish_reason":null}]}
data: {"choices":[{"delta":{"content":"Hello"},"finish_reason":null}]}
data: {"choices":[{"delta":{"content":" there"},"finish_reason":null}]}
data: {"choices":[{"delta":{"content":"!"},"finish_reason":null}]}
data: {"choices":[{"delta":{},"finish_reason":"stop"}]}
data: [DONE]
```

**CLASP Output:**
```
event: message_start
data: {"type":"message_start","message":{"id":"msg_abc","type":"message","role":"assistant","content":[],"model":"claude-sonnet-4-20250514","stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":0,"output_tokens":0}}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":" there"}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"!"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"output_tokens":3}}

event: message_stop
data: {"type":"message_stop"}
```

### Tool Call Streaming

**OpenAI Chunks:**
```
data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_abc","type":"function","function":{"name":"read_file","arguments":""}}]},"finish_reason":null}]}
data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"file"}}]},"finish_reason":null}]}
data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"_path\":\""}}]},"finish_reason":null}]}
data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"test.txt"}}]},"finish_reason":null}]}
data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"}"}}]},"finish_reason":null}]}
data: {"choices":[{"delta":{},"finish_reason":"tool_calls"}]}
data: [DONE]
```

**CLASP Output:**
```
event: message_start
data: {"type":"message_start","message":{"id":"msg_abc","type":"message","role":"assistant","content":[],"model":"claude-sonnet-4-20250514","stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":0,"output_tokens":0}}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"toolu_abc","name":"read_file","input":{}}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"file"}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"_path\":\""}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"test.txt"}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"\"}"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"tool_use","stop_sequence":null},"usage":{"output_tokens":15}}

event: message_stop
data: {"type":"message_stop"}
```

### Mixed Content (Text + Tools)

When a response contains both text and tool calls:

**OpenAI Chunks:**
```
data: {"choices":[{"delta":{"role":"assistant","content":"Let me read that file."},"finish_reason":null}]}
data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_abc","type":"function","function":{"name":"read_file","arguments":"{}"}}]},"finish_reason":null}]}
data: {"choices":[{"delta":{},"finish_reason":"tool_calls"}]}
```

**CLASP Output:**
```
event: message_start
data: {"type":"message_start",...}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Let me read that file."}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: content_block_start
data: {"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"toolu_abc","name":"read_file","input":{}}}

event: content_block_delta
data: {"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{}"}}

event: content_block_stop
data: {"type":"content_block_stop","index":1}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"tool_use"},...}

event: message_stop
data: {"type":"message_stop"}
```

## Buffering Strategy

### Tool Call Argument Accumulation

OpenAI streams tool call arguments in small chunks. CLASP:
1. Buffers argument fragments
2. Emits accumulated content as `input_json_delta` events
3. Validates JSON completeness at end

```go
type StreamState struct {
    State          StreamStateType
    ContentIndex   int
    ToolCalls      map[int]*ToolCallAccumulator
    CurrentToolIdx int
}

type ToolCallAccumulator struct {
    ID        string
    Name      string
    Arguments strings.Builder
}
```

### Content Batching

For efficiency, CLASP may batch small content deltas:
- Minimum batch size: 1 character
- Maximum latency: immediate flush
- No artificial delays

## Error Handling

### Stream Interruption

If the upstream connection fails:
1. Emit any buffered content
2. Emit `content_block_stop` for open blocks
3. Emit `message_delta` with error info
4. Emit `message_stop`

### Malformed Chunks

If a chunk fails to parse:
1. Log the error
2. Skip the malformed chunk
3. Continue processing

### Timeout Handling

CLASP implements stream timeouts:
- Chunk timeout: 30 seconds
- Total timeout: 10 minutes (configurable)

## Performance Considerations

### Latency

- First byte latency: ~50ms overhead
- Per-chunk overhead: ~1ms
- Event formatting: ~100μs

### Memory

- State machine: ~1KB per stream
- Tool call buffer: Dynamic, typically <10KB
- No full response buffering

### Concurrency

Each stream is handled independently with its own state machine, allowing concurrent streams without interference.

## Responses API Streaming

The Responses API has different streaming events:

**Responses API Events:**
```
event: response.created
event: response.output_item.added
event: response.content_part.added
event: response.output_text.delta
event: response.output_text.done
event: response.output_item.done
event: response.completed
```

CLASP translates these to Anthropic events using the same state machine pattern.
