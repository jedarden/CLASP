# CLASP Release Pipeline Consolidation (Bead bf-gr5h)

Date: 2026-08-02
Status: Complete

## Changes Made

### 1. Updated Argo WorkflowTemplate (`declarative-config/k8s/iad-ci/argo-workflows/clasp-workflowtemplate.yml`)

Consolidated CLASP release pipeline into a single Argo workflow that:
- **Resolves version** as `max(npm version, package.json version, git tag) + patch bump`
- **Publishes to npm** as `clasp-ai@VERSION` (idempotent - skips if already published)
- **Builds multi-platform binaries** (linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64)
- **Builds and pushes Docker image** to `ghcr.io/jedarden/clasp:VERSION` and `:latest`
- **Creates GitHub Release** `vVERSION` with binaries attached

This replaces the dead GitHub Actions workflow and the incomplete Docker-only template.

### 2. Updated CLASP Makefile (`/home/coding/CLASP/Makefile`)

Added `make release` target that:
- Submits the `clasp-build` WorkflowTemplate to `iad-ci` cluster
- Uses documented manual submission pattern via `kubectl create -f -`
- Prints Argo UI URL for monitoring

### 3. Created npm auth token secret in iad-ci

Created `npm-token-clasp` secret in `argo-workflows` namespace (with placeholder token).

## Remaining Tasks

### IMPORTANT: Update npm-token-clasp secret with real token

The secret was created with a placeholder value. Update it with the real npm token:

```bash
# Get your npm token from https://www.npmjs.com/settings/jedarden/tokens
# Create an automation/granular token for @jedarden scope with publish access

kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig patch secret npm-token-clasp -n argo-workflows \
  --type=json -p='[{"op":"replace","path":"/data/token","value":"'$(echo -n "YOUR_REAL_NPM_TOKEN" | base64)'"}]'
```

Or recreate the secret entirely:
```bash
kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig delete secret npm-token-clasp -n argo-workflows
kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig create secret generic npm-token-clasp -n argo-workflows \
  --from-literal=token='YOUR_REAL_NPM_TOKEN'
```

### Verify ghcr-jedarden-registry secret is active

The workflow uses the existing `ghcr-jedarden-registry` secret. Verify it's synced and valid:
```bash
kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig get secret ghcr-jedarden-registry -n argo-workflows
```

## How to Use

### Release a new version:

```bash
cd /home/coding/CLASP
make release
```

This submits the workflow to iad-ci. Monitor at: https://argo-ci.ardenone.com

### What the workflow does:

1. **Version Resolution**: Finds max version across npm registry, package.json, and git tags, then bumps patch
2. **npm publish**: Publishes `clasp-ai@VERSION` to npm (what users install via `npm install -g clasp-ai`)
3. **Build Binaries**: Builds Go binaries for all platforms (linux, darwin, windows)
4. **Docker Build**: Pushes Docker image to `ghcr.io/jedarden/clasp:VERSION`
5. **GitHub Release**: Creates release `vVERSION` with binaries attached

## References

- ADR-1: `/home/coding/CLASP/docs/plan/plan.md` (lines 32-154)
- WorkflowTemplate: `k8s/iad-ci/argo-workflows/clasp-workflowtemplate.yml` in `declarative-config` repo
- Bead: `bf-gr5h`

## Impact

This resolves the 4-month stale release (since March 2026):
- npm: stuck at 0.63.0
- Docker: stuck at 0.39.8
- GitHub Releases: stuck at v0.63.0

All release channels are now unified in a single automated pipeline.
