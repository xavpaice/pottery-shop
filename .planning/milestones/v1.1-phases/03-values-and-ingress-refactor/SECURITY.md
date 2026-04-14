---
phase: 03-values-and-ingress-refactor
plan: 01
audited: "2026-04-14"
asvs_level: L1
block_on: HIGH
---

# Security Audit — Phase 03-01: Values and Ingress Refactor

## Summary

**Threats Closed:** 5/5
**Threats Open:** 0
**Unregistered Flags:** 0 (SUMMARY.md Threat Flags section: "No new security-relevant surface beyond what the plan's threat model covers.")

## Threat Verification

| Threat ID | Category | Disposition | Status | Evidence |
|-----------|----------|-------------|--------|----------|
| T-03-01 | Elevation of Privilege | mitigate | CLOSED | `_helpers.tpl` lines 82-86: `if eq .Values.ingress.tls.mode "custom"` / `if not .Values.ingress.tls.secretName` / `fail "ingress.tls.secretName required for custom mode"` |
| T-03-02 | Denial of Service | mitigate | CLOSED | `_helpers.tpl` lines 77-81: `if eq .Values.ingress.tls.mode "letsencrypt"` / `if not .Values.ingress.tls.acme.email` / `fail "ingress.tls.acme.email required for letsencrypt mode"` |
| T-03-03 | Tampering | mitigate | CLOSED | `_helpers.tpl` lines 74-76: enum guard `and (ne ... "letsencrypt") (ne ... "selfsigned") (ne ... "custom")` / `fail "ingress.tls.mode must be one of: letsencrypt, selfsigned, custom"` |
| T-03-04 | Tampering | mitigate | CLOSED | `values.yaml` live ingress block (lines 41-50): no `annotations:` key, no `nginx.ingress.kubernetes.io` annotation. `ingress.yaml` lines 10-15: Traefik annotations hardcoded as string literals, not sourced from operator-supplied values map. |
| T-03-05 | Information Disclosure | accept | CLOSED | Accepted risk documented below. |

## Accepted Risks Log

### T-03-05 — Information Disclosure (values.yaml, no secrets in values)

- **Accepted by:** Plan author (03-01-PLAN.md threat model, disposition: accept)
- **Rationale:** This phase only references TLS secret names (e.g., `ingress.tls.secretName`), never secret contents. TLS secrets are managed externally (custom mode) or by cert-manager (Phase 4). No secret data flows through values.yaml in this phase.
- **Residual risk:** Low. An operator supplying a `secretName` value exposes only the Kubernetes Secret resource name, not its contents.
- **Re-evaluate at:** Phase 4 (cert-manager Certificate CR introduction) — confirm no ACME private key material flows through Helm values.

## Unregistered Threat Flags

None. SUMMARY.md `## Threat Flags` section states: "No new security-relevant surface beyond what the plan's threat model covers. All T-03-01 through T-03-04 mitigations are implemented via `clay.validateIngress`."

## Notes

### T-03-04 Annotation Conditionality

`ingress.yaml` renders the Traefik annotations conditionally:
- `traefik.ingress.kubernetes.io/router.entrypoints: websecure` — only when `ingress.className == "traefik"`
- `acme.cert-manager.io/http01-edit-in-place: "true"` — only when `ingress.tls.mode == "letsencrypt"`

The conditions are evaluated against values that are themselves validated by `clay.validateIngress` (mode must be a valid enum value). The annotation strings themselves are hardcoded literals — operators cannot inject arbitrary annotation content via values. The T-03-04 anti-injection property is preserved.
