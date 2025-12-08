# Tool Call Translation Guide

This document explains how CLASP translates tool/function calls between Anthropic and OpenAI formats.

## Overview

Tool calling allows models to request external actions. Each API has different terminology and structures:

| Concept | Anthropic | OpenAI Chat | OpenAI Responses |
|---------|-----------|-------------|------------------|
| Tool definition | `tools[]` | `tools[]` | `tools[]` |
| Tool invocation | `tool_use` block | `tool_calls[]` | `function_call` item |
| Tool result | `tool_result` block | `tool` message | `function_call_output` item |
| ID prefix | `toolu_` | `call_` | `fc_` |

## Tool Definition Translation

### Anthropic Format (Input)

```json
{
  "tools": [
    {
      "name": "get_weather",
      "description": "Get current weather for a location",
      "input_schema": {
        "type": "object",
        "properties": {
          "location": {
            "type": "string",
            "description": "City name"
          },
          "units": {
            "type": "string",
            "enum": ["celsius", "fahrenheit"],
            "description": "Temperature units (optional, defaults to fahrenheit)"
          }
        },
        "required": ["location", "units"]
      }
    }
  ]
}
```

### OpenAI Chat Completions Format

```json
{
  "tools": [
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
            },
            "units": {
              "type": "string",
              "enum": ["celsius", "fahrenheit"],
              "description": "Temperature units (optional, defaults to fahrenheit)"
            }
          },
          "required": ["location"]
        },
        "strict": false
      }
    }
  ]
}
```

### OpenAI Responses API Format

```json
{
  "tools": [
    {
      "type": "function",
      "name": "get_weather",
      "description": "Get current weather for a location",
      "parameters": {
        "type": "object",
        "properties": {
          "location": {
            "type": "string",
            "description": "City name"
          },
          "units": {
            "type": "string",
            "enum": ["celsius", "fahrenheit"],
            "description": "Temperature units (optional, defaults to fahrenheit)"
          }
        },
        "required": ["location"]
      },
      "function": {
        "name": "get_weather",
        "description": "Get current weather for a location",
        "parameters": {...},
        "strict": false
      }
    }
  ]
}
```

## Required Array Filtering

### The Problem

Anthropic tool schemas often mark ALL parameters as `required`, even optional ones. With `strict: true`, OpenAI rejects calls missing any "required" parameter.

### CLASP Solution

1. **Set `strict: false`** - Allows lenient validation
2. **Filter required array** - Only truly required parameters

A parameter is considered truly required if:
- It appears in the original `required` array
- It has NO `default` value
- It is NOT `nullable: true`
- It is NOT a `boolean` type (booleans are almost always optional flags)
- Its description does NOT contain optional indicators:
  - `optional`, `(optional)`
  - `defaults to`, `if not specified`
  - `set to true to`, `set to false to`
  - `if provided`, `when provided`
  - `can be omitted`, `not required`

### Example Filtering

Input schema:
```json
{
  "properties": {
    "file_path": {
      "type": "string",
      "description": "The file to read"
    },
    "offset": {
      "type": "number",
      "description": "Line offset. Only provide if file is too large."
    },
    "limit": {
      "type": "number",
      "description": "Number of lines. Defaults to 100."
    }
  },
  "required": ["file_path", "offset", "limit"]
}
```

Output (after filtering):
```json
{
  "properties": {...},
  "required": ["file_path"]
}
```

## Tool Call Translation (Response)

### OpenAI Chat Completions → Anthropic

OpenAI response:
```json
{
  "choices": [{
    "message": {
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
  }]
}
```

Translated to Anthropic:
```json
{
  "content": [
    {
      "type": "tool_use",
      "id": "toolu_abc123",
      "name": "get_weather",
      "input": {"location": "San Francisco"}
    }
  ],
  "stop_reason": "tool_use"
}
```

### OpenAI Responses API → Anthropic

OpenAI Responses output:
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

Translated to Anthropic:
```json
{
  "content": [
    {
      "type": "tool_use",
      "id": "toolu_abc123",
      "name": "get_weather",
      "input": {"location": "San Francisco"}
    }
  ],
  "stop_reason": "tool_use"
}
```

## Tool Result Translation (Request)

### Anthropic → OpenAI Chat Completions

Anthropic request:
```json
{
  "messages": [
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
  ]
}
```

Translated to OpenAI:
```json
{
  "messages": [
    {
      "role": "tool",
      "tool_call_id": "call_abc123",
      "content": "72°F, sunny"
    }
  ]
}
```

### Anthropic → OpenAI Responses API

Anthropic request:
```json
{
  "messages": [
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
  ]
}
```

