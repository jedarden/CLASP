# API-key authentication

CLASP authentication is an optional guard around proxy HTTP endpoints. It is
disabled by default. When disabled, requests are passed to the configured
endpoint handlers without requiring a credential.

## Configure authentication

Set the proxy credential and enable authentication with either CLI flags or
environment variables:

```bash
# CLI
clasp -auth -auth-api-key "proxy-key"

# Environment
CLASP_AUTH=true CLASP_AUTH_API_KEY="proxy-key" clasp
```

The accepted configuration is:

| Configuration | Meaning | Default |
| --- | --- | --- |
| `-auth` | Enable authentication | disabled |
| `-auth-api-key <key>` | Required proxy key; used with `-auth` | unset |
| `CLASP_AUTH` | Enable authentication when `true` or `1` | `false` |
| `CLASP_AUTH_API_KEY` | Required proxy key | unset |
| `CLASP_AUTH_ALLOW_ANONYMOUS_HEALTH` | Permit unauthenticated `GET /health` when `true` or `1` | `true` |
| `CLASP_AUTH_ALLOW_ANONYMOUS_METRICS` | Permit unauthenticated metrics requests when `true` or `1` | `false` |

The same settings can be supplied in a YAML configuration file:

```yaml
auth:
  enabled: true
  api_key: ${CLASP_AUTH_API_KEY}
  allow_anonymous_health: true
  allow_anonymous_metrics: false
```

When authentication is enabled, a proxy key must be configured. The normal
CLI startup path rejects an enabled configuration without
`CLASP_AUTH_API_KEY` or `-auth-api-key`.

## Send credentials

Clients may send the key in either of these request-header forms:

```text
x-api-key: <proxy-key>
Authorization: Bearer <proxy-key>
```

The `x-api-key` header takes precedence when both headers are present. For
compatibility, the `Authorization` header also accepts the raw key without the
`Bearer ` prefix. This proxy credential is separate from the upstream provider
credential such as `OPENAI_API_KEY`.

## Endpoint behavior

With authentication enabled, `/v1/messages` and the metrics endpoints require
the configured key. `/` is always public. `/health` is public by default, and
can be protected by setting `CLASP_AUTH_ALLOW_ANONYMOUS_HEALTH=false` (or `0`).
The JSON and Prometheus metrics paths are public only when
`CLASP_AUTH_ALLOW_ANONYMOUS_METRICS=true` (or `1`).

## Authentication failures

Missing or invalid credentials return HTTP `401 Unauthorized`, set
`WWW-Authenticate: Bearer`, and return `Content-Type: application/json` with
an Anthropic-compatible error envelope:

```json
{
  "type": "error",
  "error": {
    "type": "authentication_error",
    "message": "Invalid API key"
  }
}
```

For a missing key, the message is:
`Missing API key. Provide via x-api-key header or Authorization: Bearer <key>`.
