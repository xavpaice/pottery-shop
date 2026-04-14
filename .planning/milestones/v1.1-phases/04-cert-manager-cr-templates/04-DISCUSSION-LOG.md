# Phase 4: cert-manager CR Templates - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions captured in CONTEXT.md — this log preserves the discussion.

**Date:** 2026-04-14
**Phase:** 04-cert-manager-cr-templates
**Mode:** discuss
**Areas analyzed:** Template file layout, ClusterIssuer naming, Integration test TLS scope, Hook lifecycle / cleanup

## Gray Areas Presented

| Area | Selected by user |
|------|-----------------|
| Template file layout | Yes |
| ClusterIssuer naming | Yes |
| Integration test TLS scope | Yes |
| Hook lifecycle / cleanup | Yes |

## Decisions Made

### Template file layout
- **Chose:** Two files by mode — `cert-manager-letsencrypt.yaml` + `cert-manager-selfsigned.yaml`
- **Rationale:** Each file self-contained; opening it shows every resource for that mode. Easier to add/remove modes vs. interleaved conditionals.

### ClusterIssuer naming
- **Chose:** Release-derived using `clay.fullname` (e.g., `clay-letsencrypt`)
- **Rationale:** ClusterIssuers are cluster-scoped — fixed names would conflict if two clay releases exist on the same cluster. Release-derived names are safe and consistent with existing chart patterns.

### Integration test TLS scope
- **Chose:** Pre-install only — add cert-manager install step, keep `ingress.enabled=false`
- **Rationale:** TLS mode validation is handled by `helm template` in Phase 5. Integration tests don't need a real domain/DNS — pre-install is sufficient for CI-06.

### Hook lifecycle / cleanup
- **Chose:** `helm.sh/hook-delete-policy: before-hook-creation` on all cert-manager CRs
- **Rationale:** Prevents "already exists" errors on `helm upgrade` or reinstall. Standard pattern for stateless hook resources.

## Corrections Made

No corrections — all recommended options accepted.
