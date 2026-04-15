# State: Clay.nz Pottery Shop

## Project Reference

See: .planning/PROJECT.md (updated 2026-04-15)

**Core value:** The Helm chart deploys the full stack — app, database, and certificates — in a single `helm install`, with no operator pre-install required.
**Current focus:** Phase 6 — Subchart Dependencies (v1.2 Umbrella Chart)

## Current Position

Phase: 6 of 10 (Subchart Dependencies)
Plan: — (roadmap created, ready to plan)
Status: Ready to plan
Last activity: 2026-04-15 - Completed quick task 260415-ixx: health check endpoints

Progress: [░░░░░░░░░░] 0% (0 of 5 phases)

## Performance Metrics

**Velocity:**
- Total plans completed: 0 (v1.2)
- Average duration: — (no v1.2 plans yet)
- Total execution time: —

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| — | — | — | — |

*Updated after each plan completion*

## Accumulated Context

### From v1.1 (TLS)

- Hook weight sequencing pattern: RBAC at -25, webhook-wait Jobs at -20, CRs at -10 to 5
- cert-manager hook template: every ClusterIssuer/Certificate carries `helm.sh/hook: post-install,post-upgrade` + `helm.sh/hook-delete-policy: before-hook-creation`
- CI behavioral test pattern: `chart/tests/helm-template-test.sh` with named groups (G-01…), `^kind:` anchored greps
- Operator pre-install pattern: `helm repo add + helm install --version pin --namespace --create-namespace --wait`
- Lesson: Close documentation gaps at phase exit, not milestone close

### Pending Decisions (v1.2)

- CNPG Cluster CR → post-install hook (Phase 8): means `helm uninstall` won't auto-delete the Cluster — operator must clean it up manually
- `bitnami/kubectl` for webhook-wait Jobs: needs version pinning; air-gapped envs flagged as post-v1.2 (CHART-F01)

### Quick Tasks Completed

| Date | Slug | Description |
|------|------|-------------|
| 2026-04-15 | readme-docker-testing | Fix README Docker section: add DATABASE_URL to .env.example and docker run commands |
| 2026-04-15 | 260415-ixx | Add /healthz (liveness) and /readyz (readiness) endpoints; startupProbe in Helm chart and k8s manifests | 23e7069 |

## Session Continuity

Last session: 2026-04-15
Stopped at: v1.2 roadmap created — 5 phases (6–10), 19 requirements mapped
Resume file: None
