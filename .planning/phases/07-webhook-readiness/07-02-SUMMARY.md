---
phase: 07-webhook-readiness
plan: "02"
subsystem: testing
tags: [helm, kubernetes, webhook, cnpg, cert-manager, behavioral-tests, rbac, jobs]

# Dependency graph
requires:
  - phase: 07-webhook-readiness
    plan: "01"
    provides: webhook-wait-rbac.yaml and webhook-wait-jobs.yaml templates

provides:
  - Behavioral test groups G-15 through G-19 covering WBHK-01 through WBHK-04
  - G-15: resource count validation for both-operators-enabled scenario (2 Jobs, SA, CR, CRB)
  - G-16: CNPG toggle-off suppresses cnpg-webhook-wait Job
  - G-17: cert-manager toggle-off suppresses cert-manager-webhook-wait Job
  - G-18: hook annotation presence (post-install,post-upgrade and hook-weight -20)
  - G-19: no-wildcard rule enforcement for clay webhook-wait ClusterRole

affects: [ci-matrix, 08-hook-sequencing]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Scoping helm template assertions to a single source file using awk section extraction (# Source: pattern)"
    - "Using grep -cE with anchored indent (^  name:) to count top-level metadata.name without matching nested template.metadata.name"
    - "Using grep -qF for fixed-string wildcard matching (avoids regex * ambiguity)"

key-files:
  created: []
  modified:
    - chart/tests/helm-template-test.sh

key-decisions:
  - "G-15a Job count: scope to clay webhook-wait Jobs only by anchoring on ^  name: with exact Job name pattern, excluding subchart Jobs like cert-manager-startupapicheck"
  - "G-19 wildcard check: scope to webhook-wait-rbac.yaml awk section to avoid false positives from cert-manager ClusterRoles (which legitimately use wildcards)"
  - "G-19 grep pattern: use grep -qF (fixed-string) instead of grep -q to match literal '\"*\"' -- plain grep treats * as regex quantifier"

patterns-established:
  - "Awk section extraction pattern: awk '/# Source: clay\\/templates\\/TARGET\\.yaml/{p=1} /^# Source:/{if(p && !/TARGET/){p=0}} p' -- extracts a single template's output from combined helm template output"

requirements-completed:
  - WBHK-01
  - WBHK-02
  - WBHK-03
  - WBHK-04

# Metrics
duration: 8min
completed: 2026-04-15
---

# Phase 07 Plan 02: Webhook Readiness Tests Summary

**10 behavioral test assertions (G-15 through G-19) validating WBHK-01 through WBHK-04 requirements: toggle-gated Jobs, correct hook annotations, scoped RBAC, and no-wildcard ClusterRole**

## Performance

- **Duration:** 8 min
- **Started:** 2026-04-15T04:16:00Z
- **Completed:** 2026-04-15T04:24:00Z
- **Tasks:** 1
- **Files modified:** 1

## Accomplishments
- G-15 (5 subtests): validates both-enabled scenario — exactly 2 clay webhook-wait Jobs, SA named `*-webhook-wait`, ClusterRole and ClusterRoleBinding present
- G-16: confirms `cloudnative-pg.enabled=false` produces zero cnpg-webhook-wait occurrences
- G-17: confirms `cert-manager.enabled=false` produces zero cert-manager-webhook-wait occurrences
- G-18 (2 subtests): confirms both Jobs carry `helm.sh/hook: post-install,post-upgrade` and `helm.sh/hook-weight: "-20"`
- G-19: confirms clay webhook-wait ClusterRole grants no wildcard permissions (scoped assertion to avoid subchart false positives)
- All 33 tests pass (23 pre-existing G-01 through G-14 + 10 new), 0 failed

## Task Commits

Each task was committed atomically:

1. **Task 1: Add webhook readiness test groups G-15 through G-19** - `569066a` (test)

## Files Created/Modified
- `chart/tests/helm-template-test.sh` - Added G-15 through G-19 test groups and updated header to reference Phase 7 + WBHK requirements

## Decisions Made
- Used awk section extraction scoping for G-15c (SA count) and G-19 (wildcard check) to isolate clay's `webhook-wait-rbac.yaml` from subchart output
- Used `grep -qF` for literal `"*"` matching to avoid regex interpretation of `*` as a quantifier

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] G-15c SA count: grep -B1 approach replaced with awk section scoping**
- **Found during:** Task 1 (adding G-15c SA count assertion)
- **Issue:** The plan's `grep -B1 "name: release-test-clay-webhook-wait" | grep "^kind: ServiceAccount"` approach assumes `kind:` immediately precedes `name:`, but in the rendered YAML the `metadata:` key sits between them — so grep -B1 always returns 0 even when the SA exists
- **Fix:** Extract the `webhook-wait-rbac.yaml` section with awk and then count `^kind: ServiceAccount` lines within that scoped output
- **Files modified:** chart/tests/helm-template-test.sh
- **Verification:** SA_COUNT returns 1 as expected; test passes
- **Committed in:** 569066a (Task 1 commit)

