---
phase: 02-helm-ci
verified: 2026-04-14T02:23:07Z
status: human_needed
score: 11/12 must-haves verified
overrides_applied: 0
human_verification:
  - test: "Run helm install on a cluster with the CNPG operator installed and verify the app pod reaches Running state"
    expected: "App pod transitions from Init:0/1 to Running; readiness probe passes at /; no manual intervention required"
    why_human: "Requires a live Kubernetes cluster with CNPG CRDs installed. Cannot test helm install outcome programmatically without a cluster."
  - test: "Run helm install in external-DSN mode (postgres.managed=false, postgres.external.dsn pointing to a real Postgres) and verify DATABASE_URL is correctly injected"
    expected: "App pod starts, DATABASE_URL env var is populated with the external DSN value, app serves requests"
    why_human: "Requires a running external Postgres instance and cluster access to verify runtime env injection."
---

# Phase 2: Helm + CI Verification Report

**Phase Goal:** The Helm chart supports both managed CNPG and external Postgres, the app pod starts reliably on first `helm install`, and the CI pipeline validates build, tests, and Helm rendering on every push.
**Verified:** 2026-04-14T02:23:07Z
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|---------|
| 1 | `helm template` renders a CNPG Cluster resource when postgres.managed is true | ✓ VERIFIED | `cnpg-cluster.yaml` has `{{- if .Values.postgres.managed }}` guard; `managed-values.yaml` sets `managed: true`; CI runs `helm template clay chart/clay/ --values chart/clay/ci/managed-values.yaml` |
| 2 | `helm template` does NOT render a CNPG Cluster when postgres.managed is false | ✓ VERIFIED | `cnpg-cluster.yaml` wraps entire resource in `{{- if .Values.postgres.managed }}` / `{{- end }}`; `external-values.yaml` sets `managed: false` |
| 3 | DATABASE_URL is injected from CNPG secret in managed mode | ✓ VERIFIED | `deployment.yaml` lines 41–47: `secretKeyRef` references `{{ include "clay.fullname" . }}-postgres-app`, key `uri`, guarded by `{{- if .Values.postgres.managed }}` |
| 4 | DATABASE_URL is injected from postgres.external.dsn in external mode | ✓ VERIFIED | `deployment.yaml` lines 48–51: `{{- else if .Values.postgres.external.dsn }}` injects `value: {{ .Values.postgres.external.dsn \| quote }}`; `external-values.yaml` provides `dsn: "postgresql://user:pass@external-host:5432/clay"` |
| 5 | DB_PATH does not appear anywhere in values.yaml or rendered configmap output | ✓ VERIFIED | `grep -rn "DB_PATH" chart/` returns no results; `configmap.yaml` uses `range .Values.config` — DB_PATH removed from `values.yaml` config block means it cannot appear in rendered ConfigMap |
| 6 | Deployment strategy is RollingUpdate, not Recreate | ✓ VERIFIED | `deployment.yaml` line 10: `type: RollingUpdate`; no `Recreate` present |
| 7 | Init container only renders in managed mode | ✓ VERIFIED | `deployment.yaml` lines 23–36: `initContainers` block wrapped in `{{- if .Values.postgres.managed }}` / `{{- end }}` |
| 8 | CI test job does not install gcc or any C compiler | ✓ VERIFIED | `test.yml`: no `apt-get`, no `gcc` — "Install dependencies" step removed |
| 9 | CI test job runs go vet, make test, and make build with CGO_ENABLED=0 | ✓ VERIFIED | `test.yml` has `go vet ./...` step; `make test` and `make build` already set `CGO_ENABLED=0` in Makefile |
| 10 | CI helm-lint job runs helm dependency update before linting | ✓ VERIFIED | `test.yml` line 46: `helm dependency update chart/clay/`; preceded by `helm repo add cnpg` |
| 11 | CI helm-lint job lints and templates in both managed and external modes | ✓ VERIFIED | `test.yml`: 4 steps — `helm lint` x2 and `helm template` x2, each with `--values chart/clay/ci/managed-values.yaml` or `--values chart/clay/ci/external-values.yaml` |
| 12 | App pod starts reliably on first `helm install` (SC-2) | ? HUMAN NEEDED | Cannot verify pod lifecycle without a live cluster; init container and RollingUpdate strategy are correctly configured for this |

**Score:** 11/12 truths verified (1 requires human)

### Deferred Items

No items deferred. There is no Phase 3 in the roadmap to receive deferred work.

