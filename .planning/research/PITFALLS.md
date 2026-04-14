# Domain Pitfalls: TLS/Ingress Milestone

**Domain:** Adding cert-manager + Kubernetes Ingress TLS to an existing Helm chart (clay chart, already has CNPG subchart)
**Researched:** 2026-04-14
**Overall confidence:** HIGH for CRD ordering and Traefik-specific issues (multiple official sources); MEDIUM for CI patterns (community + official); HIGH for HTTP-01 routing issues (official cert-manager docs + Traefik issues)

---

## Critical Pitfalls

Mistakes that cause failed installs, certificates that never issue, or silent TLS failures that require rewrites to fix.

---

### Pitfall 1: cert-manager Webhook Not Ready When ClusterIssuer Is Applied

**What goes wrong:** The parent chart (clay) defines a ClusterIssuer resource. Helm processes subchart resources and parent chart resources in a single install wave. cert-manager's three deployments (controller, webhook, cainjector) take 30–60 seconds to become ready after their pods start. If Helm tries to create the ClusterIssuer before the webhook is Ready, the admission webhook is unreachable and the install fails with a connection refused or timeout error.

**Why it happens:** cert-manager ships a `startupapicheck` post-install Job that gates on webhook readiness — but this hook runs inside the cert-manager subchart's own release context. When cert-manager is embedded as a subchart dependency, the parent chart templates (ClusterIssuer, Certificate) can be submitted to the API server before that Job completes, because Helm does not enforce cross-release hook ordering.

**Consequences:** `helm install` exits with an error; the ClusterIssuer is never created; all Certificate resources are stuck in `Pending`; the user must run `helm upgrade --install` again after the cert-manager pods stabilize, which often succeeds but is not self-healing.

**Prevention:**
- Gate the ClusterIssuer template on a Helm hook annotation `helm.sh/hook: post-install,post-upgrade` with `helm.sh/hook-weight: "5"`. This ensures Helm applies it after all non-hook resources in the parent chart and after the cert-manager subchart's own `startupapicheck` hook.
- Add a Kubernetes Job (post-install hook, weight 0) that runs `kubectl wait --for=condition=Available deployment/cert-manager deployment/cert-manager-webhook deployment/cert-manager-cainjector -n cert-manager --timeout=120s` before the ClusterIssuer hook (weight 5). Requires a ServiceAccount with permission to get/list/watch deployments in the cert-manager namespace.
- This is the same class of timing issue that was solved for CNPG with the `pg_isready` init container. The cert-manager equivalent requires a hook-based Job because cert-manager's readiness is API-server-side, not app-container-side.

**Detection:** `kubectl describe clusterissuer letsencrypt-http01` shows `Status: False, Reason: Errored`, and events mention webhook unreachable or connection refused to the cert-manager-webhook service.

**Confidence:** HIGH — documented in official cert-manager subchart guide and multiple community reports.

---

### Pitfall 2: `helm template` CI Fails With "no matches for kind Certificate" (CRD Not Present)

**What goes wrong:** The CI pipeline runs `helm template` to validate chart rendering. The chart includes `Certificate` and `ClusterIssuer` resources (cert-manager CRDs). In a clean CI environment with no live cluster, those CRDs do not exist in any API server. `helm template` by default calls the Kubernetes API to validate resource types. The render fails with:

```
Error: no matches for kind "Certificate" in version "cert-manager.io/v1"
Error: no matches for kind "ClusterIssuer" in version "cert-manager.io/v1"
```

**Why it happens:** cert-manager does not ship CRDs in the Helm `crds/` directory (the standard Helm 3 location that gets installed before templates). Instead, cert-manager bundles CRDs as regular templates when `crds.enabled=true` / `installCRDs=true`. This means the CRDs are part of the rendered output but are not guaranteed to be in the API server before other resources reference them. In CI, the API server has no cert-manager CRDs at all.

