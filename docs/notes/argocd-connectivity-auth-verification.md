# ArgoCD namespace — connectivity and auth verification (consolidated)

Consolidates the bounded verification of the configured kube context against the
`argocd` namespace, gathered from beads `clasp-a0c32d49` (context), `clasp-f4b3c4c5`
(connectivity), and `clasp-e73d3f9b` (RBAC), plus one bounded reproduction of the
headless failure. Umbrella: `clasp-c86a7e10`. Recorded 2026-09-10.

**Verdicts: Connectivity PASS · Auth (read Applications) PASS · headless use of the
configured context FAIL (by design of the OIDC browser flow).**

Every `kubectl` invocation below was wrapped in `timeout 15`; no unbounded call was
made at any point.

## 1. What this box is configured with — `clasp-a0c32d49`

Config-file reads only, no API server contacted:

```
$ timeout 15 kubectl config current-context
apexalgo-iad-kalshi-oidc
```

```
$ timeout 15 kubectl config view --minify
# context apexalgo-iad-kalshi-oidc -> cluster iad-kalshi
#   server: https://hcp-e00b0a44-a342-4b61-8500-d4c90ece0c2d.spot.rackspace.com
#   (Rackspace Spot hosted control plane; user "oidc" via a kubectl oidc-login
#    exec plugin against login.spot.rackspace.com)
```

Full kubeconfig cluster list: `ardenone-cluster -> http://traefik-ardenone-cluster:8001`,
`iad-kalshi -> https://hcp-e00b0a44-a342-4b61-8500-d4c90ece0c2d.spot.rackspace.com`.

## 2. Connectivity — PASS (`clasp-f4b3c4c5`, re-confirmed live)

```
$ timeout 15 kubectl --server=http://traefik-ardenone-cluster:8001 get ns argocd
NAME     STATUS   AGE
argocd   Active   169d
# exit 0
```

## 3. Auth / RBAC — PASS (`clasp-e73d3f9b`, re-confirmed live)

```
$ timeout 15 kubectl --server=http://traefik-ardenone-cluster:8001 auth whoami
system:serviceaccount:devpod-observer:devpod-observer
# (UID 44c55d07-5de3-4698-a65c-e0420d2502b8, pod kubectl-proxy-c65cb5dc6-j8b6j)

$ timeout 15 kubectl --server=http://traefik-ardenone-cluster:8001 auth can-i get applications.argoproj.io -n argocd
yes
# exit 0

$ timeout 15 kubectl --server=http://traefik-ardenone-cluster:8001 auth can-i create applications.argoproj.io -n argocd
no
# exit 1 — expected: "auth can-i" exits 1 on a "no"; the read-only proxy RBAC cannot write
```

The `yes` was ground-truthed beyond SelfSubjectAccessReview with a real Application
list on ardenone-cluster (miroir, miroir-dev, twitterapi-proxy-ardenone-cluster,
whisper-stt).

## 4. CAVEAT — the configured context cannot authenticate headless from this box

The active context `apexalgo-iad-kalshi-oidc` authenticates via a `kubectl
oidc-login` exec credential plugin whose token cache is empty. It does not fail
fast: it starts an authorization-code browser flow, and on this headless box it
blocks until killed. Reproduced once, bounded (exit 124 = killed by `timeout 15`;
the hang continues until then):

```
$ timeout 15 kubectl auth can-i get applications.argoproj.io -n argocd
error: could not open the browser: exec: "xdg-open,x-www-browser,www-browser": executable file not found in $PATH

Please visit the following URL in your browser manually: http://localhost:18000/
error: get-token: authentication error: authcode-browser error: authentication error: authorization code flow error: oauth2 error: authorization error: authorization error: context canceled
# exit 124
```

`--request-timeout` does not cover the exec plugin, so `timeout` is the only bound
that works against this context. **Every PASS above was therefore obtained via
`--server` against the documented credential-free, read-only tailnet kubectl-proxy
endpoints** (`http://traefik-<cluster>:8001`, and `http://kubectl-proxy-iad-kalshi:8001`
for iad-kalshi) — not through the configured context.

Related but distinct: `iad-kalshi`, the cluster that context points at, has an
`argocd` namespace but no ArgoCD installed, so Application reads there would fail
for lack of the CRD regardless of auth.

## 5. Re-verification — 2026-09-10 (`clasp-99015c0b`)

The §2 connectivity and §3 RBAC checks re-run and consolidated verbatim so the
umbrella `clasp-c86a7e10` closes against a durable record. Same day as the
original recording. Same method as everywhere above: the credential-free
`--server` tailnet endpoint, every call wrapped in `timeout 15`, configured
context untouched (§4 still applies, including the `iad-kalshi` CRD note).

