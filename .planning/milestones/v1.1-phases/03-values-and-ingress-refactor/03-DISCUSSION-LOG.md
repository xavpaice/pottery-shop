# Phase 3: Values and Ingress Refactor - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions captured in CONTEXT.md — this log preserves the discussion.

**Date:** 2026-04-14
**Phase:** 03-values-and-ingress-refactor
**Mode:** discuss
**Areas discussed:** Backward compat strategy, ingressClassName source, Default TLS mode

## Gray Areas Presented

| # | Area | Options offered | User chose |
|---|------|-----------------|------------|
| 1 | Backward compat strategy | Hard break / Deprecation comments / Dual-path support | Hard break |
| 2 | ingressClassName source | Configurable (traefik default) / Hardcoded in template | Configurable, traefik default |
| 3 | Default TLS mode | Fail at render / Default selfsigned / Default no-TLS | No default — fail at render |

## Discussion Detail

### Backward Compat Strategy
- **Options presented:** (a) Hard break — remove old keys entirely; (b) Deprecation comments — keep commented-out old keys with migration note; (c) Dual-path support — template handles both old and new shapes
- **User chose:** Hard break (recommended option)
- **Captured as:** D-01, D-02 — remove arrays, add comment block documenting migration shape

### ingressClassName Source
- **Options presented:** (a) Configurable `ingress.className: traefik` in values.yaml (recommended); (b) Hardcoded `traefik` in template
- **User chose:** Configurable, traefik default (recommended option)
- **Captured as:** D-05 — `ingress.className: traefik` in values.yaml, template conditionally renders only when non-empty

### Default TLS Mode
- **Options presented:** (a) No default — fail at render with clear error (recommended); (b) Default to selfsigned; (c) Default to no-TLS (plain HTTP)
- **User chose:** No default — fail at render (recommended option)
- **Captured as:** D-11 — `clay.validateIngress` always fails when `tls.mode` is empty/unset

## Corrections

No corrections — all recommended options confirmed.
