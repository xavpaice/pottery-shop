# Project Research Summary

**Project:** Pottery Shop — v1.1 TLS/Ingress Milestone
**Domain:** Helm-managed Kubernetes TLS with cert-manager + Traefik
**Researched:** 2026-04-14
**Confidence:** HIGH

## Executive Summary

The v1.1 TLS milestone exposes the pottery shop over HTTPS by adding a Kubernetes Ingress resource and cert-manager-managed TLS certificates to the existing clay Helm chart. The chart must support three TLS modes — `letsencrypt` (HTTP-01 ACME), `selfsigned`, and `custom` (BYO secret) — all controlled through a new `ingress.tls.mode` discriminator in values.yaml. The recommended approach mirrors the CNPG pattern already established in Phase 2: cert-manager is installed as a separate `helm install` step before the clay chart (not as a `Chart.yaml` subchart dependency), and the clay chart creates cert-manager CRs (ClusterIssuer, Certificate) that the pre-installed operator reconciles.

The most important architectural decision surfaced by research is that cert-manager must NOT be embedded as a `Chart.yaml` dependency subchart. The official cert-manager documentation explicitly prohibits subchart embedding because cert-manager owns cluster-scoped resources (webhooks, ClusterRoles) that conflict when installed more than once per cluster. The existing CNPG integration already follows the correct pre-install pattern, and cert-manager must follow it exactly. In practical terms: `integration-test.yml` gets a cert-manager pre-install step, the clay chart adds ClusterIssuer and Certificate templates (not a Chart.yaml dependency), and CRD timing is handled with Helm post-install hook weights rather than a subchart readiness wait.

The key risks are: (1) the cert-manager admission webhook not being ready when the clay chart's ClusterIssuer is applied — mitigated by annotating ClusterIssuer and Certificate templates as `helm.sh/hook: post-install,post-upgrade`; (2) Let's Encrypt rate limiting during development — mitigated by defaulting to the staging ACME endpoint; and (3) a global Traefik HTTP-to-HTTPS redirect blocking the HTTP-01 ACME challenge path — mitigated by scoping the redirect to app routes only and adding `acme.cert-manager.io/http01-edit-in-place: "true"` to the app Ingress.

## Key Findings

### Recommended Stack

cert-manager v1.20.2 (April 2026 stable release) is the version to pin. Use the HTTPS Helm repository (`https://charts.jetstack.io`) rather than the OCI registry — a known Helm issue causes `v`-prefixed OCI tags to fail SemVer matching in some Helm versions. The `crds.enabled: true` flag (not the deprecated `installCRDs`) is required to register cert-manager CRDs during install. The Ingress resource uses `networking.k8s.io/v1` (stable since Kubernetes 1.19) with `ingressClassName: traefik` — the old `kubernetes.io/ingress.class` annotation is deprecated and must not be used.

**Core technologies:**
- cert-manager v1.20.2: TLS certificate lifecycle management — only option with Traefik + Let's Encrypt ACME on k3s
- Helm repo `https://charts.jetstack.io`: reliable pre-install source — OCI registry has v-prefix tag issues with some Helm versions
- `networking.k8s.io/v1 Ingress`: Traefik-compatible ingress — stable API, controller-agnostic, no Traefik CRD lock-in
- `ingressClassName: traefik`: required for k3s Traefik — explicit class prevents silent routing failures on multi-controller clusters
- Let's Encrypt staging ACME (`acme-staging-v02`): safe default for development — production endpoint has hard rate limits that block re-issuance for 7 days

### Expected Features