**2. [Rule 1 - Bug] G-15a Job count: scoped to clay webhook-wait Jobs to exclude cert-manager-startupapicheck**
- **Found during:** Task 1 (running tests after initial insertion)
- **Issue:** The plan used `grep -c "^kind: Job"` on the full helm output which counted 3 Jobs (cert-manager-startupapicheck + 2 webhook-wait Jobs), failing the `== 2` assertion
- **Fix:** Changed to `grep -cE "^  name: release-test-clay-(cnpg|cert-manager)-webhook-wait"` anchored on 2-space indent (metadata.name level) to count only the two clay webhook-wait Jobs
- **Files modified:** chart/tests/helm-template-test.sh
- **Verification:** JOB_COUNT returns 2; test passes
- **Committed in:** 569066a (Task 1 commit)

**3. [Rule 1 - Bug] G-19 wildcard grep: changed to grep -qF for fixed-string matching**
- **Found during:** Task 1 (running tests after initial insertion)
- **Issue:** `grep -q '"*"'` treats `*` as a regex quantifier (zero-or-more of `"`), matching any line containing `"` — which includes version labels like `"1.0.0"`. This caused false positive wildcard detection even on the awk-scoped section.
- **Fix:** Changed to `grep -qF '"*"'` (fixed-string mode) to match the literal four-character sequence `"*"`
- **Files modified:** chart/tests/helm-template-test.sh
- **Verification:** grep -qF correctly distinguishes `verbs: ["*"]` (FOUND) from `version: "1.0.0"` (NOT FOUND); test passes
- **Committed in:** 569066a (Task 1 commit)

---

**Total deviations:** 3 auto-fixed (all Rule 1 - Bug)
**Impact on plan:** All three fixes corrected grep assertion logic so tests accurately validate the Plan 01 templates. No behavior change to the templates themselves; no scope creep.

## Issues Encountered
- Helm chart dependencies (`chart/clay/charts/*.tgz`) not present in worktree. Copied from main repo to enable `helm template` verification. The `.tgz` files are gitignored and were not committed (consistent with Plan 01 behavior).

## Known Stubs
None.

## Threat Flags
None — this plan modifies only a test script; no new network endpoints, auth paths, file access patterns, or schema changes introduced.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Behavioral tests for all four WBHK requirements are complete and passing
- Phase 08 (hook-sequencing) can proceed: cnpg-cluster.yaml post-install hook conversion will be testable via the existing test harness by extending with additional groups
- No blockers

## Self-Check: PASSED

- FOUND: chart/tests/helm-template-test.sh
- CONFIRMED: 569066a (Task 1 commit)
- CONFIRMED: bash chart/tests/helm-template-test.sh exits 0 with "33 passed, 0 failed"
- CONFIRMED: G-15a through G-19 all appear as PASS in output
- CONFIRMED: G-01 through G-14 all still pass (no regressions)

---
*Phase: 07-webhook-readiness*
*Completed: 2026-04-15*
