---
phase: 09-ci-test-matrix
plan: "01"
subsystem: helm-chart
tags: [ci, helm, values-files, operator-toggles]
dependency_graph:
  requires: []
  provides: [CI-01, CI-02, CI-03, CI-04]
  affects: [chart/clay/ci/]
tech_stack:
  added: []
  patterns: [CI values file per test scenario]
key_files:
  created:
    - chart/clay/ci/ci-bundled-values.yaml
    - chart/clay/ci/ci-preinstalled-values.yaml
    - chart/clay/ci/ci-external-db-values.yaml
    - chart/clay/ci/ci-mixed-values.yaml
  modified: []
  deleted:
    - chart/clay/ci/managed-values.yaml
    - chart/clay/ci/external-values.yaml
decisions:
  - "Four separate CI values files replace two generic files — one file per test scenario for clear CI matrix coverage"
  - "No ingress block in any CI values file — TLS coverage stays in tls-* files only"
metrics:
  duration: "56s"
  completed: "2026-04-15T05:09:30Z"
  tasks_completed: 2
  files_created: 4
  files_deleted: 2
---

# Phase 09 Plan 01: CI Values File Matrix Summary

**One-liner:** Four CI values files cover all operator toggle combinations (both-bundled, pre-installed, external-DB, mixed) replacing two generic legacy files.

## What Was Built

Created four new YAML files in `chart/clay/ci/` to cover the full operator toggle matrix required by CI-01 through CI-04, and deleted the two obsolete legacy files they replace.

| File | Mode | cloudnative-pg | cert-manager | postgres.managed |
|------|------|----------------|--------------|-----------------|
| ci-bundled-values.yaml | Both bundled (CI-01) | enabled: true | enabled: true | true |
| ci-preinstalled-values.yaml | Both pre-installed (CI-02) | enabled: false | enabled: false | true |
| ci-external-db-values.yaml | External DB (CI-03) | enabled: false | enabled: false | false |
| ci-mixed-values.yaml | Mixed (CI-04) | enabled: true | enabled: false | true |

**Deleted:**
- `chart/clay/ci/managed-values.yaml` (superseded by ci-bundled-values.yaml and ci-preinstalled-values.yaml)
- `chart/clay/ci/external-values.yaml` (superseded by ci-external-db-values.yaml)

The `chart/clay/ci/` directory now contains exactly 7 files: 4 new ci-* files + 3 unchanged tls-* files.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Create four CI values files | 2b5d5cb | 4 new files in chart/clay/ci/ |
| 2 | Delete obsolete managed-values.yaml and external-values.yaml | 7e78d03 | 2 files deleted |

## Verification

All four new files pass `helm lint` (0 chart failures; missing-dependencies warnings are expected as subchart tarballs are not present in CI lint environment).

## Deviations from Plan

None - plan executed exactly as written.

## Known Stubs

None - these are static configuration files with no data flow or UI rendering.

## Threat Flags

None — new files are static CI-only YAML with dummy credentials documented as non-production and a fake example DSN. Matches threat model T-09-01 and T-09-02 (both accepted).

## Self-Check: PASSED

- chart/clay/ci/ci-bundled-values.yaml: FOUND
- chart/clay/ci/ci-preinstalled-values.yaml: FOUND
- chart/clay/ci/ci-external-db-values.yaml: FOUND
- chart/clay/ci/ci-mixed-values.yaml: FOUND
- chart/clay/ci/managed-values.yaml: confirmed deleted (does not exist)
- chart/clay/ci/external-values.yaml: confirmed deleted (does not exist)
- Commit 2b5d5cb: FOUND (Task 1)
- Commit 7e78d03: FOUND (Task 2)
- File count in chart/clay/ci/: 7 (CORRECT)
