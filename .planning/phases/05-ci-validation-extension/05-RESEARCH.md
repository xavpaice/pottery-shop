# Phase 5: CI Validation Extension — Research

**Researched:** 2026-04-14
**Domain:** GitHub Actions CI / Helm lint-and-template pipeline / CI values files
**Confidence:** HIGH

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**D-01:** All three TLS CI values files use managed Postgres mode (`postgres.managed: true`, `instances: 1`, `storage.size: 1Gi`). Matches the primary deployment mode. No external DSN needed.

**D-02:** Secrets follow the existing CI placeholder pattern: `ADMIN_PASS: "ci-test-only"`, `SESSION_SECRET: "ci-test-session-secret-not-for-production"`.

**D-03:** Files are self-contained (no layering) — each is a single `--values` argument, mirroring how `managed-values.yaml` and `external-values.yaml` are used today.

**D-04 (letsencrypt):** `chart/clay/ci/tls-letsencrypt-values.yaml` — `ingress.enabled: true`, `ingress.host: shop.example.com`, `ingress.tls.mode: letsencrypt`, `ingress.tls.acme.email: admin@example.com`. No `acme.production` override (defaults to `false` = staging).

**D-05 (selfsigned):** `chart/clay/ci/tls-selfsigned-values.yaml` — `ingress.enabled: true`, `ingress.host: shop.example.com`, `ingress.tls.mode: selfsigned`. No email or secretName needed.

**D-06 (custom):** `chart/clay/ci/tls-custom-values.yaml` — `ingress.enabled: true`, `ingress.host: shop.example.com`, `ingress.tls.mode: custom`, `ingress.tls.secretName: my-tls`. Name `my-tls` matches what `helm-template-test.sh` already uses for custom-mode assertions.

**D-07:** Six new steps added to the existing `helm-lint` job (not a new job). Helm is already set up there.

**D-08:** Six steps follow the exact naming pattern of existing steps:
  - `Lint (TLS — letsencrypt mode)` / `Template (TLS — letsencrypt mode)`
  - `Lint (TLS — selfsigned mode)` / `Template (TLS — selfsigned mode)`
  - `Lint (TLS — custom mode)` / `Template (TLS — custom mode)`

**D-09:** All `helm template` steps run without `--validate` (default behavior). cert-manager CRDs are not present on the CI runner.

**D-10:** `helm-template-test.sh` added as the final step in `helm-lint` job: `run: chart/tests/helm-template-test.sh`. All 23 assertions (G-01..G-14, covering INGR-01..04, TLS-01, TLS-02, TLS-03) run in CI on every push.

### Claude's Discretion

- Exact step ordering within the six new TLS steps (lint-then-template per mode, or all lints then all templates — either is fine; lint-then-template per mode is the established pattern)
- Whether to add a comment block in `test.yml` separating the new TLS steps from the existing managed/external steps

### Deferred Ideas (OUT OF SCOPE)

None — discussion stayed within phase scope.
</user_constraints>

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| CI-04 | `chart/clay/ci/` contains three TLS values files (`tls-letsencrypt-values.yaml`, `tls-selfsigned-values.yaml`, `tls-custom-values.yaml`) for lint/template validation | Pattern from managed-values.yaml verified; all three modes lint-pass with correct values [VERIFIED: local helm lint] |
| CI-05 | `test.yml` Helm validation job extended with six steps (helm lint + helm template for each TLS mode) | Existing step pattern in test.yml confirmed; helm-template-test.sh verified 23/23 PASS from repo root [VERIFIED: local run] |
</phase_requirements>

---

## Summary

Phase 5 is a pure configuration and CI extension phase — no chart template changes are required. Phase 4 is complete: all cert-manager templates (letsencrypt, selfsigned, custom) render correctly and the behavioral test script (`chart/tests/helm-template-test.sh`) passes all 23 assertions (G-01..G-14) against the current chart.

The deliverables are three new YAML files under `chart/clay/ci/` and seven new steps in the `helm-lint` job of `.github/workflows/test.yml`. The structure of every file and every step is fully determined by existing patterns in the codebase — managed-values.yaml provides the Postgres baseline, external-values.yaml shows formatting conventions, and the existing `Lint (managed mode)` / `Template (managed mode)` steps provide the exact naming and command template to follow.

