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
