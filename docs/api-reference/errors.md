# Anthropic-compatible error responses

CLASP exposes the Anthropic Messages API error envelope for every `/v1/messages`
request, regardless of whether the request is sent to OpenAI Chat Completions,
the OpenAI Responses API, a custom OpenAI-compatible provider, or Anthropic
passthrough.

Every non-streaming error is JSON with this shape:

```json
{
  "type": "error",
  "error": {
    "type": "invalid_request_error",
    "message": "..."
  }
}
```

## Status and error type contract

| Situation | HTTP status | `error.type` | Message |
| --- | ---: | --- | --- |
| Invalid client request or upstream validation failure | 400 | `invalid_request_error` | `The upstream provider rejected the request.` |
| Invalid HTTP method | 405 | `invalid_request_error` | `Method not allowed` |
| Missing or invalid CLASP/upstream credentials | 401 | `authentication_error` | A credential-specific message is returned. |
| Upstream permission failure | 403 | `permission_error` | `The upstream provider denied access to this request.` |
| Unknown model or upstream resource | 404 | `not_found_error` | `The requested model or upstream resource was not found.` |
| Request exceeds provider limit | 413 | `request_too_large` | `The request is too large for the upstream provider.` |
| CLASP or upstream rate limit | 429 | `rate_limit_error` | A retryable rate-limit message is returned. |
| Upstream service unavailable/overloaded | 529 | `overloaded_error` | `The upstream provider is temporarily overloaded.` |
| Other upstream HTTP, network, or malformed-response failure | 502 | `api_error` | A stable provider-failure message is returned. |
| Upstream request timeout | 504 | `api_error` | `The upstream provider request timed out.` |

Provider-specific response bodies are logged only after secret masking and are
not copied into the public error contract. This prevents OpenAI-compatible
providers from changing the client-visible schema or leaking internal details.

## Streaming errors

Once an SSE response has started, its HTTP status cannot be changed. CLASP
therefore emits the same envelope in an Anthropic `error` event:

```text
event: error
data: {"type":"error","error":{"type":"api_error","message":"The upstream provider stream failed."}}

```

Malformed Chat Completions and Responses API SSE events, upstream read errors,
and stream timeouts all use this event. The stream processor stops after the
error instead of silently skipping malformed provider data.