**Must have (table stakes):**
- `ingress.host` single string field — drives Ingress rule, Certificate dnsNames, and TLS secret name from one value
- `ingress.tls.mode` discriminator (`letsencrypt` | `selfsigned` | `custom`) — gates which templates are rendered
- `ingressClassName: traefik` on Ingress spec — required for Traefik to claim the Ingress resource
- cert-manager pre-installed before clay chart — operator must exist before ClusterIssuer CRs are applied
- `crds.enabled: true` on cert-manager install — registers CRDs so ClusterIssuer/Certificate resources are accepted
- ClusterIssuer for letsencrypt mode — HTTP-01 ACME with `ingressClassName: traefik` in solver spec
- `cert-manager.io/cluster-issuer` annotation on Ingress — triggers ingress-shim auto-creation of Certificate
- `traefik.ingress.kubernetes.io/router.entrypoints: websecure` annotation — routes HTTPS traffic through Traefik port 443
- selfsigned mode via two-step CA bootstrap — SelfSigned ClusterIssuer issues CA cert; CA ClusterIssuer issues app cert
- custom mode — no issuer created; Ingress tls.secretName wired to user-supplied secret name
- CI values files for all three TLS modes — five total lint targets (existing 2 + new 3)
- `helm template` (not `--validate`) in CI — avoids cert-manager CRD absence failures in CI environment

**Should have (differentiators):**
- Staging issuer default for letsencrypt mode — prevents burning production LE rate limits during dev
- TLS secret name defaulted from `ingress.host` in `_helpers.tpl` — single definition, referenced by both Ingress and Certificate templates
- `clay.validateIngress` helper in `_helpers.tpl` — fail-fast at render time if `acme.email` missing in letsencrypt mode or `secretName` missing in custom mode
- `acme.cert-manager.io/http01-edit-in-place: "true"` on app Ingress — prevents timing/routing issues with k3s single-IP LoadBalancer

**Defer (v2+):**
- DNS-01 ACME challenge — requires cloud provider API credentials, adds significant complexity
- Multiple TLS hosts per Ingress — single-domain app, no benefit
- Gateway API (HTTPRoute) — Traefik supports it but adds CRD complexity; existing Ingress API works
- trust-manager / CA bundle distribution — only needed for mTLS workloads

### Architecture Approach

The clay Helm chart continues its established pattern: cluster-scoped operators (CNPG, cert-manager) are pre-installed separately, and the clay chart creates only the operator-specific CRs (CNPG `Cluster`, cert-manager `ClusterIssuer` and `Certificate`). Two new template files are added (`cluster-issuer.yaml`, `certificate.yaml`), the existing `ingress.yaml` is updated, and `_helpers.tpl` gains a validation helper. The `ingress:` block in values.yaml is restructured from the scaffold-generated multi-host list to a single-host, mode-driven shape. No changes to Go source, Dockerfile, or CNPG templates.

**Major components:**
1. `templates/cluster-issuer.yaml` (NEW) — renders a letsencrypt or selfsigned ClusterIssuer based on `tls.mode`; skipped in custom mode; uses Helm post-install hook annotation to avoid webhook timing race
2. `templates/certificate.yaml` (NEW) — renders a cert-manager Certificate CR pointing at the ClusterIssuer; skipped in custom mode; secret name derived from `_helpers.tpl`
3. `templates/ingress.yaml` (MODIFY) — refactored to single `ingress.host`, explicit `ingressClassName: traefik`, Traefik websecure annotation, mode-driven TLS block
4. `templates/_helpers.tpl` (MODIFY) — adds `clay.tlsSecretName` and `clay.validateIngress` helpers
5. `chart/clay/ci/tls-*.yaml` (NEW x3) — CI values files for letsencrypt, selfsigned, and custom modes
6. `.github/workflows/integration-test.yml` (MODIFY) — cert-manager pre-install step before clay install
7. `.github/workflows/test.yml` (MODIFY) — six new `helm lint` / `helm template` steps for three TLS mode CI files

### Critical Pitfalls

1. **cert-manager webhook not ready when ClusterIssuer is applied** — annotate ClusterIssuer and Certificate templates as `helm.sh/hook: post-install,post-upgrade` with `hook-weight: "5"`; optionally add a weight-0 hook Job waiting on cert-manager deployment readiness before the ClusterIssuer fires

2. **`helm template` CI fails with "no matches for kind Certificate"** — do NOT pass `--validate` to `helm template` in CI; plain `helm lint` + `helm template` without `--validate` covers the essential rendering check without requiring live CRDs