**Note — golangci-lint (CI-02 partial):** REQUIREMENTS.md CI-02 specifies `go vet + golangci-lint + go test`. The plan's must_haves omit golangci-lint. RESEARCH.md explicitly recommended skipping it for this phase (Pattern 3, lines 476–479: "Skip golangci-lint for now — it requires an action install step and the phase scope is focused on Postgres/Helm"). The plan claims CI-02 as complete. The omission is a documented scope decision, not an oversight, but `golangci-lint` is not currently run anywhere in CI.

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|---------|--------|---------|
| `chart/clay/Chart.yaml` | CNPG subchart dependency at version 0.28.0 | ✓ VERIFIED | Contains `cloudnative-pg 0.28.0` dependency with `condition: cloudnative-pg.enabled` |
| `chart/clay/values.yaml` | postgres block with managed/external config, no DB_PATH | ✓ VERIFIED | Has `postgres:` block; no `DB_PATH`; `UPLOAD_DIR` preserved |
| `chart/clay/templates/cnpg-cluster.yaml` | CNPG Cluster CRD template conditional on postgres.managed | ✓ VERIFIED | `apiVersion: postgresql.cnpg.io/v1`, `kind: Cluster`, guarded by `if .Values.postgres.managed` |
| `chart/clay/templates/deployment.yaml` | Init container, DATABASE_URL injection, RollingUpdate strategy | ✓ VERIFIED | All three features present and correctly gated |
| `.github/workflows/test.yml` | Updated test job (no gcc) and extended helm-lint job (dependency update, dual-mode lint+template) | ✓ VERIFIED | No gcc; go vet added; helm dependency update + 4-step dual-mode validation |
| `chart/clay/ci/managed-values.yaml` | Test values for managed CNPG mode | ✓ VERIFIED | `managed: true`, `cloudnative-pg: enabled: true`, 1 instance, 1Gi storage |
| `chart/clay/ci/external-values.yaml` | Test values for external DSN mode | ✓ VERIFIED | `managed: false`, `cloudnative-pg: enabled: false`, dummy DSN present |
| `chart/clay/.gitignore` | Excludes charts/ directory | ✓ VERIFIED | Contains `charts/` |

### Key Link Verification

(Note: gsd-tools returned false negatives for Plan 01 key-links due to regex escaping of `.` in Helm template syntax. Manual verification used below.)

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `deployment.yaml` | `cnpg-cluster.yaml` | secretKeyRef name = Cluster name + `-app` | ✓ WIRED | Cluster: `{fullname}-postgres`; secretKeyRef: `{fullname}-postgres-app` — consistent CNPG naming convention |
| `deployment.yaml` | `cnpg-cluster.yaml` | init container host = Cluster name + `-rw` | ✓ WIRED | Cluster: `{fullname}-postgres`; pg_isready host: `{fullname}-postgres-rw` — consistent CNPG naming convention |
| `chart/clay/Chart.yaml` | `chart/clay/values.yaml` | subchart condition key `cloudnative-pg.enabled` | ✓ WIRED | `Chart.yaml` has `condition: cloudnative-pg.enabled`; `values.yaml` has `cloudnative-pg: enabled: true` |
| `test.yml` | `chart/clay/ci/managed-values.yaml` | helm lint/template --values references | ✓ WIRED | Referenced 2x in test.yml (lint + template) |
| `test.yml` | `chart/clay/ci/external-values.yaml` | helm lint/template --values references | ✓ WIRED | Referenced 2x in test.yml (lint + template) |
| `test.yml` | `chart/clay/Chart.yaml` | helm dependency update downloads subchart | ✓ WIRED | `helm dependency update chart/clay/` present; preceded by `helm repo add cnpg` |

### Data-Flow Trace (Level 4)

Not applicable — this phase produces Helm chart templates and CI workflow YAML, not components that render dynamic runtime data. The data flow (DATABASE_URL → app container) is verified at the Helm template level; runtime behavior requires human cluster testing.

### Behavioral Spot-Checks

Helm is not installed in this environment, so `helm template` spot-checks cannot be run. The CI workflow (test.yml) would perform these checks on every push.

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| helm template syntax valid (managed mode) | `helm template clay chart/clay/ --values chart/clay/ci/managed-values.yaml` | SKIP — no helm binary | ? SKIP |
| helm template syntax valid (external mode) | `helm template clay chart/clay/ --values chart/clay/ci/external-values.yaml` | SKIP — no helm binary | ? SKIP |
| No DB_PATH in values.yaml | `grep -rn "DB_PATH" chart/` | No output | ✓ PASS |
| No gcc/apt-get in test.yml | `grep -n "apt-get\|gcc" .github/workflows/test.yml` | No output | ✓ PASS |
| All 4 commits exist in git | `git log --oneline 446bdb3 8b25c79 9db12f7 0594f7d` | All 4 found | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|---------|
| HELM-01 | 02-01 | Add CNPG operator as Helm subchart dependency (chart 0.28.0, condition: `cloudnative-pg.enabled`) | ✓ SATISFIED | `Chart.yaml` dependencies block; version 0.28.0; condition key present |
| HELM-02 | 02-01 | Add `postgres` block to values.yaml with managed, cluster.instances, cluster.storage.size, storageClass | ✓ SATISFIED | `values.yaml` lines 86–98: complete postgres block |
| HELM-03 | 02-01 | Create `templates/cnpg-cluster.yaml` rendered only when `postgres.managed: true` | ✓ SATISFIED | `cnpg-cluster.yaml` exists with `{{- if .Values.postgres.managed }}` guard |
| HELM-04 | 02-01 | Inject `DATABASE_URL` from CNPG-generated Secret (`<cluster>-app`, key `uri`) in managed mode | ✓ SATISFIED | `deployment.yaml`: `secretKeyRef` to `{fullname}-postgres-app`, key `uri` |
| HELM-05 | 02-01 | Inject `DATABASE_URL` from `postgres.external.dsn` plain value in external mode | ✓ SATISFIED | `deployment.yaml`: `{{- else if .Values.postgres.external.dsn }}` injects plain value |
| HELM-06 | 02-01 | Add timing mitigation for CNPG Secret race (initContainer or startup probe) | ✓ SATISFIED | `deployment.yaml`: `wait-for-postgres` init container with `pg_isready` in managed mode |
| HELM-07 | 02-01 | Remove `DB_PATH` SQLite artifact from values and configmap | ✓ SATISFIED | `DB_PATH` absent from `values.yaml`; configmap uses range, auto-propagates removal |
| CI-01 | 02-02 | Update build job: set `CGO_ENABLED=0`, remove CGO cross-compile steps | ✓ SATISFIED | gcc step removed; `make build` uses `CGO_ENABLED=0` |
| CI-02 | 02-02 | Add test job: `go vet` + `golangci-lint` + `go test` (with testcontainers-go Postgres) | ⚠️ PARTIAL | `go vet ./...` added; `make test` covers testcontainers-go; **golangci-lint is absent** — RESEARCH.md explicitly deferred it, but it is still listed in the requirement |
| CI-03 | 02-02 | Add Helm validation job: `helm lint` + `helm template` render check | ✓ SATISFIED | helm-lint job runs lint x2 + template x2 for both modes after dependency update |

