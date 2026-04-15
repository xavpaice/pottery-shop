# State: Clay.nz Pottery Shop

## Project Reference

See: .planning/PROJECT.md (updated 2026-04-15)

**Core value:** The Helm chart deploys the full stack — app, database, and certificates — in a single `helm install`, with no operator pre-install required.
**Current focus:** Defining requirements for v1.2 Umbrella Chart

## Current Position

Phase: Not started (defining requirements)
Plan: —
Status: Defining requirements
Last activity: 2026-04-15 — Milestone v1.2 started

## Accumulated Context

### From v1.1 (TLS)
- Hook weight sequencing pattern: RBAC at -25, webhook-wait Jobs at -20, CRs at -10 to 5
- cert-manager hook template established: every ClusterIssuer/Certificate carries `helm.sh/hook: post-install,post-upgrade` + `helm.sh/hook-delete-policy: before-hook-creation`
- CI behavioral test pattern: `chart/tests/helm-template-test.sh` with named groups (G-01…), `^kind:` anchored greps
- Operator pre-install pattern: `helm repo add + helm install --version pin --namespace --create-namespace --wait`
- Lesson: Close documentation gaps (checkbox state, STATE.md, ROADMAP progress table) at phase exit, not milestone close

### Quick Tasks Completed

| Date | Slug | Description |
|------|------|-------------|
| 2026-04-15 | readme-docker-testing | Fix README Docker section: add DATABASE_URL to .env.example and docker run commands; add full Docker testing workflow |