```
$ timeout 15 kubectl --server=http://traefik-ardenone-cluster:8001 get ns argocd
NAME     STATUS   AGE
argocd   Active   169d
# exit 0

$ timeout 15 kubectl --server=http://traefik-ardenone-cluster:8001 auth whoami
ATTRIBUTE                                           VALUE
Username                                            system:serviceaccount:devpod-observer:devpod-observer
UID                                                 44c55d07-5de3-4698-a65c-e0420d2502b8
Groups                                              [system:serviceaccounts system:serviceaccounts:devpod-observer system:authenticated]
Extra: authentication.kubernetes.io/credential-id   [JTI=3f63e346-bdac-4cf9-91d7-dcdd2a3b7542]
Extra: authentication.kubernetes.io/node-name       [k3s-agent-d]
Extra: authentication.kubernetes.io/node-uid        [3f264d64-03c8-4e68-a553-d18599a4b1ab]
Extra: authentication.kubernetes.io/pod-name        [kubectl-proxy-c65cb5dc6-j8b6j]
Extra: authentication.kubernetes.io/pod-uid         [362b8043-3188-476b-8d9b-ac562cc0e03a]
# exit 0 — same SA, UID and proxy pod as §3; the kubectl-proxy has not rotated

$ timeout 15 kubectl --server=http://traefik-ardenone-cluster:8001 auth can-i get applications.argoproj.io -n argocd
yes
# exit 0

$ timeout 15 kubectl --server=http://traefik-ardenone-cluster:8001 auth can-i create applications.argoproj.io -n argocd
no
# exit 1 — "auth can-i" exits 1 on a "no"; read-only proxy RBAC still cannot write
```

Verdicts unchanged: Connectivity PASS · Read Applications PASS · write denied by
the read-only RBAC · headless use of `apexalgo-iad-kalshi-oidc` still FAIL (§4).

## 6. Re-verification — 2026-09-10 (`clasp-44873a3d`, `clasp-ebce3662`)

A second same-day re-run, dispatched as two scoped children of umbrella
`clasp-c86a7e10`: `clasp-44873a3d` (connectivity half) and its successor
`clasp-ebce3662` (RBAC half). Same method as everywhere above: the
credential-free `--server` tailnet endpoint, every call wrapped in `timeout 30`,
configured context untouched (§4 still applies, including the `iad-kalshi` CRD
note). Output captured verbatim with exit codes:

```
$ timeout 30 kubectl --server=http://traefik-ardenone-cluster:8001 get ns argocd
NAME     STATUS   AGE
argocd   Active   169d
# exit 0

$ timeout 30 kubectl --server=http://traefik-ardenone-cluster:8001 auth whoami
ATTRIBUTE                                           VALUE
Username                                            system:serviceaccount:devpod-observer:devpod-observer
UID                                                 44c55d07-5de3-4698-a65c-e0420d2502b8
Groups                                              [system:serviceaccounts system:serviceaccounts:devpod-observer system:authenticated]
Extra: authentication.kubernetes.io/credential-id   [JTI=42e5e7c3-fb49-4647-8c09-93505fa63a16]
Extra: authentication.kubernetes.io/node-name       [k3s-agent-d]
Extra: authentication.kubernetes.io/node-uid        [3f264d64-03c8-4e68-a553-d18599a4b1ab]
Extra: authentication.kubernetes.io/pod-name        [kubectl-proxy-c65cb5dc6-j8b6j]
Extra: authentication.kubernetes.io/pod-uid         [362b8043-3188-476b-8d9b-ac562cc0e03a]
# exit 0 — same SA, UID and proxy pod as §3/§5; only the per-session JTI rotated

$ timeout 30 kubectl --server=http://traefik-ardenone-cluster:8001 auth can-i get applications.argoproj.io -n argocd
yes
# exit 0

$ timeout 30 kubectl --server=http://traefik-ardenone-cluster:8001 auth can-i list applications.argoproj.io -n argocd
yes
# exit 0
```

New in this round: the **`list`** verb was checked alongside `get` — collection
reads need `list`, not `get`, so `can-i list → yes` is what actually authorizes
`kubectl get applications.argoproj.io -n argocd`. Ground-truthed end-to-end
(also first-hand here, matching what `clasp-ebce3662` recorded):

```
$ timeout 30 kubectl --server=http://traefik-ardenone-cluster:8001 get applications.argoproj.io -n argocd
NAME                                SYNC STATUS   HEALTH STATUS
miroir                              Unknown       Healthy
miroir-dev                          Unknown       Healthy
twitterapi-proxy-ardenone-cluster   Unknown       Unknown
whisper-stt                         Unknown       Healthy
# exit 0
```

No error occurred in either half — this is a grant, not an RBAC denial, missing
CRD, or unreachable API server (those present differently: `Error from server
(Forbidden)`, a CRD warning, or a timeout). Verdicts unchanged: Connectivity
PASS · read (get + list) Applications PASS · headless use of
`apexalgo-iad-kalshi-oidc` still FAIL (§4).

## 7. Re-verification — 2026-09-10 (`clasp-5fbd8d34`, `clasp-1090dcb4`, `clasp-1d300508`)

