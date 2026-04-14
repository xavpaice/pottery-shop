---
status: partial
phase: 03-values-and-ingress-refactor
source: [03-01-SUMMARY.md]
started: 2026-04-14T05:30:00Z
updated: 2026-04-14T05:35:00Z
---

## Current Test

[testing paused — 1 item outstanding]

## Tests

### 1. Custom TLS mode renders correctly
expected: |
  helm template with ingress.enabled=true, host, tls.mode=custom, tls.secretName=my-tls renders
  an Ingress with ingressClassName: traefik, traefik annotation, and spec.tls[0].secretName: my-tls.
  No ClusterIssuer or Certificate appears.
result: pass
note: static — ingressClassName from .Values.ingress.className, Traefik annotation gated on className==traefik, clay.tlsSecretName used for secretName (ingress.yaml:10-18,32)

### 2. Missing host fails at render time
expected: |
  helm template with ingress.enabled=true but no host set fails with "ingress.host must be set".
result: pass
note: static — fail "ingress.host must be set (value must not be blank)" at _helpers.tpl:69

### 3. Missing acme.email fails for letsencrypt mode
expected: |
  helm template with tls.mode=letsencrypt but no acme.email fails with error about ingress.tls.acme.email.
result: pass
note: static — fail "ingress.tls.acme.email required for letsencrypt mode" at _helpers.tpl:79

### 4. No nginx annotations in default output
expected: |
  No nginx.ingress.kubernetes.io keys in live config or rendered output. proxy-body-size absent.
result: pass
note: static — nginx only appears inside # MIGRATION NOTE comment block in values.yaml:27; zero live config lines

### 5. helm lint passes with CI values files
expected: |
  helm lint chart/clay -f chart/clay/ci/managed-values.yaml exits 0, 0 chart(s) failed.
  helm lint chart/clay -f chart/clay/ci/external-values.yaml exits 0, 0 chart(s) failed.
result: blocked
blocked_by: other
reason: "helm not installed in this environment — needs local helm run by user"

## Summary

total: 5
passed: 4
issues: 0
pending: 0
skipped: 0
blocked: 1

## Gaps

[none yet]