Translated to Responses API:
```json
{
  "input": [
    {
      "type": "function_call_output",
      "call_id": "fc_abc123",
      "output": "72°F, sunny"
    }
  ]
}
```

## Tool ID Translation

CLASP translates tool IDs between formats:

| From | To | Transformation |
|------|-----|----------------|
| `toolu_xyz` | `call_xyz` | Strip prefix, add `call_` |
| `toolu_xyz` | `fc_xyz` | Strip prefix, add `fc_` |
| `call_xyz` | `toolu_xyz` | Strip prefix, add `toolu_` |
| `call_xyz` | `fc_xyz` | Strip prefix, add `fc_` |
| `fc_xyz` | `toolu_xyz` | Strip prefix, add `toolu_` |

### Implementation

```go
func translateToolCallID(id string) string {
    if strings.HasPrefix(id, "call_") {
        return "fc_" + strings.TrimPrefix(id, "call_")
    }
    if strings.HasPrefix(id, "toolu_") {
        return "fc_" + strings.TrimPrefix(id, "toolu_")
    }
    if strings.HasPrefix(id, "fc_") {
        return id
    }
    return "fc_" + id
}
```

## Tool Choice Translation

### Anthropic → OpenAI

| Anthropic | OpenAI Chat | Responses API |
|-----------|-------------|---------------|
| `{"type": "auto"}` | `"auto"` | `"auto"` |
| `{"type": "any"}` | `"required"` | `"required"` |
| `{"type": "none"}` | `"none"` | `"none"` |
| `{"type": "tool", "name": "X"}` | `{"type": "function", "function": {"name": "X"}}` | Same |

## Streaming Tool Calls

### OpenAI Chat Completions Streaming

Tool calls stream in chunks:

```
data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_abc","function":{"name":"get_weather","arguments":""}}]}}]}
data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"loc"}}]}}]}
data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"ation\":"}}]}}]}
data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"SF\"}"}}]}}]}
```

CLASP accumulates these into complete tool calls before emitting Anthropic events.

### Anthropic Streaming Events

```
event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"toolu_abc","name":"get_weather","input":{}}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"location\":"}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"\"SF\"}"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}
```

## Common Issues

### 1. Missing Optional Parameters

**Problem:** OpenAI rejects tool call because "required" parameter is missing.

**Solution:** CLASP filters the required array and sets `strict: false`.

### 2. Invalid Tool ID Format

**Problem:** Responses API rejects tool result with `call_` or `toolu_` ID.

**Solution:** CLASP translates all IDs to `fc_` prefix for Responses API.

### 3. Nested Arguments

**Problem:** Arguments might be double-encoded or pre-parsed.

**Solution:** CLASP normalizes to proper JSON string format.

### 4. Schema Format Types

**Problem:** OpenAI doesn't support all JSON Schema format types (e.g., `"format": "uri"`).

**Solution:** CLASP removes unsupported format types from schemas.

## Testing Tool Call Translation

### Automated Test Script

Location: `research/remote-devpod/test-clasp-tool-calling.sh`

This script validates that tool schema translation works correctly with real Claude Code + OpenAI integration.

### What It Tests

The test spawns 3 concurrent Task agents through CLASP, which exercises:
- Tool schema translation (Anthropic → OpenAI Responses API)
- Required array filtering (removes optional params)
- `strict: false` mode (lenient validation)
- Tool ID translation (`toolu_` → `fc_`)
- Function call/output round-trip

### Running the Test

```bash
cd research/remote-devpod
./test-clasp-tool-calling.sh
```

### Test Configuration

The script creates a CLASP profile at `~/.clasp/profiles/test-openai.json`:
- Provider: `openai`
- Model: `gpt-5.1-codex` (uses Responses API)
- Auto-launches Claude Code with skip-permissions
- Debug logging enabled

### Test Prompt

The prompt spawns 3 concurrent research agents:
```
Create a new folder in research/remote-devpod and spawn 3 agents at the same
time to conduct deep research...
```

This exercises the Task tool which has multiple optional parameters (`model`, `resume`, `run_in_background`) that would fail with `strict: true`.

### Pass Criteria

- CLASP proxy starts on port 8080
- Claude Code launches and receives prompt
- Task tool calls succeed (3 agents spawn)
- No "Invalid tool parameters" errors
- No "missing required field" errors

### Verification

```bash
# Check for errors
grep -i 'invalid\|error\|required' clasp-test-*.log

# Verify research files created
ls -la research/remote-devpod/*.md
```

### Monitoring

```bash
# Watch live execution
tmux attach -t clasp-test-*

# Watch CLASP logs
tail -f clasp-test-*.log
```
