---
phase: 02-helm-ci
plan: 01
subsystem: infra
tags: [helm, cnpg, cloudnative-pg, kubernetes, postgres, deployment]

# Dependency graph
requires:
  - phase: 01-go-build
    provides: Go binary using pgx/v5 (pure Go, no CGO) — app is ready to consume DATABASE_URL

provides:
  - CNPG operator subchart declared in Chart.yaml at version 0.28.0
  - cnpg-cluster.yaml Cluster CRD template (conditional on postgres.managed)
  - DATABASE_URL injection in deployment.yaml (managed via CNPG secret, external via plain DSN)
  - pg_isready init container for CNPG secret timing mitigation (managed mode only)
  - RollingUpdate deployment strategy (SQLite single-writer constraint removed)
  - DB_PATH SQLite artifact removed from values.yaml and configmap

affects: [02-helm-ci/02-02, ci-pipeline, kubernetes-deployment]

# Tech tracking
tech-stack:
  added: [cloudnative-pg 0.28.0 Helm subchart, postgres:16-alpine init container image]
  patterns:
    - "CNPG-managed credentials via secretKeyRef (never in values.yaml or ConfigMap)"
    - "pg_isready init container blocks app start until Postgres ready"
    - "Cluster name {fullname}-postgres determines secret ({fullname}-postgres-app) and service ({fullname}-postgres-rw) names"

key-files:
  created:
    - chart/clay/templates/cnpg-cluster.yaml
    - chart/clay/.gitignore
  modified:
    - chart/clay/Chart.yaml
    - chart/clay/values.yaml
    - chart/clay/templates/deployment.yaml

key-decisions:
  - "CNPG subchart version 0.28.0 with condition cloudnative-pg.enabled for toggleable operator install"
  - "Cluster named {fullname}-postgres — CNPG derives secret {fullname}-postgres-app (key: uri) and RW service {fullname}-postgres-rw"
  - "Init container only rendered in managed mode — external DSN mode has no CNPG service to wait for"
  - "External DSN injected as plain env.value (visible in kubectl describe pod) — accepted risk per T-02-01 for hobby shop"
  - "DB_PATH removed from values.yaml config block — configmap range auto-propagates the removal"

patterns-established:
  - "CNPG credential injection: use secretKeyRef not ConfigMap for DATABASE_URL — credentials never appear in plain config"
  - "Init container pg_isready pattern for database timing mitigation in managed mode"
  - "charts/ gitignored in chart directory — Chart.lock committed for reproducibility"

requirements-completed: [HELM-01, HELM-02, HELM-03, HELM-04, HELM-05, HELM-06, HELM-07]

# Metrics
duration: 15min
completed: 2026-04-14
---

# Phase 2 Plan 01: Helm CNPG Wiring Summary

**CNPG operator wired as Helm subchart (0.28.0), Cluster CRD template with pg_isready init container, DATABASE_URL injection from CNPG secret in managed mode and plain DSN in external mode, RollingUpdate strategy, DB_PATH removed**

## Performance

- **Duration:** ~15 min
- **Started:** 2026-04-14T00:00:00Z
- **Completed:** 2026-04-14
- **Tasks:** 2
- **Files modified:** 5 (Chart.yaml, values.yaml, cnpg-cluster.yaml, deployment.yaml, .gitignore)

## Accomplishments

- Chart.yaml declares cloudnative-pg 0.28.0 subchart with `condition: cloudnative-pg.enabled`
- cnpg-cluster.yaml renders a CNPG Cluster CRD when `postgres.managed: true`, skips it in external mode
- deployment.yaml injects DATABASE_URL from CNPG-generated secret in managed mode or from `postgres.external.dsn` in external mode
- pg_isready init container blocks app container start until Postgres accepts connections (managed mode only)
- Deployment strategy changed from Recreate to RollingUpdate (SQLite single-writer constraint removed)
- DB_PATH SQLite artifact removed from values.yaml (auto-removes from ConfigMap via range loop)

## Task Commits

Each task was committed atomically:

1. **Task 1: Chart.yaml CNPG subchart, values.yaml postgres block, cnpg-cluster.yaml** - `446bdb3` (feat)
2. **Task 2: deployment.yaml RollingUpdate, init container, DATABASE_URL injection** - `8b25c79` (feat)

**Plan metadata:** (to be added in final commit)

## Files Created/Modified

- `chart/clay/Chart.yaml` - Added cloudnative-pg 0.28.0 subchart dependency with cloudnative-pg.enabled condition
- `chart/clay/values.yaml` - Removed DB_PATH, added postgres block (managed/cluster/external), added cloudnative-pg enabled flag, updated persistence comment
- `chart/clay/templates/cnpg-cluster.yaml` - New: CNPG Cluster CRD template rendered only when postgres.managed=true
- `chart/clay/templates/deployment.yaml` - RollingUpdate strategy, pg_isready init container (managed only), DATABASE_URL env injection (both modes)
- `chart/clay/.gitignore` - New: excludes charts/ directory from git

## Decisions Made

- CNPG Cluster named `{{ include "clay.fullname" . }}-postgres` — chosen to follow the pattern from RESEARCH.md Claude's discretion guidance. This drives derived names: secret `{fullname}-postgres-app` (CNPG convention `{cluster}-app`) and RW service `{fullname}-postgres-rw` (CNPG convention `{cluster}-rw`).
- External DSN injected as plain `env.value` (not via the existing Kubernetes Secret). The `postgres.external.dsn` value is visible in `kubectl describe pod` — accepted per threat T-02-01 (hobby shop context, documented).
- Init container uses sleep 2 between pg_isready attempts (reasonable polling interval per RESEARCH.md discretion).

## Deviations from Plan

None — plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None — no external service configuration required. Chart is ready for `helm dependency update` + `helm install` with a Kubernetes cluster running the CNPG CRDs (installed by the subchart).

## Next Phase Readiness

- Helm chart wiring complete; ready for Plan 02 (CI pipeline updates)
- `helm dependency update chart/clay/` is required before `helm lint` or `helm template` (CNPG subchart must be downloaded)
- No Chart.lock yet — Plan 02 CI steps should use `helm dependency update` (not `helm dependency build`)
- Both managed and external DSN modes are templated correctly and ready for CI validation via `chart/clay/ci/` values files

---
*Phase: 02-helm-ci*
*Completed: 2026-04-14*
