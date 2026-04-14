---
phase: 05-ci-validation-extension
reviewed: 2026-04-14T00:00:00Z
depth: standard
files_reviewed: 4
files_reviewed_list:
  - chart/clay/ci/tls-letsencrypt-values.yaml
  - chart/clay/ci/tls-selfsigned-values.yaml
  - chart/clay/ci/tls-custom-values.yaml
  - .github/workflows/test.yml
findings:
  critical: 0
  warning: 2
  info: 1
  total: 3
status: issues_found
---

# Phase 05: Code Review Report

**Reviewed:** 2026-04-14T00:00:00Z
**Depth:** standard
**Files Reviewed:** 4
**Status:** issues_found

## Summary

Four files were reviewed: three new CI values fixtures for TLS modes (letsencrypt, selfsigned, custom) and the GitHub Actions workflow that exercises them. The CI values files are structurally sound and consistent with the existing `managed-values.yaml` / `external-values.yaml` baseline. The workflow additions correctly add lint and template steps for each new TLS mode.

One bug was found in `chart/tests/helm-template-test.sh` (referenced by the workflow) that renders the G-08 lint-gate tests permanently inert — a `|| true` placed inside a command substitution means `$?` is always `0`, so the tests can never report a lint failure. This is a logic error in the test harness, not the reviewed files themselves, but the workflow step at line 72-73 of `test.yml` invokes that script and would silently pass even if `helm lint` exited non-zero. The issue is included here because the reviewed workflow owns that invocation.

A second warning covers a missing `helm lint` coverage gap in the behavioral test script for the three new TLS values files (G-08 only covers managed/external CI values, not the new TLS fixtures).

## Warnings

### WR-01: G-08 lint exit-code check is permanently broken — `$?` always 0

**File:** `chart/tests/helm-template-test.sh:186-203`
**Issue:** Both G-08 subtests (lines 186-204) capture `helm lint` output via command substitution with `|| true` placed _inside_ the substitution:

```bash
LINT_MANAGED=$("${HELM}" lint ... 2>&1 || true)
LINT_MANAGED_EXIT=$?
```

Because `|| true` is inside `$(...)`, the subshell always exits `0`. The variable assignment on the left then also succeeds with `$?=0`. As a result `LINT_MANAGED_EXIT` is always `0` regardless of whether `helm lint` actually fails. The `else` branch at lines 192-194 and 201-203 is dead code. If the chart develops a lint error, G-08a and G-08b will still report PASS.

The workflow step at `.github/workflows/test.yml:72-73` invokes this script unconditionally, so this broken gate runs on every CI execution without protecting against regressions.

**Fix:** Capture exit code separately, _outside_ the command substitution:

```bash
LINT_MANAGED=$("${HELM}" lint "${CHART_DIR}" -f "${CHART_DIR}/ci/managed-values.yaml" 2>&1) || LINT_MANAGED_EXIT=$?
LINT_MANAGED_EXIT=${LINT_MANAGED_EXIT:-0}
```

Or use a two-step pattern that is easier to read:

```bash
set +e
LINT_MANAGED=$("${HELM}" lint "${CHART_DIR}" -f "${CHART_DIR}/ci/managed-values.yaml" 2>&1)
LINT_MANAGED_EXIT=$?
set -e
```

Apply the same fix to `LINT_EXTERNAL` / `LINT_EXTERNAL_EXIT` at lines 196-203.

---

### WR-02: Behavioral test script has no G-08 lint coverage for new TLS CI values files

**File:** `chart/tests/helm-template-test.sh:183-204` (and `.github/workflows/test.yml:54-70`)
**Issue:** The G-08 block in `helm-template-test.sh` only lints `managed-values.yaml` and `external-values.yaml`. The three new TLS values files (`tls-letsencrypt-values.yaml`, `tls-selfsigned-values.yaml`, `tls-custom-values.yaml`) are exercised by dedicated lint and template steps in `test.yml` (lines 54-70) but not inside the behavioral test script. This creates an asymmetry: if someone runs `helm-template-test.sh` standalone (as documented), TLS lint coverage is silently absent.

The workflow does provide direct coverage via the inline `helm lint` steps (lines 54-70), so CI as a whole is not blind to TLS lint failures. However, the gap matters for local developer runs of the test script and for future maintainers who may assume G-08 covers all CI values files.

**Fix:** Add G-08c, G-08d, G-08e subtests in `helm-template-test.sh` to cover the three TLS fixtures:

```bash
for VALUES_FILE in tls-letsencrypt-values.yaml tls-selfsigned-values.yaml tls-custom-values.yaml; do
    set +e
    LINT_OUT=$("${HELM}" lint "${CHART_DIR}" -f "${CHART_DIR}/ci/${VALUES_FILE}" 2>&1)
    LINT_EXIT=$?
    set -e
    if [ ${LINT_EXIT} -eq 0 ]; then
        pass "G-08 SC-5: helm lint with ${VALUES_FILE} exits 0"
    else
        fail "G-08 SC-5: helm lint with ${VALUES_FILE} exits 0" \
             "helm lint exited ${LINT_EXIT}. Output: ${LINT_OUT}"
    fi
done
```

## Info

### IN-01: Workflow step name for behavioral tests does not list TLS requirement IDs

**File:** `.github/workflows/test.yml:72`
**Issue:** The step name is `Behavioral tests (INGR-01..04, TLS-01..03)`. The referenced requirements are correct, but the step name omits SC-5 (which is explicitly tested by G-08 in the script). Minor but inconsistent with how other steps name their requirements.

**Fix:** Update the step name:
```yaml
- name: Behavioral tests (INGR-01..04, TLS-01..03, SC-5)
```

---

_Reviewed: 2026-04-14T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
