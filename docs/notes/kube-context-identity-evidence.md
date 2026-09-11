# Default kube context identity — config-file evidence

Bead `clasp-5c5aed98` (child 1 of the split of `clasp-0ffdb0a2`, headless-auth
evidence). Recorded 2026-09-11.

**Method:** every command below reads only the local kubeconfig file — none
contacts an API server, and `kubectl config view` does not execute the user's
`exec` credential plugin, so the OIDC flow cannot run or block. Each call was
still wrapped in `timeout 15` per house convention. Output was passed through a
redaction filter (`jq`) *before* recording; the redaction replaces any key
matching `secret|token|password|credential|*-data` with `<redacted>` and any
`key=value` argument or JWT-shaped string in ordinary values.

## 1. Context name

```
$ timeout 15 kubectl config current-context
apexalgo-iad-kalshi-oidc
# exit 0
```

Matches the expected `apexalgo-iad-kalshi-oidc`.

## 2. Minified kubeconfig (verbatim, redacted)

```
$ timeout 15 kubectl config view --minify -o json
{
  "kind": "Config",
  "apiVersion": "v1",
  "clusters": [
    {
      "name": "iad-kalshi",
      "cluster": {
        "server": "https://hcp-e00b0a44-a342-4b61-8500-d4c90ece0c2d.spot.rackspace.com",
        "certificate-authority-data": "<redacted>"
      }
    }
  ],
  "users": [
    {
      "name": "oidc",
      "user": {
        "exec": {
          "command": "kubectl",
          "args": [
            "oidc-login",
            "get-token",
            "--oidc-issuer-url=https://login.spot.rackspace.com/",
            "--oidc-client-id=mwG3lUMV8KyeMqHe4fJ5Bb3nM1vBvRNa",
            "--oidc-extra-scope=openid",
            "--oidc-extra-scope=profile",
            "--oidc-extra-scope=email",
            "--oidc-auth-request-extra-params=organization=org_KsELolwAOxl3Zxfm",
            "--token-cache-dir=~/.kube/cache/oidc-login/org_KsELolwAOxl3Zxfm"
          ],
          "env": null,
          "apiVersion": "client.authentication.k8s.io/v1beta1",
          "provideClusterInfo": false,
          "interactiveMode": "IfAvailable"
        }
      }
    }
  ],
  "contexts": [
    {
      "name": "apexalgo-iad-kalshi-oidc",
      "context": {
        "cluster": "iad-kalshi",
        "user": "oidc",
        "namespace": "default"
      }
    }
  ],
  "current-context": "apexalgo-iad-kalshi-oidc"
}
# exit 0
```

**Redaction note:** the only withheld value actually present in the raw output
was `certificate-authority-data` (the cluster CA bundle — public material, not a
credential, withheld because it is bulky and irrelevant here). No
`client-secret`, `id-token`, `refresh-token`, or `access-token` field exists in
this kubeconfig at all: redaction preserves key names, and no such key appears.
The exec argument list above is verbatim; it contains identifiers only
(`--oidc-client-id` is a public OAuth client identifier, not a secret — this is
a secret-less public-client flow).

## 3. What this proves for the headless question

- Context `apexalgo-iad-kalshi-oidc` binds cluster `iad-kalshi`
  (`https://hcp-e00b0a44-a342-4b61-8500-d4c90ece0c2d.spot.rackspace.com`, a
  Rackspace Spot hosted control plane) to user `oidc`.
- The user authenticates exclusively through the exec plugin
  `kubectl oidc-login get-token` (int128 kubelogin) against
  `https://login.spot.rackspace.com/`, with `apiVersion
  client.authentication.k8s.io/v1beta1` and `interactiveMode: IfAvailable`.
- There is no static token, client secret, or refresh token in the kubeconfig —
  the only token material lives in the plugin's cache at
  `~/.kube/cache/oidc-login/org_KsELolwAOxl3Zxfm`.
- Therefore, when that cache is empty, the only way to obtain a token is the
  authorization-code + browser flow — which is exactly why headless use of this
  context blocks and why every cluster read from this box must go through the
  credential-free tailnet endpoints instead. The live failure demonstration is
  the parent bead's job (`clasp-0ffdb0a2`); see also the consolidated note
  `argocd-connectivity-auth-verification.md`.

## 4. Token cache state on disk — bead `clasp-7cd679fc` (child 2 of the split)

Recorded 2026-09-11 ~00:02 -0400.

**Method:** metadata-only inspection — `find -printf` and `stat` on paths,
types, sizes, modes and mtimes. No file content was read, printed, piped or
recorded; no `kubectl` command of any kind was run by this bead (the
timeout-guarded probe is child 3, `clasp-5568901c`). File *names* below are
cache-key hashes and lock names — identifiers, not token material.

### Cache inventory

Cache root: `/home/coding/.kube/cache/oidc-login` (the `--token-cache-dir`
recorded in §2, with `~` = `/home/coding`). Complete recursive listing:

