# Phase 5: CI Validation Extension - Context

**Gathered:** 2026-04-14
**Status:** Ready for planning

<domain>
## Phase Boundary

Add three TLS-mode CI values files under `chart/clay/ci/`, extend the `helm-lint` CI job in `test.yml` with six new lint + template steps (one lint + one template per TLS mode), and wire the existing behavioral test script (`chart/tests/helm-template-test.sh`) as a final step in that same job.

All validation must pass without cert-manager CRDs present on the runner — `helm lint` and `helm template` (without `--validate`) satisfy this constraint.

Scope: three new YAML files, CI workflow edits only. No chart template changes.

</domain>

<decisions>
## Implementation Decisions

### CI Values File Postgres Baseline
- **D-01:** All three TLS CI values files use **managed Postgres mode** (`postgres.managed: true`, `instances: 1`, `storage.size: 1Gi`). Matches the primary deployment mode. No external DSN needed.
- **D-02:** Secrets follow the existing CI placeholder pattern: `ADMIN_PASS: "ci-test-only"`, `SESSION_SECRET: "ci-test-session-secret-not-for-production"`.
- **D-03:** Files are self-contained (no layering) — each is a single `--values` argument, mirroring how `managed-values.yaml` and `external-values.yaml` are used today.

### CI Values File Content Per Mode
- **D-04 (letsencrypt):** `chart/clay/ci/tls-letsencrypt-values.yaml` — `ingress.enabled: true`, `ingress.host: shop.example.com`, `ingress.tls.mode: letsencrypt`, `ingress.tls.acme.email: admin@example.com`. No `acme.production` override (defaults to `false` = staging, which is correct for CI).
- **D-05 (selfsigned):** `chart/clay/ci/tls-selfsigned-values.yaml` — `ingress.enabled: true`, `ingress.host: shop.example.com`, `ingress.tls.mode: selfsigned`. No email or secretName needed.
- **D-06 (custom):** `chart/clay/ci/tls-custom-values.yaml` — `ingress.enabled: true`, `ingress.host: shop.example.com`, `ingress.tls.mode: custom`, `ingress.tls.secretName: my-tls`. Name `my-tls` matches what `helm-template-test.sh` already uses for custom-mode assertions.

### test.yml Structure
- **D-07:** Six new steps added to the **existing `helm-lint` job** (not a new job). Helm is already set up there — no extra runner or setup step needed.
- **D-08:** Six steps follow the exact naming pattern of existing steps:
  - `Lint (TLS — letsencrypt mode)` / `Template (TLS — letsencrypt mode)`
  - `Lint (TLS — selfsigned mode)` / `Template (TLS — selfsigned mode)`
  - `Lint (TLS — custom mode)` / `Template (TLS — custom mode)`
- **D-09:** All `helm template` steps run **without `--validate`** (default behavior). cert-manager CRDs are not present on the CI runner — `--validate` would fail.
- **D-10:** `helm-template-test.sh` added as the **final step** in `helm-lint` job: `run: chart/tests/helm-template-test.sh`. All 23 assertions (G-01..G-14, covering INGR-01..04, TLS-01, TLS-02, TLS-03) run in CI on every push.

### Claude's Discretion
- Exact step ordering within the six new TLS steps (lint-then-template per mode, or all lints then all templates — either is fine; lint-then-template per mode is the established pattern)
- Whether to add a comment block in `test.yml` separating the new TLS steps from the existing managed/external steps

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements
- `.planning/REQUIREMENTS.md` — Phase 5 requirements: CI-04 (three TLS values files), CI-05 (six new lint/template steps)

### Phase 5 success criteria (ROADMAP.md)
- `.planning/ROADMAP.md` §Phase 5 — Six success criteria define exactly what must exist and pass; read before writing anything

### Current CI workflow (read before modifying)
- `.github/workflows/test.yml` — Append six steps + `helm-template-test.sh` invocation to the `helm-lint` job; existing steps are the structural template to follow

### Existing CI values files (pattern to replicate)
- `chart/clay/ci/managed-values.yaml` — Baseline for secrets + managed Postgres shape used in D-01..D-03
- `chart/clay/ci/external-values.yaml` — Reference for file formatting and placeholder conventions

### Behavioral test script
- `chart/tests/helm-template-test.sh` — Script to invoke as final step in helm-lint job (D-10); 23 assertions, G-01..G-14; already has correct shebang and `set -euo pipefail`

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `chart/clay/ci/managed-values.yaml` — Direct template for D-01..D-03; copy secrets + managed postgres block into each TLS file, then add ingress block
- `chart/tests/helm-template-test.sh` — Already executable, already resolves CHART_DIR relative to its own location; no modifications needed

### Established Patterns
- `helm lint chart/clay/ --values <file>` — exact invocation format from existing steps; replicate for all three TLS modes
- `helm template clay chart/clay/ --values <file>` — exact invocation format; replicate for all three TLS modes
- Step naming convention: `Lint (<mode> mode)` / `Template (<mode> mode)` — keep parallel with existing step names
- Secrets placeholder values `ci-test-only` / `ci-test-session-secret-not-for-production` — copy verbatim from managed-values.yaml (D-02)

### Integration Points
- `test.yml` `helm-lint` job — six steps + behavioral test step inserted after existing managed/external steps
- `chart/clay/ci/` directory — add three new YAML files alongside existing `managed-values.yaml` and `external-values.yaml`
- `ingress.tls.secretName: my-tls` in custom-values.yaml — must match what `helm-template-test.sh` CUSTOM_INGRESS uses (it does: `--set ingress.tls.secretName=my-tls`)

</code_context>

<specifics>
## Specific Ideas

- User selected managed Postgres baseline (not external DSN) — TLS files mirror the primary deployment mode
- `helm-template-test.sh` goes in helm-lint job as a final step (not a new job) — Helm already set up, no extra runner cost
- `ingress.tls.secretName: my-tls` in custom CI file intentionally matches the existing `CUSTOM_INGRESS` flags in the behavioral test script to keep values consistent

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 05-ci-validation-extension*
*Context gathered: 2026-04-14*
