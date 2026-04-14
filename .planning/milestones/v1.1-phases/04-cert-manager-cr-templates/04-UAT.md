---
status: diagnosed
phase: 04-cert-manager-cr-templates
source: [04-01-SUMMARY.md, 04-02-SUMMARY.md]
started: 2026-04-14T00:00:00Z
updated: 2026-04-14T12:00:00Z
---

## Current Test

[testing complete]

## Tests

### 1. Template Test Suite Passes
expected: Run `chart/tests/helm-template-test.sh` from the repo root. All 23 assertions pass — G-01 through G-14 (including the 6 new TLS tests). Script exits 0 with a "Tests passed" or equivalent summary line. No assertion failures printed.
result: pass

### 2. letsencrypt Mode Renders ClusterIssuer + Certificate
expected: Run `helm template clay chart/clay --set ingress.enabled=true --set ingress.tls.mode=letsencrypt --set ingress.host=example.com`. Output contains exactly 1 ClusterIssuer and 1 Certificate resource (top-level `kind:` declarations). The Certificate references the ACME ClusterIssuer. The Ingress has a `cert-manager.io/cluster-issuer` annotation.
result: pass

### 3. selfsigned Mode Renders 4-Resource CA Bootstrap
expected: Run `helm template clay chart/clay --set ingress.enabled=true --set ingress.tls.mode=selfsigned --set ingress.host=example.com`. Output contains exactly 2 ClusterIssuers and 2 Certificates (4 total cert-manager resources). No ACME/letsencrypt resources appear. Resources include a SelfSigned root issuer, a CA certificate (isCA: true), a CA issuer, and an app certificate.
result: pass

### 4. custom Mode Renders No cert-manager Resources
expected: Run `helm template clay chart/clay --set ingress.enabled=true --set ingress.tls.mode=custom --set ingress.host=example.com`. Output contains zero ClusterIssuer or Certificate resources — no cert-manager CRs rendered in custom mode.
result: pass

### 5. Ingress TLS Block Guarded by Mode
expected: Run `helm template clay chart/clay --set ingress.enabled=true` (no `tls.mode` set). The rendered Ingress has no `tls:` section — the block is gated on `.Values.ingress.tls.mode` being non-empty (WR-02 fix).
result: issue
reported: "Error: execution error at (clay/templates/ingress.yaml:2:4): ingress.tls.mode must be set (letsencrypt|selfsigned|custom)"
severity: major

### 6. CI Workflow Has cert-manager Pre-Install Step
expected: Open `.github/workflows/integration-test.yml`. The file contains two cert-manager steps (helm repo add jetstack + helm install cert-manager) positioned between the CNPG install step and the clay chart install step. The install uses cert-manager v1.20.2, `crds.enabled=true`, a dedicated namespace, `--wait --timeout 3m`.
result: pass

## Summary

total: 6
passed: 5
issues: 1
pending: 0
skipped: 0
blocked: 0

## Gaps

- truth: "Ingress renders without a tls: section when ingress.tls.mode is unset"
  status: failed
  reason: "User reported: Error: execution error at (clay/templates/ingress.yaml:2:4): ingress.tls.mode must be set (letsencrypt|selfsigned|custom)"
  severity: major
  test: 5
  root_cause: "clay.validateIngress in _helpers.tpl unconditionally requires ingress.tls.mode to be non-empty whenever ingress is enabled, but the TLS block in ingress.yaml was updated (WR-02) to make TLS optional via an if-guard. The validation was not updated to match."
  artifacts:
    - path: "chart/clay/templates/_helpers.tpl:71-73"
      issue: "unconditional fail if tls.mode is blank"
    - path: "chart/clay/templates/ingress.yaml:2"
      issue: "calls validateIngress unconditionally when ingress.enabled=true"
    - path: "chart/clay/values.yaml:46"
      issue: "tls.mode defaults to empty string"
  missing:
    - "Make tls.mode validation conditional — only enforce when a mode is actually set, or remove it and let the if-guard in ingress.yaml be sufficient"
  debug_session: ""
