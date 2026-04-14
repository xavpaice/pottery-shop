---
phase: 04-cert-manager-cr-templates
plan: 02
subsystem: infra
tags: [cert-manager, helm, kubernetes, tls, ci, github-actions, integration-test, helm-template-test]

# Dependency graph
requires:
  - phase: 04-cert-manager-cr-templates/04-01
    provides: "cert-manager-letsencrypt.yaml and cert-manager-selfsigned.yaml templates; ingress.yaml cluster-issuer annotation"
provides:
  - "integration-test.yml: cert-manager v1.20.2 pre-install step (jetstack/cert-manager) before clay chart install"
  - "helm-template-test.sh: G-09 through G-14 behavioral assertions covering TLS-01 and TLS-02 success criteria"
affects: [05-ci-tls-validation]

# Tech tracking
tech-stack:
  added: [jetstack/cert-manager v1.20.2 (CI pre-install)]
  patterns:
    - "cert-manager pre-install mirrors CNPG pattern: separate repo add step + install step with --wait --timeout 3m"
    - "Use ^kind: anchor in grep counts to match only top-level resource declarations (not nested issuerRef.kind fields)"
    - "TLS mode variables (LETSENCRYPT_INGRESS, SELFSIGNED_INGRESS) parallel CUSTOM_INGRESS pattern for DRY test setup"

key-files:
  created: []
  modified:
    - .github/workflows/integration-test.yml
    - chart/tests/helm-template-test.sh

key-decisions:
  - "Use ^kind: grep anchor for ClusterIssuer/Certificate counting — prevents false positives from issuerRef.kind fields inside Certificate specs (selfsigned mode has 4 raw matches but only 2 top-level resources)"
  - "No change to clay chart install step in integration-test.yml (ingress.enabled=false per D-16) — TLS mode CI validation is handled by helm template tests, not live cluster installs"

patterns-established:
  - "Operator pre-install pattern: helm repo add + helm install with --version pin, --namespace, --create-namespace, --wait, --timeout 3m"
  - "Helm template test variables: define mode-specific --set flag bundles as shell variables for reuse across test cases"

requirements-completed: [CI-06]

# Metrics
duration: 10min
completed: 2026-04-14
---

# Phase 4 Plan 02: CI Workflow and Template Test Extension Summary

**cert-manager v1.20.2 pre-install step added to integration-test.yml and 23-assertion helm-template-test.sh covering TLS-01/TLS-02 letsencrypt, selfsigned, and custom modes**

## Performance

- **Duration:** ~10 min
- **Started:** 2026-04-14T09:58:08Z
- **Completed:** 2026-04-14T10:03:16Z
- **Tasks:** 2 of 2
- **Files modified:** 2

## Accomplishments

- Added cert-manager v1.20.2 pre-install steps to `.github/workflows/integration-test.yml` between CNPG install and clay chart install — pinned version (T-04-06), `crds.enabled=true` (not deprecated `installCRDs=true`), dedicated namespace, `--wait --timeout 3m` (T-04-08)
- Extended `chart/tests/helm-template-test.sh` with 6 new test cases (G-09 through G-14, 13 sub-assertions total) covering TLS-01 (letsencrypt ClusterIssuer, Certificate, Ingress annotation) and TLS-02 (selfsigned four-resource bootstrap, no ACME bleed-through, custom zero-resource check)
- All 23 test assertions pass; G-01 through G-08 regression-free

## Task Commits

1. **Task 1: Add cert-manager pre-install steps to integration-test.yml** - `0b6398e` (feat)
2. **Task 2: Extend helm-template-test.sh with TLS-01 and TLS-02 test assertions** - `8f7de17` (feat)

## Files Created/Modified

- `.github/workflows/integration-test.yml` - Two new steps (Add cert-manager Helm repo, Install cert-manager) inserted between CNPG install and clay chart install
- `chart/tests/helm-template-test.sh` - Header updated with TLS-01/TLS-02 requirements; LETSENCRYPT_INGRESS and SELFSIGNED_INGRESS variables added; G-09 through G-14 test cases appended before Summary section

## Decisions Made

- Used `^kind:` line-start anchor in grep count assertions for ClusterIssuer and Certificate. Without the anchor, `grep -c "kind: ClusterIssuer"` returns 4 for selfsigned mode (2 top-level + 2 inside `issuerRef.kind` fields in Certificate specs). The anchor ensures counts match actual top-level Kubernetes resource declarations.
- No change to the clay chart install step (`ingress.enabled=false` per D-16). TLS mode verification is the responsibility of `helm template` tests, which do not require a live cluster. The cert-manager pre-install is present so that future phases or manual test runs enabling ingress do not hit missing CRD errors.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Used `^kind:` anchor in grep count assertions**
- **Found during:** Task 2 (writing G-12 selfsigned count assertions)
- **Issue:** The plan specified "Count `kind: ClusterIssuer` occurrences -- expect exactly 2" for selfsigned mode, but naïve `grep -c "kind: ClusterIssuer"` returns 4 (2 top-level ClusterIssuer resources + 2 `kind: ClusterIssuer` strings inside `issuerRef.kind` fields of the Certificate specs). Test would fail with wrong expected count.
- **Fix:** Changed count greps to `grep -c "^kind: ClusterIssuer"` and `grep -c "^kind: Certificate"` to anchor at line start, matching only top-level resource type declarations.
- **Files modified:** chart/tests/helm-template-test.sh
- **Verification:** All count assertions pass: selfsigned=2+2, letsencrypt=1+1, custom=0+0
- **Committed in:** `8f7de17` (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (Rule 1 - Bug)
**Impact on plan:** Required for test correctness. No scope creep.

## Issues Encountered

None beyond the grep anchor fix documented above.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Phase 5 (CI TLS validation): CI values files for letsencrypt, selfsigned, and custom modes can now be added; `helm-template-test.sh` has the behavioral assertions ready
- Existing lint CI (`managed-values.yaml`, `external-values.yaml`) verified passing — no regression introduced
- All Phase 4 success criteria for CI-06 are met

---
*Phase: 04-cert-manager-cr-templates*
*Completed: 2026-04-14*
