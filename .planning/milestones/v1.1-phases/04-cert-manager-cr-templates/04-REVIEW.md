---
phase: 04-cert-manager-cr-templates
reviewed: 2026-04-14T00:00:00Z
depth: standard
files_reviewed: 5
files_reviewed_list:
  - .github/workflows/integration-test.yml
  - chart/clay/templates/cert-manager-letsencrypt.yaml
  - chart/clay/templates/cert-manager-selfsigned.yaml
  - chart/clay/templates/ingress.yaml
  - chart/tests/helm-template-test.sh
findings:
  critical: 0
  warning: 4
  info: 4
  total: 8
status: issues_found
---

# Phase 04: Code Review Report

**Reviewed:** 2026-04-14T00:00:00Z
**Depth:** standard
**Files Reviewed:** 5
**Status:** issues_found

## Summary

This phase adds cert-manager ClusterIssuer and Certificate CRs for both Let's Encrypt (ACME HTTP-01) and self-signed CA modes, wires the ingress template to the new TLS modes, and adds a Helm-template regression test script. The design is sound overall. The hook-weight sequencing in the self-signed CA chain is correct and the test coverage is thorough.

Four warnings and four info items were found. The most significant concerns are: the `cert-manager-letsencrypt.yaml` Certificate CR is missing a `namespace` field (ClusterIssuers are cluster-scoped but Certificates are namespaced), the ingress TLS block is always rendered even when `tls.mode` is unset/disabled, and the integration test workflow does not exercise the cert-manager code path it installs.

---

## Warnings

### WR-01: Certificate CR in letsencrypt template missing `namespace` field

**File:** `chart/clay/templates/cert-manager-letsencrypt.yaml:29-46`
**Issue:** The `Certificate` resource in the letsencrypt template (lines 29-46) does not specify `namespace: {{ .Release.Namespace }}`. The corresponding Certificate in `cert-manager-selfsigned.yaml` (line 19) does include the namespace field. Certificate is a namespaced resource; omitting the namespace causes cert-manager to look in the namespace where the CRD controller runs (`cert-manager`), not in the app namespace, so the resulting TLS Secret will be created in the wrong namespace (or the resource will be rejected if RBAC scopes differ).

**Fix:**
```yaml
# cert-manager-letsencrypt.yaml, Certificate metadata block
metadata:
  name: {{ include "clay.fullname" . }}-tls-cert
  namespace: {{ .Release.Namespace }}    # add this line — mirrors selfsigned template line 20
  labels:
    {{- include "clay.labels" . | nindent 4 }}
```

---

### WR-02: Ingress TLS block is always rendered, regardless of `tls.mode`

**File:** `chart/clay/templates/ingress.yaml:32-35`
**Issue:** The `tls:` block at lines 32-35 is rendered unconditionally inside the `if .Values.ingress.enabled` guard. There is no check on `tls.mode`. This is problematic in two ways: (1) when `tls.mode` is empty the `clay.tlsSecretName` helper returns `<fullname>-tls` (a non-existent secret), so the Ingress references a secret that will never be populated; (2) if a user intends HTTP-only ingress (arguably not supported by the current values schema, but `tls.mode` starts as `""` in values.yaml), the TLS block is still emitted. The validate helper does catch the empty mode case at render time, but only from `ingress.yaml:2`, so the ordering is fine — however the rendered output will still be wrong if validation somehow passes with an unexpected mode value in the future.

This is a latent correctness risk: if `validateIngress` is ever relaxed or bypassed, a broken TLS block renders silently.

**Fix:** Guard the TLS block on a non-empty mode (or explicitly on known modes), mirroring the per-mode conditional used for annotations:
```yaml
  {{- if .Values.ingress.tls.mode }}
  tls:
    - secretName: {{ include "clay.tlsSecretName" . }}
      hosts:
        - {{ .Values.ingress.host | quote }}
  {{- end }}
```

---

### WR-03: Integration test installs cert-manager but never exercises a cert-manager code path

**File:** `.github/workflows/integration-test.yml:69-82`
**Issue:** cert-manager is installed at lines 69-82, but the `helm install clay` step at line 99 sets `--set ingress.enabled=false`, which skips all cert-manager CRs. The operator is installed for nothing, and the cert-manager templates added in this phase have zero coverage in CI. A regression in the ClusterIssuer or Certificate templates would not be caught.

**Fix:** Add a second install/verify pass (or a separate job step) with `ingress.enabled=true`, `ingress.tls.mode=selfsigned`, and a dummy host. The self-signed mode requires no external DNS or ACME challenge and is therefore safe for CI. For example:
```yaml
- name: Verify cert-manager templates render and apply
  run: |
    helm upgrade clay chart/clay/ \
      --namespace clay \
      --set image.tag=pr-${{ github.event.pull_request.number }} \
      --set imagePullSecrets[0].name=ghcr-pull-secret \
      --set secrets.ADMIN_PASS=ci-test-only \
      --set secrets.SESSION_SECRET=ci-test-session-secret-not-for-production \
      --set ingress.enabled=true \
      --set ingress.host=pottery.ci.local \
      --set ingress.tls.mode=selfsigned \
      --timeout 5m \
      --wait
    kubectl get clusterissuers -n clay
    kubectl get certificates -n clay
```

---

### WR-04: Shell test script uses unquoted variable expansions in `helm template` calls

