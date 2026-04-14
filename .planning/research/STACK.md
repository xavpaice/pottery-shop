# Stack Research: TLS/Ingress Milestone (v1.1)

**Researched:** 2026-04-14
**Overall confidence:** HIGH (all version pins and annotation keys verified against official sources)
**Scope:** New additions only. The existing Go, pgx, CNPG, and Docker stack from the Postgres milestone are not repeated here.

---

## cert-manager

### Version

**Latest stable:** v1.20.2 (released 2026-04-11)
**Currently supported:** v1.20.x and v1.19.x (each release supported until 2 subsequent releases; approximately 4-month windows)

| Component | Version | Notes |
|-----------|---------|-------|
| cert-manager | v1.20.2 | Latest stable as of April 2026 |
| Helm chart | v1.20.2 | Chart version matches appVersion for cert-manager |

Source: https://cert-manager.io/docs/releases/

### Helm Chart Coordinates

**Recommended repository:** `https://charts.jetstack.io` (legacy HTTPS, updated hours after OCI)
**OCI alternative:** `oci://quay.io/jetstack/charts` — authoritative source, published immediately on release

Use the HTTPS repository for Chart.yaml subchart dependencies. There is a known Helm issue where the `v`-prefixed version tag in OCI registries (`v1.20.2`) breaks `helm dependency update` in some Helm versions because Helm normalizes the SemVer and cannot match the tag. The HTTPS repository resolves this reliably.

Source: https://github.com/helm/helm/issues/11107 (v-prefix OCI tag issue)

### Chart.yaml Dependency Entry

Mirror the existing CNPG pattern exactly:

```yaml
dependencies:
  - name: cert-manager
    repository: https://charts.jetstack.io
    version: "v1.20.2"
    condition: certManager.enabled
```

The `condition` field (`certManager.enabled`) lets operators skip installing cert-manager when it is already present in the cluster — same rationale as the existing `cnpg.enabled` condition on the CNPG subchart.

**Fetch after editing Chart.yaml:**
```bash
helm dependency update chart/clay/
```

### Required values.yaml Entry

The parent chart's `values.yaml` must pass `crds.enabled: true` to the subchart, or CRDs will not be installed and all cert-manager CRs (Certificate, ClusterIssuer) will be rejected.

```yaml
certManager:
  enabled: true
  crds:
    enabled: true
    # keep: true prevents CRD deletion on helm uninstall (optional but recommended)
    keep: true
```

**Key note:** `installCRDs` (the old flag) is deprecated as of v1.15+. Use `crds.enabled` instead.

Source: https://cert-manager.io/docs/installation/helm/

---

## Kubernetes Ingress Resource (Traefik-specific)

### ingressClassName

k3s ships Traefik as the default ingress controller. The correct `ingressClassName` value is `traefik`.

```yaml
spec:
  ingressClassName: traefik
```

The old annotation `kubernetes.io/ingress.class: traefik` is deprecated since Kubernetes 1.18 and should not be used in new resources.

### Annotations for TLS with cert-manager

Two annotation keys matter for the Ingress resource itself:

| Annotation | Value | Purpose |
|------------|-------|---------|
| `cert-manager.io/cluster-issuer` | name of your ClusterIssuer | Triggers ingress-shim to auto-create a Certificate resource |
| `traefik.ingress.kubernetes.io/router.entrypoints` | `websecure` | Tells Traefik to route this Ingress on the HTTPS entrypoint (port 443) |

The `cert-manager.io/cluster-issuer` annotation is preferred over `cert-manager.io/issuer` because ClusterIssuers are cluster-scoped — they work regardless of which namespace the Ingress lives in. An Issuer would have to exist in the same namespace as the Ingress.

When cert-manager's ingress-shim sees the `cert-manager.io/cluster-issuer` annotation on an Ingress that also has a `tls` block, it automatically creates a Certificate CR with `spec.secretName` matching the `tls.secretName`. You do not need to create the Certificate resource manually for Let's Encrypt or self-signed modes.

Source: https://cert-manager.io/docs/usage/ingress/

### Complete Ingress Annotation Block (letsencrypt mode)

```yaml
annotations:
  traefik.ingress.kubernetes.io/router.entrypoints: websecure
  cert-manager.io/cluster-issuer: letsencrypt-prod
```

### TLS block in Ingress spec

```yaml
tls:
  - hosts:
      - {{ .Values.ingress.host }}
    secretName: {{ include "clay.fullname" . }}-tls
```

The `secretName` is where cert-manager will store the provisioned TLS certificate. It is created in the same namespace as the Ingress.

---

## ClusterIssuer Resources (Three TLS Modes)

### Mode 1: letsencrypt (HTTP-01 ACME) — Default

```yaml
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: {{ include "clay.fullname" . }}-letsencrypt
spec:
  acme:
    server: https://acme-v02.api.letsencrypt.org/directory
    email: {{ .Values.tls.letsencrypt.email }}
    privateKeySecretRef:
      name: {{ include "clay.fullname" . }}-letsencrypt-account-key
    solvers:
      - http01:
          ingress:
            ingressClassName: traefik
```

**Traefik / k3s HTTP-01 note:** The solver must use `ingressClassName: traefik` (not the legacy `class: traefik`). cert-manager creates a temporary Ingress to serve the ACME challenge on `/.well-known/acme-challenge/`. With `ingressClassName: traefik`, Traefik picks it up correctly. Using the legacy `class` field caused regressions after cert-manager v1.5.4 because cert-manager stopped setting the `kubernetes.io/ingress.class` annotation.