3. **Global Traefik HTTP-to-HTTPS redirect blocks HTTP-01 ACME challenge** — do not configure a global entrypoint-level redirect; scope redirect middleware to app-specific routes only; add `acme.cert-manager.io/http01-edit-in-place: "true"` to the app Ingress so the challenge path shares the existing LoadBalancer IP

4. **HTTP-01 solver Ingress ignored by Traefik (wrong ingressClassName)** — set `ingressClassName: traefik` explicitly in the ClusterIssuer solver spec; do not use the deprecated `class` annotation field (deprecated since cert-manager v1.5.4)

5. **selfsigned mode: SelfSigned issuer used directly produces untrusted end-entity cert** — implement two-step CA bootstrap: SelfSigned ClusterIssuer issues a CA certificate (`isCA: true`), then a CA ClusterIssuer backed by that CA issues the app certificate

6. **TLS secret name mismatch between Ingress and Certificate** — define the secret name in exactly one `_helpers.tpl` function (`clay.tlsSecretName`) and reference it in both templates; add a CI `helm template | grep` check to assert both values match

## Implications for Roadmap

The work is small enough to fit in a single phase but has a clear internal build order based on template dependencies. Splitting into three sequential steps is the recommended approach: values restructure first (everything references it), then templates, then CI extension.

### Phase 1: Values and Ingress Refactor

**Rationale:** The new `ingress.tls.mode` structure and single `ingress.host` field are the foundation every other template references. Completing this first means all subsequent templates can be validated with `helm template` immediately as they are added.
**Delivers:** Updated `values.yaml` (single-host, tls.mode shape), updated `ingress.yaml` (Traefik annotations, ingressClassName, mode-driven TLS block), `_helpers.tpl` additions (`clay.tlsSecretName`, `clay.validateIngress`)
**Addresses:** ingress.host, ingressClassName: traefik, router.entrypoints: websecure, TLS secret name single-definition
**Avoids:** TLS secret name mismatch (Pitfall 6) — secret name defined once in helpers before any template uses it

### Phase 2: cert-manager CR Templates

**Rationale:** With values structure and helpers in place, ClusterIssuer and Certificate templates can be built and validated via `helm template` locally without a cluster. ClusterIssuer and Certificate templates are independent of each other within this phase.
**Delivers:** `templates/cluster-issuer.yaml` (letsencrypt + selfsigned issuers, post-install hook annotated), `templates/certificate.yaml` (gated on `ne tls.mode "custom"`, post-install hook annotated)
**Avoids:** Webhook timing race (Pitfall 1) via hook annotations; selfsigned CA bootstrap pitfall (Pitfall 5) via two-step CA pattern; rate limit pitfall via staging ACME default

### Phase 3: CI and Integration Test Extension

**Rationale:** CI validation of all three TLS modes is a hard milestone requirement and the lowest-risk step — no cluster access needed for `helm lint`/`helm template`, and the integration test cert-manager pre-install step mirrors the already-working CNPG pre-install pattern exactly.
**Delivers:** Three CI values files (`ci/tls-letsencrypt.yaml`, `ci/tls-selfsigned.yaml`, `ci/tls-custom.yaml`), six new `test.yml` lint/template steps, cert-manager pre-install step in `integration-test.yml`
**Avoids:** CI CRD-absence failures (Pitfall 2) by using `helm template` without `--validate`; k3s single-IP LoadBalancer timing issues (Pitfall 7) by using `selfsigned` mode in integration tests

### Phase Ordering Rationale

- Values must come before templates because all template references use values keys by path
- ClusterIssuer and Certificate templates are independent of each other and can be written in parallel within Phase 2, but both depend on Phase 1 values shape
- CI extension comes last because it validates Phase 1 and 2 outputs; running CI validation before templates are stable wastes iteration cycles
- The cert-manager pre-install step in integration-test.yml mirrors an already-understood pattern (CNPG), making it a low-risk addition at any point in Phase 3

### Research Flags

