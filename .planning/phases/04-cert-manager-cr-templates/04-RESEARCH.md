# Phase 4: cert-manager CR Templates - Research

**Researched:** 2026-04-14
**Domain:** Helm chart templating — cert-manager ClusterIssuer and Certificate CRs, Helm hooks, CI workflow
**Confidence:** HIGH

## Summary

Phase 4 is a pure Helm template authoring phase: two new template files (`cert-manager-letsencrypt.yaml`, `cert-manager-selfsigned.yaml`), a one-annotation addition to `ingress.yaml`, and a CI step insertion into `integration-test.yml`. No Go code, no values.yaml changes, and no new helpers are required — all values (`ingress.tls.mode`, `ingress.tls.acme.email`, `ingress.tls.acme.production`) and helpers (`clay.tlsSecretName`, `clay.fullname`, `clay.labels`) already exist from Phase 3.

All user decisions are locked in CONTEXT.md. Research verified exact cert-manager CR API shapes, Helm hook annotation semantics, the confirmed existence of cert-manager v1.20.2, and the correct Helm install flags. The selfsigned mode uses a four-resource two-step CA bootstrap pattern documented in the official cert-manager SelfSigned docs. Hook weights are quoted strings; ascending numeric order controls creation sequence.

**Primary recommendation:** Write templates directly using the verified CR specs below. No new helpers are needed. The only non-trivial design risk is the `privateKeySecretRef` in the ACME ClusterIssuer — it must be a release-scoped name to avoid cross-release collision.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**D-01:** Two files by mode — `chart/clay/templates/cert-manager-letsencrypt.yaml` and `chart/clay/templates/cert-manager-selfsigned.yaml`. Each file is self-contained.

**D-02:** Each file is gated by `{{- if and .Values.ingress.enabled (eq .Values.ingress.tls.mode "<mode>") }}` at the top level.

**D-03:** All resource names are release-derived using `clay.fullname`:
- letsencrypt ClusterIssuer: `{{ include "clay.fullname" . }}-letsencrypt`
- selfsigned root ClusterIssuer: `{{ include "clay.fullname" . }}-selfsigned-root`
- selfsigned CA Certificate: `{{ include "clay.fullname" . }}-ca`
- selfsigned CA ClusterIssuer: `{{ include "clay.fullname" . }}-selfsigned-ca`
- App Certificate (both modes): `{{ include "clay.fullname" . }}-tls-cert`

**D-04:** All resources carry standard `clay.labels` and a `clay.fullname`-based name.

**D-05:** Every ClusterIssuer and Certificate resource carries:
```yaml
annotations:
  helm.sh/hook: post-install,post-upgrade
  helm.sh/hook-delete-policy: before-hook-creation
```

**D-06:** Add `cert-manager.io/cluster-issuer: {{ include "clay.fullname" . }}-letsencrypt` to `ingress.yaml` metadata annotations, scoped to `{{- if eq .Values.ingress.tls.mode "letsencrypt" }}`.

**D-07:** ACME HTTP-01 challenger. Staging endpoint by default; production opt-in via `ingress.tls.acme.production: true`:
- staging: `https://acme-staging-v02.api.letsencrypt.org/directory`
- production: `https://acme-v02.api.letsencrypt.org/directory`

**D-08:** `spec.acme.email` from `{{ .Values.ingress.tls.acme.email }}`

**D-09:** HTTP-01 solver uses `ingress.className` `{{ .Values.ingress.className }}` (traefik by default).

**D-10:** `spec.secretName` from `{{ include "clay.tlsSecretName" . }}`

**D-11:** `spec.issuerRef.kind: ClusterIssuer`, `spec.issuerRef.name: {{ include "clay.fullname" . }}-letsencrypt`

**D-12:** `spec.dnsNames: [{{ .Values.ingress.host }}]`

**D-13:** Four resources in `cert-manager-selfsigned.yaml` in logical order: SelfSigned ClusterIssuer → CA Certificate (isCA: true) → CA ClusterIssuer → App Certificate.

