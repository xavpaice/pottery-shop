---
phase: 04-cert-manager-cr-templates
verified: 2026-04-14T11:00:00Z
status: passed
score: 13/13
overrides_applied: 0
---

# Phase 4: cert-manager CR Templates — Verification Report

**Phase Goal:** The chart renders ClusterIssuer and Certificate resources for letsencrypt and selfsigned modes, all cert-manager resources use post-install hook annotations to avoid webhook timing races, and the integration test workflow installs cert-manager before the clay chart.
**Verified:** 2026-04-14T11:00:00Z
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | letsencrypt mode renders a ClusterIssuer with ACME HTTP-01 solver and a Certificate CR | VERIFIED | `helm template` with `tls.mode=letsencrypt` renders `kind: ClusterIssuer` (line 4 cert-manager-letsencrypt.yaml) and `kind: Certificate` (line 29); G-09a and G-10a pass |
| 2 | letsencrypt mode renders staging ACME URL by default, production URL when acme.production=true | VERIFIED | Default output contains `acme-staging-v02.api.letsencrypt.org/directory`; `--set ingress.tls.acme.production=true` switches to `acme-v02.api.letsencrypt.org/directory`; G-09b passes |
| 3 | letsencrypt mode Ingress carries cert-manager.io/cluster-issuer annotation matching ClusterIssuer name | VERIFIED | Ingress renders `cert-manager.io/cluster-issuer: release-test-clay-letsencrypt`; ClusterIssuer name is `release-test-clay-letsencrypt`; G-11 passes |
| 4 | selfsigned mode renders four resources in CA bootstrap chain: SelfSigned root, CA cert, CA issuer, app cert | VERIFIED | `helm template` with `tls.mode=selfsigned` renders 2 ClusterIssuers + 2 Certificates (4 resources); chain: selfsigned-root -> ca cert (isCA: true) -> selfsigned-ca -> app cert; G-12a/b/c/d pass |
| 5 | selfsigned mode renders zero ACME resources | VERIFIED | No `acme-staging-v02` in selfsigned output; no ACME fields rendered; G-13 passes |
| 6 | custom mode renders zero ClusterIssuer and zero Certificate resources | VERIFIED | `helm template` with `tls.mode=custom` shows ClusterIssuer count=0, Certificate count=0; G-14a/b pass |
| 7 | TLS secret name in Ingress and Certificate CR are identical (from clay.tlsSecretName) | VERIFIED | Both letsencrypt and selfsigned modes: Ingress `secretName: release-test-clay-tls`, app Certificate `secretName: release-test-clay-tls`; both use `{{ include "clay.tlsSecretName" . }}` helper |
| 8 | All cert-manager resources carry post-install,post-upgrade hook and before-hook-creation delete policy | VERIFIED | All 6 cert-manager resources (2 in letsencrypt, 4 in selfsigned) carry `helm.sh/hook: post-install,post-upgrade` and `helm.sh/hook-delete-policy: before-hook-creation`; G-09c/G-10b pass |
| 9 | integration-test.yml installs cert-manager v1.20.2 before the clay chart install step | VERIFIED | `Install cert-manager` step (line 74) appears before `Install chart on CMX cluster` step (line 84); version pinned to `--version 1.20.2` |
| 10 | cert-manager is installed with crds.enabled=true (not the deprecated installCRDs=true) | VERIFIED | `--set crds.enabled=true` present in integration-test.yml; no `installCRDs` string anywhere |
| 11 | cert-manager is installed in its own namespace (cert-manager) with --create-namespace | VERIFIED | `--namespace cert-manager --create-namespace` present; mirrors CNPG pattern |
| 12 | The clay chart install step still uses ingress.enabled=false | VERIFIED | `--set ingress.enabled=false` remains on clay helm install step (line 105) |
| 13 | helm-template-test.sh validates letsencrypt, selfsigned, and custom modes with G-09 through G-14 | VERIFIED | 6 new test cases (G-09 through G-14, 13 sub-assertions); all 23 total assertions pass with 0 failures |

