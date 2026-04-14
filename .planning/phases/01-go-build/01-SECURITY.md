---
phase: 01
slug: go-build
status: verified
threats_open: 0
asvs_level: L1
created: 2026-04-14
---

# Phase 01 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| App -> Database | SQL queries cross from Go application to Postgres; all user-provided data flows through parameterized queries | Product data, user input (low sensitivity) |
| Environment -> App | DATABASE_URL env var contains Postgres connection string with credentials | Postgres credentials (high sensitivity) |
| App -> Postgres | Connection pool crosses network boundary to database server | SQL queries, result sets |
| Test env -> Postgres container | testcontainers creates an ephemeral container with known credentials; acceptable for test isolation | Test credentials (non-production) |

---

## Threat Register

| Threat ID | Category | Component | Disposition | Mitigation | Status |
|-----------|----------|-----------|-------------|------------|--------|
| T-01-01 | Injection | product.go SQL queries | mitigate | All queries use `$N` parameterized placeholders at lines 39, 47, 54, 61, 104, 112, 133, 138, 144 — zero string concatenation in SQL | closed |
| T-01-02 | Tampering | 00001_initial_schema.sql | accept | Migration file embedded at compile time via go:embed; no runtime file access; goose_db_version tracks applied migrations | closed |
| T-01-03 | Information Disclosure | DATABASE_URL in go.mod | accept | go.mod declares dependencies only, not connection strings; DATABASE_URL is an env var not present in module files | closed |
| T-01-04 | Information Disclosure | DATABASE_URL in env | mitigate | `cmd/server/main.go:41` — `log.Fatal("DATABASE_URL must be set")` on empty; connection string never logged; production injection via Kubernetes Secret (Phase 2) | closed |
| T-01-05 | Denial of Service | pgxpool connection exhaustion | accept | Default pgxpool config sufficient for hobby pottery shop; pool has built-in max connection limits and health checks | closed |
| T-01-06 | Tampering | Goose migration files | mitigate | `internal/migrations/migrations.go:5` — `//go:embed *.sql`; `cmd/server/main.go:54-58` — `goose.SetBaseFS(migrations.FS)` + `goose.Up(db, ".")`; SQL embedded at compile time, runtime filesystem cannot alter migrations | closed |
| T-01-07 | Elevation of Privilege | Dockerfile runs as root | accept | Pre-existing condition, not introduced by this phase; Kubernetes securityContext can restrict at deploy time (Phase 2 scope) | closed |
| T-01-08 | Information Disclosure | testcontainers hardcoded credentials | accept | Test-only credentials (`user: "test"`, `password: "test"`) in ephemeral container destroyed after test run; never deployed to production | closed |
| T-01-09 | Denial of Service | testcontainers Docker dependency | accept | Tests fail fast with clear error when Docker unavailable; development dependency only, not a production concern | closed |

*Status: open · closed*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-01-01 | T-01-02 | Migration SQL embedded at compile time; runtime tampering not possible by design | Claude GSD | 2026-04-14 |
| AR-01-02 | T-01-03 | go.mod contains no credentials; DATABASE_URL is environment-injected, not in module files | Claude GSD | 2026-04-14 |
| AR-01-03 | T-01-05 | Hobby pottery shop traffic profile; default pgxpool limits are adequate; no evidence of abuse risk | Claude GSD | 2026-04-14 |
| AR-01-04 | T-01-07 | Root container is pre-existing condition; Kubernetes securityContext hardening deferred to Phase 2 (CNPG/Helm) | Claude GSD | 2026-04-14 |
| AR-01-05 | T-01-08 | Hardcoded test credentials exist only in ephemeral testcontainers instances; no path to production | Claude GSD | 2026-04-14 |
| AR-01-06 | T-01-09 | Docker daemon dependency in tests; failure is fast and explicit; development-only, not production risk | Claude GSD | 2026-04-14 |

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-04-14 | 9 | 9 | 0 | gsd-security-auditor (sonnet) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-04-14