**Consequences:** `helm template` fails in CI for every run. The naive fix — disabling CRDs via `cert-manager.enabled: false` in CI values — means CI no longer validates the Certificate and ClusterIssuer resources at all, giving false confidence.

**Prevention:**
- Use `helm template --include-crds` to emit CRDs in the output, then validate with `kubeconform` using a schema registry that includes cert-manager CRDs, rather than relying on live API server validation.
- Alternatively, run CI in two steps: (1) `helm template --include-crds` piped to `kubeconform -schema-location default -schema-location 'https://raw.githubusercontent.com/datreeio/CRDs-catalog/main/{{.Group}}/{{.ResourceKind}}_{{.ResourceAPIVersion}}.json'`; (2) `helm lint` for values schema checks.
- The existing pattern in `chart/clay/ci/` (managed-values.yaml, external-values.yaml) must be extended with three new TLS-mode CI values files (tls-letsencrypt.yaml, tls-selfsigned.yaml, tls-custom.yaml). Each must pass independently.
- Note: `helm template` without a live cluster ignores API validation by default unless `--validate` is passed. Confirm your CI does not pass `--validate` to avoid this failure mode.

**Detection:** CI step exits non-zero with "no matches for kind Certificate"; `helm template` without `--include-crds` produces no Certificate or ClusterIssuer output when `cert-manager.enabled: false` in CI values.

**Confidence:** HIGH — reproduced across many cert-manager GitHub issues (#2110, #3116, #354) and confirmed by current cert-manager Helm installation docs.

---

### Pitfall 3: Traefik Global HTTP-to-HTTPS Redirect Blocks HTTP-01 ACME Challenge

**What goes wrong:** The app Ingress or the Traefik entrypoint is configured to redirect all HTTP traffic to HTTPS. When cert-manager's HTTP-01 solver creates its challenge Ingress, Let's Encrypt's validation server hits `http://<domain>/.well-known/acme-challenge/<token>` over plain HTTP on port 80. Traefik's global redirect intercepts that request and issues a 301 to `https://`. Let's Encrypt follows the redirect but the HTTPS entrypoint either has no valid cert yet (chicken-and-egg) or the redirect itself causes the challenge to fail. The challenge returns `wrong status code '301', expected '200'`. The certificate is never issued.

**Why it happens:** Traefik's `redirectScheme` middleware applied globally via an entrypoint catches all routes including the cert-manager solver ingress. The cert-manager HTTP-01 solver creates a temporary Ingress to serve the challenge token, but that Ingress inherits the global redirect unless specifically excluded.

**Consequences:** Certificate issuance is permanently stuck. The Certificate resource shows `Reason: PresentError` or `Reason: Failed`. No error is obvious at the Ingress level — the app Ingress looks correct. The only recovery is to disable the global redirect, configure Traefik to exclude the ACME path, or switch to DNS-01.

**Prevention:**
- Do not configure Traefik's global HTTP-to-HTTPS redirect until after the first certificate has been successfully issued (bootstrap order matters for the first deployment).
- Use Traefik router priority: the cert-manager HTTP01 solver Ingress needs a higher priority than the redirect middleware router. In Traefik, this means adding `traefik.ingress.kubernetes.io/router.priority: "100"` (or a sufficiently high value) to the ClusterIssuer's `ingressTemplate.metadata.annotations`.
- The recommended long-term approach: apply `redirectScheme` middleware only to specific app Ingress routes, not via the entrypoint globally. This is simpler to reason about and avoids the ACME conflict entirely.
- Note for K3s: K3s ships Traefik with a pre-configured default TLS entrypoint that may globally redirect HTTP to HTTPS depending on the K3s version and HelmChartConfig settings. Verify the K3s Traefik configuration before assuming port 80 is passthrough.

**Detection:** `kubectl describe challenge <name>` shows `Reason: Failed to present challenge: error...301 or 404 returned`; `curl -v http://<domain>/.well-known/acme-challenge/<any-token>` from outside the cluster returns a 301.

**Confidence:** HIGH — documented in Traefik GitHub issues #7825 and #8786, Traefik community forum, corroborated by cert-manager troubleshooting docs.

---

### Pitfall 4: cert-manager HTTP-01 Solver Ingress Uses Wrong `ingressClassName` and Traefik Ignores It

**What goes wrong:** The ClusterIssuer's HTTP01 solver block does not specify `ingressClassName: traefik` (or specifies it via the old `class` annotation style). cert-manager creates the challenge Ingress without a recognized ingressClassName. Traefik ignores the challenge Ingress (it does not manage Ingresses with no class or a class it doesn't own). The challenge solver pod is running but unreachable. The challenge times out.

