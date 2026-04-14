---
phase: 02-helm-ci
reviewed: 2026-04-14T00:00:00Z
depth: standard
files_reviewed: 8
files_reviewed_list:
  - .github/workflows/test.yml
  - chart/clay/.gitignore
  - chart/clay/Chart.yaml
  - chart/clay/ci/external-values.yaml
  - chart/clay/ci/managed-values.yaml
  - chart/clay/templates/cnpg-cluster.yaml
  - chart/clay/templates/deployment.yaml
  - chart/clay/values.yaml
findings:
  critical: 2
  warning: 3
  info: 3
  total: 8
status: issues_found
---

# Phase 02: Code Review Report

**Reviewed:** 2026-04-14T00:00:00Z
**Depth:** standard
**Files Reviewed:** 8
**Status:** issues_found

## Summary

This phase adds a Helm chart (`chart/clay/`) for the pottery-shop application targeting CloudNative-PG (CNPG), and a GitHub Actions workflow that lints and templates the chart in CI. The overall structure is sound — the two-mode design (managed CNPG cluster vs external DSN) is correctly gated throughout — but several issues were found ranging from hardcoded default secrets in `values.yaml` (critical) to a plaintext DSN in the CI values file (critical), along with three warnings around missing environment variable fallback logic, an insecure init-container image tag, and a missing `envFrom`/`env` ordering hazard.

---

## Critical Issues

### CR-01: Hardcoded default credentials in values.yaml

**File:** `chart/clay/values.yaml:44-45`
**Issue:** `ADMIN_PASS` is set to `"changeme"` and `SESSION_SECRET` is set to `"change-this-to-a-random-string"`. These are rendered verbatim into the Kubernetes Secret via `secret.yaml`. Helm charts are frequently deployed with default values — any cluster where an operator forgets to override these values ships with a known admin password and a predictable HMAC signing key. The session cookie signature can then be forged by anyone who knows the default.

**Fix:** Remove the default values and require the caller to supply them, or set them to empty strings and add a `NOTES.txt` / `_helpers.tpl` validation that fails the render when they are empty:

```yaml
# values.yaml — remove defaults entirely
secrets:
  ADMIN_PASS: ""          # REQUIRED — must be set at deploy time
  SESSION_SECRET: ""      # REQUIRED — must be set at deploy time
  SMTP_USER: ""
  SMTP_PASS: ""
```

Add a validation guard in `templates/_helpers.tpl` or a dedicated `templates/validate.yaml`:

```yaml
{{- if not .Values.secrets.ADMIN_PASS }}
  {{- fail "secrets.ADMIN_PASS must be set" }}
{{- end }}
{{- if not .Values.secrets.SESSION_SECRET }}
  {{- fail "secrets.SESSION_SECRET must be set" }}
{{- end }}
```

---

### CR-02: Plaintext DSN with credentials in CI values file

**File:** `chart/clay/ci/external-values.yaml:10`
**Issue:** `dsn: "postgresql://user:pass@external-host:5432/clay"` stores a credential-containing connection string in a committed YAML file. While `user:pass` is a placeholder here, this pattern is routinely cargo-culted into real deployments. More concretely, the `deployment.yaml` template at line 51 injects this DSN value directly as an unmasked environment variable (`value: {{ .Values.postgres.external.dsn | quote }}`), bypassing Kubernetes Secret handling — meaning the password is visible in plain text in the Pod spec to anyone with `kubectl get pod -o yaml` or `kubectl describe pod` access.

**Fix:** In the CI file, use a fake DSN without embedded credentials to make the template testable while not encouraging the credential-in-value pattern:

```yaml
# ci/external-values.yaml
postgres:
  external:
    dsn: "postgresql://clay-db.example.com:5432/clay"
```

In `deployment.yaml`, source the DSN from a Secret reference instead of a plain value. Operators should be instructed to pre-create a secret containing the DSN, and the chart should reference it:

```yaml
# deployment.yaml — replace lines 48-51
{{- else if .Values.postgres.external.dsnSecretRef }}
env:
  - name: DATABASE_URL
    valueFrom:
      secretKeyRef:
        name: {{ .Values.postgres.external.dsnSecretRef.name | quote }}
        key: {{ .Values.postgres.external.dsnSecretRef.key | default "dsn" | quote }}
{{- end }}
```

At minimum, add a `values.yaml` comment warning that `dsn` must never contain credentials; they should be stored in a pre-created Secret.

---

## Warnings

### WR-01: DATABASE_URL is silently absent when neither condition is true

**File:** `chart/clay/templates/deployment.yaml:41-52`
**Issue:** The `env` block (which sets `DATABASE_URL`) is only rendered when `postgres.managed == true` OR when `postgres.external.dsn` is non-empty. If an operator sets `postgres.managed: false` and leaves `postgres.external.dsn: ""` (the default), the application container launches with no `DATABASE_URL` at all. The app will crash at startup — but Helm will render and apply the manifest without complaint. This is a silent misconfiguration trap.

