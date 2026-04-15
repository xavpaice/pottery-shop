# Clay.nz — Pottery Shop

## What This Is

An e-commerce pottery shop built in Go that sells one-of-a-kind clay and sculpture pieces. The app manages a product catalog with image uploads, a session-based shopping cart, and order placement via email. It deploys to Kubernetes via Helm, with CloudNative-PG managing the in-cluster PostgreSQL database and cert-manager handling TLS.

## Core Value

The Helm chart deploys the full stack — app, database, and certificates — in a single `helm install`, with no operator pre-install required.

## Current Milestone: v1.2 Umbrella Chart

**Goal:** Make the clay Helm chart an umbrella chart that optionally installs the CNPG operator and cert-manager as subchart dependencies, controlled by values toggles — eliminating the requirement to pre-install operators separately.

**Target features:**
- Conditional subchart dependencies in Chart.yaml (`cloudnative-pg.enabled`, `cert-manager.enabled`)
- values.yaml toggles and values.schema.json entries for operator subchart control
- Webhook-readiness Jobs (CNPG + cert-manager) with RBAC to solve CRD ordering problem
- cnpg-cluster.yaml converted to post-install hook with correct weight ordering
- CI test matrix covering all 4 toggle combinations (both bundled, pre-installed, external DB, mixed)
- Documentation updated for new operator subchart modes and upgrade path

## Requirements

### Validated

<!-- Shipped in v1.0 and v1.1 -->

- ✓ Go app builds as pure CGO-free binary (no SQLite) — v1.0
- ✓ PostgreSQL via pgx/v5 with Goose migrations — v1.0
- ✓ testcontainers-go integration tests replace SQLite in-memory tests — v1.0
- ✓ CNPG operator installed via Helm subchart (0.28.0) with Cluster CRD and DATABASE_URL injection — v1.0
- ✓ CI pipeline: go vet, tests, CGO-free build, helm-lint dual-mode — v1.0
- ✓ Helm chart ingress: single-host Traefik scalar with mode-driven TLS — v1.1
- ✓ cert-manager ClusterIssuer + Certificate templates for letsencrypt and selfsigned modes — v1.1
- ✓ cert-manager resources carry helm.sh/hook annotations to prevent upgrade conflicts — v1.1
- ✓ CI extended with TLS lint+template steps and 23-assertion behavioral test script — v1.1

### Active

<!-- v1.2 scope — umbrella chart -->

- [ ] Chart.yaml has conditional subchart dependencies for cloudnative-pg and cert-manager
- [ ] values.yaml has cloudnative-pg.enabled and cert-manager.enabled toggles
- [ ] values.schema.json validated for new subchart toggle keys
- [ ] Webhook-readiness Jobs block CR creation until operators are serving
- [ ] cnpg-cluster.yaml is a post-install hook, sequenced after webhook-wait Job
- [ ] Hook weights are ordered correctly across all hook resources
- [ ] CI matrix covers: both bundled, operators pre-installed, external DB only, mixed
- [ ] README explains all toggle combinations and upgrade path from pre-installed operators

### Out of Scope

- Multi-tenant / multi-seller support — separate future milestone (spec exists in docs/)
- Payment gateway integration — not planned; order-by-email model is intentional
- Mobile app — web-first
- Real-time features (websockets, live inventory) — complexity not justified by current scale

## Context

- Deployed to Kubernetes via Helm (`chart/clay/`)
- CNPG operator was previously a prerequisite (manually installed before `helm install`)
- cert-manager was previously a prerequisite (manually installed before `helm install`)
- Hook weight sequencing pattern already established for cert-manager resources (v1.1)
- Integration tests run on CMX (k3s via Replicated) with GHCR image push
- `chart/tests/helm-template-test.sh` is the behavioral test harness (named groups G-01…)
- CI values files live in `chart/clay/ci/` — one per test scenario

## Constraints

- **Tech stack**: Go + pgx/v5 — no CGO, no SQLite
- **Helm**: Single chart manages both CNPG Cluster and app; values structure must stay backward-compatible
- **Kubernetes**: CNPG operator and cert-manager install to their own namespaces (`cnpg-system`, `cert-manager`) — subchart namespace configuration must be verified
- **Permissions**: Webhook-readiness RBAC requires ClusterRole/Binding — implies cluster-admin at install time (already required for operator install)

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Hook weights for cert-manager CRs (-10 to 5) | Prevents upgrade "already exists" errors on immutable resources | ✓ Good |
| `clay.tlsSecretName` helper in _helpers.tpl | Single source of truth for TLS secret name across Ingress and Certificate | ✓ Good |
| Staging ACME endpoint as default | Prevents rate-limit accidents during development | ✓ Good |
| CNPG Cluster CR → post-install hook (v1.2) | Required to sequence after webhook-wait Job; means helm uninstall won't auto-delete it | — Pending |
| `bitnami/kubectl` for webhook-wait Jobs | Convenient but needs version pinning; air-gapped envs need alternative | — Pending |

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
*Last updated: 2026-04-15 — Milestone v1.2 started*
