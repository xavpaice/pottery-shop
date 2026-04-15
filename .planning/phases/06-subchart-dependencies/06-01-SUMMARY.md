---
phase: 06-subchart-dependencies
plan: "01"
subsystem: infra
tags: [helm, cloudnative-pg, cert-manager, subchart, kubernetes]

# Dependency graph
requires: []
provides:
  - Helm dependencies block in Chart.yaml with cloudnative-pg@0.28.0 and cert-manager@v1.20.2
  - Boolean toggle keys cloudnative-pg.enabled and cert-manager.enabled in values.yaml
  - Condition fields wired from Chart.yaml dependencies to values.yaml keys
affects:
  - 06-02 (values schema validation of new toggle keys)
  - 06-03 (helm dependency update — reads Chart.yaml dependencies block)

# Tech tracking
tech-stack:
  added:
    - cloudnative-pg Helm chart 0.28.0 (https://cloudnative-pg.github.io/charts)
    - cert-manager Helm chart v1.20.2 (https://charts.jetstack.io)
  patterns:
    - Helm condition field pattern: dependency condition references dot-path in values.yaml
    - Exact patch version pinning for operator subcharts (D-04)
    - Default-true toggles for full umbrella install

key-files:
  created: []
  modified:
    - chart/clay/Chart.yaml
    - chart/clay/values.yaml

key-decisions:
  - "cloudnative-pg.enabled and postgres.managed are independent controls: one gates operator bundling, the other gates Cluster CR rendering (D-02)"
  - "Both subchart toggles default to true — aligns with core value of single helm install deploying full stack (D-05)"
  - "No namespace override on subcharts — operators install into same namespace as release (D-01)"
  - "Exact patch versions pinned: cloudnative-pg@0.28.0 and cert-manager@v1.20.2 — prevents silent operator drift (D-04)"

patterns-established:
  - "Operator toggle pattern: top-level values key <chart-name>.enabled wired to Chart.yaml condition field"
  - "Version pinning pattern: quote-wrapped exact semver strings in dependencies block"

requirements-completed:
  - CHART-01
  - CHART-02

# Metrics
duration: 8min
completed: 2026-04-15
---

# Phase 06 Plan 01: Subchart Dependency Declarations Summary

**Conditional Helm subchart dependencies wired for cloudnative-pg@0.28.0 and cert-manager@v1.20.2 with boolean toggle defaults and operator-install vs Cluster-CR separation comments**

## Performance

- **Duration:** 8 min
- **Started:** 2026-04-15T04:35:00Z
- **Completed:** 2026-04-15T04:43:00Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments
- Added `dependencies:` block to Chart.yaml with both operator subcharts pinned at exact patch versions
- Wired `condition:` fields to `cloudnative-pg.enabled` and `cert-manager.enabled` values keys
- Appended top-level toggle keys to values.yaml with `enabled: true` defaults
- Comments in values.yaml clearly explain the D-01/D-02 design: `cloudnative-pg.enabled` controls operator bundling independently of `postgres.managed` which controls Cluster CR rendering
- Helm lint passes (0 charts failed); dependency WARNING is expected before Plan 03 runs `helm dependency update`

## Task Commits

Each task was committed atomically:

1. **Task 1: Add subchart dependencies block to Chart.yaml** - `f6446f4` (feat)
2. **Task 2: Add subchart toggle keys to values.yaml** - `ae3002f` (feat)

**Plan metadata:** (committed with SUMMARY.md)

## Files Created/Modified
- `chart/clay/Chart.yaml` - Added `dependencies:` block with cloudnative-pg@0.28.0 and cert-manager@v1.20.2, condition fields pointing to values keys
- `chart/clay/values.yaml` - Appended `cloudnative-pg.enabled: true` and `cert-manager.enabled: true` top-level keys with explanatory comments

## Decisions Made
- Confirmed D-02 distinction: `postgres.managed` (render Cluster CR) is separate from `cloudnative-pg.enabled` (bundle operator) — both must be independently settable for pre-installed operator scenarios
- cert-manager version string uses `v` prefix (`"v1.20.2"`) matching the official charts.jetstack.io repo convention
- `--skip-dependencies` flag removed in Helm 3.20; lint runs without it and the dependency WARNING (not error) is acceptable until Plan 03

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
- `helm lint --skip-dependencies` flag does not exist in Helm 3.20 (flag was removed). Ran lint without the flag — result was 0 charts failed, 1 WARNING about missing dependency tarballs. This is expected behavior and does not indicate a problem; Plan 03 runs `helm dependency update` to fetch tarballs.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Chart.yaml dependencies block is ready for Plan 03 (`helm dependency update` will read it to fetch tarballs)
- values.yaml toggle keys are ready for Plan 02 (values.schema.json validation of `cloudnative-pg.enabled` and `cert-manager.enabled`)
- No blockers — foundational wiring complete

## Self-Check: PASSED

- chart/clay/Chart.yaml: FOUND
- chart/clay/values.yaml: FOUND
- .planning/phases/06-subchart-dependencies/06-01-SUMMARY.md: FOUND
- Commit f6446f4 (Task 1): FOUND
- Commit ae3002f (Task 2): FOUND

---
*Phase: 06-subchart-dependencies*
*Completed: 2026-04-15*
