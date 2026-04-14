# Pottery Shop — Postgres Migration

## What This Is

An e-commerce pottery shop built in Go that is being migrated from SQLite to PostgreSQL. The app manages a product catalog with image uploads, a session-based shopping cart, order placement via email, and an admin area. It deploys to Kubernetes via Helm, and this work adds CloudNative-PG as the database layer — either managed in-cluster via the CNPG operator (installed as a Helm subchart) or pointed at an external Postgres via a DSN in values.yaml.

## Core Value

The app runs reliably on Postgres with zero SQLite anywhere — CNPG manages the in-cluster database lifecycle, and the Go binary is a pure CGO-free build.

## Requirements

### Validated

- ✓ Product catalog with image upload and thumbnail generation — existing
- ✓ Session-based shopping cart (signed cookies) — existing
- ✓ Order placement with email notification — existing
- ✓ Admin area with HTTP Basic Auth (CRUD for products) — existing
- ✓ Kubernetes deployment via Helm chart — existing
- ✓ Multi-stage Docker build pushed to GHCR via GitHub Actions — existing

### Validated

- ✓ Replace go-sqlite3 (CGO) with pgx/v5 (pure Go) as the database driver — Validated in Phase 1: Go + Build
- ✓ Update SQL dialect from SQLite to Postgres (types, syntax, sequences) — Validated in Phase 1: Go + Build
- ✓ Docker build works without CGO (pure Go binary) — Validated in Phase 1: Go + Build

### Validated

- ✓ Add CloudNative-PG operator as a Helm subchart dependency — Validated in Phase 2: Helm + CI
- ✓ Create a CNPG Cluster resource in the Helm chart (default: 1 instance) — Validated in Phase 2: Helm + CI
- ✓ Mount the CNPG-generated Secret into the app pod as DATABASE_URL env var — Validated in Phase 2: Helm + CI
- ✓ Support external Postgres via postgres.external.dsn in values.yaml — Validated in Phase 2: Helm + CI
- ✓ CI pipeline validates build, tests, and Helm rendering on every push — Validated in Phase 2: Helm + CI

### Active

- [ ] Expose the app over HTTPS via Kubernetes Ingress with TLS termination
- [ ] cert-manager installed as a Helm subchart dependency
- [ ] Three TLS modes configurable in values.yaml: letsencrypt, selfsigned, custom
- [ ] ClusterIssuer for Let's Encrypt HTTP-01 ACME (default mode)
- [ ] ingress.host value drives Ingress hostname and Certificate resource
- [ ] CI validation extended for all three TLS modes

### Out of Scope

- SQLite as a local-dev fallback — full replacement, Postgres everywhere
- Data migration from SQLite — fresh start, no existing data to carry over
- Manual Kubernetes Secret management — CNPG generates and owns credentials

## Context

- The current codebase uses `github.com/mattn/go-sqlite3` which requires CGO. Switching to `pgx/v5` (or `pgx/v5` via `database/sql`) gives a pure Go build.
- SQL may need dialect adjustments: SQLite uses `INTEGER PRIMARY KEY` for auto-increment; Postgres uses `SERIAL` or `GENERATED ALWAYS AS IDENTITY`. Date/time handling also differs.
- CNPG creates a K8s Secret (`<cluster-name>-app`) with a ready-to-use connection string — the app should read this as `DATABASE_URL`.
- The Helm chart currently lives in `chart/clay/`. The CNPG operator will be added as a subchart dependency via `chart/clay/Chart.yaml`.
- When `postgres.external.dsn` is set in values.yaml, the chart must skip creating the CNPG Cluster resource and instead inject the DSN directly as `DATABASE_URL`.
- Several security concerns in CONCERNS.md (CSRF, directory listing, weak defaults) are pre-existing and out of scope for this milestone.

## Constraints

- **Tech stack**: Go + `pgx/v5` — no CGO, no go-sqlite3
- **Database**: PostgreSQL only — no SQLite anywhere
- **Kubernetes**: CloudNative-PG operator (CNPG) as Helm subchart
- **Helm**: Single chart manages both CNPG Cluster and app; values.yaml controls mode (managed vs external)
- **Compatibility**: Existing Helm values structure must remain backward-compatible where possible

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| pgx over lib/pq | Pure Go (no CGO), better performance, actively maintained | ✓ Implemented — Phase 1 |
| CNPG as subchart | Simpler install — one `helm install` gets everything | ✓ Implemented — Phase 2 |
| External PG via DSN string | Simplest interface; user sets one value, chart injects as env var | ✓ Implemented — Phase 2 |
| No SQLite fallback | Clean cut reduces complexity; local dev uses Postgres too | ✓ Implemented — Phase 1 |
| Default 1 CNPG instance | Right for hobby/staging; HA (3) configurable via values.yaml | ✓ Implemented — Phase 2 |
| pg_isready init container | Timing mitigation — prevents race between app and CNPG cluster ready | ✓ Implemented — Phase 2 |

## Current Milestone: v1.1 TLS

**Goal:** Expose the pottery shop over HTTPS via a Kubernetes Ingress with automated TLS certificate management.

**Target features:**
- cert-manager as Helm subchart (condition-gated, same pattern as CNPG)
- Kubernetes Ingress resource with Traefik annotations
- Three TLS modes via values.yaml: `letsencrypt` (HTTP-01, default), `selfsigned`, `custom` (BYO Secret)
- `ingress.host` drives Ingress hostname and Certificate resource
- CI Helm validation extended to cover all three TLS modes

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition** (via `/gsd-transition`):
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After each milestone** (via `/gsd-complete-milestone`):
1. Full review of all sections
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state

---
*Last updated: 2026-04-14 — Milestone v1.1 TLS started (Ingress HTTPS, cert-manager subchart, three TLS modes, Traefik, HTTP-01 ACME)*