The only potential source of error in this phase is a values file that passes lint/template in isolation but fails the behavioral test script assertion for `G-08` (which checks lint exit code for CI values files). Local testing confirmed all three TLS modes lint-pass with values matching the locked decisions.

**Primary recommendation:** Write the three values files first, verify each with `helm lint` and `helm template` locally, then append the seven steps to `test.yml`. Zero chart modifications needed.

---

## Standard Stack

### Core

| Tool | Version | Purpose | Source |
|------|---------|---------|--------|
| Helm 3.x | 3.x (via `azure/setup-helm@v4`) | lint, template rendering | [VERIFIED: .github/workflows/test.yml line 40] |
| GitHub Actions | — | CI runner platform | [VERIFIED: .github/workflows/test.yml] |
| `azure/setup-helm@v4` | v4 | Install Helm on runner | [VERIFIED: test.yml line 40] |
| bash | system | `helm-template-test.sh` runtime | [VERIFIED: script shebang `#!/usr/bin/env bash`] |

### No New Dependencies

This phase introduces no new libraries, tools, or actions. Everything runs with what the existing `helm-lint` job already provides.

---

## Architecture Patterns

### Existing CI Structure (test.yml)

```
jobs:
  test:          # Go build, vet, test — ubuntu-latest
  helm-lint:     # Helm lint+template — ubuntu-latest, Helm already installed
```

Phase 5 adds seven steps exclusively to `helm-lint`. No new jobs, no new setup steps.

### Existing Step Pattern (replicate exactly)

```yaml
- name: Lint (managed mode)
  run: helm lint chart/clay/ --values chart/clay/ci/managed-values.yaml

- name: Template (managed mode)
  run: helm template clay chart/clay/ --values chart/clay/ci/managed-values.yaml
```

The six new TLS steps follow this pattern verbatim — only the `name` and `--values` path differ.

### CI Values File Pattern (replicate from managed-values.yaml)

```yaml
# CI test values for <mode> TLS mode
secrets:
  ADMIN_PASS: "ci-test-only"
  SESSION_SECRET: "ci-test-session-secret-not-for-production"

postgres:
  managed: true
  cluster:
    instances: 1
    storage:
      size: 1Gi
      storageClass: ""
  external:
    dsn: ""

ingress:
  enabled: true
  host: shop.example.com
  tls:
    mode: <mode>
    # mode-specific keys below
```

[VERIFIED: managed-values.yaml + local helm lint for all three modes]

### helm-template-test.sh Invocation

The script uses `SCRIPT_DIR` to resolve `CHART_DIR` relative to its own location. It can be called from any directory, including the repo root (which is the default working directory for GitHub Actions steps after `actions/checkout`).

```yaml
- name: Behavioral tests (INGR-01..04, TLS-01..03)
  run: chart/tests/helm-template-test.sh
```

[VERIFIED: ran from /shared/pottery-shop (repo root) — 23 passed, 0 failed]

### Anti-Patterns to Avoid

- **Using `--validate` in `helm template` steps:** cert-manager CRDs are not installed on the GitHub Actions runner. Adding `--validate` causes the step to fail because it queries the cluster API. Current CI pattern correctly omits it. [VERIFIED: STATE.md decision, D-09]
- **Adding ingress block without all required keys:** `helm lint` will pass with `ingress.enabled: false` but will fail template rendering with `ingress.enabled: true` and no `host` set — the `clay.validateIngress` helper enforces this at render time. All three CI values files must include `host`, `tls.mode`, and mode-specific required keys. [VERIFIED: live chart test]
- **Mismatching secretName between custom values file and test script:** The behavioral test script's `CUSTOM_INGRESS` array uses `--set ingress.tls.secretName=my-tls`. The custom CI values file must also use `my-tls` for consistency, though the lint/template steps do not directly validate this alignment. [VERIFIED: test script line 43, D-06]

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead |
|---------|-------------|-------------|
| Helm runner setup | Custom install script | `azure/setup-helm@v4` (already used) |
| TLS mode validation in CI | Shell conditionals | Chart's `clay.validateIngress` helper (fires during `helm template`) |

---

## Runtime State Inventory

Step 2.5 SKIPPED — Phase 5 is not a rename/refactor/migration phase. It creates new files and appends to an existing workflow.

