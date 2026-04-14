# Project Retrospective

*A living document updated after each milestone. Lessons feed forward into future planning.*

## Milestone: v1.1 — TLS

**Shipped:** 2026-04-14
**Phases:** 6 (incl. 3.1 insertion) | **Plans:** 11 | **Commits:** ~128

### What Was Built

- Helm chart ingress refactored from multi-host nginx array to single-host Traefik scalar with mode-driven TLS and fail-fast `clay.validateIngress` helpers
- cert-manager ClusterIssuer + Certificate templates for letsencrypt (ACME HTTP-01 staging, production opt-in) and selfsigned (4-resource CA bootstrap) modes; custom mode uses BYO secret
- All cert-manager resources carry `helm.sh/hook: post-install,post-upgrade` to prevent upgrade "already exists" errors
- `integration-test.yml` extended with cert-manager v1.20.2 pre-install step (mirrors CNPG pattern)
- CI extended with six TLS lint+template steps and a 23-assertion `helm-template-test.sh` covering all three TLS modes and INGR-01..04
- Previously (v1.0): full SQLite→Postgres migration (pgx/v5, Goose, CGO removal, testcontainers), CNPG subchart, and CI pipeline

### What Worked

- **Phase 3.1 insertion pattern**: recognizing that verification artifacts were missing and inserting a decimal phase (3.1) to close the gap retroactively — clean, non-disruptive, traceable
- **`clay.tlsSecretName` as a single source of truth**: defining the TLS secret name once in `_helpers.tpl` prevented Ingress/Certificate mismatch that would be a silent runtime failure
- **Hook-weight sequencing for selfsigned**: explicit weights (-10/-5/0/5) on the 4-resource CA bootstrap eliminated ordering ambiguity that Helm's default parallel install would cause
- **`^kind:` grep anchor in tests**: catching the false-positive issue (4 matches vs 2 top-level resources for selfsigned) early during test writing, not debugging later
- **Staging ACME default**: defaulting to the staging endpoint prevented rate-limit accidents during development and test runs

### What Was Inefficient

- **REQUIREMENTS.md checkboxes not updated after Phase 4**: TLS-01, TLS-02, CI-06 were verified in VERIFICATION.md but checkboxes remained unchecked — required manual fix at milestone close
- **ROADMAP.md progress table went stale**: the table at the bottom of ROADMAP.md showed Phases 2–4 as "Not started" even after they completed — progress tracking was split between the narrative checklist (kept current) and the table (not)
- **STATE.md not updated after each phase**: `stopped_at` and `Current Position` still referenced Phase 3 even after Phase 5 completed — the node CLI had accurate counts but the human-readable section was stale
- **No milestone audit**: skipped `/gsd-audit-milestone` before close — not a blocking problem here, but the cross-phase integration check would have surfaced the stale requirement checkboxes earlier

### Patterns Established

- **Operator pre-install pattern**: `helm repo add + helm install --version pin --namespace --create-namespace --wait --timeout 3m` — established for CNPG in v1.0, confirmed for cert-manager in v1.1; any future cluster operator follows this shape
- **cert-manager hook template**: every ClusterIssuer and Certificate carries `helm.sh/hook: post-install,post-upgrade` + `helm.sh/hook-delete-policy: before-hook-creation` — prevents upgrade "already exists" errors on immutable resources
- **Helm template behavioral test script** (`chart/tests/helm-template-test.sh`): pattern of named test groups (G-01 through G-14) with sub-assertions; anchored `^kind:` greps for resource counts
- **Decimal phase for retroactive closure**: when implementation is done but verification artifacts are missing, insert a `.1` phase rather than patching the original phase — keeps history clean

### Key Lessons

1. **Close documentation gaps at phase exit, not milestone close.** Requirement checkboxes, ROADMAP progress table, and STATE.md should all be updated by the phase workflow, not left for milestone completion to discover.
2. **Helm hook annotations are mandatory for cert-manager CRs.** Without them, `helm upgrade` fails with "already exists" on ClusterIssuer and Certificate — this is not optional hardening.
3. **`^kind:` anchoring in grep counts prevents false positives.** Nested `kind:` fields inside resource specs will produce wrong counts; always anchor to line start when counting resource types in `helm template` output.
4. **The `clay.tlsSecretName` helper pattern is reusable.** Any time two separate template files must reference the same derived value, extract it to `_helpers.tpl` as a named template — eliminates silent drift.

### Cost Observations

- Model mix: ~100% sonnet (all phases used balanced profile)
- Sessions: multiple across 2 days
- Notable: yolo mode + auto_advance meant phases chained without manual confirmation — effective for infrastructure-only work with no UI

---

## Cross-Milestone Trends

### Process Evolution

| Milestone | Phases | Plans | Key Change |
|-----------|--------|-------|------------|
| v1.0 | 2 | 5 | Established GSD workflow on this project |
| v1.1 | 6 (incl. 3.1) | 11 | First use of decimal insertion phase; first milestone archive |

### Cumulative Quality

| Milestone | Integration Tests | Helm Behavioral Assertions | CI Jobs |
|-----------|------------------|---------------------------|---------|
| v1.0 | testcontainers-go (product CRUD) | 0 | build, test, helm lint x2 |
| v1.1 | + TLS pre-install | 23 (G-01 to G-14) | + 6 TLS lint/template + behavioral test |

### Top Lessons (Verified Across Milestones)

1. **Operator pre-install pattern beats subchart embedding** — both CNPG and cert-manager needed pre-install; cluster-scoped CRDs + webhook races make subcharts impractical for operators
2. **Fail-fast helpers at render time beat runtime debugging** — `clay.validateSecrets` (v1.0) and `clay.validateIngress` (v1.1) both proved their value by catching config errors before any cluster interaction
