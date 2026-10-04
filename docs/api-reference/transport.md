# Upstream transport, pooling, and retries

CLASP uses one `http.Client` and one `http.Transport` per proxy handler. The
transport is shared by all requests handled by that proxy, so completed
requests to the same provider can reuse idle TCP/TLS connections.

## Connection pool and timeouts

The transport defaults are:

| Setting | Value | Meaning |
| --- | ---: | --- |
| Dial timeout | 30s | Maximum time to establish a TCP connection |
| TCP keep-alive | 30s | Keep-alive period for an established connection |
| Maximum idle connections | 100 | Pool-wide idle connection limit |
| Maximum idle connections per host | 100 | Per-provider-host idle connection limit |
| Idle connection timeout | 90s | When an unused pooled connection is removed |
| TLS handshake timeout | 10s | Maximum time for a TLS handshake |
| Upstream request timeout | 300s | Default maximum for the complete upstream exchange |

`CLASP_HTTP_TIMEOUT` (or `http_client.timeout_sec` in the YAML configuration)
changes the upstream request timeout. The `http.Client` timeout includes waiting
for response headers and reading the response body, including an SSE stream. A
streaming request therefore needs a timeout long enough for the entire stream,
not just its first token.

The same client is also used by provider health checks. The pool is not shared
between separate CLASP processes.

## Retry policy

Retries are deliberately bounded and apply only to replayable, non-streaming
requests. Each request gets three total attempts: the initial attempt plus at
most two retries. The delay before retrying is exponential and deterministic:

| Retry | Delay |
| ---: | ---: |
| 1 | 500ms |
| 2 | 1s |

CLASP retries these upstream status codes:

- `408 Request Timeout`
- `425 Too Early`
- `429 Too Many Requests`
- `500 Internal Server Error`
- `502 Bad Gateway`
- `503 Service Unavailable`
- `504 Gateway Timeout`
- `529` provider overload

Network transport errors (`net.Error`) and unexpected EOF are also retryable
when the request is replayable. Caller cancellation and caller deadlines stop
the retry loop immediately. Authentication, validation, permission, not-found,
payload-size, and other non-transient HTTP responses are returned without a
retry. After the final retryable HTTP response, CLASP returns that response to
the normal fallback/error path rather than hiding its status as a generic
client error.

The retry count is fixed; the queue's `max_retries` setting controls queue
admission behavior and does not change this upstream transport policy.

### Replay safety

CLASP creates a fresh request body for each permitted retry. A request marked
non-replayable is sent exactly once, even when the provider returns a transient
status or a network error. This prevents a retry from duplicating work whose
side effects cannot safely be repeated.

There is an unavoidable ambiguity when a connection fails after the provider
has received a request but before CLASP receives a response. For that reason,
automatic retries are a best-effort availability feature for replayable work,
not an end-to-end exactly-once guarantee. Configure provider-side idempotency
keys when the provider supports them and the operation has externally visible
side effects.

## Cancellation and streaming

The incoming request context is attached to every upstream request. A client
disconnect, server cancellation, or deadline cancels the in-flight provider
request and any pending backoff timer. CLASP does not start another attempt
after cancellation.

Streaming requests are non-replayable and are never automatically retried.
This applies even if the first upstream response has a retryable status, since
reissuing a generation could duplicate provider work or produce two streams.
Once a successful SSE response starts, its HTTP status cannot be changed; an
upstream read/parse/timeout failure is emitted as an Anthropic `error` SSE event
and the stream ends. The response body is closed after completion so a fully
consumed non-streaming response can return its connection to the pool.
