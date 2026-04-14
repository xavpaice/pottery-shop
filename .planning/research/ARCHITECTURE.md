# Architecture: TLS/Ingress Integration

**Project:** pottery-shop — v1.1 TLS milestone
**Researched:** 2026-04-14
**Overall Confidence:** HIGH (official cert-manager docs + Traefik docs + verified CNPG pattern from existing codebase)

---

## Critical Finding: How "Subchart" Is Actually Implemented

Before designing the cert-manager integration, it is essential to understand that the CNPG precedent is NOT a Helm subchart in the `Chart.yaml dependencies:` sense.

Current `chart/clay/Chart.yaml` has NO `dependencies:` block. The `charts/` directory is empty. CNPG is installed as a separate, prior `helm install` step in `.github/workflows/integration-test.yml`:

```yaml
- name: Install CNPG operator
  run: |
    helm install cnpg cnpg/cloudnative-pg \
      --namespace cnpg-system \
      --create-namespace \
      --wait \
      --timeout 3m
```

The clay chart then creates a CNPG `Cluster` resource (in `templates/cnpg-cluster.yaml`) that the separately-installed CNPG operator reconciles. The `cnpg-cluster.yaml` template uses `{{- if .Values.postgres.managed }}` to gate rendering.

This is the correct architecture for cluster-scoped operators. cert-manager must follow the same pattern — installed separately before the clay chart, not as a Helm subchart dependency.

**Why subcharting cert-manager is wrong:** cert-manager is explicitly documented as something that "should never be embedded as a sub-chart of other Helm charts — cert-manager manages non-namespaced (cluster-scoped) resources in your cluster and care must be taken to ensure it is installed exactly once." The CRD installation timing race condition (CRD created after dependent resources) has been an unresolved Helm issue since 2019.

---

## Architecture Overview: How the Pieces Fit Together

```
cert-manager operator (separate helm install, cert-manager namespace)
  └── watches for Certificate and ClusterIssuer CRs cluster-wide

clay Helm chart (chart/clay/)
  ├── templates/cluster-issuer.yaml     (NEW — ClusterIssuer CR, gated on tls.mode)
  ├── templates/certificate.yaml        (NEW — Certificate CR, gated on tls.enabled and mode != custom)
  ├── templates/ingress.yaml            (MODIFY — add Traefik annotations, ingressClassName, tls block)
  ├── templates/cnpg-cluster.yaml       (EXISTING — unchanged)
  ├── templates/deployment.yaml         (EXISTING — unchanged)
  └── ... (other existing templates)
```

The clay chart creates cert-manager CR resources (ClusterIssuer, Certificate) that the separately-installed cert-manager operator reconciles. This is identical in pattern to how the clay chart creates a CNPG `Cluster` resource that the CNPG operator reconciles.

---

## New Templates Required

### 1. `templates/cluster-issuer.yaml` (NEW)

This template is conditional on `ingress.tls.mode`. It is NOT rendered when `mode: custom` because the user brings their own TLS secret.

**letsencrypt mode:**
```yaml
{{- if and .Values.ingress.enabled (eq .Values.ingress.tls.mode "letsencrypt") }}
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: {{ include "clay.fullname" . }}-letsencrypt
  labels:
    {{- include "clay.labels" . | nindent 4 }}
spec:
  acme:
    server: https://acme-v02.api.letsencrypt.org/directory
    email: {{ .Values.ingress.tls.acme.email | required "ingress.tls.acme.email is required for letsencrypt mode" }}
    privateKeySecretRef:
      name: {{ include "clay.fullname" . }}-letsencrypt-account
    solvers:
      - http01:
          ingress:
            ingressClassName: traefik
{{- end }}
```

**selfsigned mode:**
```yaml
{{- if and .Values.ingress.enabled (eq .Values.ingress.tls.mode "selfsigned") }}
---
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: {{ include "clay.fullname" . }}-selfsigned
  labels:
    {{- include "clay.labels" . | nindent 4 }}
spec:
  selfSigned: {}
{{- end }}
```

A single file can contain both blocks (they are mutually exclusive). Use `---` separators.

### 2. `templates/certificate.yaml` (NEW)

This template creates the cert-manager `Certificate` resource. Not rendered in `custom` mode (user supplies the Secret directly). The Certificate resource names the target Secret that Ingress references in `tls.secretName`.

```yaml
{{- if and .Values.ingress.enabled (ne .Values.ingress.tls.mode "custom") }}
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: {{ include "clay.fullname" . }}-tls
  labels:
    {{- include "clay.labels" . | nindent 4 }}
spec:
  secretName: {{ include "clay.fullname" . }}-tls
  dnsNames:
    - {{ .Values.ingress.host | required "ingress.host is required when ingress.enabled=true" | quote }}
  issuerRef:
    {{- if eq .Values.ingress.tls.mode "letsencrypt" }}
    name: {{ include "clay.fullname" . }}-letsencrypt
    {{- else if eq .Values.ingress.tls.mode "selfsigned" }}
    name: {{ include "clay.fullname" . }}-selfsigned
    {{- end }}
    kind: ClusterIssuer
    group: cert-manager.io
{{- end }}
```