**D-14:** Hook-weight used to sequence the four selfsigned resources: root="-10", ca-cert="-5", ca-issuer="0", app-cert="5".

**D-15:** cert-manager pre-install steps in `integration-test.yml`:
```yaml
- name: Add cert-manager Helm repo
  run: helm repo add jetstack https://charts.jetstack.io

- name: Install cert-manager
  run: |
    helm install cert-manager jetstack/cert-manager \
      --namespace cert-manager \
      --create-namespace \
      --set crds.enabled=true \
      --wait \
      --timeout 3m
```
Version: cert-manager v1.20.2 (pinned via `--version 1.20.2`).

**D-16:** Clay chart install step keeps `--set ingress.enabled=false`. TLS mode validation handled by `helm template` in Phase 5.

### Claude's Discretion

- `helm.sh/hook-weight` string values (exact integers may shift — as long as relative order is root < ca-cert < ca-issuer < app-cert)
- Whether to add a `# ACME staging vs production` inline comment to the ClusterIssuer template for operator clarity
- Exact `tolerations`/`solvers` structure in the letsencrypt ClusterIssuer if Traefik requires additional ingress annotations for the solver

### Deferred Ideas (OUT OF SCOPE)

- None — discussion stayed within phase scope
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| TLS-01 | User can enable Let's Encrypt mode (`ingress.tls.mode: letsencrypt`) — HTTP-01 ACME ClusterIssuer, staging ACME URL by default, production opt-in via `ingress.tls.acme.production: true` | Verified exact ClusterIssuer ACME spec from official cert-manager docs; staging/production URLs confirmed |
| TLS-02 | User can enable self-signed mode (`ingress.tls.mode: selfsigned`) — two-step CA bootstrap: SelfSigned ClusterIssuer → CA cert → CA ClusterIssuer → app cert | Verified four-resource bootstrap pattern from official cert-manager SelfSigned docs; hook-weight sequencing verified |
| CI-06 | `integration-test.yml` includes a cert-manager v1.20.2 pre-install step (mirrors existing CNPG pre-install pattern) | v1.20.2 existence confirmed (released 2025-04-11); Helm repo URL verified; `crds.enabled=true` flag verified as current syntax |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| cert-manager Helm chart (jetstack) | v1.20.2 | Installs CRDs + controller in CI pre-step | Pinned per D-15; released 2025-04-11 |
| cert-manager CR API | cert-manager.io/v1 | ClusterIssuer and Certificate resource apiVersion | Current stable API group |

No new Go dependencies. No new Helm chart dependencies. This phase is Helm YAML only.

**Installation (CI pre-step only — not chart dependency):**
```bash
helm repo add jetstack https://charts.jetstack.io
helm install cert-manager jetstack/cert-manager \
  --version 1.20.2 \
  --namespace cert-manager \
  --create-namespace \
  --set crds.enabled=true \
  --wait \
  --timeout 3m
```