A third same-day round, dispatched as three scoped split children and
consolidated here from their close records: `clasp-5fbd8d34` (configured context
and its headless limitation — config-file reads only, no cluster call),
`clasp-1090dcb4` (connectivity), and `clasp-1d300508` (RBAC). Same method as
everywhere above: the credential-free `--server` tailnet endpoint, every cluster
call wrapped in `timeout 30`, configured context untouched (§4 still applies,
including the `iad-kalshi` CRD note).

### Recorded context — `clasp-5fbd8d34`

No API server was contacted in this child. Values are recorded verbatim from its
close record (jsonpath extracted only the server field — no `--raw`, no
`--flatten`, no credential material in the output):

```
$ kubectl config current-context
apexalgo-iad-kalshi-oidc
# KUBECONFIG unset -> kubectl reads /home/coding/.kube/config (mode 600, mtime 2026-09-09 22:21)

$ kubectl config view --minify -o jsonpath={.clusters[0].cluster.server}
https://hcp-e00b0a44-a342-4b61-8500-d4c90ece0c2d.spot.rackspace.com
# Rackspace Spot hosted control plane — matches §1 verbatim
```

The headless limitation is re-confirmed as documented in §4: the context
authenticates via a `kubectl oidc-login` exec credential plugin whose token
cache is empty, so on this headless box it starts an authorization-code browser
flow and blocks until killed (reproduced exit 124 under `timeout 15`;
`--request-timeout` does not cover the exec plugin). That is why every live
check in this round — as in every round above — runs through the tailnet
kubectl-proxy endpoint instead.

### Connectivity — PASS (`clasp-1090dcb4`)

```
$ timeout 30 kubectl --server=http://traefik-ardenone-cluster:8001 get ns argocd
NAME     STATUS   AGE
argocd   Active   169d
# exit 0 — stderr empty
```

### RBAC — PASS (`clasp-1d300508`)

```
$ timeout 30 kubectl --server=http://traefik-ardenone-cluster:8001 auth can-i get applications.argoproj.io -n argocd
yes
# exit 0

$ timeout 30 kubectl --server=http://traefik-ardenone-cluster:8001 auth can-i list applications.argoproj.io -n argocd
yes
# exit 0

$ timeout 30 kubectl --server=http://traefik-ardenone-cluster:8001 get applications.argoproj.io -n argocd
# exit 0 — ground-truth read succeeded; the 4 Applications as recorded by the child:
miroir (Sync Unknown/Healthy)
miroir-dev (Sync Unknown/Healthy)
twitterapi-proxy-ardenone-cluster (Sync Unknown/Unknown)
whisper-stt (Sync Unknown/Healthy)
```

Both `can-i` probes answered `yes` with exit 0 (the exit-1-on-`no` caveat from
§3 never triggers this round), and the real read succeeded — a grant, not a
Forbidden, missing-CRD, or timeout outcome.

**Verdicts: Connectivity PASS · Application read (get + list) PASS · headless
use of `apexalgo-iad-kalshi-oidc` FAIL (§4).**

## 8. Re-verification — 2026-09-10 (`clasp-560fffc1`, `clasp-59456b91`, `clasp-57517e00`, `clasp-04e168cf`)

A fourth same-day round, dispatched as four scoped split children of umbrella
`clasp-c86a7e10` and consolidated here verbatim from their close records:
`clasp-560fffc1` (configured context and API server), `clasp-59456b91`
(reachability), `clasp-04e168cf` (RBAC), and `clasp-57517e00` (bounded
reproduction of the headless failure). Same method as every round above: the
credential-free `--server` tailnet endpoint for every cluster call, configured
context untouched except where the §4 failure was deliberately reproduced under
`timeout 15`.

**Verdicts: reachability PASS · RBAC PASS · headless-context auth FAIL (§4, by
design of the OIDC browser flow).**

### Configured context — `clasp-560fffc1`

Config-file reads only, no API server contacted; neither command invokes the
exec plugin, so neither can hang (both exit 0):

```
$ timeout 15 kubectl config current-context
apexalgo-iad-kalshi-oidc

$ timeout 15 kubectl config view --minify --output jsonpath={.clusters[0].cluster.server}
https://hcp-e00b0a44-a342-4b61-8500-d4c90ece0c2d.spot.rackspace.com
# Rackspace Spot hosted control plane for iad-kalshi — matches §1/§7 verbatim
```

### Reachability — PASS (`clasp-59456b91`)

```
$ timeout 15 kubectl --server=http://traefik-ardenone-cluster:8001 get ns argocd
NAME     STATUS   AGE
argocd   Active   169d
# exit 0
```

The child's second probe repeated the same read through the configured context
with no `--server` override and hit the §4 hang — exit 124, killed by the
timeout, stderr as reproduced verbatim in the headless section below.

### RBAC — PASS (`clasp-04e168cf`)

