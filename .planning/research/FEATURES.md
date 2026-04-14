# Feature Landscape: TLS/Ingress Milestone

**Domain:** Helm-managed Kubernetes TLS with cert-manager + Traefik
**Researched:** 2026-04-14
**Overall confidence:** HIGH (cert-manager official docs, Traefik docs, verified patterns)

---

## Context: Existing Chart Structure

The chart at `chart/clay/` already has:
- `ingress.yaml` template with a standard `{{- if .Values.ingress.enabled }}` gate
- `values.yaml` with an `ingress:` block that uses the nginx annotation and a raw TLS secret reference
- CNPG as a Helm subchart via `Chart.yaml` dependency with a `condition:` field — this is the **established pattern** for optional operator subcharts in this chart
- Two CI test values files in `chart/clay/ci/` (one per postgres mode)

The TLS work extends the existing `ingress:` block and adds cert-manager as a second subchart following the same condition-gated dependency pattern already used for CNPG.

---

## Table Stakes

Features that must be present for the milestone to be complete. Missing any one makes the milestone fail.

| Feature | Why Essential | Complexity | Notes |
|---------|--------------|------------|-------|
| `ingress.host` single field | Drives Ingress hostname, Certificate commonName, tls.hosts — one value, no duplication | Low | Replaces current `ingress.hosts[].host` array for this single-host app |
| `ingress.tls.mode` discriminator | Controls which of three cert paths is active | Low | Values: `letsencrypt`, `selfsigned`, `custom` |
| `ingressClassName: traefik` on Ingress | Required for Traefik to pick up the Ingress resource | Low | Current `ingress.className: ""` must default to `"traefik"` |
| cert-manager subchart dependency | One `helm install` deploys cert-manager — same pattern as CNPG | Low | `condition: cert-manager.enabled` in `Chart.yaml` |
| `crds.enabled: true` on cert-manager subchart | Without CRDs, Certificate/Issuer/ClusterIssuer resources don't exist | Low | `crds.enabled` is the current cert-manager flag (replaces deprecated `installCRDs`) |
| `ClusterIssuer` for letsencrypt mode | Cluster-scoped issuer for HTTP-01 ACME, works across namespaces | Medium | Needs `acme.email` in values.yaml; creates two issuers (staging + prod) |
| HTTP-01 solver with `ingressClassName: traefik` | cert-manager creates temporary Ingress via Traefik to serve ACME challenge token | Medium | The solver's `ingressClassName` must match the app's ingress class |
| `cert-manager.io/cluster-issuer` annotation on Ingress | Triggers ingress-shim to auto-create Certificate resource | Low | Replaces manual Certificate resource creation |
| `selfsigned` mode: SelfSigned ClusterIssuer | In-cluster self-signed certificate, no ACME required | Low | Browser shows untrusted warning — expected for dev/internal |
| `custom` mode: reference existing Secret | User pre-creates `kubernetes.io/tls` Secret, chart just wires it to Ingress tls.secretName | Low | No issuer created; no cert-manager annotation on Ingress |
| CI values files for all three TLS modes | Chart validation must cover all code paths | Low | Three new files in `chart/clay/ci/`: `tls-letsencrypt.yaml`, `tls-selfsigned.yaml`, `tls-custom.yaml` |
| `traefik.ingress.kubernetes.io/router.entrypoints: websecure` annotation | Routes HTTPS traffic through Traefik's 443 entrypoint | Low | Required for TLS termination at Traefik |

---

## Differentiators

Features that make the operator experience noticeably better. Not required for correctness, but worth the low build cost.

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Staging issuer default for letsencrypt mode | Prevents burning Let's Encrypt production rate limits during dev/test | Low | Default to staging; operator sets `ingress.tls.letsencrypt.production: true` to flip |
| `cert-manager.enabled` auto-derived from `ingress.tls.mode` | Operators don't need to set two independent flags that must always match | Medium | `{{- if ne .Values.ingress.tls.mode "custom" }}` — only install cert-manager when needed |
| TLS secret name defaulting to `{{ ingress.host }}-tls` | Predictable naming without operator needing to specify it | Low | Computed in `_helpers.tpl`; overridable via `ingress.tls.secretName` |
| ACME email validation in `helm lint` / `NOTES.txt` | Fail loudly if `letsencrypt` mode selected but no email provided | Low | Add to `NOTES.txt` with `required` function or conditional warning |
| Let's Encrypt staging CA in `ingress.tls.letsencrypt.server` | Advanced users can override the ACME server URL entirely | Low | Pre-populate with staging URL; document production URL in comments |

---

## Anti-Features