| path (relative to cache root) | type | size | mode | mtime | age |
|---|---|---|---|---|---|
| `.` | directory | 4096 | 700 | 2026-08-19 07:32:20.912 -0400 | ~32.5 weeks |
| `org_KsELolwAOxl3Zxfm/` | directory | 4096 | 700 | 2026-08-07 19:21:29.735 -0400 | ~34.7 days |
| `org_KsELolwAOxl3Zxfm/90e1f62b22c246b31866092f2b59453a9d1218ab22f4e4bb659eae988cbd2cc6.lock` | regular file | 0 | 600 | 2026-08-07 19:21:29.735 -0400 | ~34.7 days |
| `60c449c69bd1768da2596f5be9516125e14747cce38141a74f0218ab4bf3cd54.lock` | regular file | 0 | 600 | 2026-08-19 07:32:20.912 -0400 | ~32.5 weeks |
| `90e1f62b22c246b31866092f2b59453a9d1218ab22f4e4bb659eae988cbd2cc6.lock` | regular file | 0 | 600 | 2026-05-03 06:46:49.644 -0400 | ~131 days |

Reading: kubelogin stores each cached token as a real entry file keyed by the
same hash that names the lock (`90e1f62b…` = hash of the public issuer +
client-id cache key — the identical hash appears at the cache root from an
older flat-layout run, and inside `org_KsELolwAOxl3Zxfm/` for the current
per-org layout). Here **every entry position holds only a zero-byte `.lock`**;
there is no token file of any kind, so there is no id-token and no refresh
token on disk. The org-dir lock (2026-08-07) is a leftover from a run that was
killed after lock acquisition but before any token could be written — the
newest entry anywhere in the cache is that empty lock, 34+ days stale. A
sweep for other cache roots found only this one (a literal `~/` directory at
`/home/coding/~` turned out to hold unexpanded-tilde NEEDLE state files, not
kube caches — ruled out).

### Prediction

**Cache empty -> expect browser-block killed by timeout (exit 124).**

Rationale: no cached token and no refresh token means the plugin cannot satisfy
`get-token` from disk; with `interactiveMode: IfAvailable` on a headless,
non-tty invocation it falls through to the authorization-code + browser flow —
which cannot complete here — and blocks, until the `timeout 15` guard kills it.
This is the parent's expected failure, now predicted from disk state alone,
consistent with the 2026-08-07 lock leftover (same failure mode, last observed).
If child 3's run instead authenticates, that is a deviation from this
prediction and from the expected failure, and should be recorded as such.

**No token material was printed, piped, or recorded anywhere in this section.**

## 5. Live probe — bead `clasp-5568901c` (child 3 of the split)

Recorded 2026-09-11 ~00:09 -0400, minutes after §4's prediction.

**Method:** the parent bead's probe, run exactly as specified and **never
without the guard** — the guard is what makes this probe completable on a
headless box rather than a hang. Combined stdout+stderr was redirected into a
throwaway temp capture file, the exit code taken from `$?` immediately after,
and the recorded text below passed through the same redaction-filter family as
§1 before being written down.

### Command as run

```
timeout 15 kubectl auth can-i get applications.argoproj.io -n argocd
```

(Shell redirection `> "$OUT" 2>&1` captured the combined output to the temp
file; the probe line itself is verbatim and unmodified.)

### Verbatim combined output (after redaction filter)

```
error: could not open the browser: exec: "xdg-open,x-www-browser,www-browser": executable file not found in $PATH

Please visit the following URL in your browser manually: http://localhost:18000/
error: get-token: authentication error: authcode-browser error: authentication error: authorization code flow error: oauth2 error: authorization error: authorization error: context canceled
```

**Redaction note:** the filter (JWT-shaped strings, `code=`/`state=`/`nonce=`/
`token=`/`secret=` query-style pairs, 40+-char hex runs) applied to the capture
was a **verified no-op** — `diff` of before/after identical (capture: 386
bytes, 4 lines, no CRs, single trailing newline). The output contains no
token-shaped material at all. The only URL printed is the plugin's own local
callback listener (`http://localhost:18000/`) — bare, no query parameters, no
secrets.

### Exit code

```
124
```

### Reading

This **confirms §4's prediction** — no deviation:

- With the token cache empty (§4: only zero-byte `.lock` files, no token entry
  anywhere), `kubectl oidc-login get-token` could not serve the exec plugin
  from disk and fell through to the authorization-code + browser flow.
- Line 1: the browser exec failed outright — this box has no `xdg-open`,
  `x-www-browser`, or `www-browser`.
- Line 3: kubelogin fell back to "visit this URL manually" and **blocked on its
  localhost callback listener**. That 15-second block is the parent bead's
  suspected hang, now observed live.
- Line 4 is the plugin's reaction to the guard's SIGTERM at t=15s (`context
  canceled`), surfaced by `kubectl` as an exec-plugin authentication error —
  an artifact of the kill, not an independent auth verdict. No API server
  request was ever made; the failure is entirely inside credential acquisition.
- Exit `124` is `timeout` reporting the job was killed on the deadline — the
  probe never reached an authorization decision (`Yes`/`No` never printed).

**Net:** parent `clasp-0ffdb0a2`'s expected failure is now demonstrated
end-to-end — empty cache (§4) → browser flow (§2 config) → headless block →
guard kill, exit 124. Headless use of `apexalgo-iad-kalshi-oidc` is
structurally impossible, reinforcing the tailnet-endpoint rule for every
cluster read from this box. Consolidated context:
`argocd-connectivity-auth-verification.md`.
