---
phase: 05-ci-validation-extension
plan: "01"
subsystem: infra
tags: [helm, ci, tls, cert-manager, letsencrypt, selfsigned, custom]

# Dependency graph
requires:
  - phase: 04-cert-manager-cr-templates
    provides: Ingress TLS templates for letsencrypt/selfsigned/custom modes in the clay chart
provides:
  - chart/clay/ci/tls-letsencrypt-values.yaml — self-contained CI values for letsencrypt TLS mode
  - chart/clay/ci/tls-selfsigned-values.yaml — self-contained CI values for selfsigned TLS mode
  - chart/clay/ci/tls-custom-values.yaml — self-contained CI values for custom TLS mode (secretName: my-tls)
affects:
  - CI workflow (integration-test.yml) — plan 05-02 extends lint/template jobs to use these files
  - helm-template-test.sh — tls-custom-values.yaml secretName: my-tls matches CUSTOM_INGRESS variable

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Self-contained CI values files: each file is a complete --values overlay, no layering required"
    - "TLS CI baseline: managed postgres + CI placeholder secrets + ingress TLS block"

key-files:
  created:
    - chart/clay/ci/tls-letsencrypt-values.yaml
    - chart/clay/ci/tls-selfsigned-values.yaml
    - chart/clay/ci/tls-custom-values.yaml
  modified: []

key-decisions:
  - "Each TLS CI file is self-contained (D-03): includes managed Postgres baseline and CI placeholder secrets — no layering with other values files"
  - "custom mode secretName: my-tls matches CUSTOM_INGRESS variable in helm-template-test.sh (D-06)"
  - "letsencrypt mode uses staging ACME email admin@example.com — consistent with chart default (production: false)"

patterns-established:
  - "TLS CI values pattern: managed postgres + ci-test placeholder secrets + ingress.enabled=true + tls.mode"

requirements-completed: [CI-04]

# Metrics
duration: 1min
completed: "2026-04-14"
---

# Phase 5 Plan 01: CI Validation Extension — TLS Values Files Summary

**Three self-contained TLS CI values files added to chart/clay/ci/ covering letsencrypt, selfsigned, and custom modes, each passing helm lint and helm template independently**

## Performance

- **Duration:** ~1 min
- **Started:** 2026-04-14T23:29:28Z
- **Completed:** 2026-04-14T23:29:45Z
- **Tasks:** 1 completed
- **Files modified:** 3 created

## Accomplishments

- Created `tls-letsencrypt-values.yaml` with ACME staging email, host, and `mode: letsencrypt`
- Created `tls-selfsigned-values.yaml` with `mode: selfsigned` and no extra keys (no acme, no secretName)
- Created `tls-custom-values.yaml` with `mode: custom` and `secretName: my-tls` matching `CUSTOM_INGRESS` in `helm-template-test.sh`
- All three files pass `helm lint` and `helm template` independently
- letsencrypt renders 1 ClusterIssuer + 1 Certificate + Ingress; selfsigned renders 2 ClusterIssuers + 2 Certificates + Ingress; custom renders Ingress only (zero cert-manager resources)

## Task Commits

Each task was committed atomically:

1. **Task 1: Create three TLS CI values files** - `c33244b` (feat)

**Plan metadata:** (docs commit follows this summary)

## Files Created/Modified

- `chart/clay/ci/tls-letsencrypt-values.yaml` — Self-contained CI values for letsencrypt TLS mode (ACME staging, email: admin@example.com)
- `chart/clay/ci/tls-selfsigned-values.yaml` — Self-contained CI values for selfsigned TLS mode (two-step CA bootstrap, no ACME)
- `chart/clay/ci/tls-custom-values.yaml` — Self-contained CI values for custom TLS mode (secretName: my-tls matching test script)

## Decisions Made

None - followed plan as specified. File contents are exact as dictated in the plan's `<action>` block.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None. All six helm operations (3x lint, 3x template) passed on first run.

## Known Stubs

None. These files are complete CI test fixtures — no placeholder data flows to UI rendering.

## Threat Flags

No new security-relevant surface introduced. The CI values files use intentional placeholder secrets (`ci-test-only`) matching the established pattern in `managed-values.yaml` and `external-values.yaml`. No new network endpoints, auth paths, or schema changes.

## Next Phase Readiness

- CI-04 fully satisfied: all three TLS mode CI values files exist and validate cleanly
- Ready for plan 05-02: extend `.github/workflows/integration-test.yml` lint/template jobs to exercise these three new files
- `tls-custom-values.yaml` secretName (`my-tls`) is consistent with `helm-template-test.sh` CUSTOM_INGRESS variable — no coordination needed

---
*Phase: 05-ci-validation-extension*
*Completed: 2026-04-14*
