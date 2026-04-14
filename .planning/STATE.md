---
gsd_state_version: 1.0
milestone: v1.1
milestone_name: TLS
status: planning
stopped_at: Roadmap created — ready to plan Phase 3
last_updated: "2026-04-14T00:00:00.000Z"
last_activity: 2026-04-14
progress:
  total_phases: 3
  completed_phases: 0
  total_plans: 0
  completed_plans: 0
  percent: 0
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-04-14)

**Core value:** The app runs reliably on Postgres with zero SQLite anywhere — CNPG manages the in-cluster database lifecycle, and the Go binary is a pure CGO-free build.
**Current focus:** Milestone v1.1 — TLS (Phases 3–5)

## Current Position

Phase: Phase 3 — Values and Ingress Refactor (not started)
Plan: —
Status: Roadmap created, ready to plan Phase 3
Last activity: 2026-04-14 — Roadmap created for v1.1 TLS milestone

Progress: [░░░░░░░░░░] 0%

## Performance Metrics

**Velocity:**

- Total plans completed: 5 (prior milestone)
- Average duration: —
- Total execution time: —

*Updated after each plan completion*

## Accumulated Context

### Decisions

Decisions are logged in PROJECT.md Key Decisions table.

Prior milestone decisions:
- pgx over lib/pq — pure Go, no CGO, actively maintained ✓
- CNPG as subchart — one `helm install` gets everything ✓
- External PG via DSN string — simplest interface ✓
- pg_isready init container — timing mitigation for CNPG secret race ✓

v1.1 TLS decisions (from research):
- cert-manager as pre-install step, not subchart — official docs prohibit subchart embedding for cluster-scoped operators; mirrors CNPG pattern
- Staging ACME endpoint default — prevents burning Let's Encrypt production rate limits during dev/testing
- selfsigned mode uses two-step CA bootstrap — SelfSigned ClusterIssuer issues CA cert; CA ClusterIssuer issues app cert (avoids untrusted end-entity cert)
- `clay.tlsSecretName` defined once in _helpers.tpl — prevents Ingress/Certificate secret name mismatch
- `helm template` without `--validate` in CI — avoids cert-manager CRD absence failures in CI environment

### Pending Todos

None.

### Blockers/Concerns

- Chart.yaml current state: research notes CNPG was moved out of the Clay chart. Verify `chart/clay/Chart.yaml` has no `dependencies:` block before Phase 4 to confirm no subchart cleanup is needed.
- K3s Traefik HTTP redirect: whether the current k3s Traefik deployment has a global HTTP-to-HTTPS redirect enabled is infrastructure-dependent. Use `selfsigned` mode for integration tests to sidestep HTTP-01 entirely.

## Session Continuity

Last session: 2026-04-14
Stopped at: Roadmap created for v1.1 TLS milestone (Phases 3–5)
Resume: `/gsd-plan-phase 3`
