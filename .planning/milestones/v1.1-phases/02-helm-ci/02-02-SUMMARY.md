---
phase: 02-helm-ci
plan: 02
subsystem: ci
tags: [ci, github-actions, helm, cgo-free, helm-lint]
dependency_graph:
  requires: []
  provides: [CI-01, CI-02, CI-03]
  affects: [.github/workflows/test.yml, chart/clay/ci/]
tech_stack:
  added: []
  patterns: [helm-chart-testing-ci-values, dual-mode-helm-validation]
key_files:
  created:
    - chart/clay/ci/managed-values.yaml
    - chart/clay/ci/external-values.yaml
  modified:
    - .github/workflows/test.yml
decisions:
  - Replaced single make helm-lint with explicit per-mode helm lint + helm template calls for comprehensive CI coverage
  - CI values files follow helm/chart-testing convention in chart/clay/ci/ directory
metrics:
  duration: ~5 minutes
  completed: 2026-04-14
---

# Phase 02 Plan 02: CI Pipeline Updates (CGO-free + Dual-mode Helm) Summary

**One-liner:** Dropped gcc from CI test job, added go vet, and extended helm-lint job with CNPG subchart dependency resolution and dual-mode lint+template validation using per-mode CI values files.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Update test.yml test job — remove gcc, add go vet | 9db12f7 | .github/workflows/test.yml |
| 2 | CI values files and helm-lint job — dual-mode lint + template | 0594f7d | chart/clay/ci/managed-values.yaml, chart/clay/ci/external-values.yaml, .github/workflows/test.yml |

## What Was Built

### Task 1: CGO-free CI Test Job
- Removed the "Install dependencies" step (`sudo apt-get install -y gcc`) — a leftover from the SQLite/CGO era
- Added "Run vet" step (`go vet ./...`) between dependency verification and tests
- `make test` and `make build` already use `CGO_ENABLED=0` — no Makefile changes needed
- Docker daemon on `ubuntu-latest` satisfies testcontainers-go requirements — no extra setup needed

### Task 2: Dual-mode Helm CI Validation
- Created `chart/clay/ci/managed-values.yaml` — sets `postgres.managed: true`, `cloudnative-pg.enabled: true`, 1 instance, 1Gi storage
- Created `chart/clay/ci/external-values.yaml` — sets `postgres.managed: false`, `cloudnative-pg.enabled: false`, includes dummy DSN `postgresql://user:pass@external-host:5432/clay`
- Replaced single `make helm-lint` step with full validation pipeline:
  1. `helm repo add cnpg https://cloudnative-pg.github.io/charts`
  2. `helm dependency update chart/clay/` — downloads CNPG subchart before lint
  3. `helm lint chart/clay/ --values chart/clay/ci/managed-values.yaml`
  4. `helm lint chart/clay/ --values chart/clay/ci/external-values.yaml`
  5. `helm template clay chart/clay/ --values chart/clay/ci/managed-values.yaml`
  6. `helm template clay chart/clay/ --values chart/clay/ci/external-values.yaml`

## Verification Results

All 7 verification criteria passed:
1. No `apt-get` or `gcc` in test.yml
2. `go vet ./...` step present in test job
3. `helm repo add cnpg` step present in helm-lint job
4. `helm dependency update chart/clay/` step present in helm-lint job
5. Both lint and template for both managed and external modes
6. `managed-values.yaml` sets `managed: true` and `cloudnative-pg.enabled: true`
7. `external-values.yaml` sets `managed: false` and `cloudnative-pg.enabled: false`
8. `integration-test.yml` was not modified

## Deviations from Plan

None — plan executed exactly as written.

## Known Stubs

None. The CI values files contain dummy credentials (`postgresql://user:pass@external-host:5432/clay`) intentionally for template validation only — this is documented in the plan's threat model as T-02-06 (accepted).

## Threat Flags

No new security-relevant surface introduced beyond what is documented in the plan's threat model:
- T-02-05: CNPG chart downloaded from official HTTPS repo during CI (accepted)
- T-02-06: Dummy DSN in external-values.yaml committed intentionally (accepted)

## Self-Check: PASSED

All created files exist on disk. Both task commits (9db12f7, 0594f7d) verified in git log.
