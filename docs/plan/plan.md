# CLASP — Plan

CLASP predates this workspace's `docs/plan/plan.md` convention, so this file
does not retroactively fabricate the original design. The real planning
history lives where it was actually written:

- `README.md` — user-facing feature/config/usage reference (source of truth
  for what CLASP claims to do)
- `docs/gap-analysis.md` — competitor feature comparison and running
  "Future Improvements" log (kept fairly current through v0.49.0)
- `docs/api-reference/`, `docs/translation-guides/` — protocol translation
  internals
- `initial-prompt.md`, `prompt.md` — the original autonomous-loop brief that
  bootstrapped the repo

## What CLASP actually is

A single-binary Go proxy (`cmd/clasp`) that sits in front of Claude Code and
translates the Anthropic Messages API to OpenAI-compatible Chat Completions /
Responses APIs in real time, so Claude Code can be pointed at OpenAI, Azure,
OpenRouter, Gemini, Grok, Qwen, MiniMax, DeepSeek, LiteLLM, Ollama, or any
custom OpenAI-compatible endpoint via `ANTHROPIC_BASE_URL`. It has no
server-side deployment of its own — it ships as a distributable artifact via
npm (`clasp-ai`), a Go module (`go install`), Docker images, and GitHub
release binaries, and users run it locally or in their own containers.

Going forward, architecturally significant decisions for this repo are
recorded below as ADRs, newest last.

---

## ADR-1: 2026-07-20 — Revive and unify the CLASP release pipeline

### Context

CLASP ships exclusively as published artifacts (npm package, Docker image,
Go module, GitHub release binaries) — there is no live service to check, so
"is the deployed artifact healthy" reduces to "do the published packages
reflect the repo." As of 2026-07-20 they do not, across every channel:

- **npm (`clasp-ai`)** — the README's "recommended" install path
  (`npm install -g clasp-ai` / `npx clasp-ai`). Registry shows the latest
  published version is `0.63.0`, published `2026-03-20T06:51:46Z`. Nothing
  has been published since.
- **GitHub Releases (binaries)** — `gh release list` shows the latest release
  is `v0.63.0`, created `2026-03-20`, matching the npm stall exactly.
- **`ghcr.io/jedarden/clasp`** (the Docker image the README and `Makefile`
  both reference) — tag list tops out at `0.39.8`, older still than the npm/
  release stall.
- **`ronaldraygun/clasp`** on Docker Hub — this is the *actual* push target
  of the `clasp-build` Argo WorkflowTemplate on `iad-ci`
  (`k8s/iad-ci/argo-workflows/clasp-workflowtemplate.yml` in
  `declarative-config`), created 2026-05-27. The repository does not exist
  on Docker Hub (`object not found`) and `kubectl get workflows -n
  argo-workflows | grep clasp` on `iad-ci` returns zero rows — this template
  has **never run once** since it was created. There is also no Sensor/
  EventSource wired to it (or to anything else in the `argo-workflows`
  namespace on `iad-ci`), so nothing triggers it on push.

Meanwhile `main` kept moving for four more months after the March stall:
LiteLLM backend provider, prompt-cache simulation wired into the handler,
compaction/multi-window context, MCP server mode, Grok/Qwen/MiniMax
providers, stream-timeout UX, a real `.air.toml` dev loop, and a README
rewrite — none of it reachable by a user who installs CLASP the way the
README tells them to.

This is not just a documentation staleness problem. `clasp update`
(`cmd/clasp/handlers.go: handleUpdateCommand`) — CLASP's own self-update
command from the original spec (`initial-prompt.md`: "CLASP should also have
some form of self-updating capability") — shells out to
`npm view clasp-ai version` to decide whether an update exists. Because npm
is stuck at `0.63.0`, the self-update mechanism itself is structurally
incapable of noticing four months of missed releases; it will happily report
"you're up to date" against a version that is itself stale.

Root cause: GitHub Actions is disabled at the repo level for `jedarden/CLASP`
(`gh api repos/jedarden/CLASP/actions/permissions` → `enabled: false`),
consistent with this workspace's fleet-wide "GitHub Actions are disabled —
use Argo instead" policy. `.github/workflows/release.yml` (the old
version-bump + npm-publish + gh-release automation) has been dead since
Actions were turned off, and nothing replaced its full scope — the Argo
`clasp-build` template that was eventually created only builds/pushes a
Docker image (to a registry nobody uses), never publishes npm or uploads
release binaries, and even that has never been triggered.

