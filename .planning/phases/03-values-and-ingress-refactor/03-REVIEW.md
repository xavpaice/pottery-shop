---
phase: 03-values-and-ingress-refactor
reviewed: 2026-04-14T00:00:00Z
depth: standard
files_reviewed: 3
files_reviewed_list:
  - chart/clay/values.yaml
  - chart/clay/templates/_helpers.tpl
  - chart/clay/templates/ingress.yaml
findings:
  critical: 1
  warning: 3
  info: 2
  total: 6
status: issues_found
---

# Phase 03: Code Review Report

**Reviewed:** 2026-04-14
**Depth:** standard
**Files Reviewed:** 3
**Status:** issues_found

## Summary

Three Helm chart files were reviewed: the values defaults, the template helpers, and the ingress template. The overall design is sound — the mode-driven TLS abstraction, fail-fast validation helpers, and migration notes are well-executed. One critical issue and three warnings were found.

The critical issue is a real personal email address committed as a default value. Warnings center on: unconditional Traefik-specific annotations that break non-Traefik controllers silently, an empty-string whitespace bypass in secret validation, and validation placement in deployment.yaml rather than secret.yaml allowing partial-render bypass.

---

## Critical Issues

### CR-01: Personal email address hardcoded in default values

**File:** `chart/clay/values.yaml:60`
**Issue:** `ORDER_EMAIL: "xavpaice@gmail.com"` commits a real personal email address into the repository as a shipped default. Every install that does not explicitly override this value will silently route order emails to that address. Even if intended as a placeholder, it will be indexed by public repo searches.
**Fix:** Replace with an empty string and document it as required, consistent with how `ADMIN_PASS` and `SESSION_SECRET` are handled. If the deployment truly requires a default, use a clearly fake placeholder such as `orders@example.com`.

```yaml
# REQUIRED — recipient address for order notification emails
ORDER_EMAIL: ""
```

Optionally, add a validation check in `_helpers.tpl` analogous to `clay.validateSecrets`:

```
{{- if not .Values.config.ORDER_EMAIL }}
  {{- fail "config.ORDER_EMAIL must be set" }}
{{- end }}
```

---

## Warnings

### WR-01: Traefik annotations rendered unconditionally for all ingressClassName values

**File:** `chart/clay/templates/ingress.yaml:10-11`
**Issue:** The annotations block is hardcoded without any guard:

```yaml
annotations:
  traefik.ingress.kubernetes.io/router.entrypoints: websecure
  acme.cert-manager.io/http01-edit-in-place: "true"
```

`values.yaml` line 43 explicitly documents that users can set `className: ""` (or another value) for non-Traefik controllers. When a user does this, both annotations still render. `traefik.ingress.kubernetes.io/router.entrypoints` will be silently ignored by nginx/ingress-nginx and similar controllers, meaning the router entrypoint constraint is never applied — a silent misconfiguration. `acme.cert-manager.io/http01-edit-in-place` is similarly only relevant in ACME/letsencrypt mode.

**Fix:** Gate the Traefik annotation on `className` being `"traefik"`, and gate the cert-manager annotation on TLS mode being `"letsencrypt"`:

```yaml
  annotations:
    {{- if eq .Values.ingress.className "traefik" }}
    traefik.ingress.kubernetes.io/router.entrypoints: websecure
    {{- end }}
    {{- if eq .Values.ingress.tls.mode "letsencrypt" }}
    acme.cert-manager.io/http01-edit-in-place: "true"
    {{- end }}
```

### WR-02: Secret validation bypassed by whitespace-only values

**File:** `chart/clay/templates/_helpers.tpl:55-60`
**Issue:** The `clay.validateSecrets` helper uses `if not .Values.secrets.ADMIN_PASS`. In Go/Sprig templates, `not` returns `true` for empty string `""` (triggering fail) but returns `false` for any non-empty string — including `" "` (a single space) or a tab character. A user who accidentally sets `ADMIN_PASS: " "` will pass validation but the running app will accept a single-space password or session secret.

```yaml
{{- if not .Values.secrets.ADMIN_PASS }}
  {{- fail "secrets.ADMIN_PASS must be set" }}
{{- end }}
```

**Fix:** Trim whitespace before the check using Sprig's `trim` function:

```
{{- if not (.Values.secrets.ADMIN_PASS | trim) }}
  {{- fail "secrets.ADMIN_PASS must be set (value must not be blank)" }}
{{- end }}
{{- if not (.Values.secrets.SESSION_SECRET | trim) }}
  {{- fail "secrets.SESSION_SECRET must be set (value must not be blank)" }}
{{- end }}
```

The same pattern applies to `clay.validateIngress` for `ingress.host` at line 68.

### WR-03: Secret validation not called from secret.yaml — partial-render bypass

**File:** `chart/clay/templates/secret.yaml:1-12`  
**Related:** `chart/clay/templates/_helpers.tpl:54-61`
**Issue:** `clay.validateSecrets` is invoked at the top of `deployment.yaml` (line 1), but not in `secret.yaml`. Helm renders templates individually, and tools like `helm template --show-only templates/secret.yaml` or ArgoCD selective sync can render `secret.yaml` without rendering `deployment.yaml`. In that scenario, a Secret with empty `ADMIN_PASS` or `SESSION_SECRET` is written to the cluster without any validation failure.

**Fix:** Add the validation call at the top of `secret.yaml`:

```yaml
{{- include "clay.validateSecrets" . }}
apiVersion: v1
kind: Secret
...
```

---

## Info

### IN-01: Hardcoded production BASE_URL in default values

**File:** `chart/clay/values.yaml:54`
**Issue:** `BASE_URL: "https://clay.nz"` is a production URL shipped as the default. Any staging, development, or CI install that forgets to override this value will generate links pointing to the production domain. This is unlikely to cause a security incident but can produce confusing behavior in non-production environments.
**Fix:** Default to an empty string or a clearly local placeholder, with a comment indicating it must be overridden per environment:

```yaml
BASE_URL: ""   # REQUIRED — set to the public URL for this deployment (e.g. https://shop.example.com)
```

### IN-02: Image tag "latest" in default values is a mutable reference

**File:** `chart/clay/values.yaml:7`
**Issue:** `tag: latest` is a mutable tag that resolves to a different image digest on every pull, making deployments non-reproducible. The chart already acknowledges this risk in the `waitForPostgres` comment (line 118-119) but applies the same mutable-tag pattern to the app image itself.
**Fix:** The default cannot easily be a real digest without knowing the released version. Document the risk in the comment and add a note in NOTES.txt or the migration guide to pin the tag at deploy time:

```yaml
image:
  repository: ghcr.io/xavpaice/pottery-shop
  tag: latest          # Override with a pinned tag (e.g. v0.2.0) for reproducible deploys
  pullPolicy: IfNotPresent
```

Consider setting `pullPolicy: Always` when `tag: latest` is used, or enforce pinning via a validation helper for production values.

---

_Reviewed: 2026-04-14_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
