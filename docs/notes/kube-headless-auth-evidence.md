# Headless kube auth failure — evidence summary

Bead `clasp-cadee0e6` (child 4 of the split of `clasp-0ffdb0a2`, the bead this
summary exists to close). Recorded 2026-09-11. Composes the evidence of
children 1–3, all captured verbatim in
`kube-context-identity-evidence.md`:

- **Child 1** `clasp-5c5aed98` — default-context identity (config-file only)
- **Child 2** `clasp-7cd679fc` — OIDC token-cache state on disk (metadata-only)
- **Child 3** `clasp-5568901c` — the live, timeout-guarded probe

Every command across children 1–3 ran under a `timeout 15` guard, and all
recorded output passed through a redaction filter before being written down.
No credential value appears anywhere in this doc.

## 1. The exact command run (child 3, `clasp-5568901c`)

Run deliberately **without** `--server`, i.e. against the default context
`apexalgo-iad-kalshi-oidc`, exactly as the parent bead specified and never
without the guard:

```
timeout 15 kubectl auth can-i get applications.argoproj.io -n argocd
```

(Combined stdout+stderr was redirected to a throwaway temp capture file; the
exit code was taken from `$?` immediately after. The probe line is verbatim.)

### Verbatim combined output

```
error: could not open the browser: exec: "xdg-open,x-www-browser,www-browser": executable file not found in $PATH

Please visit the following URL in your browser manually: http://localhost:18000/
error: get-token: authentication error: authcode-browser error: authentication error: authorization code flow error: oauth2 error: authorization error: authorization error: context canceled
```

### Exit code

```
124
```

`124` is `timeout`'s own code: the probe was killed on the 15-second deadline.
`Yes`/`No` was never printed — **no API server request was ever made**; the
failure is entirely inside credential acquisition. Line 3 is the plugin's
fallback prompt with its bare local callback listener (no query parameters,
nothing secret). The redaction filter over the 386-byte capture was verified a
**no-op** — the output contained no token-shaped material at all.

## 2. Why it failed — composed evidence from children 1–2

**Context identity** (child 1, `clasp-5c5aed98`; config-file capture only, no
API server contacted, exec plugin not executed):

- Current context `apexalgo-iad-kalshi-oidc` binds cluster `iad-kalshi`
  (`https://hcp-e00b0a44-a342-4b61-8500-d4c90ece0c2d.spot.rackspace.com`, a
  Rackspace Spot hosted control plane) to user `oidc`.
- The kubeconfig holds **no** static token, client secret, or refresh token:
  authentication is exclusively the exec plugin `kubectl oidc-login get-token`
  (int128 kubelogin) against `https://login.spot.rackspace.com/`, with
  `interactiveMode: IfAvailable` and token cache dir
  `~/.kube/cache/oidc-login/org_KsELolwAOxl3Zxfm`. (`--oidc-client-id` in that
  argument list is a public OAuth client identifier, not a secret — a
  secret-less public-client flow. The one value the child withheld,
  `certificate-authority-data`, is recorded as `<redacted>` there; it is
  public CA material, not a credential.)

**Cache state** (child 2, `clasp-7cd679fc`; `find`/`stat` metadata only — no
file contents read, no kubectl run):

- The cache root holds **only zero-byte `.lock` files** — the newest dated
  2026-08-07, 34+ days stale. No token entry file exists anywhere: no
  id-token, no refresh token on disk.

Combined: with nothing obtainable from disk, `get-token` fell through to the
authorization-code + browser flow — line 1 shows the browser exec failing
outright (no `xdg-open`/`x-www-browser`/`www-browser` on this box), line 3
shows the "visit this URL manually" fallback **blocking on its localhost
callback**, and the guard's SIGTERM at t=15s produced line 4 (`context
canceled`) and exit 124.

## 3. Expected vs actual

**Expected** (parent `clasp-0ffdb0a2`, predicted from disk state alone by
child 2 *before* the probe ran): empty token cache → `authcode-browser` flow
blocks waiting for an interactive browser visit → `timeout` kills it at 15 s →
**exit 124**.

**Actual:** exactly that. The oidc-login browser flow **was blocked and killed
by the timeout guard, exit 124** — the context did **not** authenticate, so
there is **no deviation**. Child 2's alternative branch (a token now cached →
unexpected success output) did not occur: no token has existed in the cache
for 34+ days, and the probe wrote none. Child 2's block-vs-success prediction
was confirmed, and `clasp-0ffdb0a2`'s suspected hang is now demonstrated live,
end to end: empty cache → browser flow → headless block → guard kill.

## 4. Quotable verdict

> **VERDICT:** Every cluster read from this box must go via `--server` against
> the credential-free tailnet kubectl-proxy endpoints
> (`http://traefik-<cluster>:8001`, and `http://kubectl-proxy-iad-kalshi:8001`
> for `iad-kalshi`) instead of the default context, because
> `apexalgo-iad-kalshi-oidc` authenticates solely through a browser-driven
> OIDC flow that cannot run headless — with an empty token cache it blocks on
> a localhost callback and is killed by `timeout` at exit 124 before any API
> request is made.