Phases with standard patterns (no additional research needed):
- **Phase 1 (Values/Ingress):** Helm values restructuring and Ingress template editing are well-understood; exact template syntax is fully documented in ARCHITECTURE.md
- **Phase 2 (CR Templates):** ClusterIssuer and Certificate YAML is fully specified in STACK.md and ARCHITECTURE.md with exact template blocks; hook annotation pattern is documented
- **Phase 3 (CI):** helm lint/template CI extension mirrors existing patterns; cert-manager pre-install is a documented one-command step

No phase requires a `/gsd-research-phase` pass — all template content, version pins, annotation keys, and CI patterns are fully specified in the research files.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | cert-manager v1.20.2 verified at cert-manager.io/docs/releases/; Helm repo verified; `crds.enabled` flag verified against v1.15+ changelog |
| Features | HIGH | Table-stakes derived from official cert-manager + Traefik docs; existing chart structure verified from git history |
| Architecture | HIGH | Critical "no subchart" finding sourced from official cert-manager Helm docs; CNPG pre-install pattern confirmed from integration-test.yml |
| Pitfalls | HIGH | All critical pitfalls traced to official cert-manager GitHub issues or Traefik GitHub issues with reproducible symptoms and detection steps |

**Overall confidence:** HIGH

### Gaps to Address

- **Chart.yaml current state:** FEATURES.md notes git history shows "move cnpg out of the Clay chart" — CNPG is no longer a subchart dependency. Verify `chart/clay/Chart.yaml` has no `dependencies:` block before Phase 2 to confirm no subchart cleanup is needed for cert-manager either.
- **K3s Traefik HTTP redirect config:** Whether the current k3s Traefik deployment has a global HTTP-to-HTTPS redirect enabled is infrastructure-dependent. Use `selfsigned` mode for integration tests to sidestep HTTP-01 entirely (recommended in ARCHITECTURE.md); verify port 80 passthrough manually before any letsencrypt production deployment.
- **Helm hook RBAC for webhook-wait Job:** The optional weight-0 hook Job that waits on cert-manager deployment readiness requires a ServiceAccount with permissions in the `cert-manager` namespace. Skip on first implementation; add only if the post-install hook weight approach proves insufficient in integration testing.

## Sources

### Primary (HIGH confidence)
- cert-manager releases: https://cert-manager.io/docs/releases/
- cert-manager Helm install (subchart warning, crds.enabled): https://cert-manager.io/docs/installation/helm/
- cert-manager Ingress annotations (ingress-shim): https://cert-manager.io/docs/usage/ingress/
- cert-manager HTTP-01 solver (ingressClassName, edit-in-place): https://cert-manager.io/docs/configuration/acme/http01/
- cert-manager SelfSigned issuer (CA bootstrap pattern): https://cert-manager.io/docs/configuration/selfsigned/
- cert-manager ACME troubleshooting: https://cert-manager.io/docs/troubleshooting/acme/
- cert-manager ingress-class compatibility (v1.5.4 breaking change): https://cert-manager.io/docs/releases/upgrading/ingress-class-compatibility/
- Traefik cert-manager integration: https://doc.traefik.io/traefik/v3.4/user-guides/cert-manager/
- Let's Encrypt rate limits: https://letsencrypt.org/docs/rate-limits/
- Let's Encrypt staging environment: https://letsencrypt.org/docs/staging-environment/

### Secondary (MEDIUM confidence)
- Using cert-manager as subchart (webhook timing, hook pattern): https://skarlso.github.io/2024/07/02/using-cert-manager-as-a-subchart-with-helm/
- Traefik global redirect blocks ACME challenge: https://community.traefik.io/t/globally-enabled-http-to-https-blocks-cert-manager-http-challenge/20047
- Helm OCI v-prefix tag issue: https://github.com/helm/helm/issues/11107

### Tertiary (issue reports corroborating documented behavior)
- cert-manager "no matches for kind Certificate" in CI: https://github.com/cert-manager/cert-manager/issues/2110
- cert-manager HTTP-01 ingressClassName regression (v1.5.4): https://github.com/cert-manager/cert-manager/issues/4537
- Traefik HTTPS redirect blocks ACME: https://github.com/traefik/traefik/issues/7825

---
*Research completed: 2026-04-14*
*Ready for roadmap: yes*
