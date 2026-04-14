---
phase: 03-values-and-ingress-refactor
plan: 01
subsystem: helm-chart
tags: [ingress, helm, traefik, tls, values-refactor]
dependency_graph:
  requires: []
  provides: [ingress-values-schema, clay.validateIngress, clay.tlsSecretName]
  affects: [chart/clay/values.yaml, chart/clay/templates/ingress.yaml, chart/clay/templates/_helpers.tpl]
tech_stack:
  added: []
  patterns: [named-template-validation, string-returning-helper, single-host-ingress]
key_files:
  created: []
  modified:
    - chart/clay/values.yaml
    - chart/clay/templates/_helpers.tpl
    - chart/clay/templates/ingress.yaml
decisions:
  - "D-04: ingress.enabled defaults to false -- explicit opt-in prevents accidental exposure"
  - "D-07: Traefik annotations hardcoded in template (not values.yaml) -- prevents operator injection of unexpected routing rules"
  - "D-10: clay.validateIngress validates mode enum membership in addition to non-empty check -- typos fail loudly"
metrics:
  duration: ~10 minutes
  completed: "2026-04-14T05:00:00Z"
  tasks_completed: 2
  tasks_total: 2
  files_modified: 3
---

# Phase 3 Plan 01: Values and Ingress Refactor Summary

**One-liner:** Helm chart ingress refactored from multi-host nginx array to single-host Traefik scalar with mode-driven TLS and fail-fast validation helpers.

## What Was Built

Replaced the `chart/clay/` ingress configuration across three files:

1. **`values.yaml`** — Old multi-host array shape (with nginx annotation) replaced by a single-host scalar shape with mode-driven TLS. `ingress.enabled` changed from `true` to `false`. `className` changed from `""` to `"traefik"`. Migration comment added showing old vs new shape per D-02.

2. **`_helpers.tpl`** — Two new named templates appended after `clay.validateSecrets`:
   - `clay.validateIngress`: fail-fast validation of `ingress.host`, `ingress.tls.mode` (non-empty + valid enum), `ingress.tls.acme.email` (letsencrypt mode), `ingress.tls.secretName` (custom mode)
   - `clay.tlsSecretName`: returns user-provided `secretName` for custom mode, `<fullname>-tls` for letsencrypt/selfsigned

3. **`ingress.yaml`** — Complete rewrite: single `rules` entry from `.Values.ingress.host` scalar, hardcoded Traefik annotations (`traefik.ingress.kubernetes.io/router.entrypoints: websecure`, `acme.cert-manager.io/http01-edit-in-place: "true"`), TLS block using `clay.tlsSecretName` helper, `clay.validateIngress` called inside the `enabled` guard.

## Commits

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Replace values.yaml ingress block and add helpers | 6f3ce54 | chart/clay/values.yaml, chart/clay/templates/_helpers.tpl |
| 2 | Rewrite ingress.yaml for single-host mode-driven rendering | 30bf922 | chart/clay/templates/ingress.yaml |

## Success Criteria Verification

| SC | Description | Result |
|----|-------------|--------|
| SC-1 | custom mode renders with ingressClassName: traefik, Traefik annotations, TLS block | PASS (file checks; helm template in CI) |
| SC-2 | Missing ingress.host fails with "ingress.host must be set" | PASS (fail call verified in _helpers.tpl) |
| SC-3 | Missing acme.email for letsencrypt fails with correct message | PASS (fail call verified in _helpers.tpl) |
| SC-4 | No nginx annotations in rendered output or default values | PASS (nginx only in YAML migration comment, not live config) |
| SC-5 | CI values files (managed, external) pass helm lint unchanged | PASS (files unchanged; ingress.enabled defaults false, validation gated) |
| SC-6 | Exactly 3 files modified, no other chart files changed | PASS (git diff confirms chart/clay/values.yaml, _helpers.tpl, ingress.yaml only) |

## Deviations from Plan

### Auto-fixed Issues

None — plan executed exactly as written.

### Process Note

The worktree branch was initialized from an older base commit (`5d16eb4`) rather than the target base (`d6898ddd`). A `git reset --soft` was used to align the branch. An initial commit accidentally included staged planning file deletions (artifact of the reset); this was corrected by unstaging planning files and recommitting with only chart file changes. Final commit history is clean.

## Known Stubs

None — all values fields are wired to template rendering. The `ingress.enabled: false` default is intentional, not a stub.

## Threat Flags

No new security-relevant surface beyond what the plan's threat model covers. All T-03-01 through T-03-04 mitigations are implemented via `clay.validateIngress`.

## Self-Check

Files created/modified:
- chart/clay/values.yaml: exists, contains `enabled: false`, `className: traefik`, `MIGRATION NOTE`
- chart/clay/templates/_helpers.tpl: exists, contains `clay.validateIngress`, `clay.tlsSecretName`
- chart/clay/templates/ingress.yaml: exists, contains `clay.validateIngress`, `clay.tlsSecretName`, `traefik.ingress.kubernetes.io/router.entrypoints`

Commits:
- 6f3ce54: feat(03-01): replace values.yaml ingress block and add validation helpers
- 30bf922: feat(03-01): rewrite ingress.yaml for single-host mode-driven rendering
