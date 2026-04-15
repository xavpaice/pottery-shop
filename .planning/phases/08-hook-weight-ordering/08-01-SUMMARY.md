---
phase: 08-hook-weight-ordering
plan: 01
subsystem: infra
tags: [helm, cnpg, cert-manager, hooks, hook-weight, kubernetes]

# Dependency graph
requires:
  - phase: 07-webhook-readiness
    provides: webhook-wait Jobs with hook-weight -20 and RBAC at -25; cert-manager-letsencrypt.yaml with hook annotations but no hook-weight

provides:
  - cnpg-cluster.yaml as post-install,post-upgrade hook at weight -10 with before-hook-creation delete policy
  - cert-manager-letsencrypt.yaml ClusterIssuer at hook-weight -5 and Certificate at hook-weight 0
  - G-20 through G-23 behavioral test assertions proving full weight-ordering sequence

affects: [09-ci-matrix, future phases that install or upgrade the clay chart]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Hook weight ordering: RBAC -25 < Jobs -20 < CNPG Cluster -10 < cert-manager CRs -5 to 0"
    - "Stateful CR hook delete policy: before-hook-creation only (not hook-succeeded) to prevent accidental DB destruction"
    - "Quoted-key annotation style for cnpg-cluster.yaml matching webhook-wait-jobs.yaml pattern"

key-files:
  created: []
  modified:
    - chart/clay/templates/cnpg-cluster.yaml
    - chart/clay/templates/cert-manager-letsencrypt.yaml
    - chart/tests/helm-template-test.sh

key-decisions:
  - "cnpg-cluster.yaml uses before-hook-creation only (not hook-succeeded) — stateful database CR must not be deleted when hook completes"
  - "cnpg-cluster.yaml uses quoted-key annotation style matching webhook-wait-jobs.yaml convention"
  - "ClusterIssuer at weight -5, Certificate at weight 0 — issuerRef dependency requires ClusterIssuer to exist first"

patterns-established:
  - "Hook weight ordering sequence fully established: RBAC -25, Jobs -20, CNPG Cluster -10, cert-manager CRs -5 to 0"
  - "TDD approach for Helm template changes: write failing behavioral tests first, then modify templates"

requirements-completed: [HOOK-01, HOOK-02]

# Metrics
duration: 2min
completed: 2026-04-15
---

# Phase 8 Plan 01: Hook Weight Ordering Summary

**cnpg-cluster.yaml promoted to post-install hook at weight -10 with before-hook-creation delete policy; cert-manager-letsencrypt ClusterIssuer at -5 and Certificate at 0; 7 new G-20 through G-23 behavioral tests prove full RBAC -25 < Jobs -20 < Cluster -10 < CRs -5..0 ordering sequence**

## Performance

- **Duration:** 2 min
- **Started:** 2026-04-15T04:05:50Z
- **Completed:** 2026-04-15T04:07:38Z
- **Tasks:** 2 (TDD: RED + GREEN)
- **Files modified:** 3

## Accomplishments

- Added `"helm.sh/hook": post-install,post-upgrade`, `"helm.sh/hook-weight": "-10"`, and `"helm.sh/hook-delete-policy": before-hook-creation` to cnpg-cluster.yaml, ensuring Helm sequences the CNPG Cluster CR after webhook-wait Jobs complete
- Added `helm.sh/hook-weight: "-5"` to letsencrypt ClusterIssuer and `helm.sh/hook-weight: "0"` to letsencrypt Certificate, completing the full hook weight chain
- Added 7 new behavioral assertions (G-20a/b/c, G-21, G-22, G-23a/b) to helm-template-test.sh; all 40 tests pass with zero regressions from G-01 through G-19

## Task Commits

Each task was committed atomically:

1. **Task 1: Add G-20 through G-23 hook-weight ordering assertions (RED)** - `d41fc1e` (test)
2. **Task 2: Add hook annotations to cnpg-cluster and cert-manager-letsencrypt (GREEN)** - `18d7b94` (feat)

## Files Created/Modified

- `chart/clay/templates/cnpg-cluster.yaml` - Added hook annotations block between labels and spec
- `chart/clay/templates/cert-manager-letsencrypt.yaml` - Added hook-weight -5 (ClusterIssuer) and 0 (Certificate)
- `chart/tests/helm-template-test.sh` - Updated header, added G-20 through G-23 test groups (90 lines added)

## Decisions Made

- Used `before-hook-creation` only for cnpg-cluster.yaml (not `hook-succeeded`): the CNPG Cluster CR is a stateful running database — deleting it after hook completion would destroy the database. This directly mitigates threat T-08-01 from the plan's threat model.
- Matched existing annotation quote styles per file: quoted-key (`"helm.sh/hook"`) for cnpg-cluster.yaml (matching webhook-wait-jobs.yaml), unquoted-key (`helm.sh/hook-weight:`) for cert-manager-letsencrypt.yaml (matching existing annotations in that file).
- ClusterIssuer weight -5, Certificate weight 0: Certificate's `issuerRef` points to the ClusterIssuer, so ClusterIssuer must be created first.

## Deviations from Plan

None - plan executed exactly as written.

Note: The plan specified "41 passed, 0 failed" but the actual count is 40. The plan counted 8 new tests (G-20a/b/c + G-21 + G-22 + G-23a/b = 7 distinct test IDs). The existing 33 tests + 7 new = 40 total. G-21 and G-22 reuse `OUTPUT_G20` rather than running separate helm invocations, which is correct and intentional per the plan's action spec. All test IDs are present and pass.

## Issues Encountered

Chart dependencies were missing (`charts/` directory empty) causing helm template to fail. Ran `helm dependency build` in `chart/clay/` to download cloudnative-pg and cert-manager subcharts before running tests. This is a standard worktree setup step, not a code issue.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Full hook weight ordering sequence established and verified: RBAC -25, Jobs -20, CNPG Cluster -10, cert-manager CRs -5 to 0
- 40 behavioral tests pass covering all requirements through Phase 8
- Ready for Phase 9 (CI matrix) — chart installs with correct hook sequencing in all operator toggle combinations

---
*Phase: 08-hook-weight-ordering*
*Completed: 2026-04-15*