Via the credential-free read-only endpoint `http://traefik-ardenone-cluster:8001`:

```
$ kubectl --server=http://traefik-ardenone-cluster:8001 auth can-i get applications.argoproj.io -n argocd
yes
# exit 0

$ kubectl --server=http://traefik-ardenone-cluster:8001 auth whoami
# Username: system:serviceaccount:devpod-observer:devpod-observer — the
# kubectl-proxy SA, pod kubectl-proxy-c65cb5dc6-j8b6j on k3s-agent-d, groups
# [system:serviceaccounts:devpod-observer system:authenticated]. Same SA and
# proxy pod as §3/§5/§6 — this SA is the identity behind the verdict.

$ kubectl --server=http://traefik-ardenone-cluster:8001 auth can-i create applications.argoproj.io -n argocd
no
# exit 1 — "auth can-i" exits 1 on a "no"; the read-only proxy RBAC still cannot write
```

New in this round, the child ruled out CRD-absence on ardenone-cluster so the
`yes`/`no` above are genuine RBAC decisions rather than a missing-CRD artifact
(distinct from the §4 `iad-kalshi` situation):

```
$ kubectl --server=http://traefik-ardenone-cluster:8001 api-resources --api-group=argoproj.io
# lists applications — argoproj.io/v1alpha1, namespaced, kind Application (served)

$ kubectl --server=http://traefik-ardenone-cluster:8001 get applications.argoproj.io -n argocd
# exit 0 — ground-truth read returned the same 4 Applications as §6:
# miroir, miroir-dev, twitterapi-proxy-ardenone-cluster, whisper-stt
```

(The last two outputs are recorded as captured by the child; the api-resources
listing and app names are verbatim from its close record, which did not
preserve the raw table rendering.)

### Headless-context auth — FAIL (`clasp-57517e00`, reproduced bounded)

`clasp-57517e00` re-ran the §4 reproduction with instrumentation and confirmed
the documented failure byte-for-byte — stderr identical, including the
`localhost:18000` callback URL and the SIGTERM-induced `context canceled` tail
(§4 lines 73–77):

```
$ timeout 15 kubectl get ns argocd </dev/null   # current context apexalgo-iad-kalshi-oidc
# OUTCOME: HUNG — no output, killed by timeout SIGTERM; elapsed 15.01s
# exit 124 · stdout: 0 bytes · stderr: 386 bytes, verbatim:
error: could not open the browser: exec: "xdg-open,x-www-browser,www-browser": executable file not found in $PATH

Please visit the following URL in your browser manually: http://localhost:18000/
error: get-token: authentication error: authcode-browser error: authentication error: authorization code flow error: oauth2 error: authorization error: authorization error: context canceled
```

Pre-conditions verified immediately before the run: the plugin token cache dir
`~/.kube/cache/oidc-login/org_KsELolwAOxl3Zxfm/` held only a 0-byte `.lock`
file (cache empty), and `xdg-open`, `x-www-browser`, `www-browser` all miss
`PATH`. New data points beyond §4: the failure is **verb-independent** (§4
captured it via `auth can-i`; this round via `get ns`, and `clasp-59456b91`'s
second probe hit the same wall), stdout is empty with all output on stderr, and
the hang burns the full timeout (15.01s to the kill).

Round verdict: **reachability PASS · RBAC PASS (read granted, write denied, CRD
confirmed served) · headless use of `apexalgo-iad-kalshi-oidc` FAIL.** §4 and
the tailnet-endpoint method remain the standing caveats; nothing regressed
against any earlier round.

## 9. Kube context and connection mode — 2026-09-10 (`clasp-ea29db38`)

Config-file reads only to identify the context `clasp-cb1a55f5` left active —
no exec-plugin invocation, so no hang (all exit 0):

```
$ timeout 10 kubectl config current-context
apexalgo-iad-kalshi-oidc

$ timeout 10 kubectl config view --minify -o jsonpath='{.clusters[0].cluster.server}'
https://hcp-e00b0a44-a342-4b61-8500-d4c90ece0c2d.spot.rackspace.com
# matches §1/§7/§8 verbatim — Rackspace Spot hosted control plane

$ timeout 10 kubectl config view --minify -o jsonpath='cluster={.contexts[0].context.cluster} user={.contexts[0].context.user} clusterName={.clusters[0].name}'
cluster=iad-kalshi user=oidc clusterName=iad-kalshi
```

Auth mode, verbatim shape: the context's user `oidc` carries a single `exec`
credential block (`client.authentication.k8s.io/v1beta1`,
`provideClusterInfo: false`) running `kubectl oidc-login get-token` against
`https://login.spot.rackspace.com/`, client-id `mwG3lUMV8KyeMqHe4fJ5Bb3nM1vBvRNa`,
scopes `openid profile email`, auth-request extra param
`organization=org_KsELolwAOxl3Zxfm`, token cache dir
`~/.kube/cache/oidc-login/org_KsELolwAOxl3Zxfm`. No client certs, static
tokens, or external auth-provider fields on the user or cluster.