### 3. `templates/ingress.yaml` (MODIFY)

The existing ingress.yaml uses the old multi-host `ingress.hosts[]` structure. This must be refactored to use a single `ingress.host` scalar (simpler, matches the single-hostname use case). The Traefik-specific annotation and `ingressClassName` must be added.

Key changes:
- Add `ingressClassName: traefik` to spec (replace the current empty `className` approach)
- Add Traefik entrypoint annotation: `traefik.ingress.kubernetes.io/router.entrypoints: websecure`
- Set `tls.secretName` to the cert-manager-provisioned secret name (derived from `clay.fullname`)
- Gate the TLS block on `ingress.tls.mode != ""` (i.e., TLS is being used)
- Preserve `ingress.annotations` pass-through for user-supplied annotations

```yaml
{{- if .Values.ingress.enabled }}
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: {{ include "clay.fullname" . }}
  labels:
    {{- include "clay.labels" . | nindent 4 }}
  annotations:
    traefik.ingress.kubernetes.io/router.entrypoints: websecure
    {{- with .Values.ingress.annotations }}
    {{- toYaml . | nindent 4 }}
    {{- end }}
spec:
  ingressClassName: traefik
  rules:
    - host: {{ .Values.ingress.host | required "ingress.host is required" | quote }}
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: {{ include "clay.fullname" . }}
                port:
                  number: {{ .Values.service.port }}
  {{- if .Values.ingress.tls.mode }}
  tls:
    - secretName: {{ if eq .Values.ingress.tls.mode "custom" }}{{ .Values.ingress.tls.secretName | required "ingress.tls.secretName required in custom mode" }}{{ else }}{{ include "clay.fullname" . }}-tls{{ end }}
      hosts:
        - {{ .Values.ingress.host | quote }}
  {{- end }}
{{- end }}
```

---

## values.yaml Structure for Ingress Block

The existing `ingress` block in values.yaml uses a multi-host list structure (`ingress.hosts[]`) that was likely Helm scaffold boilerplate. It must be replaced with a flatter single-host structure that reflects real usage.

**Replace the existing `ingress:` block with:**

```yaml
# -- Ingress configuration
ingress:
  enabled: true
  # -- The hostname for the Ingress rule and Certificate dnsNames
  host: clay.nz
  # -- Additional annotations merged into the Ingress metadata
  annotations:
    nginx.ingress.kubernetes.io/proxy-body-size: "50m"   # keep for compat; traefik ignores nginx annotations
  tls:
    # -- TLS mode: letsencrypt | selfsigned | custom | "" (disabled)
    mode: letsencrypt
    # -- For mode=custom: name of a pre-existing Kubernetes TLS Secret
    secretName: ""
    acme:
      # -- Email for Let's Encrypt account registration (required in letsencrypt mode)
      email: ""
```

**Mode semantics:**
| `tls.mode` | ClusterIssuer rendered? | Certificate rendered? | tls.secretName source |
|---|---|---|---|
| `letsencrypt` | Yes (ACME) | Yes | `<fullname>-tls` (cert-manager) |
| `selfsigned` | Yes (selfSigned) | Yes | `<fullname>-tls` (cert-manager) |
| `custom` | No | No | `ingress.tls.secretName` (user-supplied) |
| `""` (empty) | No | No | No TLS block in Ingress |

**Backward compatibility:** The old `ingress.hosts[]`, `ingress.tls[]`, and `ingress.className` keys are being removed and replaced. Because the active milestone explicitly redesigns TLS configuration, backward compatibility for those specific sub-keys is not required. The top-level `ingress.enabled` key is preserved.

---

## CI Values Files: Three TLS Modes

Add three new CI value files (or add three new Helm lint/template steps to the existing ones):

**`chart/clay/ci/tls-letsencrypt-values.yaml`:**
```yaml
secrets:
  ADMIN_PASS: "ci-test-only"
  SESSION_SECRET: "ci-test-session-secret-not-for-production"
postgres:
  managed: false
  external:
    dsn: "postgresql://clay-db.example.com:5432/clay"
ingress:
  enabled: true
  host: clay.example.com
  tls:
    mode: letsencrypt
    acme:
      email: test@example.com
```