Features to explicitly not build in this milestone.

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| DNS-01 ACME challenge | Requires cloud provider API credentials in the cluster — significant complexity, out of scope for this app | HTTP-01 only; document DNS-01 as a future option for wildcard certs |
| Multiple TLS hosts per ingress | This is a single-domain pottery shop; multi-host config adds template complexity with no benefit | Single `ingress.host` string |
| cert-manager `Issuer` (namespaced) instead of `ClusterIssuer` | ClusterIssuer is the standard for shared platform cert-manager installs; Issuer adds namespace coupling | ClusterIssuer only |
| Storing cert-manager ACME account key in values.yaml | Credentials in Helm values end up in release history (world-readable in-cluster) | cert-manager manages ACME account key in its own Secret automatically |
| Gateway API (HTTPRoute) instead of Ingress | Traefik supports it but adds CRD complexity; existing Ingress template already works | Stick with Kubernetes Ingress API |
| Separate cert-manager namespace | cert-manager documentation warns against it as a subchart; same-namespace install is simpler | Install cert-manager in the app release namespace |
| IngressRoute (Traefik CRD) | Requires Traefik CRDs; standard Kubernetes Ingress works and is controller-agnostic | Use `networking.k8s.io/v1` Ingress |

---

## values.yaml Interface Design

This is the complete proposed `ingress:` block for all three TLS modes. The existing `ingress.hosts[]` array is replaced by `ingress.host` (single string) because this app always has one hostname.

### Proposed values.yaml structure

```yaml
ingress:
  enabled: true
  className: traefik          # Ingress controller class; "traefik" for k3s/k3d defaults
  annotations: {}             # Additional annotations; cert-manager annotation added automatically
  host: clay.example.com      # REQUIRED: single hostname for the app

  tls:
    mode: letsencrypt         # One of: letsencrypt | selfsigned | custom

    # -- letsencrypt mode: HTTP-01 ACME via Let's Encrypt
    letsencrypt:
      email: ""               # REQUIRED when mode=letsencrypt
      production: false       # true = production LE certs; false = staging (browser-untrusted but no rate limits)

    # -- custom mode: reference a pre-existing kubernetes.io/tls Secret
    custom:
      secretName: ""          # REQUIRED when mode=custom; must exist before helm install

# cert-manager subchart -- condition-gated, same pattern as cloudnative-pg
cert-manager:
  enabled: true               # Set false when mode=custom (cert-manager not needed)
  crds:
    enabled: true             # Install cert-manager CRDs; required on first install
```

### Mode-specific operator usage

**letsencrypt mode (default — production with valid CA)**
```yaml
ingress:
  host: clay.example.com
  tls:
    mode: letsencrypt
    letsencrypt:
      email: ops@example.com
      production: true
cert-manager:
  enabled: true
  crds:
    enabled: true
```

**selfsigned mode (internal/dev — browser shows untrusted warning)**
```yaml
ingress:
  host: clay.internal
  tls:
    mode: selfsigned
cert-manager:
  enabled: true
  crds:
    enabled: true
```

**custom mode (BYO certificate — operator pre-creates Secret)**
```yaml
ingress:
  host: clay.example.com
  tls:
    mode: custom
    custom:
      secretName: clay-tls-prod
cert-manager:
  enabled: false              # cert-manager not needed in custom mode
```

---

## Template Logic Summary

### `ingress.yaml` — Annotation injection by mode

```yaml
# Mode-driven annotation (ingress-shim picks this up automatically)
{{- if eq .Values.ingress.tls.mode "letsencrypt" }}
  cert-manager.io/cluster-issuer: {{ if .Values.ingress.tls.letsencrypt.production }}clay-letsencrypt-prod{{ else }}clay-letsencrypt-staging{{ end }}
{{- else if eq .Values.ingress.tls.mode "selfsigned" }}
  cert-manager.io/cluster-issuer: clay-selfsigned
{{- end }}
# Traefik HTTPS entrypoint
traefik.ingress.kubernetes.io/router.entrypoints: websecure
```

TLS secret name in `spec.tls`:
- `letsencrypt` / `selfsigned`: `{{ .Values.ingress.host }}-tls` (cert-manager creates this Secret)
- `custom`: `{{ .Values.ingress.tls.custom.secretName }}`

### New template files needed

| Template | Purpose |
|----------|---------|
| `templates/cert-manager-clusterissuer.yaml` | ClusterIssuer resources — letsencrypt (staging + prod) and selfsigned; gated by mode |

No separate `Certificate` resource is needed — ingress-shim creates it automatically from the Ingress annotation + tls.secretName. This is the standard cert-manager pattern.

---

## Feature Dependencies