### Decision

Adopt a single Argo Workflow on `iad-ci` as the canonical, sole release
pipeline for CLASP, replacing the dead `release.yml` and superseding the
existing `clasp-build` template's narrow scope. The pipeline must, in one
run:

1. Resolve the release version as
   `max(package.json version, latest `v*` git tag, published npm `clasp-ai`
   version) + patch bump` — never derive it from `package.json` alone, which
   is what caused today's drift (repo `package.json` currently reads
   `0.50.23`, *below* the already-published `0.63.0`).
2. `npm publish` to `clasp-ai` (this is what `clasp update` and the README's
   recommended install both depend on — it is the must-fix channel).
3. Build and push exactly one Docker image tag set, to
   `ghcr.io/jedarden/clasp` — the registry already referenced by README,
   `Makefile`, and `Dockerfile`. Retire `ronaldraygun/clasp` as a target
   rather than maintaining two.
4. Build release binaries (`make build-all`) and attach them to a GitHub
   Release (`gh release create`/`upload`), keeping `clasp update`'s binary
   fallback path and the Go-install path consistent with the same version.

Until a push-triggered Sensor/EventSource exists on `iad-ci` (none exist
today, for any repo, not just CLASP — a fleet-level gap out of scope for this
ADR), the pipeline is invoked manually via a new `make release` target that
submits the workflow with `kubectl create -f -` against
`iad-ci.kubeconfig`, so releasing CLASP is a one-command action instead of
four independent manual `make` targets that have to be remembered and run in
sequence (which is how this drifted in the first place).

### Alternatives Considered

- **Re-enable GitHub Actions for this repo.** Rejected — directly
  contradicts this workspace's standing policy ("Never re-enable GH
  Actions — use Argo instead").
- **Keep the status quo (`make npm-publish`, `make docker-push`,
  `make release-binaries` run manually, ad hoc).** Rejected — this *is* the
  status quo, and it silently produced a four-month, three-channel staleness
  gap that nobody noticed until this audit. A process with no automation and
  no alerting around "did the last release actually happen" is not a
  process.
- **Fix only the `clasp-build` Argo template in place (Docker-only) and
  leave npm/GitHub Releases as separate manual steps.** Rejected — npm is
  the more consequential channel (it's the recommended install path *and*
  what `clasp update` polls); fixing Docker alone would leave the worse half
  of the problem in place.
- **Keep both `ghcr.io/jedarden/clasp` and `ronaldraygun/clasp` as parallel
  Docker targets.** Rejected — two registries that both claim to be "the"
  CLASP image, one of which is unreferenced by any user-facing doc, is how
  registry-target confusion happens; consolidate to the one the docs already
  point at.

### Consequences

- **Positive:** one command releases npm + Docker + binaries together, so
  they can no longer drift independently; `clasp update` and the README's
  install instructions become trustworthy again; version numbering stops
  regressing.
- **Negative / follow-up work required:** the `iad-ci` cluster needs an npm
  publish token available as a Workflow secret (does not appear to exist
  yet — the current template only wires a GitHub token); the
  `clasp-workflowtemplate.yml` manifest in `declarative-config` needs to be
  rewritten to add the npm-publish and release-binary steps and to point
  Docker at `ghcr.io/jedarden/clasp`, and that change must go through the
  normal GitOps flow (edit manifest in `declarative-config` → commit → push
  → ArgoCD sync) — it is **not** performed as part of this audit, since this
  audit's write scope is the CLASP repo itself. A follow-up bead tracks it.
  Push-triggered auto-release (a Sensor/EventSource) remains out of scope
  here — it's a fleet-wide gap, not specific to CLASP.