**Why it happens:** Two distinct issues compound:
1. Since cert-manager v1.5.4, the solver stopped setting `kubernetes.io/ingress.class` annotations and switched to the `spec.ingressClassName` field. Ingress controllers that watch the old annotation (including some Traefik configurations) may not pick up the challenge Ingress.
2. If no `ingressClassName` is set in the ClusterIssuer solver, cert-manager creates the challenge Ingress with no class — meaning all ingress controllers may try to serve it, or none do.

**Consequences:** HTTP-01 challenges silently fail. Certificate status shows `Reason: PresentError`. The solver pod is running and healthy but unreachable because no ingress controller routes to it.

**Prevention:**
- Always set `ingressClassName: traefik` explicitly in the ClusterIssuer HTTP01 solver:
  ```yaml
  solvers:
    - http01:
        ingress:
          ingressClassName: traefik
  ```
- Do not use the deprecated `class` field (annotation-based); use `ingressClassName` with the spec field.
- Ensure a Traefik `IngressClass` object named `traefik` exists in the cluster. K3s ships with this by default. If using a custom name, pass `--providers.kubernetesingress.ingressclass=<name>` to the Traefik deployment.
- If using `ingressTemplate` in the ClusterIssuer to add custom annotations, keep it minimal — adding incorrect annotations can cause Traefik to route challenge traffic incorrectly.

**Detection:** `kubectl get ingress -A` shows a challenge Ingress (`cm-acme-http-solver-*`) with no `CLASSNAME` column value; `kubectl describe ingress <challenge-ingress>` shows no controller-side events or address assignment.

**Confidence:** HIGH — official cert-manager ingress class compatibility documentation, cert-manager issue #4537.

---

### Pitfall 5: cert-manager Official Docs Warn Against Subchart Embedding — But the Pattern Is Used Anyway

**What goes wrong:** cert-manager manages cluster-scoped resources: `ClusterIssuer`, `ClusterRole`, `ClusterRoleBinding`, `ValidatingWebhookConfiguration`, and `MutatingWebhookConfiguration`. When cert-manager is installed as a subchart, and the same chart is installed multiple times in the same cluster (e.g., staging and production environments sharing a cluster), the second install either fails because the cluster-scoped resources already exist, or it installs a second cert-manager that fights the first over webhook ownership.

**Why it happens:** Helm's subchart model was designed for namespace-scoped resources. Cluster-scoped resources owned by a Helm release are tied to that release name, meaning two releases will each try to own the same webhook configurations.

**Consequences:** Second `helm install` fails with "already exists" on webhook resources; or both cert-manager instances run and their webhooks conflict, causing admission errors across the cluster.

**Prevention:**
- The cert-manager official documentation states: "Be sure never to embed cert-manager as a sub-chart of other Helm charts; cert-manager manages non-namespaced resources in your cluster and care must be taken to ensure that it is installed exactly once."
- For this project (single-tenant pottery shop, one deploy per cluster), this is not an active runtime risk. However, document in values.yaml that operators running multiple clay releases in the same cluster must set `cert-manager.enabled: false` after the first install.
- Follow the same `condition: cert-manager.enabled` pattern already used for CNPG in the chart dependencies.
- Never install the clay chart twice in the same cluster without setting `cert-manager.enabled: false` on subsequent installs.

