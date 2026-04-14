# Phase 4: cert-manager CR Templates - Context

**Gathered:** 2026-04-14
**Status:** Ready for planning

<domain>
## Phase Boundary

Create ClusterIssuer and Certificate Kubernetes CRs for the `letsencrypt` and `selfsigned` TLS modes. All cert-manager resources carry `helm.sh/hook` annotations (post-install/post-upgrade). Add the `cert-manager.io/cluster-issuer` annotation to the Ingress template for letsencrypt mode. Add a cert-manager pre-install step to `integration-test.yml`.

Scope: two new template files (`cert-manager-letsencrypt.yaml`, `cert-manager-selfsigned.yaml`), a small annotation addition to `ingress.yaml`, and a CI workflow update.

No values.yaml structural changes — all required values already exist from Phase 3 (`ingress.tls.mode`, `ingress.tls.acme.email`, `ingress.tls.acme.production`).

</domain>

<decisions>
## Implementation Decisions

### Template File Organization
- **D-01:** Two files by mode — `chart/clay/templates/cert-manager-letsencrypt.yaml` and `chart/clay/templates/cert-manager-selfsigned.yaml`. Each file is self-contained: opening it shows every resource rendered for that mode.
- **D-02:** Each file is gated by `{{- if and .Values.ingress.enabled (eq .Values.ingress.tls.mode "<mode>") }}` at the top level. No rendering when ingress is disabled or a different mode is active.

### ClusterIssuer and Certificate Resource Naming
- **D-03:** All resource names are release-derived using `clay.fullname` to avoid cluster-scoped naming conflicts if multiple clay releases exist on the same cluster.
  - letsencrypt ClusterIssuer: `{{ include "clay.fullname" . }}-letsencrypt`
  - selfsigned root ClusterIssuer: `{{ include "clay.fullname" . }}-selfsigned-root`
  - selfsigned CA Certificate: `{{ include "clay.fullname" . }}-ca`
  - selfsigned CA ClusterIssuer: `{{ include "clay.fullname" . }}-selfsigned-ca`
  - App Certificate (both modes): `{{ include "clay.fullname" . }}-tls-cert`
- **D-04:** All resources carry standard `clay.labels` and a `clay.fullname`-based name — consistent with existing chart resources.

### Hook Annotations on All cert-manager CRs
- **D-05:** Every ClusterIssuer and Certificate resource carries:
  ```yaml
  annotations:
    helm.sh/hook: post-install,post-upgrade
    helm.sh/hook-delete-policy: before-hook-creation
  ```
  `before-hook-creation` ensures Helm deletes the old resource before re-creating on each `helm upgrade` or `helm install`, avoiding "already exists" errors.

### cert-manager.io/cluster-issuer Annotation on Ingress
- **D-06:** Add `cert-manager.io/cluster-issuer: {{ include "clay.fullname" . }}-letsencrypt` to `ingress.yaml` metadata annotations, scoped to `{{- if eq .Values.ingress.tls.mode "letsencrypt" }}`. Value references the same release-derived ClusterIssuer name (D-03) — no hardcoding, no name mismatch.

### letsencrypt ClusterIssuer Spec
- **D-07:** ACME HTTP-01 challenger. Staging endpoint by default; production opt-in via `ingress.tls.acme.production: true`:
  - staging: `https://acme-staging-v02.api.letsencrypt.org/directory`
  - production: `https://acme-v02.api.letsencrypt.org/directory`
- **D-08:** `spec.acme.email` from `{{ .Values.ingress.tls.acme.email }}` (already validated non-empty by `clay.validateIngress`).
- **D-09:** HTTP-01 solver uses `ingress` class `{{ .Values.ingress.className }}` (i.e., `traefik` by default).

### letsencrypt Certificate Spec
- **D-10:** `spec.secretName` from `{{ include "clay.tlsSecretName" . }}` (the Phase 3 helper) — identical value to `ingress.spec.tls[0].secretName`, preventing name mismatch.
- **D-11:** `spec.issuerRef.kind: ClusterIssuer`, `spec.issuerRef.name: {{ include "clay.fullname" . }}-letsencrypt`.
- **D-12:** `spec.dnsNames: [{{ .Values.ingress.host }}]`.

