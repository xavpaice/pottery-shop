# Roadmap: Pottery Shop — Postgres Migration

## Overview

A brownfield Go pottery shop migrates from SQLite (CGO) to PostgreSQL (pure Go). Phase 1 completes all code-level changes — driver swap, SQL dialect fixes, schema migration via Goose, CGO removal from the build, and testcontainers-go integration tests — so the app can be validated locally against a real Postgres container before any cluster work begins. Phase 2 wires the Kubernetes delivery layer: CNPG operator as a Helm subchart, the Cluster resource, secret injection, timing mitigation, and CI pipeline jobs.

## Phases

**Phase Numbering:**
- Integer phases (1, 2): Planned milestone work
- Decimal phases (1.1, 1.2): Urgent insertions (marked with INSERTED)

- [x] **Phase 1: Go + Build** — Driver swap, SQL dialect fixes, Goose migrations, CGO removal, and local integration tests (completed 2026-04-13)
- [ ] **Phase 2: Helm + CI** — CNPG subchart, Cluster resource, secret injection, timing fix, and CI pipeline

## Phase Details

### Phase 1: Go + Build
**Goal**: The Go application connects to Postgres, all SQL is Postgres-compatible, the binary builds without CGO, and integration tests pass against a real Postgres container — all verifiable without a Kubernetes cluster.
**Depends on**: Nothing (first phase)
**Requirements**: APP-01, APP-02, APP-03, APP-04, APP-05, APP-06, BUILD-01, BUILD-02, TEST-01, TEST-02
**Success Criteria** (what must be TRUE):
  1. `go build ./...` succeeds with `CGO_ENABLED=0` and no reference to go-sqlite3 anywhere in the module
  2. The app starts, runs Goose migrations, and serves requests when `DATABASE_URL` points to a local Postgres instance
  3. All INSERT operations return the correct generated `id` (not zero) and product CRUD works end-to-end
  4. Integration tests pass with `go test ./...` using a testcontainers-go Postgres container — no SQLite, no mocks
  5. Docker `docker build` produces a working image with no CGO dependencies and no cross-compile scaffold
**Plans:** 3/3 plans complete

Plans:
- [x] 01-01-PLAN.md — Driver swap, SQL dialect fixes in product.go, Goose migration file
- [x] 01-02-PLAN.md — main.go pgxpool/Goose wiring, Dockerfile and Makefile CGO removal
- [x] 01-03-PLAN.md — testcontainers-go integration tests, go-sqlite3 removal, end-to-end verification

---

### Phase 2: Helm + CI
**Goal**: The Helm chart supports both managed CNPG and external Postgres, the app pod starts reliably on first `helm install`, and the CI pipeline validates build, tests, and Helm rendering on every push.
**Depends on**: Phase 1
**Requirements**: HELM-01, HELM-02, HELM-03, HELM-04, HELM-05, HELM-06, HELM-07, CI-01, CI-02, CI-03
**Success Criteria** (what must be TRUE):
  1. `helm lint chart/clay` and `helm template chart/clay` both pass with no errors in managed mode and in external-DSN mode
  2. After `helm install` on a cluster with the CNPG operator, the app pod reaches `Running` and its readiness probe passes without manual intervention
  3. `DATABASE_URL` is injected from the CNPG-generated Secret in managed mode and from `postgres.external.dsn` in external mode — no `DB_PATH` reference remains anywhere in the chart
  4. The CI pipeline runs build, lint/test, and Helm validation jobs on every push and reports failures before merge
**Plans:** 2 plans

Plans:
- [x] 02-01-PLAN.md — Helm chart wiring: CNPG subchart dependency, values.yaml postgres block, Cluster CRD template, deployment.yaml updates (strategy, init container, DATABASE_URL injection), DB_PATH removal
- [x] 02-02-PLAN.md — CI pipeline: remove gcc from test job, add go vet, extend helm-lint job with dependency resolution and dual-mode lint+template validation

---

## Progress

**Execution Order:**
Phases execute in numeric order: 1 → 2

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Go + Build | 3/3 | Complete    | 2026-04-13 |
| 2. Helm + CI | 0/2 | Not started | - |