**Detection:** `helm install` second release fails with "already exists" on ValidatingWebhookConfiguration or ClusterRole resources.

**Confidence:** HIGH — official cert-manager Helm documentation contains this explicit warning.

---

## Moderate Pitfalls

---

### Pitfall 6: Using Production Let's Encrypt During Development Hits Rate Limits

**What goes wrong:** The developer uses the production Let's Encrypt ACME endpoint (`https://acme-v02.api.letsencrypt.org/directory`) during chart development and iterates on the configuration. Each failed challenge attempt counts against rate limits. The production hard limit is 5 failed validations per hostname per hour and 50 certificates per registered domain per week. After hitting the limit, no new certificates can be issued for that domain for up to a week.

**Why it happens:** The difference between staging and production issuers is a single URL in the ClusterIssuer spec. It is easy to copy a production example from documentation and not swap to staging.

**Consequences:** Certificate issuance is blocked for 7 days for the target domain. There is no way to reset this from the cert-manager or Kubernetes side.

**Prevention:**
- Define two ClusterIssuers in the chart: one for staging (`letsencrypt-staging`) and one for production (`letsencrypt-production`). Default the values to use staging.
- Or provide a `tls.acme.server` values key that defaults to the staging URL with a comment explaining how to switch to production once the setup is confirmed working.
- Let's Encrypt staging URL: `https://acme-staging-v02.api.letsencrypt.org/directory`. Staging certs are not browser-trusted but are functionally equivalent for testing the issuance pipeline.
- All CI values files must use the staging ACME URL.

**Detection:** `kubectl describe order <name>` or `kubectl describe certificaterequest <name>` shows `urn:ietf:params:acme:error:rateLimited`.

**Confidence:** HIGH — Let's Encrypt rate limits documentation, cert-manager issue #3267.

---

### Pitfall 7: `acme.cert-manager.io/http01-edit-in-place` Missing on K3s/Traefik (Single LoadBalancer IP)

**What goes wrong:** When using Traefik (especially the K3s bundled version with a single shared LoadBalancer IP), cert-manager's default behavior creates a new Ingress resource for each HTTP-01 challenge. If Traefik assigns each Ingress to the same IP (which it does), the DNS A record already points to the right IP and this is fine. However, if the cert-manager solver creates a new Ingress that is delayed in getting an address assigned (because the controller is busy or the Ingress is not processed immediately), the challenge validation runs before the solver is reachable. The challenge returns a 404 or timeout.

More critically: some Traefik configurations create a new entrypoint per Ingress, resulting in the solver Ingress getting no external IP at all if the IngressClass configuration does not map correctly.

**Why it happens:** cert-manager's default HTTP-01 solver creates a separate Ingress resource per challenge rather than modifying the existing app Ingress. On ingress controllers that share a single LoadBalancer IP across all Ingresses (K3s Traefik), this is usually fine — but timing and routing edge cases exist.

**Prevention:**
- Add the annotation `acme.cert-manager.io/http01-edit-in-place: "true"` to the app's Ingress resource. This instructs cert-manager to modify the existing app Ingress (adding the challenge path as an additional rule) rather than creating a new one. The challenge path is served through the same LoadBalancer IP and same Traefik router as the app.
- This is the safest pattern for K3s + Traefik regardless of whether the new-Ingress approach would have worked.

**Detection:** `kubectl get ingress -A` shows a new `cm-acme-http-solver-*` Ingress with no IP assigned or a different IP than the app Ingress.

**Confidence:** MEDIUM — documented in cert-manager HTTP01 docs; K3s+Traefik single-IP behavior is community-confirmed but infrastructure-dependent.

---

### Pitfall 8: TLS Secret Name Mismatch Between Ingress, Certificate, and cert-manager Output

**What goes wrong:** Three places must reference the same secret name: (1) the app Ingress `tls[].secretName`, (2) the Certificate `spec.secretName`, and (3) the secret that cert-manager actually creates. If any of these drift — due to Helm template logic producing different values — Traefik serves its default "fake certificate" (a self-signed stub) rather than the issued certificate.

