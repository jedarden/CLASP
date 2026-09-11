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

§5 extends this summary with the round-2 re-split capture
(`clasp-e4955511`, later the same day) — its close reason is quoted
character-for-character and reproduces §1's capture exactly. §6 extends it
again with the round-3 re-split capture (`clasp-8484a909`, same day) — its
close reason is quoted character-for-character and is byte-identical to §1
and §5, making the determinism finding three-round.

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

## 5. Round-2 capture — re-split probe child `clasp-e4955511` (2026-09-11)

Later the same day, `clasp-0ffdb0a2` was re-split into a fresh child chain.
Its probe child `clasp-e4955511` (closed 2026-09-11T05:05Z) re-ran the same
guarded probe; the evidence below is quoted from its close reason
character-for-character. The chain's cache-inventory child `clasp-2cc96fa3`
closed with "PREDICTION: probe will block on the browser flow (expect exit
124)" from a cache dir holding only a zero-byte lock file — confirmed on both
prongs.

### The exact command as run (guard included, no `--server`)

```
timeout 15 kubectl auth can-i get applications.argoproj.io -n argocd
```

### Verbatim combined stdout+stderr (3 lines, blank line between first and second)

```
error: could not open the browser: exec: "xdg-open,x-www-browser,www-browser": executable file not found in $PATH

Please visit the following URL in your browser manually: http://localhost:18000/
error: get-token: authentication error: authcode-browser error: authentication error: authorization code flow error: oauth2 error: authorization error: authorization error: context canceled
```

### Exit code

```
124
```

`timeout`'s own code — proving the 15-second guard did the killing; an instant
plugin failure would have exited 1 immediately.

### Reproduction check

This capture is **character-for-character identical** to the round-1 capture
in §1 — same three lines, same `localhost:18000` callback URL, same exit 124.
The failure is deterministic, not transient.

### Confirmed default context identity

`apexalgo-iad-kalshi-oidc` — unchanged across both rounds: cluster `iad-kalshi`
(Rackspace Spot hosted control plane) via the `kubectl oidc-login get-token`
exec plugin against `https://login.spot.rackspace.com/`, with **no** static
token and an **empty** token cache (only a zero-byte `.lock` file, stale since
2026-08-07). Neither probe cached a token; nothing about the context changed
between rounds.

### One-line verdict (quotable verbatim)

Every cluster read from this box must use an explicit
--server=http://traefik-<cluster>:8001 endpoint because the default context's
exec auth blocks headless.

(For `iad-kalshi` specifically the endpoint is
`http://kubectl-proxy-iad-kalshi:8001` — that cluster has no Traefik route.
§4 above is the fully-qualified version of the same verdict.)

## 6. Round-3 capture — re-split probe child `clasp-8484a909` (2026-09-11)

`clasp-0ffdb0a2` was re-split a third time into an umbrella + 4-child chain
(commit `75885a5`); its probe child `clasp-8484a909` (closed
2026-09-11T05:55Z) re-ran the same guarded probe. The evidence below is
quoted from its close reason character-for-character.

### The exact command as run (guard included, no `--server`)

```
timeout 15 kubectl auth can-i get applications.argoproj.io -n argocd
```

Combined stdout+stderr went to a throwaway `mktemp` capture file; the exit
code came from `$?` immediately after; the rounds-1–2 redaction filter
(JWT-shape / long-base64-run / credential-keyword scans) was re-verified a
**no-op** (0/0/0 hits) before anything was written down, and the capture
file was deleted afterward.

### Verbatim combined stdout+stderr (3 text lines, blank line between first and second)

```
error: could not open the browser: exec: "xdg-open,x-www-browser,www-browser": executable file not found in $PATH

Please visit the following URL in your browser manually: http://localhost:18000/
error: get-token: authentication error: authcode-browser error: authentication error: authorization code flow error: oauth2 error: authorization error: authorization error: context canceled
```

### Exit code

```
124
```

### Determinism across rounds 1–3

This capture is **character-for-character identical** to the round-1 capture
in §1 (child 3, `clasp-5568901c`) and the round-2 capture in §5
(`clasp-e4955511`) — same three lines, same `localhost:18000` callback URL,
same **386 bytes**, same exit 124 (`timeout`'s own code: the 15-second guard
did the killing; no `Yes`/`No` was printed, so no API server request was ever
made — the failure sits entirely inside credential acquisition). Three
independent probes across one day produced byte-identical failures: the
headless block is deterministic, not transient.

### Default context identity and cache state — unchanged, no deviation

Child 1 re-confirmed the identity at capture time via config-file read only
(no API server contacted, exec plugin not executed); re-checked when this
section was written: current context `apexalgo-iad-kalshi-oidc` (user `oidc`,
cluster `iad-kalshi`) — identical to §1/§2/§5. The token cache still holds
**only zero-byte `.lock` files** (3 files, newest mtime 2026-08-19) — no
id-token and no refresh token has ever existed on disk, matching §2's
inventory. The round-3 probe cached no token. **The context did not
authenticate; there is no deviation from rounds 1–2.**

## 7. Close-reason block for the parent `clasp-0ffdb0a2` (ready to paste)

Rounds 1–2 closed their child chains but never delivered the parent a
packaged close reason, which is why it kept failing verification after its
children closed. The block below is that packaging, quoted from child 1's
(`clasp-8484a909`) close reason — the §6 capture — verified against the live
store. The worker closing the parent can paste it verbatim as the `--reason`
of `bead close clasp-0ffdb0a2` without re-deriving anything:

```
Round-3 timeout-guarded auth can-i probe (child 1 clasp-8484a909 of the round-3 re-split). Command, verbatim, deliberately no --server:

timeout 15 kubectl auth can-i get applications.argoproj.io -n argocd

Verbatim combined stdout+stderr (3 text lines, blank line between first and second):
---
error: could not open the browser: exec: "xdg-open,x-www-browser,www-browser": executable file not found in $PATH

Please visit the following URL in your browser manually: http://localhost:18000/
error: get-token: authentication error: authcode-browser error: authentication error: authorization code flow error: oauth2 error: authorization error: authorization error: context canceled
---

Exit code: 124 — timeout's own code: the 15-second guard did the killing; no Yes/No was printed, so no API server request was ever made; the failure sits entirely inside credential acquisition.

Attestation: the probe was never run without the "timeout 15" guard.

Quotable verdict (one line): Every cluster read from this box must use an explicit --server=http://traefik-<cluster>:8001 endpoint (http://kubectl-proxy-iad-kalshi:8001 for iad-kalshi) because the default context's exec auth blocks headless.

Determinism: this capture is character-for-character identical to the round-1 capture (clasp-5568901c, §1) and the round-2 capture (clasp-e4955511, §5), same 386 bytes. No credential-shaped material appears anywhere in the output.
```