Documented credential-free tailnet kubectl-proxy endpoint for the same cluster
(CLAUDE.md "Kubernetes Access" table, row `iad-kalshi` — the one row with no
Traefik; the Tailscale operator exposes the proxy Service directly):
`http://kubectl-proxy-iad-kalshi:8001`. Probed live this round — named in §4's
caveat prose but exercised for the first time here:

```
$ timeout 10 kubectl --server=http://kubectl-proxy-iad-kalshi:8001 get ns kube-system -o jsonpath='{.metadata.uid} {.metadata.name}'
0738fad8-7667-4635-8dd5-384d00810ecb kube-system
# exit 0
```

### Connection mode chosen for the later checks

**Explicit `--server http://kubectl-proxy-iad-kalshi:8001` for checks against
this cluster — never the bare `apexalgo-iad-kalshi-oidc` context.** One-line
justification: the context's oidc-login exec plugin cannot authenticate
headless from this box (browser-less, token cache empty — §4/§8), while the
tailnet proxy is credential-free and read-only, as the exit-0 probe above shows.

The bare-context failure was re-confirmed live immediately before writing this
(exit code correctly attributed this time — §8's child inferred 124 from the
elapsed time; captured directly here), stderr byte-identical to §8 including
the `localhost:18000` callback URL:

```
$ timeout 10 kubectl get ns kube-system </dev/null   # current context apexalgo-iad-kalshi-oidc
# exit 124 · stdout: 0 bytes · stderr: 386 bytes, byte-identical to §8
error: could not open the browser: exec: "xdg-open,x-www-browser,www-browser": executable file not found in $PATH

Please visit the following URL in your browser manually: http://localhost:18000/
error: get-token: authentication error: authcode-browser error: authentication error: authorization code flow error: oauth2 error: authorization error: authorization error: context canceled
```

Round verdict: **context identified as `apexalgo-iad-kalshi-oidc` →
`iad-kalshi` (server and auth mode captured above); connection mode = explicit
`--server` tailnet endpoint; both endpoint reachability (exit 0) and the §4
headless caveat (exit 124) re-confirmed live.** No regression against any
earlier round.

## 10. Application read on the §9 connection — NO on iad-kalshi, yes on
ardenone-cluster — 2026-09-10 (`clasp-b371077d`)

Dispatched as: "Prove the identity used by the working connection from
`clasp-d8f8411b` may read ArgoCD Applications in the argocd namespace."
`clasp-d8f8411b`'s working connection is the §9 mode — explicit
`--server http://kubectl-proxy-iad-kalshi:8001` against **iad-kalshi** — so the
probe ran exactly as dispatched there, under `timeout 30`. The answer on that
cluster is **no**, with the §4/§8 caveat as its root cause; the
ardenone-cluster contrast (first-hand this round) answers **yes**.

### iad-kalshi — NO (as dispatched, `clasp-b371077d`)

```
$ timeout 30 kubectl --server=http://kubectl-proxy-iad-kalshi:8001 auth can-i get applications.argoproj.io -n argocd
Warning: the server doesn't have a resource type 'applications' in group 'argoproj.io'

no
# exit 1 — "auth can-i" exits 1 on a "no"

$ timeout 30 kubectl --server=http://kubectl-proxy-iad-kalshi:8001 auth whoami
ATTRIBUTE                                           VALUE
Username                                            system:serviceaccount:devpod-observer:devpod-observer
UID                                                 a8682a81-b2d8-4af6-a7b6-ca85ddb36360
Groups                                              [system:serviceaccounts system:serviceaccounts:devpod-observer system:authenticated]
Extra: authentication.kubernetes.io/credential-id   [JTI=7a51dad5-c24b-418c-bf82-02bf18a59619]
Extra: authentication.kubernetes.io/node-name       [prod-instance-17854395072200685]
Extra: authentication.kubernetes.io/node-uid        [d9213868-f030-4f84-8f41-b81e5827d970]
Extra: authentication.kubernetes.io/pod-name        [kubectl-proxy-57b49c88bf-zqhvn]
Extra: authentication.kubernetes.io/pod-uid         [9627eb25-2a1c-4ab1-b09d-19023f49efd7]
# exit 0 — the answering identity is iad-kalshi's kubectl-proxy SA
```

Attribution of the `no`, probed in the same round:

```
$ timeout 30 kubectl --server=http://kubectl-proxy-iad-kalshi:8001 api-resources --api-group=argoproj.io
NAME   SHORTNAMES   APIVERSION   NAMESPACED   KIND
# exit 0 — empty table: no argoproj.io resource is served at all

$ timeout 30 kubectl --server=http://kubectl-proxy-iad-kalshi:8001 get crd -o name | grep -i argo
# grep exit 1, no matches — zero ArgoCD CRDs on the cluster

$ timeout 30 kubectl --server=http://kubectl-proxy-iad-kalshi:8001 get deploy,sts -n argocd
No resources found in argocd namespace.
# exit 0 — no ArgoCD workloads in the namespace either
```

