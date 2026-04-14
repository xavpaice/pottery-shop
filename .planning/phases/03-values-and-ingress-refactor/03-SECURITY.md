---
phase: 03
slug: values-and-ingress-refactor
status: verified
threats_open: 0
asvs_level: L1
created: 2026-04-14
---

# Phase 03 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| Operator → Helm values | Untrusted input: values.yaml overrides via --set or -f at deploy time | TLS mode, host, secretName, ACME email (no secret data) |
| Chart → Kubernetes API | Rendered YAML submitted to API server; malformed resources rejected | Ingress resource YAML |

---

## Threat Register

| Threat ID | Category | Component | Disposition | Mitigation | Status |
|-----------|----------|-----------|-------------|------------|--------|
| T-03-01 | Elevation of Privilege | `clay.validateIngress` — custom mode empty secretName | mitigate | `_helpers.tpl` lines 82-86: fails with "ingress.tls.secretName required for custom mode" when `tls.mode=custom` and `secretName` is empty | closed |
| T-03-02 | Denial of Service | `clay.validateIngress` — letsencrypt empty email | mitigate | `_helpers.tpl` lines 77-81: fails with "ingress.tls.acme.email required for letsencrypt mode" when `tls.mode=letsencrypt` and `acme.email` is empty | closed |
| T-03-03 | Tampering | `clay.validateIngress` — invalid tls.mode value | mitigate | `_helpers.tpl` lines 74-76: three-way `ne` enum guard fails with "ingress.tls.mode must be one of: letsencrypt, selfsigned, custom" on unrecognised value | closed |
| T-03-04 | Tampering | `values.yaml` ingress block + `ingress.yaml` template | mitigate | `values.yaml` live ingress block has no `annotations:` key, no `nginx.ingress.kubernetes.io` string. Both Traefik annotations are hardcoded string literals in `ingress.yaml` — no operator injection vector | closed |
| T-03-05 | Information Disclosure | `values.yaml` — no secret data in values | accept | See Accepted Risks Log | closed |

*Status: open · closed*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-03-01 | T-03-05 | Phase 03 only references TLS secret names (not contents). TLS secrets are managed externally (custom mode) or by cert-manager (Phase 04). No secret data flows through values.yaml at any point in this phase. | Plan author | 2026-04-14 |

*Accepted risks do not resurface in future audit runs.*

---

## Auditor Notes

**T-03-04 — annotation conditionality observation (non-blocking):**
The Traefik annotations are hardcoded string literals (no injection vector), but their rendering is mode-conditional: the `websecure` entrypoint annotation renders when `className` is non-empty, and the ACME annotation is present unconditionally. This is reasonable operational behaviour — operators who override `className` to a non-Traefik value receive the Traefik annotations anyway (harmless for non-Traefik controllers, which ignore unknown annotations). The tamper-resistance property the threat was designed to protect is intact.

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-04-14 | 5 | 5 | 0 | gsd-security-auditor |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-04-14