**`chart/clay/ci/tls-selfsigned-values.yaml`:**
```yaml
secrets:
  ADMIN_PASS: "ci-test-only"
  SESSION_SECRET: "ci-test-session-secret-not-for-production"
postgres:
  managed: false
  external:
    dsn: "postgresql://clay-db.example.com:5432/clay"
ingress:
  enabled: true
  host: clay.example.com
  tls:
    mode: selfsigned
```

**`chart/clay/ci/tls-custom-values.yaml`:**
```yaml
secrets:
  ADMIN_PASS: "ci-test-only"
  SESSION_SECRET: "ci-test-session-secret-not-for-production"
postgres:
  managed: false
  external:
    dsn: "postgresql://clay-db.example.com:5432/clay"
ingress:
  enabled: true
  host: clay.example.com
  tls:
    mode: custom
    secretName: my-existing-tls-secret
```

`helm lint` and `helm template` for each of these validate that the correct resources are rendered without errors. Because cert-manager CRDs are not present in CI, `helm lint --strict` will warn about unknown resource types — use `helm lint` (not `--strict`) and `helm template` (which does not contact the API server) to validate.

---

## CI Workflow Changes (test.yml)

Add to the `helm-lint` job:

```yaml
- name: Lint (TLS letsencrypt mode)
  run: helm lint chart/clay/ --values chart/clay/ci/tls-letsencrypt-values.yaml

- name: Lint (TLS selfsigned mode)
  run: helm lint chart/clay/ --values chart/clay/ci/tls-selfsigned-values.yaml

- name: Lint (TLS custom mode)
  run: helm lint chart/clay/ --values chart/clay/ci/tls-custom-values.yaml

- name: Template (TLS letsencrypt mode)
  run: helm template clay chart/clay/ --values chart/clay/ci/tls-letsencrypt-values.yaml

- name: Template (TLS selfsigned mode)
  run: helm template clay chart/clay/ --values chart/clay/ci/tls-selfsigned-values.yaml

- name: Template (TLS custom mode)
  run: helm template clay chart/clay/ --values chart/clay/ci/tls-custom-values.yaml
```

`helm template` renders manifests locally without any cluster connection, so cert-manager CRDs being absent in CI is not a problem.

---

## Integration Test Workflow Changes (integration-test.yml)

Add a cert-manager pre-install step, mirroring the CNPG pattern exactly:

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

This step must come BEFORE the `helm install clay` step, for the same reason CNPG is installed first: the clay chart creates `ClusterIssuer` and `Certificate` CRs that cert-manager must be able to reconcile.

In the `helm install clay` step, remove `--set ingress.enabled=false` (or supply a `--values tls-selfsigned-values.yaml` override) so the TLS resources are exercised in integration testing. Note that HTTP-01 challenges require public DNS and a publicly reachable cluster — for CI, `selfsigned` mode is the correct choice.

---

## Build Order

Dependencies flow in this sequence. Each step is a prerequisite for the next.

**Step 1: values.yaml restructure**
Refactor the `ingress:` block to the new single-host, `tls.mode`-based structure. This is the foundation everything else references. The change is purely to `chart/clay/values.yaml`.

**Step 2: Ingress template update**
Modify `chart/clay/templates/ingress.yaml` to use `ingress.host` (singular), add Traefik `ingressClassName: traefik` and the `router.entrypoints: websecure` annotation, and rewrite the TLS block to use mode-driven conditional logic. This can be validated immediately with `helm template` using the new values structure.

**Step 3: ClusterIssuer template**
Create `chart/clay/templates/cluster-issuer.yaml` with both letsencrypt and selfsigned blocks, each gated by `eq .Values.ingress.tls.mode`. Validate with `helm template` for each mode.

**Step 4: Certificate template**
Create `chart/clay/templates/certificate.yaml` gated on `ne .Values.ingress.tls.mode "custom"`. Validate with `helm template`.

**Step 5: CI values files**
Create the three new ci/ values files for letsencrypt, selfsigned, and custom modes.

**Step 6: CI workflow update (test.yml)**
Add the six new lint and template steps to `test.yml`. These require no cluster access so they work immediately.

**Step 7: Integration test update (integration-test.yml)**
Add the cert-manager pre-install step and update the clay install step to include selfsigned TLS mode values.

**Step 8: README / values documentation**
Document the three TLS modes and the cert-manager pre-requisite.

The critical dependency: Steps 3 and 4 both depend on Step 1 (values structure) and Step 2 (ingress hostname key). Steps 3 and 4 are independent of each other and can be built in parallel.

---

## Conditional Rendering Pattern (Consistency with cnpg-cluster.yaml)

The existing `cnpg-cluster.yaml` uses: `{{- if .Values.postgres.managed }}`.

The new TLS templates follow the same idiom, gating on `ingress.tls.mode` string comparison:

