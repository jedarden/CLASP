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
