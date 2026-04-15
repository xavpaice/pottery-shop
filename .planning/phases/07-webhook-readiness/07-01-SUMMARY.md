---
phase: 07-webhook-readiness
plan: "01"
subsystem: infra
tags: [helm, kubernetes, rbac, jobs, webhook, cnpg, cert-manager]

# Dependency graph
requires:
  - phase: 06-subchart-dependencies
    provides: cloudnative-pg and cert-manager subchart toggles in values.yaml

provides:
  - ServiceAccount, ClusterRole, ClusterRoleBinding for webhook-wait Jobs (pre-install hooks at weight -25)
  - CNPG webhook-wait Job: blocks until cloudnative-pg operator deployment is ready
  - cert-manager webhook-wait Job: blocks until cert-manager-webhook deployment is ready
  - Conditional rendering: RBAC renders when either subchart enabled; each Job renders independently per toggle

affects: [08-hook-sequencing, ci-matrix]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Webhook-readiness via kubectl rollout status in a hook Job with alpine/k8s:1.35.3"
    - "RBAC wrapped in combined or-guard; Jobs get independent per-subchart guards"
    - "Hook weight ordering: RBAC -25 (pre-install), Jobs -20 (post-install)"
    - "hook-delete-policy: hook-succeeded,before-hook-creation for Jobs (vs before-hook-creation for CRs)"

key-files:
  created:
    - chart/clay/templates/webhook-wait-rbac.yaml
    - chart/clay/templates/webhook-wait-jobs.yaml
  modified: []

key-decisions:
  - "Use alpine/k8s:1.35.3 instead of bitnami/kubectl — bitnami versioned tags moved behind paywall Sept 2025"
  - "Separate guards per Job (not shared outer guard) so CNPG and cert-manager render independently"
  - "RBAC uses combined or-guard so SA/CR/CRB render whenever either operator is enabled"
  - "ClusterRole grants only get/list/watch on apps/deployments and apps/replicasets — no wildcards (T-07-01)"

patterns-established:
  - "Webhook-wait pattern: hook Job with kubectl rollout status targeting operator Deployment"
  - "RBAC weight -25 (pre-install,pre-upgrade), Job weight -20 (post-install,post-upgrade)"

requirements-completed:
  - WBHK-01
  - WBHK-02
  - WBHK-03
  - WBHK-04

# Metrics
duration: 2min
completed: 2026-04-15
---

# Phase 07 Plan 01: Webhook-Readiness Templates Summary

**Helm hook Jobs (CNPG + cert-manager) with RBAC that block CR creation until each operator's webhook deployment is confirmed ready via kubectl rollout status**

## Performance

- **Duration:** 2 min
- **Started:** 2026-04-15T03:29:06Z
- **Completed:** 2026-04-15T03:31:55Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments
- ServiceAccount, ClusterRole (get/list/watch on apps/deployments+replicasets only), and ClusterRoleBinding as pre-install hooks at weight -25, conditionally rendered when either subchart is enabled
- CNPG webhook-wait Job as post-install hook at weight -20, independently gated on `cloudnative-pg.enabled`
- cert-manager webhook-wait Job as post-install hook at weight -20, independently gated on `cert-manager.enabled`
- Verified all 4 toggle combinations (both enabled, CNPG only, cert-manager only, both disabled) produce correct output

## Task Commits

Each task was committed atomically:

1. **Task 1: Create webhook-wait RBAC hook template** - `db5f3ae` (feat)
2. **Task 2: Create webhook-wait Jobs hook template** - `f660776` (feat)

**Plan metadata:** (see final commit)

## Files Created/Modified
- `chart/clay/templates/webhook-wait-rbac.yaml` - ServiceAccount, ClusterRole, ClusterRoleBinding; pre-install hooks at weight -25; combined or-guard
- `chart/clay/templates/webhook-wait-jobs.yaml` - Two Job resources with independent guards; post-install hooks at weight -20; alpine/k8s:1.35.3 with kubectl rollout status

## Decisions Made
- **alpine/k8s:1.35.3 vs bitnami/kubectl:** The plan notes bitnami versioned tags moved behind paywall in Sept 2025. Used `alpine/k8s:1.35.3` instead (pinned version tag, satisfies T-07-02 supply-chain requirement).
- **Independent per-Job guards:** Each Job gets its own `{{- if (index .Values "..." "enabled") }}` guard so enabling only one operator renders only one Job. The RBAC uses a combined `or` guard since all three RBAC resources share the same condition.
- **hook-delete-policy for Jobs:** `hook-succeeded,before-hook-creation` (delete on success, re-create on re-run) vs CRs which use `before-hook-creation` only. This prevents stale completed Jobs from blocking re-runs.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
- Worktree was missing `chart/clay/charts/` directory (helm dependencies). Copied from main repo to enable `helm template` verification. The `.tgz` files are gitignored/untracked and were not committed.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- webhook-wait RBAC and Jobs are ready
- Phase 08 (hook-sequencing) can now convert cnpg-cluster.yaml to a post-install hook sequenced after the webhook-wait Jobs (weight > -20)
- No blockers

## Self-Check: PASSED

- FOUND: chart/clay/templates/webhook-wait-rbac.yaml
- FOUND: chart/clay/templates/webhook-wait-jobs.yaml
- FOUND: .planning/phases/07-webhook-readiness/07-01-SUMMARY.md
- CONFIRMED: db5f3ae (Task 1 commit)
- CONFIRMED: f660776 (Task 2 commit)
- PASS: helm template exits 0

---
*Phase: 07-webhook-readiness*
*Completed: 2026-04-15*
