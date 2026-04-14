---
gsd_state_version: 1.0
milestone: v1.1
milestone_name: TLS
status: complete
stopped_at: v1.1 milestone archived
last_updated: "2026-04-14T23:44:28.120Z"
last_activity: 2026-04-14
progress:
  total_phases: 6
  completed_phases: 6
  total_plans: 11
  completed_plans: 11
  percent: 100
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-04-14)

**Core value:** The app runs reliably on Postgres with zero SQLite anywhere — CNPG manages the in-cluster database lifecycle, and the Go binary is a pure CGO-free build.
**Current focus:** Milestone v1.1 complete — run `/gsd-new-milestone` to start next milestone

## Current Position

Phase: —
Plan: —
Status: v1.1 TLS milestone complete and archived
Last activity: 2026-04-14

Progress: [██████████] 100%

## Performance Metrics

**Velocity:**

- Total plans completed: 11 (v1.1 milestone)
- Average duration: ~12 min/plan
- Total execution time: ~2 days

*Updated after each plan completion*

## Accumulated Context

### Decisions

Decisions are logged in PROJECT.md Key Decisions table.

v1.1 TLS decisions:

- cert-manager as pre-install step, not subchart — official docs prohibit subchart embedding for cluster-scoped operators; mirrors CNPG pattern
- Staging ACME endpoint default — prevents burning Let's Encrypt production rate limits during dev/testing
- selfsigned mode uses two-step CA bootstrap — SelfSigned ClusterIssuer issues CA cert; CA ClusterIssuer issues app cert (avoids untrusted end-entity cert)
- `clay.tlsSecretName` defined once in _helpers.tpl — prevents Ingress/Certificate secret name mismatch
- `helm template` without `--validate` in CI — avoids cert-manager CRD absence failures in CI environment
- Hook-weight sequencing for selfsigned: root(-10) → ca-cert(-5) → ca-issuer(0) → app-cert(5)

### Pending Todos

None.

### Blockers/Concerns

None — milestone complete.

## Session Continuity

Last session: 2026-04-14
Stopped at: v1.1 milestone archived
Resume: `/gsd-new-milestone` to start next milestone
