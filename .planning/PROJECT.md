# Pottery Shop — Postgres Migration

## What This Is

An e-commerce pottery shop built in Go that has been migrated from SQLite to PostgreSQL and now deploys over HTTPS via Kubernetes Ingress with cert-manager-managed TLS certificates. The app manages a product catalog with image uploads, a session-based shopping cart, order placement via email, and an admin area. It deploys to Kubernetes via Helm — CNPG manages the in-cluster Postgres lifecycle, and cert-manager handles TLS certificate issuance for three configurable modes (letsencrypt, selfsigned, custom).

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

- ✓ Replace go-sqlite3 (CGO) with pgx/v5 (pure Go) as the database driver — v1.0
- ✓ Update SQL dialect from SQLite to Postgres (types, syntax, sequences) — v1.0
- ✓ Docker build works without CGO (pure Go binary) — v1.0
- ✓ Add CloudNative-PG operator as a Helm subchart dependency — v1.0
- ✓ Create a CNPG Cluster resource in the Helm chart (default: 1 instance) — v1.0
- ✓ Mount the CNPG-generated Secret into the app pod as DATABASE_URL env var — v1.0
- ✓ Support external Postgres via postgres.external.dsn in values.yaml — v1.0
- ✓ CI pipeline validates build, tests, and Helm rendering on every push — v1.0

- ✓ Expose the app over HTTPS via Kubernetes Ingress with TLS termination — v1.1
- ✓ Three TLS modes configurable in values.yaml: letsencrypt, selfsigned, custom — v1.1
- ✓ ClusterIssuer for Let's Encrypt HTTP-01 ACME (staging default, production opt-in) — v1.1
- ✓ Two-step CA bootstrap for selfsigned mode (avoids untrusted end-entity cert) — v1.1
- ✓ ingress.host value drives Ingress hostname and Certificate resource — v1.1
- ✓ Fail-fast clay.validateIngress helper (missing host, mode, email, secretName) — v1.1
- ✓ CI validation extended for all three TLS modes (lint + template + behavioral test) — v1.1
- ✓ cert-manager v1.20.2 pre-install step in integration-test.yml — v1.1

### Active

(None — planning next milestone)

### Out of Scope

- SQLite as a local-dev fallback — full replacement, Postgres everywhere
- Data migration from SQLite — fresh start, no existing data to carry over
- Manual Kubernetes Secret management — CNPG generates and owns credentials
- cert-manager as a Helm subchart — official docs prohibit embedding for cluster-scoped operators; mirrors CNPG pre-install pattern
- Postgres TLS — separate concern from Ingress TLS; deferred to future milestone
- DNS-01 ACME — requires DNS provider credentials; HTTP-01 sufficient for this deployment

## Context

- Codebase is pure Go (CGO_ENABLED=0): pgx/v5 for database, Goose v3 for schema migrations, testcontainers-go for integration tests.
- Helm chart (`chart/clay/`) manages CNPG operator (subchart), the CNPG Cluster resource, Deployment with init container, and Ingress with cert-manager TLS.
- Three TLS modes: `letsencrypt` (ACME HTTP-01, staging by default), `selfsigned` (4-resource CA bootstrap), `custom` (user-provided secret, no cert-manager CRs).
- All cert-manager resources carry `helm.sh/hook: post-install,post-upgrade` to avoid webhook timing races.
- CI (`test.yml`) validates: build, go vet, testcontainers integration tests, helm lint/template for managed + external + 3 TLS modes, and 23-assertion behavioral test script.

## Constraints

- **Tech stack**: Go + `pgx/v5` — no CGO, no go-sqlite3
- **Database**: PostgreSQL only — no SQLite anywhere
- **Kubernetes**: CloudNative-PG operator (CNPG) as Helm subchart
- **Helm**: Single chart manages both CNPG Cluster and app; values.yaml controls mode (managed vs external)
- **Compatibility**: Existing Helm values structure must remain backward-compatible where possible

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| pgx over lib/pq | Pure Go (no CGO), better performance, actively maintained | ✓ Implemented — v1.0 |
| CNPG as subchart | Simpler install — one `helm install` gets everything | ✓ Implemented — v1.0 |
| External PG via DSN string | Simplest interface; user sets one value, chart injects as env var | ✓ Implemented — v1.0 |
| No SQLite fallback | Clean cut reduces complexity; local dev uses Postgres too | ✓ Implemented — v1.0 |
| Default 1 CNPG instance | Right for hobby/staging; HA (3) configurable via values.yaml | ✓ Implemented — v1.0 |
| pg_isready init container | Timing mitigation — prevents race between app and CNPG cluster ready | ✓ Implemented — v1.0 |
| cert-manager as pre-install step, not subchart | Official docs prohibit subchart embedding for cluster-scoped CRDs; mirrors CNPG pattern | ✓ Implemented — v1.1 |
| Staging ACME endpoint default | Prevents burning Let's Encrypt production rate limits during dev/testing | ✓ Implemented — v1.1 |
| Selfsigned uses two-step CA bootstrap | SelfSigned ClusterIssuer issues CA cert; CA ClusterIssuer issues app cert (avoids untrusted end-entity cert) | ✓ Implemented — v1.1 |
| clay.tlsSecretName defined once in _helpers.tpl | Prevents Ingress/Certificate secret name mismatch — single source of truth | ✓ Implemented — v1.1 |
| helm template without --validate in CI | Avoids cert-manager CRD absence failures in CI; behavioral correctness verified via helm-template-test.sh | ✓ Implemented — v1.1 |
| Traefik annotations hardcoded in template (not values.yaml) | Prevents operator injection of unexpected routing rules | ✓ Implemented — v1.1 |
| Hook-weight sequencing for selfsigned CA bootstrap | 4 resources must install in order: root(-10) → ca-cert(-5) → ca-issuer(0) → app-cert(5) | ✓ Implemented — v1.1 |

---
*Last updated: 2026-04-14 after v1.1 milestone (TLS: Ingress, cert-manager, three TLS modes, Traefik, CI validation)*
