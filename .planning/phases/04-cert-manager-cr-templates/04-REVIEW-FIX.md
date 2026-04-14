---
phase: 04-cert-manager-cr-templates
fixed_at: 2026-04-14T10:17:44Z
review_path: .planning/phases/04-cert-manager-cr-templates/04-REVIEW.md
iteration: 1
findings_in_scope: 4
fixed: 3
skipped: 1
status: partial
---

# Phase 04: Code Review Fix Report

**Fixed at:** 2026-04-14T10:17:44Z
**Source review:** .planning/phases/04-cert-manager-cr-templates/04-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 4
- Fixed: 3
- Skipped: 1

## Fixed Issues

### WR-02: Ingress TLS block is always rendered, regardless of `tls.mode`

**Files modified:** `chart/clay/templates/ingress.yaml`
**Commit:** d29309e
**Applied fix:** Wrapped the `tls:` block in `{{- if .Values.ingress.tls.mode }}` / `{{- end }}` so the TLS section only renders when a mode is explicitly set, preventing a dangling reference to a non-existent secret when `tls.mode` is empty.

---

### WR-03: Integration test installs cert-manager but never exercises a cert-manager code path

**Files modified:** `.github/workflows/integration-test.yml`
**Commit:** b191e3e
**Applied fix:** Added a "Verify cert-manager templates render and apply" step after the existing deployment verification. The step runs `helm upgrade` with `ingress.enabled=true`, `ingress.tls.mode=selfsigned`, and `ingress.host=pottery.ci.local`, then runs `kubectl get clusterissuers` and `kubectl get certificates` to confirm the cert-manager CRs were created. The self-signed mode requires no external DNS or ACME challenge, making it safe for CI.

---

### WR-04: Shell test script uses unquoted variable expansions in `helm template` calls

**Files modified:** `chart/tests/helm-template-test.sh`
**Commit:** e8580aa
**Applied fix:** Changed shebang from `#!/usr/bin/env sh` to `#!/usr/bin/env bash` and `set -u` to `set -euo pipefail`. Converted all four multi-flag string variables (`REQUIRED`, `CUSTOM_INGRESS`, `LETSENCRYPT_INGRESS`, `SELFSIGNED_INGRESS`) to bash arrays. Updated all 16 call sites to use `"${VAR[@]}"` array expansion syntax. Added `|| true` to expected-failure invocations (G-03, G-04, G-05) and to lint captures (G-08a, G-08b) so `set -e` does not abort the script on intentional failures.

---

## Skipped Issues

### WR-01: Certificate CR in letsencrypt template missing `namespace` field

**File:** `chart/clay/templates/cert-manager-letsencrypt.yaml:29-46`
**Reason:** skipped: code already contains the fix; `namespace: {{ .Release.Namespace }}` is present at line 32 of the file. The reviewer noted it was missing but the implementation already includes it, matching the selfsigned template. No change needed.
**Original issue:** Certificate CR in letsencrypt template missing `namespace: {{ .Release.Namespace }}` field, causing cert-manager to create the TLS Secret in the wrong namespace.

---

_Fixed: 2026-04-14T10:17:44Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
