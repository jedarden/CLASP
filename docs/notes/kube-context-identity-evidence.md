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
