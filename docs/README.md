# CLASP API Documentation

This documentation provides a comprehensive reference for the different LLM provider APIs that CLASP translates between. The goal is to maintain a knowledge store ensuring uniform translation across vendors.

## Documentation Structure

### API Reference
- [Anthropic Messages API](./api-reference/anthropic-messages.md) - Claude's native API format
- [OpenAI Chat Completions API](./api-reference/openai-chat-completions.md) - Standard OpenAI format
- [OpenAI Responses API](./api-reference/openai-responses.md) - New stateful conversation API

### Translation Guides
- [Request Translation](./translation-guides/request-translation.md) - How requests are transformed
- [Response Translation](./translation-guides/response-translation.md) - How responses are mapped back
- [Tool Call Translation](./translation-guides/tool-calls.md) - Function/tool calling differences + testing
- [Streaming Translation](./translation-guides/streaming.md) - SSE event mapping

### Testing
- [Tool Call Test Script](../research/remote-devpod/test-clasp-tool-calling.sh) - Automated test for tool schema translation

### Examples
- [Basic Conversation](./examples/basic-conversation.md)
- [Tool Use Examples](./examples/tool-use.md)
- [Multimodal Examples](./examples/multimodal.md)

## Quick Reference

| Feature | Anthropic | OpenAI Chat | OpenAI Responses |
|---------|-----------|-------------|------------------|
| Endpoint | `/v1/messages` | `/v1/chat/completions` | `/v1/responses` |
| System Message | `system` field | First message role | `instructions` field |
| Tool Calls | `tool_use` blocks | `tool_calls` array | `function_call` items |
| Tool Results | `tool_result` blocks | Tool role message | `function_call_output` |
| Streaming | SSE events | SSE chunks | SSE chunks |
| Tool ID Prefix | `toolu_` | `call_` | `fc_` |

## Version Support

| Provider | API Version | Notes |
|----------|-------------|-------|
| Anthropic | 2023-06-01 | Messages API |
| OpenAI | v1 | Chat Completions |
| OpenAI | v1 | Responses (GPT-5+) |