### selfsigned Two-Step CA Bootstrap
- **D-13:** Four resources in `cert-manager-selfsigned.yaml`, in logical order:
  1. SelfSigned ClusterIssuer (`-selfsigned-root`) — issues the CA cert
  2. CA Certificate (`-ca`) — `isCA: true`, issued by `-selfsigned-root`, stored in a separate Secret (`{{ include "clay.fullname" . }}-ca-tls`)
  3. CA ClusterIssuer (`-selfsigned-ca`) — references the CA Certificate Secret
  4. App Certificate — `spec.issuerRef` points to `-selfsigned-ca`; `spec.secretName` from `clay.tlsSecretName`
- **D-14:** Hook-weight (`helm.sh/hook-weight`) used to sequence the four selfsigned resources in creation order: root="-10", ca-cert="-5", ca-issuer="0", app-cert="5". cert-manager is eventually consistent but ordering reduces reconciliation churn.

### Integration Test (CI-06)
- **D-15:** Add a cert-manager pre-install step to `integration-test.yml` between the CNPG install step and the clay chart install step:
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
- **D-16:** The clay chart install step keeps `--set ingress.enabled=false`. TLS mode validation is handled by `helm template` in Phase 5. No domain/DNS is needed for the integration test.

### Claude's Discretion
- `helm.sh/hook-weight` string values (exact integers may shift — as long as relative order is root < ca-cert < ca-issuer < app-cert)
- Whether to add a `# ACME staging vs production` inline comment to the ClusterIssuer template for operator clarity
- Exact `tolerations`/`solvers` structure in the letsencrypt ClusterIssuer if Traefik requires additional ingress annotations for the solver

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements
- `.planning/REQUIREMENTS.md` — Phase 4 requirements: TLS-01, TLS-02, CI-06; success criteria are the authoritative test for correctness

### Phase 4 success criteria (ROADMAP.md)
- `.planning/ROADMAP.md` §Phase 4 — Five success criteria define exactly what `helm template` output must contain for each mode; read before writing any template

### Chart files (current state — read before modifying)
- `chart/clay/templates/_helpers.tpl` — `clay.tlsSecretName` and `clay.validateIngress` already defined; `clay.fullname` is the naming base for D-03
- `chart/clay/templates/ingress.yaml` — Add `cert-manager.io/cluster-issuer` annotation here (D-06); existing annotation structure is the model
- `chart/clay/values.yaml` — `ingress.tls.acme.email`, `ingress.tls.acme.production`, `ingress.tls.mode` already present from Phase 3

### CI workflow (read before modifying)
- `.github/workflows/integration-test.yml` — Current CNPG pre-install step is the exact pattern to follow for the cert-manager pre-install (D-15)

### Phase 3 context (patterns to carry forward)
- `.planning/phases/03-values-and-ingress-refactor/03-CONTEXT.md` — D-12/D-13: `clay.tlsSecretName` helper, Traefik annotation pattern, conditional rendering approach

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `clay.tlsSecretName` helper (`_helpers.tpl`) — Certificate `spec.secretName` uses this directly; no new helper needed
- `clay.validateIngress` helper (`_helpers.tpl`) — Already validates `acme.email` for letsencrypt mode; Phase 4 adds no new validation
- `clay.fullname` helper — Naming base for all new CRs (D-03/D-04)
- `clay.labels` helper — Apply to all new CRs for consistent label sets

### Established Patterns
- Conditional rendering gate: `{{- if .Values.ingress.enabled }}` wrapping entire template files — follow same pattern in both cert-manager files
- Hook annotations: `helm.sh/hook` pattern already discussed; `before-hook-creation` is the chosen delete policy
- CNPG pre-install in `integration-test.yml` — verbatim structural template for the cert-manager pre-install step (separate repo add + install steps)

### Integration Points
- `ingress.yaml` — Needs one new annotation block for letsencrypt mode (D-06); no other changes to this file
- `integration-test.yml` — New steps inserted between CNPG install and clay chart install
- Phase 5 will add `chart/clay/ci/tls-letsencrypt-values.yaml`, `tls-selfsigned-values.yaml`, `tls-custom-values.yaml` — Phase 4 must not break existing `chart/clay/ci/managed-values.yaml` and `external-values.yaml`

</code_context>

<specifics>
## Specific Ideas

- The `cert-manager.io/cluster-issuer` annotation on the Ingress must reference the exact same release-derived name as the ClusterIssuer resource — both derive from `clay.fullname` so a name mismatch is structurally impossible
- `before-hook-creation` hook-delete-policy chosen explicitly to prevent "already exists" errors on `helm upgrade`

</specifics>

<deferred>
## Deferred Ideas

- None — discussion stayed within phase scope

</deferred>

---

*Phase: 04-cert-manager-cr-templates*
*Context gathered: 2026-04-14*