**Why it happens:** Helm templates for Ingress and Certificate are authored separately. If the secret name is hardcoded in one place and derived from a helper in another, a typo or naming inconsistency causes a silent mismatch. Traefik does not error on this — it silently falls back to its default certificate.

**Prevention:**
- Define the TLS secret name in exactly one place: a `helpers.tpl` function (e.g., `clay.tlsSecretName`) derived from `ingress.host` or `fullname`. Reference it in both the Ingress template and the Certificate template.
- In CI `helm template` output, verify that the Ingress `spec.tls[0].secretName` value equals the Certificate `spec.secretName` value using a `yq` or `grep` check.

**Detection:** Browser shows "invalid certificate" with issuer "TRAEFIK DEFAULT CERT" or "Kubernetes Ingress Controller Fake Certificate"; `kubectl get secret <name>` returns NotFound while the Certificate resource shows `Ready: True`.

**Confidence:** HIGH — documented in cert-manager Ingress usage docs and common in Traefik integration reports.

---

### Pitfall 9: `selfsigned` Mode Implemented Directly — No CA Bootstrap Step

**What goes wrong:** When implementing the `selfsigned` TLS mode, the developer creates a `SelfSigned` ClusterIssuer and uses it directly to issue the app's Certificate. The Certificate is issued and shows `Ready: True`, but clients receive an "unknown certificate authority" error because the cert-manager `SelfSigned` issuer signs certificates with their own private key — not with a CA.

**Why it happens:** The name "SelfSigned issuer" implies it produces self-signed certificates suitable for direct use. In cert-manager's model, the `SelfSigned` issuer is designed only for bootstrapping a CA certificate, not for issuing end-entity certificates directly.

**Consequences:** The selfsigned mode appears to work (certificate is issued, pod starts) but all clients fail TLS verification. curl returns `SSL certificate problem: self signed certificate`. The `ca.crt` field in the resulting secret is empty, making it impossible to distribute a trust anchor.

**Prevention:**
- Implement the two-step self-signed pattern:
  1. `SelfSigned` ClusterIssuer issues a CA `Certificate` with `isCA: true`
  2. A `CA` ClusterIssuer backed by that CA Certificate issues the app's `Certificate`
- The CA certificate's `ca.crt` key from the CA secret can be distributed to clients or added to the cluster's trusted CA bundle.
- Template the two-step pattern behind `{{- if eq .Values.tls.mode "selfsigned" }}` guards.

**Detection:** `openssl s_client -connect <host>:443` shows `verify error:num=18:self signed certificate`; `kubectl get secret <tls-secret> -o jsonpath='{.data.ca\.crt}'` returns empty.

**Confidence:** HIGH — cert-manager SelfSigned issuer documentation explicitly states it is for CA bootstrapping only.

---

### Pitfall 10: `ingress.className` Currently Empty in values.yaml — Traefik May Still Work But Is Fragile

**What goes wrong:** The existing clay chart has `ingress.className: ""` (empty) in values.yaml. The Ingress template only emits the `ingressClassName` spec field if the value is non-empty. In a K3s cluster where Traefik is set as the default IngressClass, this works by accident. In any cluster without a configured default IngressClass, the app Ingress is silently ignored. More importantly, when cert-manager creates the HTTP-01 solver Ingress with `ingressClassName: traefik`, the mismatch between the app Ingress (no class) and the solver Ingress (explicit class) can cause routing confusion.

**Why it happens:** The existing guard `{{- if .Values.ingress.className }}` was correct for the original chart, but adding Traefik-specific TLS requires making the class explicit.

**Prevention:**
- Add `ingress.className: traefik` to the default values.yaml when shipping the TLS milestone.
- The Ingress template already handles this correctly — it just needs a non-empty value.
- All three CI values files (tls-letsencrypt.yaml, tls-selfsigned.yaml, tls-custom.yaml) must explicitly set `ingress.className: traefik`.
- Document that the value must match whatever IngressClass name Traefik is configured with on the target cluster.