**Score:** 13/13 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `chart/clay/templates/cert-manager-letsencrypt.yaml` | ClusterIssuer (ACME HTTP-01) + Certificate CR for letsencrypt mode | VERIFIED | 47 lines; contains `kind: ClusterIssuer`, `kind: Certificate`, both with hook annotations; gated on `ingress.enabled + tls.mode=letsencrypt` |
| `chart/clay/templates/cert-manager-selfsigned.yaml` | Four-resource CA bootstrap for selfsigned mode | VERIFIED | 68 lines; 2x ClusterIssuer + 2x Certificate; `selfSigned: {}`, `isCA: true`, hook-weights -10/-5/0/5; gated on `ingress.enabled + tls.mode=selfsigned` |
| `chart/clay/templates/ingress.yaml` | cert-manager.io/cluster-issuer annotation for letsencrypt mode | VERIFIED | Annotation `cert-manager.io/cluster-issuer: {{ include "clay.fullname" . }}-letsencrypt` at line 15, inside letsencrypt conditional block |
| `.github/workflows/integration-test.yml` | cert-manager v1.20.2 pre-install step before clay chart | VERIFIED | Two steps added at lines 71-82: `Add cert-manager Helm repo` and `Install cert-manager` with pinned version, crds.enabled=true, dedicated namespace |
| `chart/tests/helm-template-test.sh` | TLS-01 and TLS-02 behavioral test assertions | VERIFIED | G-09 through G-14 added; header updated with TLS-01/TLS-02 requirements; LETSENCRYPT_INGRESS and SELFSIGNED_INGRESS variables defined |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `cert-manager-letsencrypt.yaml` | `ingress.yaml` | ClusterIssuer name referenced by Ingress annotation | VERIFIED | ClusterIssuer name: `{{ include "clay.fullname" . }}-letsencrypt` (line 5); Ingress annotation value: `{{ include "clay.fullname" . }}-letsencrypt` (line 15); both resolve to `release-test-clay-letsencrypt` |
| `cert-manager-letsencrypt.yaml` | `_helpers.tpl` | Certificate spec.secretName from clay.tlsSecretName | VERIFIED | `secretName: {{ include "clay.tlsSecretName" . }}` at line 39; runtime value `release-test-clay-tls` matches Ingress TLS block |
| `cert-manager-selfsigned.yaml` | `_helpers.tpl` | Certificate spec.secretName from clay.tlsSecretName | VERIFIED | `secretName: {{ include "clay.tlsSecretName" . }}` at line 61 (app cert); runtime value `release-test-clay-tls` matches Ingress TLS block |
| `cert-manager-selfsigned.yaml` internal | CA cert -> CA ClusterIssuer | CA secretName `{{ include "clay.fullname" . }}-ca-tls` consistent | VERIFIED | CA Certificate `spec.secretName` (line 29) and CA ClusterIssuer `spec.ca.secretName` (line 47) both use identical template expression |
| `integration-test.yml` | `cert-manager-letsencrypt.yaml` | cert-manager CRDs registered before clay chart | VERIFIED | `Install cert-manager` step (line 74) is positioned after `Install CNPG operator` (line 61) and before `Install chart on CMX cluster` (line 84) |
| `helm-template-test.sh` | `cert-manager-letsencrypt.yaml` | helm template assertions verify letsencrypt mode | VERIFIED | G-09, G-10, G-11 use `tls.mode=letsencrypt` flag set; all pass |
| `helm-template-test.sh` | `cert-manager-selfsigned.yaml` | helm template assertions verify selfsigned mode | VERIFIED | G-12, G-13 use `tls.mode=selfsigned` flag set; all pass |

### Data-Flow Trace (Level 4)

Not applicable — these are Helm template files and a CI workflow. There is no runtime data rendering; the "data" is Helm values flowing into rendered YAML. The value-to-template flow is verified directly through `helm template` spot-checks in the behavioral checks above.

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| letsencrypt mode renders ClusterIssuer + Certificate + hook annotations | `helm template ... tls.mode=letsencrypt` | `kind: ClusterIssuer`, `kind: Certificate`, `helm.sh/hook: post-install,post-upgrade` all present | PASS |
| letsencrypt uses staging URL by default, production on flag | `helm template ... acme.production=true` | staging: `acme-staging-v02`; production: `acme-v02` | PASS |
| selfsigned mode renders 2 ClusterIssuers + 2 Certificates | `helm template ... tls.mode=selfsigned` | Count: ClusterIssuer=2, Certificate=2; `selfSigned: {}` and `isCA: true` present | PASS |
| CA secretName consistent between CA cert and CA ClusterIssuer | grep ca-tls in selfsigned template | Both at lines 29 and 47 use `{{ include "clay.fullname" . }}-ca-tls` | PASS |
| custom mode: zero ClusterIssuer, zero Certificate | `helm template ... tls.mode=custom` | ClusterIssuer count=0, Certificate count=0 | PASS |
| TLS secret name identical in Ingress and app Certificate | grep secretName in letsencrypt output | Both `release-test-clay-tls` — Ingress TLS block and Certificate spec.secretName | PASS |
| integration-test.yml cert-manager install order | grep step names + line numbers | cert-manager at line 74, clay chart at line 84; order correct | PASS |
| All 23 helm-template-test.sh assertions pass | `chart/tests/helm-template-test.sh` | 23 passed, 0 failed | PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| TLS-01 | 04-01-PLAN.md | User can enable Let's Encrypt mode — HTTP-01 ACME ClusterIssuer, staging default, production opt-in | SATISFIED | `cert-manager-letsencrypt.yaml` implements ClusterIssuer with ACME HTTP-01 solver; staging endpoint by default; `acme.production: true` switches to production; G-09/G-10/G-11 pass |
| TLS-02 | 04-01-PLAN.md | User can enable self-signed mode — two-step CA bootstrap | SATISFIED | `cert-manager-selfsigned.yaml` implements 4-resource bootstrap: SelfSigned root -> CA cert (isCA: true) -> CA ClusterIssuer -> app cert; G-12/G-13 pass |
| CI-06 | 04-02-PLAN.md | integration-test.yml includes cert-manager v1.20.2 pre-install step | SATISFIED | Two steps added: repo add (jetstack) + install (v1.20.2, crds.enabled=true, namespace cert-manager, --wait --timeout 3m) |

### Anti-Patterns Found

None. Scanned all five modified/created files for TODO/FIXME/PLACEHOLDER/stub patterns. No issues found. All template gates (`{{- if and .Values.ingress.enabled (eq .Values.ingress.tls.mode ...) }}`) correctly block rendering in modes that don't apply.

### Human Verification Required

None. All must-haves for this phase are Helm template rendering behaviors and CI configuration, which are fully verifiable programmatically through `helm template`, `helm lint`, and file inspection. No UI, external service, or runtime behavior requires human validation.

### Gaps Summary

No gaps. All 13 truths verified, all 5 artifacts present and substantive, all 7 key links wired, all 3 requirements satisfied, all 23 behavioral assertions pass, no anti-patterns found.

The phase goal is fully achieved: the chart renders correct cert-manager ClusterIssuer and Certificate resources for both letsencrypt and selfsigned modes, all resources carry the required hook annotations, the integration test workflow installs cert-manager before the clay chart, and a comprehensive behavioral test suite (G-09 through G-14) validates all three TLS modes.

---

_Verified: 2026-04-14T11:00:00Z_
_Verifier: Claude (gsd-verifier)_