```
ingress.host (set)
  → Ingress hostname
  → tls.secretName (default: {{ host }}-tls)
  → ClusterIssuer commonName (letsencrypt/selfsigned modes)

ingress.tls.mode = letsencrypt
  → cert-manager subchart (must be enabled)
  → ClusterIssuer letsencrypt-staging (always created in letsencrypt mode)
  → ClusterIssuer letsencrypt-prod (created when production: true)
  → ingress annotation: cert-manager.io/cluster-issuer
  → HTTP-01 solver with ingressClassName: traefik

ingress.tls.mode = selfsigned
  → cert-manager subchart (must be enabled)
  → ClusterIssuer selfsigned (SelfSigned type, no ACME)
  → ingress annotation: cert-manager.io/cluster-issuer

ingress.tls.mode = custom
  → cert-manager NOT required (can be disabled)
  → Pre-existing kubernetes.io/tls Secret referenced by name
  → NO cert-manager annotation on Ingress

cert-manager subchart enabled
  → crds.enabled: true (required on first install to register CRDs)
  → webhook timing: ClusterIssuer cannot be created until webhook is ready
```

---

## Relationship to Existing CNPG Pattern

The cert-manager subchart uses the same `condition:` mechanism in `Chart.yaml` as the CNPG operator:

```yaml
dependencies:
  - name: cert-manager
    version: "v1.17.x"        # pin to current stable
    repository: "https://charts.jetstack.io"
    condition: cert-manager.enabled
```

The `cert-manager.enabled` value in `values.yaml` is the same condition switch. When `mode: custom`, operators set `cert-manager.enabled: false` to skip the subchart entirely, identical to setting `postgres.managed: false` to skip CNPG.

This means the chart has a consistent pattern: every optional system dependency is a subchart gated by `<subchart-key>.enabled` in `values.yaml`.

---

## CI Test Values Files Required

| File | Mode | What it validates |
|------|------|------------------|
| `ci/tls-letsencrypt.yaml` | letsencrypt + staging issuer | ACME ClusterIssuer renders, Ingress annotation set, cert-manager enabled |
| `ci/tls-selfsigned.yaml` | selfsigned | SelfSigned ClusterIssuer renders, Ingress annotation set |
| `ci/tls-custom.yaml` | custom | No ClusterIssuer, no annotation, secretName wired to Ingress tls |

These join the existing `ci/managed-values.yaml` and `ci/external-values.yaml` files. `helm lint` in CI should iterate all five.

---

## Complexity Notes

| Area | Assessment |
|------|-----------|
| Ingress template changes | Low — add annotation logic and simplify host from array to string |
| ClusterIssuer template | Low — ~30 lines of YAML, straightforward if/else by mode |
| cert-manager subchart wiring | Low — identical pattern to CNPG already implemented |
| CRD timing pitfall | Medium — cert-manager webhook must be ready before ClusterIssuer is created; Helm's `--wait` flag and `startupapicheck` (built into cert-manager chart) mitigate but don't eliminate this |
| HTTP-01 challenge routing | Medium — Traefik must have port 80 publicly reachable for LE challenge; fails silently in private clusters |
| Three-mode CI coverage | Low — three small values files, no new test infrastructure needed |
| Backward compatibility | Medium — existing `ingress.hosts[]` array must be considered; either migrate cleanly or support both shapes |

---

## Critical Constraints from Existing Chart

1. The existing `values.yaml` has `ingress.hosts` as an **array** and `ingress.tls` as a **list of secretName/hosts pairs**. The new design flattens this to `ingress.host` (string) and `ingress.tls.mode` (string). This is a **breaking change** to values structure — acceptable for a new milestone, but must be documented.

2. The existing ingress template iterates `range .Values.ingress.hosts`. After this change, the template uses `ingress.host` directly. The current nginx annotation (`nginx.ingress.kubernetes.io/proxy-body-size`) should be moved into the Traefik annotation block or kept as a passthrough annotation.

3. `Chart.yaml` currently has no `dependencies:` block (CNPG was moved out per git history: "move cnpg out of the Clay chart"). This means **CNPG is no longer a subchart** — the pattern reference in `PROJECT.md` describes the intended design but it may be installed separately. The cert-manager subchart dependency approach still applies as the clean pattern, but verify the current chart state before implementing.

---

## Sources

- cert-manager Ingress annotations (ingress-shim): https://cert-manager.io/docs/usage/ingress/
- cert-manager HTTP-01 solver: https://cert-manager.io/docs/configuration/acme/http01/
- cert-manager SelfSigned issuer: https://cert-manager.io/docs/configuration/selfsigned/
- cert-manager Helm installation (v1.20.2, OCI registry, crds.enabled): https://cert-manager.io/docs/installation/helm/
- cert-manager best practices: https://cert-manager.io/docs/installation/best-practice/
- cert-manager as Helm subchart (webhook timing pitfall, post-install Job): https://skarlso.github.io/2024/07/02/using-cert-manager-as-a-subchart-with-helm/
- Traefik + cert-manager integration (ingressClassName: traefik): https://doc.traefik.io/traefik/v3.3/user-guides/cert-manager/
- cert-manager ClusterIssuer concept: https://cert-manager.io/docs/concepts/issuer/
