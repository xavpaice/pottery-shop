---
phase: 02-helm-ci
fixed_at: 2026-04-14T00:00:00Z
review_path: .planning/phases/02-helm-ci/02-REVIEW.md
iteration: 1
findings_in_scope: 5
fixed: 5
skipped: 0
status: all_fixed
---

# Phase 02: Code Review Fix Report

**Fixed at:** 2026-04-14T00:00:00Z
**Source review:** .planning/phases/02-helm-ci/02-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 5 (CR-01, CR-02, WR-01, WR-02, WR-03)
- Fixed: 5
- Skipped: 0

## Fixed Issues

### CR-01: Hardcoded default credentials in values.yaml

**Files modified:** `chart/clay/values.yaml`, `chart/clay/templates/_helpers.tpl`, `chart/clay/templates/deployment.yaml`, `chart/clay/ci/managed-values.yaml`, `chart/clay/ci/external-values.yaml`
**Commit:** 045d8ca
**Applied fix:** Cleared `secrets.ADMIN_PASS` and `secrets.SESSION_SECRET` defaults to empty strings with REQUIRED comments. Added `clay.validateSecrets` helper in `_helpers.tpl` that calls `{{- fail }}` when either secret is empty. Invoked the guard at the top of `deployment.yaml`. Added test credential values to both CI values files so `helm template` in CI continues to pass.

---

### CR-02: Plaintext DSN with credentials in CI values file

**Files modified:** `chart/clay/ci/external-values.yaml`, `chart/clay/values.yaml`
**Commit:** 045d8ca
**Applied fix:** Replaced `postgresql://user:pass@external-host:5432/clay` with `postgresql://clay-db.example.com:5432/clay` (no embedded credentials) in the CI external-values file. Added a SECURITY comment under `postgres.external.dsn` in `values.yaml` warning operators never to embed credentials in the DSN value. Note: the reviewer's deeper suggestion of a `dsnSecretRef` mechanism was not applied as it would require significant template restructuring and backward-compatibility changes; the comment guard and credential-free CI placeholder address the immediate risk.

---

### WR-01: DATABASE_URL is silently absent when neither condition is true

**Files modified:** `chart/clay/templates/deployment.yaml`
**Commit:** 045d8ca
**Applied fix:** Added a Helm `{{- fail }}` guard at the top of `deployment.yaml` (after the secrets validation call) that aborts rendering when `postgres.managed` is false and `postgres.external.dsn` is empty, preventing silent misconfiguration from reaching the cluster.

---

### WR-02: Init-container uses a mutable `postgres:16-alpine` image tag

**Files modified:** `chart/clay/templates/deployment.yaml`, `chart/clay/values.yaml`
**Commit:** 045d8ca
**Applied fix:** Replaced the hardcoded `postgres:16-alpine` image in the init-container with `{{ .Values.waitForPostgres.image | default "postgres:16.3-alpine3.20" }}`. Added the `waitForPostgres.image` key to `values.yaml` defaulting to `postgres:16.3-alpine3.20` with a comment explaining the pin rationale.

---

### WR-03: `envFrom` secrets reference — no ordering guarantee in external mode

**Files modified:** `chart/clay/values.yaml`
**Commit:** 045d8ca
**Applied fix:** The primary remediation for WR-03 is the WR-01 fail guard (which prevents the silent no-DATABASE_URL case). Added the SECURITY comment on `postgres.external.dsn` in `values.yaml` to document which secret source is active in each mode. Full restructuring of the external-mode secret reference path (i.e. sourcing DATABASE_URL from a Kubernetes Secret rather than a plain value) was not applied as it requires a `dsnSecretRef` API addition that is out of scope for a lint-pass fix; flagged for human review.

---

_Fixed: 2026-04-14T00:00:00Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