So the dispatched `no` is not a useful RBAC data point about ArgoCD: iad-kalshi
has no ArgoCD installed — the `applications` type is not served (the kubectl
warning above), no CRDs exist, and the `argocd` namespace holds no ArgoCD
workloads. First-hand confirmation of the §4/§8 note ("Application reads there
would fail for lack of the CRD regardless of auth"), now as an auth answer.

### ardenone-cluster — yes (contrast, same round, first-hand)

Where ArgoCD actually runs, the same probes through the same kind of
credential-free tailnet endpoint answer as in every prior round:

```
$ timeout 30 kubectl --server=http://traefik-ardenone-cluster:8001 auth can-i get applications.argoproj.io -n argocd
yes
# exit 0

$ timeout 30 kubectl --server=http://traefik-ardenone-cluster:8001 auth can-i list applications.argoproj.io -n argocd
yes
# exit 0

$ timeout 30 kubectl --server=http://traefik-ardenone-cluster:8001 auth whoami
ATTRIBUTE                                           VALUE
Username                                            system:serviceaccount:devpod-observer:devpod-observer
UID                                                 44c55d07-5de3-4698-a65c-e0420d2502b8
Groups                                              [system:serviceaccounts system:serviceaccounts:devpod-observer system:authenticated]
Extra: authentication.kubernetes.io/credential-id   [JTI=ab083ca2-b2df-4b3b-bf2b-f6878fc95628]
Extra: authentication.kubernetes.io/node-name       [k3s-agent-d]
Extra: authentication.kubernetes.io/node-uid        [3f264d64-03c8-4e68-a65c-e0420d2502b8]
Extra: authentication.kubernetes.io/pod-name        [kubectl-proxy-c65cb5dc6-j8b6j]
Extra: authentication.kubernetes.io/pod-uid         [362b8043-3188-476b-8d9b-ac562cc0e03a]
# exit 0 — same SA, UID, node and proxy pod as §3/§5/§6/§8; only the per-session JTI rotated
```

Note the two answering identities share the SA *name*
(`devpod-observer/devpod-observer`) but are distinct identities: different UID,
node and proxy pod on each cluster.

**Round verdict: read Applications on `clasp-d8f8411b`'s connection (iad-kalshi
tailnet proxy) = NO — the resource is not served on that cluster (no ArgoCD
installed), so the argocd namespace there is not an ArgoCD target; read
Applications on ardenone-cluster = YES (get + list), re-confirmed first-hand.
Any check that actually wants to read ArgoCD Applications must use the
ardenone-cluster endpoint, not the cluster the configured context points at.**

## 11. Consolidated round — 2026-09-10 (`clasp-ea29db38`, `clasp-d8f8411b`, `clasp-b371077d`)

The three-bead round that closed the umbrella's verification chain, consolidated
here in one place with the per-check verdict: `clasp-ea29db38` (context and
connection mode), its connectivity successor `clasp-d8f8411b`, and the RBAC
child `clasp-b371077d`. Same method as every round above — explicit
credential-free `--server` tailnet endpoint, every call under `timeout`
(10 for config reads, 30 for cluster calls), bare `apexalgo-iad-kalshi-oidc`
context untouched (§4/§8/§9 still apply). Outputs below are verbatim from the
beads' close records; both live checks were re-run at documentation time and
matched byte-for-byte.

### Check 1 — context and connection mode: PASS (`clasp-ea29db38`)

Full record in §9. Config-file reads only, all exit 0:

```
$ timeout 10 kubectl config current-context
apexalgo-iad-kalshi-oidc
# server https://hcp-e00b0a44-a342-4b61-8500-d4c90ece0c2d.spot.rackspace.com,
# cluster=iad-kalshi user=oidc, single oidc-login exec credential block

$ timeout 10 kubectl --server=http://kubectl-proxy-iad-kalshi:8001 get ns kube-system -o jsonpath='{.metadata.uid} {.metadata.name}'
0738fad8-7667-4635-8dd5-384d00810ecb kube-system
# exit 0 — the chosen endpoint, probed live for the first time
```

Chosen mode: **explicit `--server http://kubectl-proxy-iad-kalshi:8001`**, never
the bare context — the oidc-login exec plugin cannot authenticate headless
(browser-less box, empty token cache; §4/§8/§9, exit 124 under `timeout`).

### Check 2 — connectivity: PASS (`clasp-d8f8411b`)

The §9 connection mode exercised against the argocd namespace itself, exit 0
well under the 30s bound:

```
$ timeout 30 kubectl --server=http://kubectl-proxy-iad-kalshi:8001 get ns argocd
NAME     STATUS   AGE
argocd   Active   129d
# exit 0 — stderr empty
```