```
cnpg-cluster.yaml:    {{- if .Values.postgres.managed }}
cluster-issuer.yaml:  {{- if and .Values.ingress.enabled (eq .Values.ingress.tls.mode "letsencrypt") }}
certificate.yaml:     {{- if and .Values.ingress.enabled (ne .Values.ingress.tls.mode "custom") }}
```

The `and .Values.ingress.enabled` guard in the TLS templates prevents rendering TLS resources when ingress is disabled entirely (e.g., in CI runs that disable ingress).

---

## Validation Helper in _helpers.tpl

Following the pattern of `clay.validateSecrets`, add a `clay.validateIngress` validation to `_helpers.tpl` that fails fast at render time:

```
{{- define "clay.validateIngress" -}}
{{- if .Values.ingress.enabled }}
  {{- if not .Values.ingress.host }}
    {{- fail "ingress.host is required when ingress.enabled=true" }}
  {{- end }}
  {{- if eq .Values.ingress.tls.mode "letsencrypt" }}
    {{- if not .Values.ingress.tls.acme.email }}
      {{- fail "ingress.tls.acme.email is required when tls.mode=letsencrypt" }}
    {{- end }}
  {{- end }}
  {{- if eq .Values.ingress.tls.mode "custom" }}
    {{- if not .Values.ingress.tls.secretName }}
      {{- fail "ingress.tls.secretName is required when tls.mode=custom" }}
    {{- end }}
  {{- end }}
{{- end }}
{{- end }}
```

Call `{{- include "clay.validateIngress" . }}` from the top of `ingress.yaml` (or `certificate.yaml`).

---

## Files: New vs Modified

| File | Action | Reason |
|---|---|---|
| `chart/clay/values.yaml` | Modify | Replace ingress block with new tls.mode structure |
| `chart/clay/templates/ingress.yaml` | Modify | Add Traefik annotations, ingressClassName, rewrite TLS section |
| `chart/clay/templates/_helpers.tpl` | Modify | Add clay.validateIngress helper |
| `chart/clay/templates/cluster-issuer.yaml` | Create | New ClusterIssuer template (letsencrypt + selfsigned) |
| `chart/clay/templates/certificate.yaml` | Create | New Certificate template (letsencrypt + selfsigned) |
| `chart/clay/ci/tls-letsencrypt-values.yaml` | Create | CI test values for letsencrypt mode |
| `chart/clay/ci/tls-selfsigned-values.yaml` | Create | CI test values for selfsigned mode |
| `chart/clay/ci/tls-custom-values.yaml` | Create | CI test values for custom mode |
| `.github/workflows/test.yml` | Modify | Add lint/template steps for three TLS modes |
| `.github/workflows/integration-test.yml` | Modify | Add cert-manager pre-install step |

**Not modified:**
- `chart/clay/Chart.yaml` — no subchart dependency added (correct, matches CNPG pattern)
- `chart/clay/templates/cnpg-cluster.yaml` — unchanged
- `chart/clay/templates/deployment.yaml` — unchanged
- `chart/clay/templates/configmap.yaml` — unchanged
- All Go source files — TLS is purely infrastructure

---

## Architecture Decision: cert-manager as Pre-Requisite, Not Subchart

**Decision:** cert-manager is installed as a separate `helm install` step before the clay chart, not as a `Chart.yaml` dependency.

**Rationale:**
1. Official cert-manager documentation explicitly states it should never be embedded as a subchart because it manages cluster-scoped (non-namespaced) resources.
2. The CNPG operator follows this exact pattern already in this project — separate install, then the clay chart creates CNPG CRs.
3. Helm subcharting cert-manager creates CRD installation timing race conditions that have remained unresolved issues since 2019.
4. cert-manager is cluster infrastructure, not application code. Installing it once cluster-wide is correct.
5. The `crds.enabled=true` flag during installation handles CRD installation safely (Helm does not delete CRDs on uninstall when they are installed this way via the crds directory).

**Consequence:** Deployers must install cert-manager before `helm install clay`. This is the same requirement as CNPG — the operational model is already established.

---

## Sources

- cert-manager official Helm installation docs: https://cert-manager.io/docs/installation/helm/
- cert-manager HTTP-01 solver docs: https://cert-manager.io/docs/configuration/acme/http01/
- cert-manager SelfSigned issuer docs: https://cert-manager.io/docs/configuration/selfsigned/
- cert-manager subchart warning (official): https://cert-manager.io/docs/installation/helm/ ("Be sure never to embed cert-manager as a sub-chart")
- Traefik cert-manager integration (v3.3): https://doc.traefik.io/traefik/v3.3/user-guides/cert-manager/
- cert-manager CRD timing issue (GitHub): https://github.com/cert-manager/cert-manager/issues/2961
- cert-manager current version (v1.20.2): https://artifacthub.io/packages/helm/cert-manager/cert-manager
