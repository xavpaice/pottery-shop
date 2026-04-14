---
phase: 03-values-and-ingress-refactor
fixed_at: 2026-04-14T00:00:00Z
review_path: .planning/phases/03-values-and-ingress-refactor/03-REVIEW.md
iteration: 1
findings_in_scope: 4
fixed: 4
skipped: 0
status: all_fixed
---

# Phase 03: Code Review Fix Report

**Fixed at:** 2026-04-14
**Source review:** .planning/phases/03-values-and-ingress-refactor/03-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 4
- Fixed: 4
- Skipped: 0

## Fixed Issues

### CR-01: Personal email address hardcoded in default values

**Files modified:** `chart/clay/values.yaml`
**Commit:** 42eb869
**Applied fix:** Replaced `ORDER_EMAIL: "xavpaice@gmail.com"` with `ORDER_EMAIL: ""` and added a comment marking it as required, consistent with how `ADMIN_PASS` and `SESSION_SECRET` are documented.

### WR-01: Traefik annotations rendered unconditionally for all ingressClassName values

**Files modified:** `chart/clay/templates/ingress.yaml`
**Commit:** 9a2a001
**Applied fix:** Wrapped `traefik.ingress.kubernetes.io/router.entrypoints` in `{{- if eq .Values.ingress.className "traefik" }}` and `acme.cert-manager.io/http01-edit-in-place` in `{{- if eq .Values.ingress.tls.mode "letsencrypt" }}`, so each annotation only renders when its relevant controller/mode is active.

### WR-02: Secret validation bypassed by whitespace-only values

**Files modified:** `chart/clay/templates/_helpers.tpl`
**Commit:** ba0da26
**Applied fix:** Added `| trim` pipe before the `not` check for `ADMIN_PASS` and `SESSION_SECRET` in `clay.validateSecrets`, and for `ingress.host` and `ingress.tls.mode` in `clay.validateIngress`. Error messages updated to indicate the value must not be blank.

### WR-03: Secret validation not called from secret.yaml — partial-render bypass

**Files modified:** `chart/clay/templates/secret.yaml`
**Commit:** eb76820
**Applied fix:** Added `{{- include "clay.validateSecrets" . }}` as the first line of `secret.yaml`, matching the pattern already used in `deployment.yaml`. This ensures validation fires even when `secret.yaml` is rendered in isolation (e.g. `helm template --show-only` or ArgoCD selective sync).

---

_Fixed: 2026-04-14_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