**Version verification:** cert-manager v1.20.2 confirmed on ArtifactHub and GitHub releases. Released 2025-04-11. [VERIFIED: https://github.com/cert-manager/cert-manager/releases/tag/v1.20.2]

**Flag verification:** `crds.enabled=true` is the current flag syntax (not `installCRDs=true`, which is the older syntax). [VERIFIED: https://cert-manager.io/docs/installation/helm/]

## Architecture Patterns

### Recommended Template File Structure
```
chart/clay/templates/
├── _helpers.tpl                    # existing — no changes needed
├── cert-manager-letsencrypt.yaml   # NEW — letsencrypt ClusterIssuer + Certificate
├── cert-manager-selfsigned.yaml    # NEW — four-resource CA bootstrap
├── ingress.yaml                    # MODIFY — add cert-manager.io/cluster-issuer annotation
└── ... (no other files touched)
```

### Pattern 1: letsencrypt ClusterIssuer (ACME HTTP-01)

**What:** A ClusterIssuer that registers with Let's Encrypt ACME and uses HTTP-01 challenge to issue certs.

**Critical fields:**
- `spec.acme.server` — staging URL by default, production opt-in
- `spec.acme.email` — required by Let's Encrypt for account registration
- `spec.acme.privateKeySecretRef.name` — stores the ACME account private key; must be release-scoped to avoid cluster-wide naming collision (use `clay.fullname` suffix)
- `spec.acme.solvers[0].http01.ingress.ingressClassName` — use `ingressClassName` field (added cert-manager v1.12), not the legacy `class` field

**Example:**
```yaml
# Source: https://cert-manager.io/docs/configuration/acme/http01/
{{- if and .Values.ingress.enabled (eq .Values.ingress.tls.mode "letsencrypt") }}
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: {{ include "clay.fullname" . }}-letsencrypt
  labels:
    {{- include "clay.labels" . | nindent 4 }}
  annotations:
    helm.sh/hook: post-install,post-upgrade
    helm.sh/hook-delete-policy: before-hook-creation
spec:
  acme:
    email: {{ .Values.ingress.tls.acme.email }}
    {{- if .Values.ingress.tls.acme.production }}
    server: https://acme-v02.api.letsencrypt.org/directory
    {{- else }}
    server: https://acme-staging-v02.api.letsencrypt.org/directory
    {{- end }}
    privateKeySecretRef:
      name: {{ include "clay.fullname" . }}-letsencrypt-key
    solvers:
      - http01:
          ingress:
            ingressClassName: {{ .Values.ingress.className }}
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: {{ include "clay.fullname" . }}-tls-cert
  labels:
    {{- include "clay.labels" . | nindent 4 }}
  annotations:
    helm.sh/hook: post-install,post-upgrade
    helm.sh/hook-delete-policy: before-hook-creation
spec:
  secretName: {{ include "clay.tlsSecretName" . }}
  dnsNames:
    - {{ .Values.ingress.host }}
  issuerRef:
    name: {{ include "clay.fullname" . }}-letsencrypt
    kind: ClusterIssuer
    group: cert-manager.io
{{- end }}
```

### Pattern 2: selfsigned Two-Step CA Bootstrap

**What:** Four resources that chain: SelfSigned issuer → CA Certificate (isCA: true) → CA Issuer → App Certificate. This avoids the pitfall of an untrusted end-entity certificate by establishing a proper CA chain.

**Hook-weight sequencing:** All four resources share `helm.sh/hook: post-install,post-upgrade`. Hook-weights control creation order (ascending = first). Strings, not integers.

**CA Certificate namespace:** Certificate CRs are namespaced. The CA Certificate must be in the same namespace as the release (Helm's `.Release.Namespace`). ClusterIssuers are cluster-scoped (no namespace).

**CA ClusterIssuer references the CA Secret by name:** The `spec.ca.secretName` in the CA ClusterIssuer must match the `spec.secretName` in the CA Certificate exactly.

**Example:**
```yaml
# Source: https://cert-manager.io/docs/configuration/selfsigned/
{{- if and .Values.ingress.enabled (eq .Values.ingress.tls.mode "selfsigned") }}
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: {{ include "clay.fullname" . }}-selfsigned-root
  labels:
    {{- include "clay.labels" . | nindent 4 }}
  annotations:
    helm.sh/hook: post-install,post-upgrade
    helm.sh/hook-delete-policy: before-hook-creation
    helm.sh/hook-weight: "-10"
spec:
  selfSigned: {}
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: {{ include "clay.fullname" . }}-ca
  namespace: {{ .Release.Namespace }}
  labels:
    {{- include "clay.labels" . | nindent 4 }}
  annotations:
    helm.sh/hook: post-install,post-upgrade
    helm.sh/hook-delete-policy: before-hook-creation
    helm.sh/hook-weight: "-5"
spec:
  isCA: true
  commonName: {{ include "clay.fullname" . }}-ca
  secretName: {{ include "clay.fullname" . }}-ca-tls
  issuerRef:
    name: {{ include "clay.fullname" . }}-selfsigned-root
    kind: ClusterIssuer
    group: cert-manager.io
---
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: {{ include "clay.fullname" . }}-selfsigned-ca
  labels:
    {{- include "clay.labels" . | nindent 4 }}
  annotations:
    helm.sh/hook: post-install,post-upgrade
    helm.sh/hook-delete-policy: before-hook-creation
    helm.sh/hook-weight: "0"
spec:
  ca:
    secretName: {{ include "clay.fullname" . }}-ca-tls
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: {{ include "clay.fullname" . }}-tls-cert
  namespace: {{ .Release.Namespace }}
  labels:
    {{- include "clay.labels" . | nindent 4 }}
  annotations:
    helm.sh/hook: post-install,post-upgrade
    helm.sh/hook-delete-policy: before-hook-creation
    helm.sh/hook-weight: "5"
spec:
  secretName: {{ include "clay.tlsSecretName" . }}
  dnsNames:
    - {{ .Values.ingress.host }}
  issuerRef:
    name: {{ include "clay.fullname" . }}-selfsigned-ca
    kind: ClusterIssuer
    group: cert-manager.io
{{- end }}
```

### Pattern 3: Ingress Annotation Addition (D-06)

Add one conditional block to the existing `ingress.yaml` annotations section:

```yaml
# Source: ingress.yaml (existing file, add this block after the existing annotations)
    {{- if eq .Values.ingress.tls.mode "letsencrypt" }}
    cert-manager.io/cluster-issuer: {{ include "clay.fullname" . }}-letsencrypt
    {{- end }}
```

### Pattern 4: CI Pre-Install Step Structure

The existing CNPG pre-install in `integration-test.yml` is the verbatim model. Add two steps between the CNPG install and the clay chart install:

```yaml
# Source: pattern from existing CNPG steps in integration-test.yml
- name: Add cert-manager Helm repo
  run: helm repo add jetstack https://charts.jetstack.io

- name: Install cert-manager
  run: |
    helm install cert-manager jetstack/cert-manager \
      --version 1.20.2 \
      --namespace cert-manager \
      --create-namespace \
      --set crds.enabled=true \
      --wait \
      --timeout 3m
```

### Anti-Patterns to Avoid

- **Using `installCRDs=true`:** This is the old flag syntax (pre-v1.15). Current syntax is `crds.enabled=true`. [VERIFIED: cert-manager.io/docs/installation/helm/]
- **Using `class:` instead of `ingressClassName:`:** The `class` field uses the deprecated `kubernetes.io/ingress.class` annotation and should not be used in new charts. Use `ingressClassName` (available since cert-manager v1.12). [VERIFIED: cert-manager.io/docs/configuration/acme/http01/]
- **Hardcoded `privateKeySecretRef` name:** The ACME account private key secret name must be release-scoped, not hardcoded (e.g., `letsencrypt-key`). Multiple clay releases on the same cluster would collide.
- **CA Certificate in wrong namespace:** Certificate CRs are namespaced. The CA Certificate for the selfsigned bootstrap must include `namespace: {{ .Release.Namespace }}`. ClusterIssuers are cluster-scoped and must NOT have a namespace.
- **Using `selfSigned: {}` as the final issuer:** Issuing an end-entity cert directly from a SelfSigned ClusterIssuer produces a self-signed leaf cert (no CA chain). The two-step bootstrap creates a CA cert first, then issues app certs through the CA.
- **Missing `group: cert-manager.io`:** The `issuerRef.group` field is required on both Certificate CRs to explicitly target the cert-manager issuer group. Omitting it works in practice but is not documented as safe.
- **Omitting `--version` in CI install:** Without `--version 1.20.2`, the CI step would use whatever latest is, breaking reproducibility.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| ACME challenge infrastructure | Custom HTTP-01 challenge server | cert-manager ClusterIssuer + cert-manager controller | Challenge routing, token generation, certificate issuance, renewal, and retry logic are all handled; hand-rolling any part misses edge cases |
| CA certificate bootstrap | Manually creating CA certs | cert-manager SelfSigned + CA issuer chain | cert-manager manages rotation, renewal, private key storage, and the CA chain automatically |
| Secret name consistency | Custom logic to sync names | `clay.tlsSecretName` helper (already exists) | The helper is the single source of truth; both Ingress and Certificate CR reference the same template output |

## Common Pitfalls

### Pitfall 1: Hook Resources and Helm Upgrade "Already Exists" Errors

**What goes wrong:** On `helm upgrade`, Helm tries to create hook resources that already exist from the previous install.

**Why it happens:** Hook resources are not part of Helm's managed resource set unless a delete policy is specified.

**How to avoid:** Use `helm.sh/hook-delete-policy: before-hook-creation` on every ClusterIssuer and Certificate. This causes Helm to delete the old resource before recreating it on each install/upgrade. D-05 mandates this.

**Warning signs:** Helm upgrade fails with "ClusterIssuer already exists" error.

### Pitfall 2: CA Secret Name Mismatch in selfsigned Bootstrap

**What goes wrong:** The CA ClusterIssuer's `spec.ca.secretName` does not match the CA Certificate's `spec.secretName`, causing the CA ClusterIssuer to fail to find its signing material.

**Why it happens:** The CA Certificate writes its key+cert to a Secret named by `spec.secretName`. The CA ClusterIssuer reads from `spec.ca.secretName`. If they differ, the ClusterIssuer is permanently broken.

**How to avoid:** Both must use `{{ include "clay.fullname" . }}-ca-tls` (a non-tlsSecretName derivation, since this is the intermediate CA Secret, not the app Secret).

**Warning signs:** CA ClusterIssuer shows status `NotReady: secret not found`.

### Pitfall 3: Webhook Timing Race Without Hooks

**What goes wrong:** Installing cert-manager CRDs and then immediately creating ClusterIssuer or Certificate CRs in the same Helm install causes "no kind ClusterIssuer is registered for version cert-manager.io/v1" errors.

**Why it happens:** cert-manager's validating webhook is not ready immediately after CRD installation; new CR submissions hit the webhook before the cert-manager pod is running.

**How to avoid:** This is why cert-manager is installed as a pre-step (not subchart) and why the clay chart's cert-manager CRs carry `post-install,post-upgrade` hook annotations. D-05 addresses this.

**Warning signs:** `helm install` fails with webhook timeout or "no kind registered" during CI.

### Pitfall 4: selfsigned App Certificate "No Common Name or SANs" Error

**What goes wrong:** The app Certificate has neither `commonName` nor `dnsNames`, causing cert-manager to reject it.

**Why it happens:** cert-manager requires at least one subject identifier.

**How to avoid:** Always set `dnsNames: [{{ .Values.ingress.host }}]` on the app Certificate. D-12 mandates this.

### Pitfall 5: `helm lint` Fails Because cert-manager CRDs Are Absent

**What goes wrong:** `helm lint` in CI (test.yml) fails because ClusterIssuer and Certificate are unknown resource types without cert-manager installed.

**Why it happens:** `helm lint` uses the Kubernetes API schema to validate resources unless `--no-validate` is used. Without cert-manager CRDs, the custom resource types are unrecognized.

**How to avoid:** Phase 5 handles this with `helm template` (not `helm lint --validate`) for TLS mode variants. Phase 4 templates must not break existing `managed-values.yaml` and `external-values.yaml` lint jobs (those don't enable ingress, so cert-manager templates are not rendered).

**Warning signs:** CI test.yml helm-lint job fails after Phase 4 merge.

**Mitigation already built in:** The gate `{{- if and .Values.ingress.enabled (eq .Values.ingress.tls.mode "letsencrypt") }}` ensures cert-manager CRs are only rendered when explicitly requested. The existing CI lint values files (`managed-values.yaml`, `external-values.yaml`) do not set `ingress.enabled=true`, so they render zero cert-manager resources. Lint continues to pass.

### Pitfall 6: `acme.cert-manager.io/http01-edit-in-place` vs `cert-manager.io/cluster-issuer`

**What goes wrong:** Confusing the two Ingress annotations.

**Distinction:**
- `acme.cert-manager.io/http01-edit-in-place: "true"` — already in `ingress.yaml` from Phase 3; tells cert-manager to add the HTTP-01 challenge path to the *existing* Ingress rather than creating a new one. This is Traefik-specific behavior.
- `cert-manager.io/cluster-issuer: <name>` — the annotation Phase 4 adds; tells cert-manager which ClusterIssuer should automatically issue a Certificate for this Ingress. This is the annotation that triggers auto-cert-management via Ingress annotation mode.

Both are needed for letsencrypt mode. Phase 4 adds the second.

## Runtime State Inventory

Step 2.5 does not apply. This is a greenfield template authoring phase — no rename, refactor, or migration involved. No runtime state to audit.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| helm | Template verification, CI | ✓ | (system) | — |
| cert-manager (cluster) | Runtime cert issuance | Not required for Phase 4 | N/A | Phase 4 only authors templates; runtime cert-manager is CI concern (integration-test.yml pre-install) |

**Phase 4 requires no external runtime dependencies for template authoring.** `helm template` validation can be run locally without a cluster. cert-manager only needs to be running at integration test time (handled by D-15).

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | `helm template` + `chart/tests/helm-template-test.sh` (existing shell-based) |
| Config file | none — invoked directly |
| Quick run command | `helm template clay chart/clay --set ingress.enabled=true --set ingress.host=shop.example.com --set ingress.tls.mode=letsencrypt --set ingress.tls.acme.email=admin@example.com` |
| Full suite command | `chart/tests/helm-template-test.sh` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| TLS-01 | letsencrypt mode renders ClusterIssuer (ACME HTTP-01, staging) + Certificate CR with hook annotations + Ingress cluster-issuer annotation | unit (helm template) | `helm template clay chart/clay --set ingress.enabled=true --set ingress.host=shop.example.com --set ingress.tls.mode=letsencrypt --set ingress.tls.acme.email=admin@example.com` | ❌ Wave 0 — new test assertions needed |
| TLS-02 | selfsigned mode renders four-resource CA bootstrap, no ACME resources | unit (helm template) | `helm template clay chart/clay --set ingress.enabled=true --set ingress.host=shop.example.com --set ingress.tls.mode=selfsigned` | ❌ Wave 0 — new test assertions needed |
| CI-06 | integration-test.yml installs cert-manager v1.20.2 before clay chart | manual verification | grep in integration-test.yml (not automatable in shell script) | ✅ verified by reading workflow file |

### Sampling Rate
- **Per task commit:** `helm template` of the mode being implemented (see quick run above)
- **Per wave merge:** `chart/tests/helm-template-test.sh` (all existing tests must still pass)
- **Phase gate:** All success criteria from ROADMAP.md §Phase 4 verified before `/gsd-verify-work`

### Wave 0 Gaps

The existing `chart/tests/helm-template-test.sh` covers INGR-01..04 and TLS-03. Phase 4 success criteria require new test assertions for TLS-01 and TLS-02. Two options for the planner:

**Option A (preferred — extend existing script):** Add test cases G-09 through G-13 to `chart/tests/helm-template-test.sh` covering the five Phase 4 success criteria. This keeps all chart behavioral tests in one script.

**Option B (acceptable):** Verify success criteria manually via `helm template` invocations during verification. Phase 5 will add CI coverage.

The planner should include a Wave 0 task to add TLS-01/TLS-02 assertions to the test script if Option A is chosen. If Option B, document as a known gap to be closed in Phase 5.

## Security Domain

Phase 4 creates Kubernetes resources that handle TLS certificate management. Applicable concerns:

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | No | N/A |
| V3 Session Management | No | N/A |
| V4 Access Control | Partial | ClusterIssuer is cluster-scoped; any namespace can reference it. Acceptable for single-tenant cluster; multi-tenant risk is documented as future scope |
| V5 Input Validation | Yes | `clay.validateIngress` already validates `acme.email` non-empty and `tls.mode` enum; no new validation required |
| V6 Cryptography | Yes | cert-manager handles key generation; ECDSA P-256 is the selfsigned CA default from official docs — do not override to weaker algorithms |

### Known Threat Patterns

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Staging ACME cert sent to production | Spoofing | Default `acme.production: false`; operator must explicitly opt in (D-07) |
| ACME account key exposure | Information Disclosure | `privateKeySecretRef` stores in a Kubernetes Secret; ensure RBAC limits access to that secret |
| ClusterIssuer namespace collision | Elevation of Privilege | All names are `clay.fullname`-derived (D-03); multiple clay releases use distinct names |
| Embedding real credentials in CI | Information Disclosure | D-16: integration test runs with `ingress.enabled=false`; no ACME registration attempted in CI |

## Code Examples

Verified patterns from official sources:

### Confirmed ACME Server URLs
```
# Source: https://cert-manager.io/docs/configuration/acme/
staging:    https://acme-staging-v02.api.letsencrypt.org/directory
production: https://acme-v02.api.letsencrypt.org/directory
```

### Confirmed cert-manager API Version
```yaml
# Source: https://cert-manager.io/docs/usage/certificate/
apiVersion: cert-manager.io/v1
```

### Confirmed `selfSigned: {}` Empty Spec
```yaml
# Source: https://cert-manager.io/docs/configuration/selfsigned/
spec:
  selfSigned: {}
```

### Confirmed Hook-Weight is a Quoted String
```yaml
# Source: https://helm.sh/docs/topics/charts_hooks/
annotations:
  helm.sh/hook-weight: "-10"   # string, not integer
```

### Existing ingress.yaml Annotation Block (model for D-06 addition)
```yaml
# Source: chart/clay/templates/ingress.yaml (current state)
  annotations:
    {{- if eq .Values.ingress.className "traefik" }}
    traefik.ingress.kubernetes.io/router.entrypoints: websecure
    {{- end }}
    {{- if eq .Values.ingress.tls.mode "letsencrypt" }}
    acme.cert-manager.io/http01-edit-in-place: "true"
    {{- end }}
    # D-06 adds here:
    {{- if eq .Values.ingress.tls.mode "letsencrypt" }}
    cert-manager.io/cluster-issuer: {{ include "clay.fullname" . }}-letsencrypt
    {{- end }}
```

Note: The two letsencrypt-gated blocks can be merged into one for cleaner YAML.

### Existing CNPG Pre-Install CI Pattern (model for D-15)
```yaml
# Source: .github/workflows/integration-test.yml (current state)
- name: Add CNPG Helm repo
  run: helm repo add cnpg https://cloudnative-pg.github.io/charts

- name: Install CNPG operator
  run: |
    helm install cnpg cnpg/cloudnative-pg \
      --namespace cnpg-system \
      --create-namespace \
      --wait \
      --timeout 3m
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `installCRDs: true` | `crds.enabled: true` | cert-manager v1.15 | Plan tasks must use `crds.enabled=true` not `installCRDs=true` |
| `class: nginx` in http01 solver | `ingressClassName: nginx` | cert-manager v1.12 | Use `ingressClassName` field in solver spec |
| Legacy Jetstack HTTP Helm repo | OCI registry `oci://quay.io/jetstack/charts/cert-manager` | cert-manager v1.14 | Both still work; D-15 uses HTTP repo (matches CNPG pattern) |

**Deprecated/outdated:**
- `kubernetes.io/ingress.class` annotation: deprecated in Kubernetes v1.18+; cert-manager `class` field wraps this annotation — do not use
- `installCRDs: true` Helm value: replaced by `crds.enabled: true` starting cert-manager v1.15

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Helm installs with `jetstack https://charts.jetstack.io` repo still serves v1.20.2 (OCI is primary source of truth) | Standard Stack | Low — both repo URLs are official; if HTTP repo lags, switch to OCI `oci://quay.io/jetstack/charts/cert-manager` |
| A2 | `cert-manager.io/cluster-issuer` Ingress annotation works with Traefik | Architecture Patterns | Low — this annotation is a cert-manager controller annotation, not an ingress-controller annotation; works regardless of ingress class |

All critical claims (ACME URLs, apiVersion, CR field names, hook-weight string type, `crds.enabled` flag, v1.20.2 existence) are VERIFIED against official documentation.

## Open Questions

1. **Merge or keep separate letsencrypt annotation blocks in ingress.yaml**
   - What we know: Two conditionals `{{- if eq .Values.ingress.tls.mode "letsencrypt" }}` exist (one for `http01-edit-in-place`, one for `cluster-issuer`)
   - What's unclear: Whether to merge them into a single conditional block for cleaner template
   - Recommendation: Merge into one block — reduces template nesting and the condition is identical

2. **Whether to add `commonName` to the app Certificate**
   - What we know: `dnsNames` is sufficient per cert-manager docs; `commonName` is optional for end-entity certs and is discouraged by RFC 5280 in favor of SANs
   - What's unclear: Some older clients may require `commonName`
   - Recommendation: Omit `commonName` on app Certificate; include only `dnsNames`

## Sources

### Primary (HIGH confidence)
- [cert-manager.io/docs/configuration/acme/http01/](https://cert-manager.io/docs/configuration/acme/http01/) — HTTP01 solver spec, `ingressClassName` vs `class` distinction
- [cert-manager.io/docs/configuration/selfsigned/](https://cert-manager.io/docs/configuration/selfsigned/) — Four-resource CA bootstrap YAML
- [cert-manager.io/docs/usage/certificate/](https://cert-manager.io/docs/usage/certificate/) — Certificate CR spec fields (secretName, issuerRef, dnsNames, isCA)
- [cert-manager.io/docs/installation/helm/](https://cert-manager.io/docs/installation/helm/) — `crds.enabled=true` flag, Helm repo URLs
- [helm.sh/docs/topics/charts_hooks/](https://helm.sh/docs/topics/charts_hooks/) — Hook annotations, hook-weight as quoted string, delete policies
- [github.com/cert-manager/cert-manager/releases/tag/v1.20.2](https://github.com/cert-manager/cert-manager/releases/tag/v1.20.2) — v1.20.2 existence and release date (2025-04-11)
- `chart/clay/templates/_helpers.tpl` — Verified `clay.tlsSecretName`, `clay.fullname`, `clay.labels`, `clay.validateIngress` all present
- `chart/clay/templates/ingress.yaml` — Verified current annotation structure for D-06 insertion point
- `.github/workflows/integration-test.yml` — Verified CNPG pre-install pattern (verbatim model for D-15)

### Secondary (MEDIUM confidence)
- [cert-manager.io/docs/configuration/acme/](https://cert-manager.io/docs/configuration/acme/) — ACME staging/production URL examples

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — cert-manager v1.20.2 verified on GitHub releases; CR API spec verified on official docs
- Architecture: HIGH — all patterns verified from official cert-manager documentation
- Pitfalls: HIGH — pitfalls derived from verified behavior (lint validation, hook semantics, CA bootstrap chain requirements)

**Research date:** 2026-04-14
**Valid until:** 2026-07-14 (cert-manager CR API is stable; hook annotation semantics are Helm stable)
