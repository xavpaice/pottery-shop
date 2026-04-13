# Requirements: Pottery Shop — Postgres Migration

**Defined:** 2026-04-13
**Core Value:** The app runs reliably on Postgres with zero SQLite anywhere — CNPG manages the in-cluster database lifecycle, and the Go binary is a pure CGO-free build.

## v1 Requirements

### Go Application

- [ ] **APP-01**: Replace `go-sqlite3` (CGO) with `pgx/v5` (pure Go) as the database driver
- [ ] **APP-02**: Fix all `?` placeholders → `$N` numbered params across all queries in product.go
- [ ] **APP-03**: Fix `LastInsertId()` → `RETURNING id` on every INSERT query
- [ ] **APP-04**: Rewrite DDL for Postgres types (IDENTITY, BOOLEAN, TIMESTAMP, NUMERIC)
- [ ] **APP-05**: Replace inline `store.Init()` with Goose v3 embedded schema migrations
- [ ] **APP-06**: Read `DATABASE_URL` env var for Postgres connection string

### Docker Build

- [ ] **BUILD-01**: Set `CGO_ENABLED=0` and strip `tonistiigi/xx` CGO cross-compile scaffold from Dockerfile
- [ ] **BUILD-02**: Docker image produces a pure Go binary with no CGO dependencies

### Helm Chart

- [ ] **HELM-01**: Add CNPG operator as Helm subchart dependency (chart 0.28.0, condition: `cloudnative-pg.enabled`)
- [ ] **HELM-02**: Add `postgres` block to values.yaml with `managed`, `cluster.instances`, `cluster.storage.size`, `cluster.storage.storageClass`
- [ ] **HELM-03**: Create `templates/cnpg-cluster.yaml` rendered only when `postgres.managed: true`
- [ ] **HELM-04**: Inject `DATABASE_URL` from CNPG-generated Secret (`<cluster>-app`, key `uri`) in managed mode
- [ ] **HELM-05**: Inject `DATABASE_URL` from `postgres.external.dsn` plain value in external mode
- [ ] **HELM-06**: Add timing mitigation for CNPG Secret race (initContainer or startup probe)
- [ ] **HELM-07**: Remove `DB_PATH` SQLite artifact from values and configmap

### Testing

- [ ] **TEST-01**: Add testcontainers-go to test suite — integration tests spin up a real Postgres container
- [ ] **TEST-02**: Existing integration tests updated to run against Postgres (not SQLite)

### CI / GitHub Actions

- [ ] **CI-01**: Update build job: set `CGO_ENABLED=0`, remove CGO cross-compile steps
- [ ] **CI-02**: Add test job: `go vet` + `golangci-lint` + `go test` (with testcontainers-go Postgres)
- [ ] **CI-03**: Add Helm validation job: `helm lint` + `helm template` render check

## v2 Requirements

### Operations

- **OPS-01**: CNPG backup configuration (WAL archiving to object storage)
- **OPS-02**: CNPG monitoring integration (Prometheus metrics from CNPG operator)
- **OPS-03**: CRD upgrade runbook documented for `helm upgrade` scenarios

### Security

- **SEC-01**: CSRF protection for POST endpoints (pre-existing concern from CONCERNS.md)
- **SEC-02**: Admin password and session secret enforcement (no hardcoded defaults)
- **SEC-03**: Directory listing disabled for `/uploads/` and `/static/`

## Out of Scope

| Feature | Reason |
|---------|--------|
| SQLite as local-dev fallback | Full replacement — clean cut reduces complexity |
| Data migration from SQLite | Fresh start — no production data to carry over |
| Manual K8s Secret management | CNPG generates and owns credentials |
| Mobile app / frontend rewrite | Out of this milestone's scope |
| Real-time features | Not relevant to database migration |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| APP-01 | Phase 1 | Pending |
| APP-02 | Phase 1 | Pending |
| APP-03 | Phase 1 | Pending |
| APP-04 | Phase 1 | Pending |
| APP-05 | Phase 1 | Pending |
| APP-06 | Phase 1 | Pending |
| BUILD-01 | Phase 1 | Pending |
| BUILD-02 | Phase 1 | Pending |
| TEST-01 | Phase 1 | Pending |
| TEST-02 | Phase 1 | Pending |
| HELM-01 | Phase 2 | Pending |
| HELM-02 | Phase 2 | Pending |
| HELM-03 | Phase 2 | Pending |
| HELM-04 | Phase 2 | Pending |
| HELM-05 | Phase 2 | Pending |
| HELM-06 | Phase 2 | Pending |
| HELM-07 | Phase 2 | Pending |
| CI-01 | Phase 2 | Pending |
| CI-02 | Phase 2 | Pending |
| CI-03 | Phase 2 | Pending |

**Coverage:**
- v1 requirements: 19 total
- Mapped to phases: 19
- Unmapped: 0 ✓

---
*Requirements defined: 2026-04-13*
*Last updated: 2026-04-13 after initial definition*