**File:** `chart/tests/helm-template-test.sh:58-60`, repeated throughout
**Issue:** The `${REQUIRED}`, `${CUSTOM_INGRESS}`, `${LETSENCRYPT_INGRESS}`, and `${SELFSIGNED_INGRESS}` variable expansions are left unquoted on the `helm` command lines (e.g., line 58-60, 76-78, 87-92). Because these variables contain multiple flags with spaces, word splitting by the shell is actually *required* here to produce separate arguments — but it is fragile: any value that contains a space or glob character (e.g., a host containing a wildcard) will break the invocation or silently mangle arguments.

The script is also declared `#!/usr/bin/env sh` (not bash) yet uses `$((...))` arithmetic expansion (POSIX sh compatible, fine) and `set -u` without `set -e` which means individual command failures (including helm rendering errors) do not abort the script — only the final summary exit code indicates failure.

**Fix:** Declare the script as `#!/usr/bin/env bash` to make quoting intent explicit, or convert the multi-flag variables to arrays so values can be quoted safely:
```bash
#!/usr/bin/env bash
set -euo pipefail

REQUIRED=(--set secrets.ADMIN_PASS=x --set secrets.SESSION_SECRET=x)
CUSTOM_INGRESS=(
  --set ingress.enabled=true
  --set ingress.host=shop.example.com
  --set ingress.tls.mode=custom
  --set ingress.tls.secretName=my-tls
)
# ... then invoke as:
OUTPUT=$("${HELM}" template release-test "${CHART_DIR}" "${REQUIRED[@]}" "${CUSTOM_INGRESS[@]}" 2>&1)
```

---

## Info

### IN-01: `cert-manager.io/cluster-issuer` annotation placed on Ingress in letsencrypt mode, but this annotation is not strictly needed when a Certificate CR is used

**File:** `chart/clay/templates/ingress.yaml:15`
**Issue:** When a `Certificate` CR explicitly references a `ClusterIssuer` (as done in `cert-manager-letsencrypt.yaml`), the `cert-manager.io/cluster-issuer` annotation on the Ingress is redundant — cert-manager responds to the Certificate CR directly. The annotation triggers cert-manager's ingress-shim, which would attempt to *also* create a Certificate from the Ingress annotation, potentially creating a duplicate or conflicting Certificate for the same secret name. This is a correctness risk if both mechanisms are active simultaneously.

**Fix:** Choose one issuing mechanism. Since the phase explicitly creates a `Certificate` CR, remove the `cert-manager.io/cluster-issuer` annotation from the Ingress (keep `acme.cert-manager.io/http01-edit-in-place: "true"` which is needed for HTTP-01 challenge routing). Alternatively, remove the explicit Certificate CR and rely solely on the Ingress annotation — but the explicit CR approach is more predictable.

---

### IN-02: `helm.sh/hook-weight` annotation absent on letsencrypt resources

**File:** `chart/clay/templates/cert-manager-letsencrypt.yaml:9`, `chart/clay/templates/cert-manager-letsencrypt.yaml:35`
**Issue:** The self-signed template correctly uses `helm.sh/hook-weight` annotations (`-10`, `-5`, `0`, `5`) to sequence the four resources in creation order. The letsencrypt template uses no hook weights on either the ClusterIssuer or the Certificate, leaving their relative creation order undefined. In practice, Helm applies resources in the order they appear in the rendered manifest, but this is an implementation detail and not guaranteed when hooks are involved. If cert-manager is slow to process the ClusterIssuer, the Certificate creation may fail with "issuer not ready".

**Fix:** Add hook weights to order the ClusterIssuer before the Certificate:
```yaml
# On the ClusterIssuer:
helm.sh/hook-weight: "-5"

# On the Certificate:
helm.sh/hook-weight: "0"
```

---

### IN-03: `acme.email` field in letsencrypt template is not quoted, risking YAML rendering edge cases

**File:** `chart/clay/templates/cert-manager-letsencrypt.yaml:13`
**Issue:** `email: {{ .Values.ingress.tls.acme.email }}` renders the email address without quotes. If a user provides an unusual value (e.g., one starting with `{` or containing `:`) the rendered YAML would be invalid. The validate helper checks the field is non-empty but does not constrain its format.

**Fix:**
```yaml
email: {{ .Values.ingress.tls.acme.email | quote }}
```

---

### IN-04: Redundant re-rendering of `helm template` output across closely adjacent test cases

**File:** `chart/tests/helm-template-test.sh:203`, `231`, `252`
**Issue:** G-09, G-10, and G-11 each independently invoke `helm template` with `${LETSENCRYPT_INGRESS}` (lines 203-205, 231-233, 252-254) and assign to separate variables (`OUTPUT_LE_09`, `OUTPUT_LE_10`, `OUTPUT_LE_11`). They test different properties of the same rendered output. This triples the helm subprocess cost for no reason. Similarly G-12 and G-13 re-render with `${SELFSIGNED_INGRESS}` redundantly.

**Fix:** Capture each distinct render once and reuse the variable across related assertions. The existing `OUTPUT_LE` and `OUTPUT_CUSTOM`/`OUTPUT_NGINX`/`OUTPUT_TLS` variables already demonstrate this pattern was sometimes followed. Consolidate G-09/G-10/G-11 into a single render and G-12/G-13 into a single render.

---

_Reviewed: 2026-04-14T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