**Let's Encrypt staging for testing:** Use `https://acme-staging-v02.api.letsencrypt.org/directory` during CI/integration testing to avoid rate limits. Switch to prod for real deployments.

Source: https://cert-manager.io/docs/configuration/acme/http01/

### Mode 2: selfsigned

```yaml
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: {{ include "clay.fullname" . }}-selfsigned
spec:
  selfSigned: {}
```

Self-signed is the simplest issuer — no dependencies, no external connectivity. Suitable for internal/staging environments where browser trust warnings are acceptable.

The Ingress annotation for this mode would be:
```yaml
cert-manager.io/cluster-issuer: {{ include "clay.fullname" . }}-selfsigned
```

Source: https://cert-manager.io/docs/configuration/selfsigned/

### Mode 3: custom (BYO Secret)

No ClusterIssuer or Certificate CR is needed. The user pre-creates a Kubernetes TLS Secret and the Ingress `tls.secretName` simply references it. cert-manager is not involved in this mode.

The chart should skip creating any issuer or certificate resource when `tls.mode: custom` is set. The values.yaml would accept `tls.custom.secretName` to reference the user-provided Secret.

---

## values.yaml Structure for TLS Modes

Recommended new `tls` block in values.yaml:

```yaml
tls:
  # mode: letsencrypt | selfsigned | custom
  mode: letsencrypt

  letsencrypt:
    # Email registered with Let's Encrypt for expiry notices
    email: ""

  custom:
    # Name of a pre-existing Kubernetes TLS Secret in the same namespace
    secretName: ""
```

**Backward compatibility:** The existing `ingress.tls` block in values.yaml (currently hardcodes `clay-tls` as the secretName) should be superseded by the new `tls` block. Keep a single `ingress.host` value rather than the current `ingress.hosts[].host` list — the milestone description specifies `ingress.host` (singular) drives everything.

**CI test values needed (three files):**

| File | tls.mode | Purpose |
|------|----------|---------|
| `ci/tls-letsencrypt-values.yaml` | `letsencrypt` (staging server) | Validates ClusterIssuer + Certificate rendering |
| `ci/tls-selfsigned-values.yaml` | `selfsigned` | Validates selfsigned issuer rendering |
| `ci/tls-custom-values.yaml` | `custom` | Validates no issuer created, custom secret ref |

---

## What NOT to Add

| Item | Reason |
|------|--------|
| Traefik IngressRoute CRD | Requires Traefik CRDs, breaks portability. Standard `networking.k8s.io/v1 Ingress` works with Traefik and is simpler. |
| trust-manager | cert-manager's optional companion for distributing CA bundles. Not needed for this app — only useful when you need to distribute a CA cert to pods for mTLS. |
| Separate Certificate CR in templates | The ingress-shim auto-creates it from the Ingress annotation. Manual Certificate CR only needed for non-Ingress use cases or if you need fine-grained control over renewal timing. |
| DNS-01 ACME solver | Requires DNS provider credentials and is more complex to configure. HTTP-01 is sufficient and simpler when the cluster has public ingress. |
| cert-manager as a cluster-level pre-install | The milestone spec wants subchart (same install pattern as CNPG). Documented warning against subchart embedding is a recommendation for public chart authors, not a hard constraint for private umbrella charts. |

---

## Integration with Existing Chart Patterns

The CNPG subchart pattern (from Phase 2) is the template to follow:

| Concern | CNPG pattern | cert-manager equivalent |
|---------|-------------|------------------------|
| Chart.yaml dependency | `condition: cnpg.enabled` | `condition: certManager.enabled` |
| values.yaml enable key | `cnpg.enabled: true` | `certManager.enabled: true` |
| CRD installation | N/A (operator installs via Helm Job) | `certManager.crds.enabled: true` |
| Conditional CR creation | `{{- if .Values.postgres.managed }}` | `{{- if eq .Values.tls.mode "letsencrypt" }}` |
| values.yaml subchart passthrough | `cnpg: {}` block | `certManager: {}` block |

The key difference: cert-manager's CRDs are installed via the Helm chart itself (controlled by `crds.enabled`), whereas CNPG's CRDs are bundled inside the operator image. This means `crds.enabled: true` is mandatory in the cert-manager values block.

---

## Summary: Definitive Version Pins

| Component | Version | Helm Repo | Confidence |
|-----------|---------|-----------|------------|
| cert-manager | v1.20.2 | https://charts.jetstack.io | HIGH — verified at cert-manager.io/docs/releases/ |
| Ingress API | networking.k8s.io/v1 | built-in | HIGH — stable since Kubernetes 1.19 |
| Let's Encrypt ACME v2 | n/a | https://acme-v02.api.letsencrypt.org | HIGH — current production endpoint |

## Sources

- cert-manager releases: https://cert-manager.io/docs/releases/
- cert-manager Helm install: https://cert-manager.io/docs/installation/helm/
- cert-manager Ingress annotations: https://cert-manager.io/docs/usage/ingress/
- cert-manager HTTP-01 solver: https://cert-manager.io/docs/configuration/acme/http01/
- cert-manager self-signed issuer: https://cert-manager.io/docs/configuration/selfsigned/
- Traefik + cert-manager integration (v3.4 docs): https://doc.traefik.io/traefik/v3.4/user-guides/cert-manager/
- Using cert-manager as subchart: https://skarlso.github.io/2024/07/02/using-cert-manager-as-a-subchart-with-helm/
- Helm OCI v-prefix issue: https://github.com/helm/helm/issues/11107