(First time this file records the **iad-kalshi** argocd namespace rather than
ardenone-cluster's — note the different AGE, 129d vs the 169d of §2/§5/§6/§7/§8:
distinct cluster, distinct namespace, both Active.)

### Check 3 — can-i read Applications: NO on iad-kalshi, yes on ardenone-cluster (`clasp-b371077d`)

Full record in §10, verbatim there. The dispatched probe on the check-2
connection:

```
$ timeout 30 kubectl --server=http://kubectl-proxy-iad-kalshi:8001 auth can-i get applications.argoproj.io -n argocd
Warning: the server doesn't have a resource type 'applications' in group 'argoproj.io'

no
# exit 1 — "auth can-i" exits 1 on a "no"
```

Attribution (§10, same round): the `no` is **not an RBAC denial** — iad-kalshi
serves no `argoproj.io` resources at all (`api-resources --api-group=argoproj.io`
→ empty table), has zero ArgoCD CRDs, and its `argocd` namespace holds no ArgoCD
workloads. The answering identity was iad-kalshi's kubectl-proxy SA
(`devpod-observer/devpod-observer`, UID `a8682a81-…`, pod
`kubectl-proxy-57b49c88bf-zqhvn`). On ardenone-cluster, where ArgoCD actually
runs, the same probes answer `yes` (get + list) via the tailnet proxy.

### Overall verdict (per check)

| Check | Verdict |
|---|---|
| Context / connection mode (`clasp-ea29db38`) | **PASS** — context `apexalgo-iad-kalshi-oidc` → iad-kalshi identified; connection mode = explicit `--server http://kubectl-proxy-iad-kalshi:8001` (credential-free, exit 0); bare-context headless auth FAIL by design (§4/§8/§9) |
| Connectivity (`clasp-d8f8411b`) | **PASS** — `get ns argocd` via the chosen endpoint → `argocd Active 129d`, exit 0, stderr empty |
| can-i read Applications (`clasp-b371077d`) | **NO on iad-kalshi** (exit 1 — but resource not served: no ArgoCD installed, not an RBAC denial) · **yes on ardenone-cluster** (exit 0, get + list) |

Standing conclusion, unchanged since §10: the cluster the configured context
points at has an `argocd` namespace but no ArgoCD; any check that actually wants
to read ArgoCD Applications must use `http://traefik-ardenone-cluster:8001`.
All cluster calls remain timeout-bounded and credential-free; nothing regressed
against any earlier round.

## 12. Re-verification against the pinned iad-kalshi endpoint — 2026-09-11
(`clasp-a4423070`, `clasp-6c87155b`, `clasp-f985699d`)

A fourth split round for umbrella `clasp-c86a7e10`, and the first consolidated
against **iad-kalshi** end to end — the cluster the configured context actually
points at — instead of ardenone-cluster. Three scoped children, assembled here
as child 4 of the split: `clasp-a4423070` (pins the one context and the
endpoint every later step must use), its reachability successor
`clasp-6c87155b`, and the RBAC child `clasp-f985699d`. Same method as every
round above: explicit credential-free `--server` tailnet endpoint, every
command under `timeout` (10 for config reads, 15 for cluster calls, 5 for the
deliberate wrong-endpoint probe) with stdin closed (`</dev/null`). All live
outputs below were re-run first-hand at documentation time (2026-09-11
01:00–01:07 UTC) and match the children's close records; stdout/stderr are
attributed per command.

### Pin — `clasp-a4423070`

Config-file reads only, no exec-plugin invocation, all exit 0:

```
$ timeout 10 kubectl config current-context
apexalgo-iad-kalshi-oidc

$ timeout 10 kubectl config view --minify -o jsonpath='{.contexts[0].context.cluster} {.contexts[0].context.user} clusterServer={.clusters[0].cluster.server}'
iad-kalshi oidc clusterServer=https://hcp-e00b0a44-a342-4b61-8500-d4c90ece0c2d.spot.rackspace.com
# matches §1/§7/§9 verbatim — Rackspace Spot hosted control plane
```

**Pinned endpoint: `http://kubectl-proxy-iad-kalshi:8001`.** iad-kalshi is the
one CLAUDE.md-table row with no Traefik kubectl-proxy route — the proxy Service
is exposed directly by the Tailscale operator — and the child's close record is
explicit that `traefik-iad-kalshi:8001` must not be substituted for it: the
name resolves (100.93.235.82) but refuses connections. That refusal is now
captured verbatim first-hand (stderr; five identical `E…memcache.go` lines,
first one shown, then the summary line):

```
$ timeout 5 kubectl --server=http://traefik-iad-kalshi:8001 get ns argocd </dev/null
E0910 21:06:41.758338 1889681 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: Get \"http://traefik-iad-kalshi:8001/api?timeout=32s\": dial tcp 100.93.235.82:8001: connect: connection refused"
# (… four more identical E-lines …)
The connection to the server traefik-iad-kalshi:8001 was refused - did you specify the right host or port?
# exit 1 — stdout empty
```