**Detection:** `kubectl get ingress <name>` shows `CLASSNAME` column empty; Traefik routing works by default IngressClass accident but breaks on clusters with multiple controllers.

**Confidence:** HIGH — Traefik ingressClass documentation and current chart inspection.

---

## Minor Pitfalls

---

### Pitfall 11: `custom` TLS Mode — Referenced Secret Must Pre-Exist Before Helm Install

**What goes wrong:** The `custom` (BYO cert) TLS mode requires an existing Kubernetes Secret containing the certificate. If the Ingress references a secret that does not exist at install time, Traefik silently serves its default certificate. The app deploys successfully and all Helm checks pass, but TLS uses the wrong certificate.

**Prevention:**
- Document clearly in values.yaml that `tls.mode: custom` requires the operator to create the TLS secret *before* running `helm install`.
- Consider a Helm pre-install hook Job that runs `kubectl get secret <secretName>` and fails fast (exit 1) if the secret is missing, making the error explicit rather than silent.

**Confidence:** HIGH — standard Kubernetes Ingress TLS behavior.

---

### Pitfall 12: Port 80 Must Be Externally Reachable for HTTP-01 — Not Just Port 443

**What goes wrong:** The cluster's LoadBalancer or external firewall exposes only port 443. Let's Encrypt HTTP-01 always validates over plain HTTP on port 80 regardless of whether the final app uses port 443. If port 80 is not reachable from the public internet, HTTP-01 permanently fails.

**Prevention:**
- Ensure the Traefik Service exposes both port 80 (web entrypoint) and port 443 (websecure entrypoint).
- Verify with `curl http://<domain>/` from outside the cluster returns any response (even 404) before attempting certificate issuance.
- If port 80 cannot be opened (firewall constraints), DNS-01 is the only viable ACME challenge type — which is out of scope for this milestone.

**Confidence:** HIGH — Let's Encrypt challenge types documentation, cert-manager HTTP01 docs.

---

### Pitfall 13: cert-manager CRD Version Drift on `helm upgrade`

