# Metrics endpoint

CLASP exposes process-local request metrics in JSON and Prometheus exposition
formats. Both endpoints report the same in-memory aggregates and are registered
by the proxy server:

| Method | Path | Content type | Purpose |
| --- | --- | --- | --- |
| `GET` | `/metrics` | `application/json` | Human- and API-readable snapshot |
| `GET` | `/metrics/prometheus` | `text/plain; version=0.0.4; charset=utf-8` | Prometheus scrape endpoint |

## JSON response

The response always includes `requests`, `performance`, `uptime`, `provider`,
and `config`:

```json
{
  "requests": {
    "total": 100,
    "successful": 98,
    "errors": 2,
    "streaming": 75,
    "tool_calls": 15,
    "success_rate": "98.00%"
  },
  "performance": {
    "avg_latency_ms": "523.50",
    "requests_per_sec": "2.34"
  },
  "uptime": "5m30s",
  "provider": "openai",
  "config": {
    "http_timeout_sec": 300
  }
}
```

Optional sections are included when the corresponding subsystem is configured:

| Section | Contents |
| --- | --- |
| `cache` | Response-cache size, capacity, hits, misses, and hit rate |
| `prompt_cache` | Prompt-cache size, capacity, hits, misses, hit rate, and saved tokens |
| `rate_limit` | Enabled flag, allowed/denied counts, rate, window, and burst |
| `fallback` | Enabled flag, attempts, successes, and success rate |
| `queue` | Enabled flag, queued/dequeued/dropped/retried/expired counts, length, and paused state |
| `circuit_breaker` | Enabled flag and current state |
| `health_checker` / `provider_health` | Provider health-check aggregates and details |
| `compaction` | Responses API hit/miss counts, hit rate, and active sessions |
| `costs` | Token costs, token totals, and cost rates |

Percentages and derived latency/rate fields in JSON are strings formatted to
two decimal places. With no observations, derived values are zero (`"0.00%"`
or `"0.00"`).

## Aggregation and metric names

Request counters are updated while handling `/v1/messages`:

- `total` counts every request that reaches the message handler, including
  validation failures.
- `successful` counts successful responses, including responses served from a
  configured cache.
- `errors` counts requests that produce a CLASP or upstream error response.
- `streaming` counts valid requests with `stream: true`.
- `tool_calls` counts valid requests containing one or more tools.
- `avg_latency_ms` is total recorded successful-request latency divided by
  `successful`. Cache hits count as successful but do not add upstream latency.
- `requests_per_sec` is `total / uptime_seconds`.

The counters are process-local and are read with atomic operations. A scrape is
an inexpensive set of atomic field reads, but it is not a transaction: under
concurrent traffic, related fields can represent slightly different instants.

The Prometheus endpoint always emits these base metric names, with a
`provider` label:

| Name | Type | Meaning |
| --- | --- | --- |
| `clasp_requests_total` | counter | Total message-handler requests |
| `clasp_requests_successful` | counter | Successful responses |
| `clasp_requests_errors` | counter | Error responses |
| `clasp_requests_streaming` | counter | Streaming requests |
| `clasp_requests_tool_calls` | counter | Requests containing tools |
| `clasp_latency_total_ms` | counter | Recorded successful-request latency |
| `clasp_uptime_seconds` | gauge | Process uptime |
| `clasp_latency_avg_ms` | gauge | Average recorded latency |
| `clasp_requests_per_second` | gauge | Requests divided by uptime |

Configured subsystems add names prefixed with `clasp_rate_limit_`,
`clasp_cache_`, `clasp_prompt_cache_`, `clasp_fallback_`, `clasp_queue_`,
`clasp_circuit_breaker_`, `clasp_provider_`, and `clasp_cost_` (plus
`clasp_tokens_`). Per-model cost metrics also have `model` and, for token
metrics, `type=input|output` labels. The endpoint emits Prometheus `HELP` and
`TYPE` comments for each metric family.

## Reset semantics

There is no reset operation on either metrics endpoint. Request counters and
uptime last for the lifetime of the CLASP process and reset only when the
process restarts.

`POST /costs?action=reset` is separate: it resets only the cost tracker used by
the `costs` JSON section and Prometheus cost/token metrics. It does not reset
request counters, latency, uptime, cache, rate-limit, queue, or health metrics.

## Authentication

Authentication is disabled by default. When `CLASP_AUTH=true` (or `1`) and
`CLASP_AUTH_API_KEY` is configured, both metrics paths require the configured
key unless `CLASP_AUTH_ALLOW_ANONYMOUS_METRICS=true` (or `1`) is set.

Clients may send the key in either form:

```text
x-api-key: <key>
Authorization: Bearer <key>
```

Missing or invalid credentials return HTTP `401`, an Anthropic-compatible JSON
authentication error, and `WWW-Authenticate: Bearer`.
