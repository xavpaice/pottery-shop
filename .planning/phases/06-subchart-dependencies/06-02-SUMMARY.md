---
phase: "06-subchart-dependencies"
plan: 2
subsystem: "helm-chart"
tags: ["helm", "schema", "validation", "subchart", "cnpg", "cert-manager"]
dependency_graph:
  requires: []
  provides:
    - "values.schema.json boolean enforcement for cloudnative-pg.enabled and cert-manager.enabled"
    - "clay.validateDB render-time guard for missing database configuration"
  affects:
    - "chart/clay/values.schema.json"
    - "chart/clay/templates/_helpers.tpl"
    - "chart/clay/templates/deployment.yaml"
tech_stack:
  added: []
  patterns:
    - "Helm JSON Schema boolean type enforcement for subchart toggle keys"
    - "Fail-fast render-time template helper (clay.validateDB) following existing clay.validateSecrets and clay.validateIngress pattern"
key_files:
  created: []
  modified:
    - "chart/clay/values.schema.json"
    - "chart/clay/templates/_helpers.tpl"
    - "chart/clay/templates/deployment.yaml"
decisions:
  - "Replace inline DB validation in deployment.yaml with clay.validateDB helper call to centralize logic and use the D-03 canonical error message"
metrics:
  duration: "85s"
  completed: "2026-04-15T02:16:42Z"
  tasks_completed: 2
  files_modified: 3
requirements:
  - "CHART-03"
---

# Phase 06 Plan 02: Schema Validation and validateDB Guard Summary

**One-liner:** JSON Schema boolean enforcement for subchart toggles plus clay.validateDB render-time guard for missing database configuration.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Add subchart toggle objects to values.schema.json | 34f7f98 | chart/clay/values.schema.json |
| 2 | Add clay.validateDB helper to _helpers.tpl | d5dcf38 | chart/clay/templates/_helpers.tpl, chart/clay/templates/deployment.yaml |

## What Was Built

**Task 1 — values.schema.json subchart toggle objects:**

Added two new property objects to the `properties` section of `values.schema.json`, each enforcing `"type": "boolean"` for the `enabled` field:

- `cloudnative-pg` — subchart toggle for the CNPG operator (D-06)
- `cert-manager` — subchart toggle for cert-manager (D-06)

`helm lint` now rejects non-boolean values (e.g., `"yes"`, `"1"`, `"notabool"`) for either toggle with a schema validation error. The `required` array remains `["image", "secrets"]` — the toggles are not required since they have defaults in values.yaml.

**Task 2 — clay.validateDB helper:**

Added `clay.validateDB` to `_helpers.tpl` following the existing fail-fast pattern of `clay.validateSecrets` and `clay.validateIngress`. The helper fails at render time with the canonical D-03 error message when neither `postgres.managed=true` nor `postgres.external.dsn` is set.

Also updated `deployment.yaml` to replace the pre-existing inline DB validation check (which had a slightly different error message: "Either postgres.managed must be true or postgres.external.dsn must be set") with `{{- include "clay.validateDB" . }}`, centralizing the logic in the helper.

## Verification Results

All success criteria confirmed:

- `python3 -m json.tool chart/clay/values.schema.json` exits 0 (valid JSON)
- `cloudnative-pg.enabled` and `cert-manager.enabled` both have `"type": "boolean"`
- `required` array unchanged: `["image", "secrets"]`
- `helm lint --set "cloudnative-pg.enabled=notabool"` produces schema error: `at '/cloudnative-pg/enabled': got string, want boolean`
- `helm template --set postgres.managed=false` (no DSN) produces: `postgres.managed or postgres.external.dsn required`
- `helm template --set postgres.managed=true` renders cleanly

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Replaced inline DB validation with clay.validateDB helper call**
- **Found during:** Task 2
- **Issue:** `deployment.yaml` already contained inline DB validation logic with a different error message ("Either postgres.managed must be true or postgres.external.dsn must be set") that would conflict with / duplicate the new `clay.validateDB` helper.
- **Fix:** Replaced the three-line inline block with `{{- include "clay.validateDB" . }}`, matching the D-03 canonical error message ("postgres.managed or postgres.external.dsn required") and consolidating the guard into the named helper.
- **Files modified:** `chart/clay/templates/deployment.yaml`
- **Commit:** d5dcf38

## Known Stubs

None — no placeholder data or TODO stubs introduced.

## Threat Flags

None — no new network endpoints, auth paths, or trust boundaries introduced. The schema and helper changes only affect Helm render-time behavior (developer-facing, not runtime user-facing).

## Self-Check: PASSED

- chart/clay/values.schema.json — modified with cloudnative-pg and cert-manager boolean objects
- chart/clay/templates/_helpers.tpl — clay.validateDB helper appended
- chart/clay/templates/deployment.yaml — clay.validateDB call site added, inline guard removed
- Commits 34f7f98 and d5dcf38 exist in git log