**What goes wrong:** cert-manager CRDs are installed by `helm install` but never updated by `helm upgrade` (Helm 3's `crds/` directory behavior: install only, never upgrade). If a future cert-manager version changes the CRD schema, `helm upgrade` silently leaves the old CRD in place. The new cert-manager controller may behave incorrectly against the old CRD schema.

**Prevention:**
- Use `crds.keep: true` (the default in recent cert-manager versions) to prevent CRD deletion on `helm uninstall`.
- Document that CRD upgrades require a manual `kubectl apply -f` of the new CRD manifests when upgrading the cert-manager version in Chart.yaml. This is the same limitation already documented for the CNPG subchart.

**Confidence:** MEDIUM — Helm 3 CRD upgrade limitation is documented behavior.

---

## Phase-Specific Warnings

| Phase Topic | Likely Pitfall | Mitigation |
|-------------|----------------|------------|
| cert-manager subchart dependency addition | Webhook not ready when ClusterIssuer applied (Pitfall 1) | Post-install hook Job with kubectl wait at hook-weight 0; ClusterIssuer at hook-weight 5 |
| ClusterIssuer template in parent chart | Applied before webhook ready (Pitfall 1) | Helm hook annotation on ClusterIssuer + Certificate templates |
| CI helm template validation — all three TLS modes | "no matches for kind Certificate" CRD error (Pitfall 2) | Use `--include-crds` + kubeconform with CRD catalog schemas |
| First certificate issuance testing | Rate limiting production endpoint (Pitfall 6) | Default to staging ACME URL; production opt-in via values override |
| HTTP-01 ACME challenge routing on K3s/Traefik | Global redirect blocks challenge path (Pitfall 3) | Scope redirect to app routes; not entrypoint-globally |
| HTTP-01 solver ingress class | Traefik ignores solver Ingress (Pitfall 4) | Set `ingressClassName: traefik` in ClusterIssuer solver spec |
| K3s + single LoadBalancer IP | Solver Ingress timing/routing issues (Pitfall 7) | Add `acme.cert-manager.io/http01-edit-in-place: "true"` to app Ingress |
| TLS secret name wiring across templates | Mismatch causes silent Traefik fallback cert (Pitfall 8) | Single `helpers.tpl` function for TLS secret name; CI grep check |
| `selfsigned` TLS mode implementation | Direct SelfSigned issuer produces untrusted cert (Pitfall 9) | Two-step: SelfSigned→CA cert→CA issuer→app cert |
| `custom` TLS mode | Secret must pre-exist; silently serves wrong cert if missing (Pitfall 11) | Document pre-condition; optional pre-install assertion hook |
| Multi-cluster or multi-release deployments | cert-manager cluster-scoped resource conflicts (Pitfall 5) | `condition: cert-manager.enabled`; document single-install-per-cluster |
| Future helm upgrade | CRD version drift (Pitfall 13) | Runbook entry: CRD upgrades require kubectl apply; mirrors CNPG pattern |

---

## Sources

- [cert-manager Helm Installation Docs](https://cert-manager.io/docs/installation/helm/) — CRD installation options, subchart warning, namespace issues (HIGH confidence)
- [cert-manager HTTP01 Solver Docs](https://cert-manager.io/docs/configuration/acme/http01/) — ingressClassName options, solver Ingress behavior, default class risk (HIGH confidence)
- [cert-manager ACME Troubleshooting Docs](https://cert-manager.io/docs/troubleshooting/acme/) — HTTP-01 failure modes, `http01-edit-in-place`, debugging steps (HIGH confidence)
- [cert-manager Ingress Class Compatibility Breaking Change](https://cert-manager.io/docs/releases/upgrading/ingress-class-compatibility/) — v1.5.4 annotation vs spec field change, Traefik-specific fix (HIGH confidence)
- [cert-manager SelfSigned Issuer Docs](https://cert-manager.io/docs/configuration/selfsigned/) — CA bootstrapping pattern; not for end-entity certs (HIGH confidence)
- [Using cert-manager as a subchart — Skarlso (2024)](https://skarlso.github.io/2024/07/02/using-cert-manager-as-a-subchart-with-helm/) — webhook readiness timing, post-install hook pattern with kubectl wait (MEDIUM confidence)
- [cert-manager GitHub Issue #6179 — CRDs shouldn't be templated in Helm](https://github.com/cert-manager/cert-manager/issues/6179) — CRD directory vs template design decision (HIGH confidence)
- [cert-manager GitHub Issue #2110 — no matches for kind Certificate](https://github.com/cert-manager/cert-manager/issues/2110) — CI rendering failure (HIGH confidence)
- [cert-manager GitHub Issue #4537 — HTTP-01 regression Traefik/Istio v1.5](https://github.com/cert-manager/cert-manager/issues/4537) — ingressClassName breaking change (HIGH confidence)
- [Traefik Community Forum — HTTP to HTTPS blocks cert-manager HTTP challenge](https://community.traefik.io/t/globally-enabled-http-to-https-blocks-cert-manager-http-challenge/20047) — global redirect conflict, router priority solution (MEDIUM confidence)
- [Traefik GitHub Issue #7825 — HTTPS redirection affects ACME HTTP challenge](https://github.com/traefik/traefik/issues/7825) — root cause of redirect-blocks-challenge (HIGH confidence)
- [Let's Encrypt Rate Limits](https://letsencrypt.org/docs/rate-limits/) — production certificate limits (HIGH confidence)
- [Let's Encrypt Staging Environment](https://letsencrypt.org/docs/staging-environment/) — staging ACME URL for safe testing (HIGH confidence)
- [cert-manager Annotated Ingress resource Docs](https://cert-manager.io/docs/usage/ingress/) — annotation-driven Certificate creation, common misconfigurations (HIGH confidence)