**Orphaned requirements check:** All requirements mapped to Phase 2 in REQUIREMENTS.md (HELM-01 through HELM-07, CI-01 through CI-03) are covered by plans 02-01 and 02-02. No orphaned requirements.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `.github/workflows/integration-test.yml` | 71 | `helm install clay chart/clay/` without prior `helm dependency update` or `helm repo add` | ⚠️ Warning | Pre-existing file (D-16 explicitly out of scope); however, the chart now declares a CNPG subchart dependency — `helm install` will fail in CI without the subchart downloaded. Not a blocker for this phase but will block the integration test workflow. |

### Human Verification Required

#### 1. App pod reaches Running on helm install (Managed CNPG mode)

**Test:** On a Kubernetes cluster with CNPG operator CRDs installed, run:
```
helm repo add cnpg https://cloudnative-pg.github.io/charts
helm dependency update chart/clay/
helm install clay chart/clay/ --values chart/clay/ci/managed-values.yaml \
  --set secrets.ADMIN_PASS=test --set secrets.SESSION_SECRET=test
kubectl wait --for=condition=Ready pod -l app.kubernetes.io/name=clay --timeout=120s
```
**Expected:** Pod transitions through `Init:0/1` (pg_isready waiting) to `Running`; readiness probe at `/` passes; no `CreateContainerConfigError` for the CNPG secret reference.
**Why human:** Requires a live cluster with CNPG operator installed. Cannot verify pod lifecycle or CNPG Secret creation programmatically without cluster access.

#### 2. App pod reaches Running on helm install (External DSN mode)

**Test:** On a Kubernetes cluster with access to a Postgres instance, run:
```
helm install clay chart/clay/ --values chart/clay/ci/external-values.yaml \
  --set postgres.external.dsn="postgresql://user:pass@host:5432/clay" \
  --set secrets.ADMIN_PASS=test --set secrets.SESSION_SECRET=test
kubectl exec -it $(kubectl get pod -l app.kubernetes.io/name=clay -o name) -- env | grep DATABASE_URL
```
**Expected:** Pod starts without init container (managed=false); `DATABASE_URL` env var is set to the external DSN value; no CNPG Cluster resource created.
**Why human:** Requires a live cluster and external Postgres; need to verify runtime env injection and absence of Cluster resource.

### Gaps Summary

No blocking gaps. All Helm chart artifacts are substantive and correctly wired. All CI artifacts are substantive and correctly wired. All key links between chart components are verified.

**One partial requirement (CI-02 golangci-lint):** The REQUIREMENTS.md specifies `golangci-lint` as part of CI-02, but RESEARCH.md explicitly recommended deferring it (scope focus on Postgres/Helm; golangci-lint requires an additional action install step). The plan's must_haves reflect this decision — `golangci-lint` is not in the truths. This is a known, documented scope reduction, not an oversight. The requirement is functionally partially satisfied (`go vet` runs but golangci-lint does not). The developer should decide whether to mark CI-02 as partially complete or add golangci-lint in a follow-up task.

**One warning (integration-test.yml):** The pre-existing `integration-test.yml` does not run `helm dependency update` before `helm lint`/`helm install`. Since Chart.yaml now declares a CNPG subchart, the integration test workflow will fail when it runs `helm lint chart/clay/` without first downloading the dependency. This was explicitly deferred (D-16), but it should be tracked.

---

_Verified: 2026-04-14T02:23:07Z_
_Verifier: Claude (gsd-verifier)_