---

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Helm 3 | lint, template steps | Installed via `azure/setup-helm@v4` in helm-lint job | 3.x (action default) | — |
| bash | helm-template-test.sh | System bash on ubuntu-latest | system | — |
| GitHub Actions runner | test.yml | ubuntu-latest (existing) | — | — |

**Missing dependencies with no fallback:** None.

**Local verification:** `helm` is available in this environment and all three TLS modes lint-pass. [VERIFIED: local helm lint]

---

## Common Pitfalls

### Pitfall 1: `ingress.tls.mode` required key omitted from values file

**What goes wrong:** `helm lint` passes (the schema allows `mode: ""`), but `helm template` fails if `ingress.enabled: true` and `ingress.tls.mode` resolves to the empty string, because the Ingress template cannot determine which TLS resources to render.

**Why it happens:** The schema marks `mode` as a string with an enum including `""`. Helm lint checks JSON schema, not template logic. Template-time validation via `clay.validateIngress` is what catches a missing or empty mode.

**How to avoid:** All three CI values files must set `ingress.tls.mode` explicitly to a non-empty valid value. [VERIFIED: live chart test]

**Warning signs:** `helm template` exits non-zero; `helm lint` exits 0.

### Pitfall 2: Forgetting mode-specific required keys

**What goes wrong:** `helm template` with `tls.mode: letsencrypt` and no `acme.email` fails with `ingress.tls.acme.email required for letsencrypt mode`. Similarly, `mode: custom` without `secretName` fails with `ingress.tls.secretName required for custom mode`.

**How to avoid:** Per locked decisions — letsencrypt file includes `ingress.tls.acme.email: admin@example.com`; custom file includes `ingress.tls.secretName: my-tls`; selfsigned file needs neither. [VERIFIED: G-04, G-05 assertions in test script pass]

### Pitfall 3: Behavioral test script not executable

**What goes wrong:** CI step `run: chart/tests/helm-template-test.sh` fails with "Permission denied".

**How to avoid:** The script is already marked executable (`-rwxr-xr-x`). No `chmod` needed. However, if the test file is re-created from scratch it would lose the executable bit. Do not re-create it — it requires no changes. [VERIFIED: `ls -la` output]

### Pitfall 4: Step added to wrong job

**What goes wrong:** Adding the behavioral test step to the `test` job (which has no Helm installed) causes "helm: command not found".

**How to avoid:** All new steps, including `helm-template-test.sh`, go exclusively in the `helm-lint` job. Helm is already installed there via `azure/setup-helm@v4`. [VERIFIED: D-07, D-10]

---

## Code Examples

### Complete tls-letsencrypt-values.yaml

```yaml
# CI test values for letsencrypt TLS mode
secrets:
  ADMIN_PASS: "ci-test-only"
  SESSION_SECRET: "ci-test-session-secret-not-for-production"

postgres:
  managed: true
  cluster:
    instances: 1
    storage:
      size: 1Gi
      storageClass: ""
  external:
    dsn: ""

ingress:
  enabled: true
  host: shop.example.com
  tls:
    mode: letsencrypt
    acme:
      email: admin@example.com
```

[VERIFIED: helm lint passes with these values]

### Complete tls-selfsigned-values.yaml

```yaml
# CI test values for selfsigned TLS mode
secrets:
  ADMIN_PASS: "ci-test-only"
  SESSION_SECRET: "ci-test-session-secret-not-for-production"

postgres:
  managed: true
  cluster:
    instances: 1
    storage:
      size: 1Gi
      storageClass: ""
  external:
    dsn: ""

ingress:
  enabled: true
  host: shop.example.com
  tls:
    mode: selfsigned
```

[VERIFIED: helm lint passes with these values]

### Complete tls-custom-values.yaml

```yaml
# CI test values for custom TLS mode
secrets:
  ADMIN_PASS: "ci-test-only"
  SESSION_SECRET: "ci-test-session-secret-not-for-production"

postgres:
  managed: true
  cluster:
    instances: 1
    storage:
      size: 1Gi
      storageClass: ""
  external:
    dsn: ""

ingress:
  enabled: true
  host: shop.example.com
  tls:
    mode: custom
    secretName: my-tls
```

[VERIFIED: helm lint passes; secretName matches CUSTOM_INGRESS in helm-template-test.sh]

### Seven new steps for test.yml helm-lint job

