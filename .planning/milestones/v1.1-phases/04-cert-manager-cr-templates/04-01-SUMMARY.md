---
phase: 04-cert-manager-cr-templates
plan: 01
subsystem: infra
tags: [cert-manager, helm, kubernetes, tls, letsencrypt, selfsigned, clusterissuer, certificate, ingress]

# Dependency graph
requires:
  - phase: 03-values-and-ingress-refactor
    provides: "clay.tlsSecretName helper, clay.validateIngress, ingress.tls.mode/acme values, Ingress template with existing letsencrypt annotation"
provides:
  - "cert-manager-letsencrypt.yaml: ACME ClusterIssuer (staging default, production opt-in) + Certificate CR with hook annotations"
  - "cert-manager-selfsigned.yaml: four-resource CA bootstrap (SelfSigned root -> CA cert -> CA issuer -> app cert) with hook-weight sequencing"
  - "ingress.yaml: cert-manager.io/cluster-issuer annotation in letsencrypt mode"
affects: [04-cert-manager-cr-templates, 05-ci-tls-validation]

# Tech tracking
tech-stack:
  added: [cert-manager.io/v1 ClusterIssuer, cert-manager.io/v1 Certificate]
  patterns:
    - "Helm hook annotations (post-install,post-upgrade + before-hook-creation) on all cert-manager CRs to prevent upgrade 'already exists' errors"
    - "Mode-gated template files: {{- if and .Values.ingress.enabled (eq .Values.ingress.tls.mode '<mode>') }}"
    - "Hook-weight sequencing for selfsigned CA bootstrap: root=-10, ca-cert=-5, ca-issuer=0, app-cert=5"
    - "Single source of truth for TLS secret name via clay.tlsSecretName helper (used by both Ingress and Certificate spec.secretName)"

key-files:
  created:
    - chart/clay/templates/cert-manager-letsencrypt.yaml
    - chart/clay/templates/cert-manager-selfsigned.yaml
  modified:
    - chart/clay/templates/ingress.yaml

key-decisions:
  - "Merge both letsencrypt annotation conditionals in ingress.yaml into a single block (per RESEARCH.md open question 1) — reduces template nesting"
  - "CA intermediate secret name uses fullname-ca-tls (not clay.tlsSecretName) — CA ClusterIssuer spec.ca.secretName must match exactly"
  - "No commonName on app Certificate — dnsNames alone satisfies cert-manager; omitting commonName follows RFC 5280 SAN preference"

patterns-established:
  - "cert-manager hook template: every ClusterIssuer and Certificate carries helm.sh/hook: post-install,post-upgrade and helm.sh/hook-delete-policy: before-hook-creation"
  - "selfsigned CA bootstrap: 4 resources in order — SelfSigned ClusterIssuer, CA Cert (isCA: true), CA ClusterIssuer, App Cert"
  - "ingressClassName field used in ACME HTTP-01 solver (not deprecated class field)"

requirements-completed: [TLS-01, TLS-02]

# Metrics
duration: 15min
completed: 2026-04-14
---

# Phase 4 Plan 01: cert-manager ClusterIssuer and Certificate Templates Summary

**ACME ClusterIssuer (staging default) + Certificate for letsencrypt mode, and four-resource SelfSigned CA bootstrap for selfsigned mode, both with Helm hook annotations for upgrade safety**

## Performance

- **Duration:** ~15 min
- **Started:** 2026-04-14T09:45:00Z
- **Completed:** 2026-04-14T09:58:08Z
- **Tasks:** 2 of 2
- **Files modified:** 3

## Accomplishments

- Created `cert-manager-letsencrypt.yaml` with ACME ClusterIssuer (staging endpoint by default, production opt-in via `ingress.tls.acme.production: true`) and Certificate CR — both with `post-install,post-upgrade` hook and `before-hook-creation` delete policy
- Created `cert-manager-selfsigned.yaml` with four-resource CA bootstrap chain: SelfSigned root ClusterIssuer (weight -10) -> CA Certificate isCA=true (weight -5) -> CA ClusterIssuer (weight 0) -> App Certificate (weight 5)
- Added `cert-manager.io/cluster-issuer` annotation to `ingress.yaml` letsencrypt block (merged with existing `acme.cert-manager.io/http01-edit-in-place` conditional)

## Task Commits

1. **Task 1: letsencrypt ClusterIssuer + Certificate + Ingress annotation** - `2b1ff2d` (feat)
2. **Task 2: selfsigned four-resource CA bootstrap** - `56aace4` (feat)

## Files Created/Modified

- `chart/clay/templates/cert-manager-letsencrypt.yaml` - ACME ClusterIssuer and Certificate CR, gated on `ingress.enabled + tls.mode=letsencrypt`
- `chart/clay/templates/cert-manager-selfsigned.yaml` - Four-resource CA bootstrap, gated on `ingress.enabled + tls.mode=selfsigned`
- `chart/clay/templates/ingress.yaml` - Added `cert-manager.io/cluster-issuer` annotation inside existing letsencrypt conditional block

## Decisions Made

- Merged the two `{{- if eq .Values.ingress.tls.mode "letsencrypt" }}` annotation blocks in `ingress.yaml` into one (RESEARCH.md open question 1 recommendation). Both annotations are identically gated, so a single block is cleaner.
- CA intermediate secret uses `{{ include "clay.fullname" . }}-ca-tls` (not `clay.tlsSecretName`) — the intermediate CA material is distinct from the app TLS secret and must not be referenced by the Ingress.
- Omitted `commonName` from the app Certificate in both modes — `dnsNames` alone is sufficient per cert-manager docs and RFC 5280 SAN preference.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None. `helm` binary found at `/tmp/helm` (not in PATH). All `helm template` and `helm lint` commands succeeded on first run.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Phase 4 Plan 02 (CI workflow): cert-manager pre-install step can now be added to `integration-test.yml` — the ClusterIssuer and Certificate templates are complete
- Phase 5 (CI TLS validation): `helm template` test assertions for TLS-01 and TLS-02 can now be written against the confirmed template output
- Existing CI lint (`managed-values.yaml`, `external-values.yaml`) verified passing — no regression introduced

---
*Phase: 04-cert-manager-cr-templates*
*Completed: 2026-04-14*