### Reachability — PASS (`clasp-6c87155b`)

```
$ timeout 15 kubectl --server=http://kubectl-proxy-iad-kalshi:8001 get ns argocd </dev/null
NAME     STATUS   AGE
argocd   Active   129d
# exit 0 — stderr empty

$ timeout 15 kubectl --server=http://kubectl-proxy-iad-kalshi:8001 version </dev/null
Client Version: v1.36.3
Kustomize Version: v5.8.1
Server Version: v1.34.9
# stderr: Warning: version difference between client (1.36) and server (1.34) exceeds the supported minor version skew of +/-1
# exit 0 — first time this file records the bare version probe; the skew warning is advisory, not a failure

$ timeout 15 kubectl --server=http://kubectl-proxy-iad-kalshi:8001 auth whoami </dev/null
ATTRIBUTE                                           VALUE
Username                                            system:serviceaccount:devpod-observer:devpod-observer
UID                                                 a8682a81-b2d8-4af6-a7b6-ca85ddb36360
Groups                                              [system:serviceaccounts system:serviceaccounts:devpod-observer system:authenticated]
Extra: authentication.kubernetes.io/credential-id   [JTI=13621b53-bf4b-4b0d-9753-fd47c3345f60]
Extra: authentication.kubernetes.io/node-name       [prod-instance-17854395072200685]
Extra: authentication.kubernetes.io/node-uid        [d9213868-f030-4f84-8f41-b81e5827d970]
Extra: authentication.kubernetes.io/pod-name        [kubectl-proxy-57b49c88bf-zqhvn]
Extra: authentication.kubernetes.io/pod-uid         [9627eb25-2a1c-4ab1-b09d-19023f49efd7]
# exit 0 — same iad-kalshi kubectl-proxy SA as §10 (UID, node and proxy pod all
# match); only the per-session JTI rotated
```

### RBAC — can-i read Applications: NO, CRD-absent, not a denial (`clasp-f985699d`)

```
$ timeout 15 kubectl --server=http://kubectl-proxy-iad-kalshi:8001 auth can-i get applications.argoproj.io -n argocd </dev/null
Warning: the server doesn't have a resource type 'applications' in group 'argoproj.io'

no
# exit 1 — stdout: no; stderr: the Warning above. The CRD-absent shape (§10),
# NOT an RBAC denial.

$ timeout 15 kubectl --server=http://kubectl-proxy-iad-kalshi:8001 api-resources --api-group=argoproj.io </dev/null
NAME   SHORTNAMES   APIVERSION   NAMESPACED   KIND
# exit 0 — empty table: no argoproj.io resource is served at all

$ timeout 15 kubectl --server=http://kubectl-proxy-iad-kalshi:8001 get ns argocd </dev/null
NAME     STATUS   AGE
argocd   Active   129d
# exit 0 — fallback probe: endpoint and identity both work, so the exit-1 above
# is about the resource, not the caller
```

Answer UNCHANGED from §10/§11: iad-kalshi → CRD absent (no ArgoCD installed —
the namespace exists but holds no ArgoCD, §10).

### Headless-OIDC caveat — still applies

The configured context is unchanged (reads above) and its oidc-login exec
plugin's token cache is still empty — the cache dir holds only a 0-byte
`.lock`, mtime 2026-08-07:

```
$ ls -la ~/.kube/cache/oidc-login/org_KsELolwAOxl3Zxfm/
total 8
drwx------ 2 coding users 4096 Aug  7 19:21 .
drwx------ 3 coding users 4096 Aug 19 07:32 ..
-rw------- 1 coding users    0 Aug  7 19:21 90e1f62b22c246b31866092f2b59453a9d1218ab22f4e4bb659eae988cbd2cc6.lock
```

So the §4/§8/§9 failure mode stands byte-for-byte; `clasp-a4423070` re-confirmed
it live the same day (`timeout 10 kubectl get ns` on the bare context blocks
until killed), and no new reproduction was run in this round. Every live check
above therefore still runs through the pinned `--server` endpoint, never the
bare context.

**Round verdict: pin PASS (context `apexalgo-iad-kalshi-oidc` → iad-kalshi;
endpoint `http://kubectl-proxy-iad-kalshi:8001`; the wrong-endpoint refusal
captured verbatim) · reachability PASS (`argocd Active 129d`, version probe
exit 0, answering identity = iad-kalshi's kubectl-proxy SA) · can-i read
Applications NO on iad-kalshi — CRD-absent, not an RBAC denial, unchanged
since §10 · headless use of `apexalgo-iad-kalshi-oidc` still FAIL (§4).**
Nothing regressed; §11's standing conclusion is unchanged — checks that
actually want to read ArgoCD Applications belong on
`http://traefik-ardenone-cluster:8001`.