```yaml
      - name: Lint (TLS — letsencrypt mode)
        run: helm lint chart/clay/ --values chart/clay/ci/tls-letsencrypt-values.yaml

      - name: Template (TLS — letsencrypt mode)
        run: helm template clay chart/clay/ --values chart/clay/ci/tls-letsencrypt-values.yaml

      - name: Lint (TLS — selfsigned mode)
        run: helm lint chart/clay/ --values chart/clay/ci/tls-selfsigned-values.yaml

      - name: Template (TLS — selfsigned mode)
        run: helm template clay chart/clay/ --values chart/clay/ci/tls-selfsigned-values.yaml

      - name: Lint (TLS — custom mode)
        run: helm lint chart/clay/ --values chart/clay/ci/tls-custom-values.yaml

      - name: Template (TLS — custom mode)
        run: helm template clay chart/clay/ --values chart/clay/ci/tls-custom-values.yaml

      - name: Behavioral tests (INGR-01..04, TLS-01..03)
        run: chart/tests/helm-template-test.sh
```

[VERIFIED: invocation pattern matches existing steps; script runs from repo root]

---

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | `chart/tests/helm-template-test.sh` (custom bash, 23 assertions) |
| Config file | none (self-contained script) |
| Quick run command | `chart/tests/helm-template-test.sh` |
| Full suite command | `chart/tests/helm-template-test.sh` |

### Phase Requirements to Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| CI-04 | Three TLS CI values files exist and produce valid lint/template output | smoke | `helm lint chart/clay/ --values chart/clay/ci/tls-{letsencrypt,selfsigned,custom}-values.yaml` | Create (Wave 0) |
| CI-05 | Six lint+template steps in test.yml, plus behavioral test step | integration | Push to main branch triggers test.yml | Edit (Wave 0) |

### Sampling Rate

- **Per task commit:** `chart/tests/helm-template-test.sh` (23 assertions, < 5 seconds)
- **Per wave merge:** `chart/tests/helm-template-test.sh` + manual `helm lint` for all five CI values files
- **Phase gate:** All 23 behavioral assertions green before `/gsd-verify-work`

### Wave 0 Gaps

- [ ] `chart/clay/ci/tls-letsencrypt-values.yaml` — new file, covers CI-04 (letsencrypt path)
- [ ] `chart/clay/ci/tls-selfsigned-values.yaml` — new file, covers CI-04 (selfsigned path)
- [ ] `chart/clay/ci/tls-custom-values.yaml` — new file, covers CI-04 (custom path)
- [ ] `.github/workflows/test.yml` — seven new steps, covers CI-05

No framework install needed — bash and helm are already available.

---

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | — |
| V3 Session Management | no | — |
| V4 Access Control | no | — |
| V5 Input Validation | no | Values files contain only CI placeholder strings |
| V6 Cryptography | no | — |

**Note:** CI values files use intentionally weak placeholder secrets (`ci-test-only`). This is the established pattern in the project (matching managed-values.yaml and external-values.yaml). The values are not deployed to any real cluster — they exist solely to satisfy Helm schema validation during lint/template. [VERIFIED: managed-values.yaml, D-02]

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| — | — | — | — |

**All claims in this research were verified against the live codebase or confirmed by running helm locally. No assumed claims.**

---

## Open Questions

None. The phase is fully defined by locked decisions and verified code inspection.

---

## Sources

### Primary (HIGH confidence — verified by running tools locally)

- Local `helm lint` and `helm template` runs — all three TLS modes confirmed passing
- `chart/tests/helm-template-test.sh` — run from repo root, 23/23 assertions passed
- `.github/workflows/test.yml` — read directly; step names and command format confirmed
- `chart/clay/ci/managed-values.yaml` — read directly; Postgres block and secrets pattern confirmed
- `chart/clay/ci/external-values.yaml` — read directly; formatting conventions confirmed
- `chart/clay/values.yaml` — read directly; ingress block structure confirmed
- `chart/clay/values.schema.json` — read directly; TLS mode enum confirmed

### Secondary (MEDIUM confidence)

None needed — all critical facts verified from the codebase directly.

---

## Metadata

**Confidence breakdown:**
- CI values file content: HIGH — all three modes lint-pass with exact values derived from locked decisions
- test.yml step structure: HIGH — replicated from existing step pattern, invocation verified
- Behavioral test script: HIGH — 23/23 assertions pass from repo root today

**Research date:** 2026-04-14
**Valid until:** Stable for this milestone; no external dependencies that could change
