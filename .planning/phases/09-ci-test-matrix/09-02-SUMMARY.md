---
phase: 09-ci-test-matrix
plan: "02"
subsystem: helm-chart
tags: [ci, helm, test-script, github-actions, operator-toggles]
dependency_graph:
  requires: [09-01]
  provides: [CI-01, CI-02, CI-03, CI-04, CI-05]
  affects: [chart/tests/helm-template-test.sh, .github/workflows/test.yml]
tech_stack:
  added: []
  patterns: [here-string grep to avoid SIGPIPE/pipefail with large helm template output]
key_files:
  created: []
  modified:
    - chart/tests/helm-template-test.sh
    - .github/workflows/test.yml
decisions:
  - "Replace echo VAR | grep with grep <<< VAR (here-string) for G-24 through G-27 assertions — avoids SIGPIPE/pipefail false-negative when grep -q exits early on a 1.2MB variable"
  - "G-08 expanded from 2 sub-assertions (managed/external) to 4 (ci-bundled/ci-preinstalled/ci-external-db/ci-mixed) per D-15"
metrics:
  duration: "315s"
  completed: "2026-04-15T05:17:13Z"
  tasks_completed: 2
  files_created: 0
  files_modified: 2
---

# Phase 09 Plan 02: CI Test Matrix Assertions Summary

**One-liner:** G-08 updated to lint all four ci-* files and G-24 through G-27 added to assert correct resource rendering for each operator toggle combination, with GitHub Actions workflow updated to match.

## What Was Built

### Task 1: helm-template-test.sh updates

Three changes to `chart/tests/helm-template-test.sh`:

1. **Header updated** — added `Phase 9: ci-test-matrix` and `CI-01, CI-02, CI-03, CI-04, CI-05` to the requirements list.

2. **G-08 replaced** — removed G-08a/b (managed-values.yaml, external-values.yaml) and replaced with G-08a through G-08d, each linting one of the four ci-* files:
   - G-08a CI-01: ci-bundled-values.yaml
   - G-08b CI-02: ci-preinstalled-values.yaml
   - G-08c CI-03: ci-external-db-values.yaml
   - G-08d CI-04: ci-mixed-values.yaml

3. **G-24 through G-27 added** — four new assertion groups covering the full operator toggle matrix:

   | Group | Mode | Assertions |
   |-------|------|------------|
   | G-24 | bundled (CI-01) | CNPG Deployment present, cert-manager Deployment present |
   | G-25 | pre-installed (CI-02) | Cluster CR present, CNPG Deployment absent |
   | G-26 | external-db (CI-03) | Cluster CR absent, webhook-wait Jobs absent |
   | G-27 | mixed (CI-04) | CNPG Deployment present, cert-manager Deployment absent |

**Total assertions:** 50 passed, 0 failed (was 38 before this plan).

### Task 2: test.yml updates

Three changes to `.github/workflows/test.yml` helm-lint job:

1. **Removed 4 old steps** — Lint/Template for managed-values.yaml and external-values.yaml
2. **Added 8 new steps** — Lint+Template pairs for all four ci-* files
3. **Updated behavioral test step name** — now includes `CI-01..05`

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Update helm-template-test.sh — G-08, G-24–G-27, header | b486f81 | chart/tests/helm-template-test.sh |
| 2 | Update test.yml — replace managed+external steps with 4 ci-* pairs | 022c863 | .github/workflows/test.yml |

## Verification

`bash chart/tests/helm-template-test.sh` exits 0 with **50 passed, 0 failed**.

`test.yml` has exactly 8 references to ci-* files (2 per file), no references to managed-values.yaml or external-values.yaml, and the behavioral test step name includes CI-01..05.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed SIGPIPE/pipefail false-negative in G-24 through G-27 grep assertions**
- **Found during:** Task 1 verification
- **Issue:** The plan-specified pattern `if echo "${OUTPUT}" | grep -q "pattern"` fails with `set -euo pipefail` when the variable is ~1.2MB (full subchart helm template output). `grep -q` exits 0 immediately after finding an early match (line 66 of 22,068), causing `echo` to receive SIGPIPE (exit 141). With `pipefail`, the pipe returns 141 (non-zero), making the `if` condition evaluate as false even when the pattern was found. Existing tests G-15..G-23 were unaffected because their matched patterns appear near the end of the output, so `echo` completes before `grep -q` exits.
- **Fix:** Replaced `echo "${OUTPUT_G24}" | grep -q` with `grep -q <<< "${OUTPUT_G24}"` (here-string). Here-strings don't use a pipe, so no SIGPIPE can occur. Applied to all 8 grep calls in G-24 through G-27.
- **Files modified:** chart/tests/helm-template-test.sh
- **Commit:** b486f81

## Known Stubs

None — test script and CI workflow have no data stubs or placeholder values.

## Threat Flags

None — changes are test infrastructure only (helm lint/template read-only operations). Matches threat model T-09-03 and T-09-04 (both dispositioned in plan frontmatter).

## Self-Check: PASSED

- chart/tests/helm-template-test.sh: FOUND
- .github/workflows/test.yml: FOUND
- Commit b486f81: FOUND (Task 1)
- Commit 022c863: FOUND (Task 2)
- helm-template-test.sh contains "Phase 9: ci-test-matrix": VERIFIED
- helm-template-test.sh contains "CI-01, CI-02, CI-03, CI-04, CI-05": VERIFIED
- helm-template-test.sh contains "G-08a CI-01: helm lint with ci-bundled-values.yaml exits 0": VERIFIED
- helm-template-test.sh does NOT contain "managed-values.yaml": VERIFIED
- helm-template-test.sh does NOT contain "external-values.yaml": VERIFIED
- helm-template-test.sh contains G-24a through G-27b: VERIFIED
- test.yml does NOT contain "managed-values.yaml": VERIFIED
- test.yml does NOT contain "external-values.yaml": VERIFIED
- test.yml contains exactly 2 references each to all four ci-* files: VERIFIED
- test.yml contains "CI-01..05" in behavioral test step name: VERIFIED
- Full test script exits 0 with 50 passed, 0 failed: VERIFIED