The `envFrom` block (lines 55-59) that mounts the ConfigMap and the application-level Secret is rendered unconditionally _outside_ the env block, meaning it appears after the env items. Kubernetes merges `env` and `envFrom` but prefers `env` for duplicate keys — the ordering is not itself a bug, but note that if a future change sets `DATABASE_URL` in the ConfigMap it would be silently shadowed.

**Fix:** Add a Helm validation guard:

```yaml
{{- if and (not .Values.postgres.managed) (not .Values.postgres.external.dsn) }}
  {{- fail "Either postgres.managed must be true or postgres.external.dsn must be set" }}
{{- end }}
```

Alternatively, add a `NOTES.txt` warning users to check the value.

---

### WR-02: Init-container uses a mutable `postgres:16-alpine` image tag

**File:** `chart/clay/templates/deployment.yaml:27`
**Issue:** The wait-for-postgres init-container is pinned to `postgres:16-alpine` with no digest or more specific tag. Alpine patch releases of this image regularly update, and `16-alpine` can change on Docker Hub at any time. In a pull-always environment this could silently introduce a different `pg_isready` binary. More importantly, scanning tools will flag mutable tags as a supply-chain risk.

**Fix:** Either use a specific patch version tag (`postgres:16.3-alpine3.20`) and make it configurable via values, or replace the init-container entirely with a lightweight busybox/netcat loop to avoid pulling a full Postgres image just for `pg_isready`:

```yaml
# values.yaml — add
waitForPostgres:
  image: postgres:16.3-alpine3.20
```

```yaml
# deployment.yaml — line 27
image: {{ .Values.waitForPostgres.image | default "postgres:16.3-alpine3.20" }}
```

---

### WR-03: `envFrom` secrets reference created by the same chart — no ordering guarantee in external mode

**File:** `chart/clay/templates/deployment.yaml:55-59`
**Issue:** The deployment unconditionally references a ConfigMap and Secret both named `{{ include "clay.fullname" . }}` via `envFrom`. The Secret template (`secret.yaml`) is always rendered regardless of mode, so this is correct for managed mode. However in external mode the CNPG-generated secret `{{ include "clay.fullname" . }}-postgres-app` is NOT referenced, and the app-level Secret from `secret.yaml` does not contain `DATABASE_URL`. Combined with WR-01, this means the app pod starts without any database URL in its environment. The warning here is that the two sources of truth (in-chart Secret vs CNPG-generated Secret) are silently inconsistent and neither the linter nor `helm template` will catch the misconfiguration.

**Fix:** This is addressed by the guard added for WR-01. Additionally, document in `values.yaml` comments which secret names are expected in each mode.

---

## Info

### IN-01: `image.tag: latest` default in values.yaml

**File:** `chart/clay/values.yaml:7`
**Issue:** The default image tag is `latest`, which is mutable and makes deployments non-reproducible. `latest` also defeats Kubernetes' `IfNotPresent` pull policy since the runtime cannot detect when `latest` has changed upstream.

**Fix:**

```yaml
image:
  tag: "0.1.0"  # pin to a specific release tag
```

Or document that `--set image.tag=<sha>` is required on every deploy.

---

### IN-02: CI workflow does not pin action versions to SHA digests

**File:** `.github/workflows/test.yml:14,17,40`
**Issue:** `actions/checkout@v4`, `actions/setup-go@v5`, and `azure/setup-helm@v4` are referenced by mutable tag refs. A compromised or accidentally updated action tag could inject malicious steps. The Go ecosystem and many security guidelines recommend SHA-pinned actions for supply-chain safety.

**Fix:**

```yaml
- uses: actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683  # v4.2.2
- uses: actions/setup-go@d60b41a563a30f0bf8a6492b40c5b2f67de3ea50  # v5.5.0
- uses: azure/setup-helm@b9e51907a09c216f16ab6b52b6b5de5a8a02b10e  # v4.3.0
```

---

### IN-03: Commented-out / placeholder ORDER_EMAIL in values.yaml

**File:** `chart/clay/values.yaml:41`
**Issue:** `ORDER_EMAIL: "xavpaice@gmail.com"` is a personal email address hardcoded as the default order recipient. This is not a security vulnerability (it goes into a ConfigMap, not a Secret) but it is a data-quality issue — deployments that forget to override this will send real customer orders to the wrong recipient.

**Fix:**

```yaml
ORDER_EMAIL: ""  # REQUIRED — set to the mailbox that receives shop orders
```

And add a Helm validation guard or `NOTES.txt` reminder.

---

_Reviewed: 2026-04-14T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
