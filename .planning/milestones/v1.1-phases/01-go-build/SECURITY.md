---
phase: 01-go-build
plans: [01-01, 01-02]
audited: "2026-04-14"
asvs_level: L1
block_on: high
---

# Security Audit — Phase 01-go-build

## Summary

**Threats Closed:** 9/9
**Threats Open:** 0
**Unregistered Flags:** 0

## Threat Verification

| Threat ID | Category | Disposition | Status | Evidence |
|-----------|----------|-------------|--------|----------|
| T-01-01 | Injection | mitigate | CLOSED | `internal/models/product.go` lines 39, 47, 54, 61, 104, 112, 133, 138, 144: all query sites use `$N` numbered placeholders; zero string concatenation in SQL strings confirmed by grep |
| T-01-02 | Tampering | accept | CLOSED | Accepted risk documented below |
| T-01-03 | Information Disclosure | accept | CLOSED | Accepted risk documented below |
| T-01-04 | Information Disclosure | mitigate | CLOSED | `cmd/server/main.go` line 39: `os.Getenv("DATABASE_URL")`; line 41: `log.Fatal("DATABASE_URL must be set")` — hard fatal on empty, no fallback. No log statement prints the connection string or pool credentials |
| T-01-05 | Denial of Service | accept | CLOSED | Accepted risk documented below |
| T-01-06 | Tampering | mitigate | CLOSED | `internal/migrations/migrations.go` line 5: `//go:embed *.sql` embeds SQL at compile time; `cmd/server/main.go` lines 54-58: `goose.SetBaseFS(migrations.FS)` and `goose.Up(db, ".")` — runtime filesystem cannot alter embedded migrations; goose_db_version table tracks applied migrations (standard goose behavior) |
| T-01-07 | Elevation of Privilege | accept | CLOSED | Accepted risk documented below |
| T-01-08 | Information Disclosure | accept | CLOSED | Accepted risk documented below |
| T-01-09 | Denial of Service | accept | CLOSED | Accepted risk documented below |

## Accepted Risks Log

### T-01-02 — Tampering (00001_initial_schema.sql embedded at compile time)

- **Accepted by:** Plan author (01-01-PLAN.md threat model, disposition: accept)
- **Rationale:** Migration file is embedded via `go:embed` at compile time. No runtime filesystem access to migration files. `goose_db_version` table tracks which migrations have been applied, preventing re-execution.
- **Residual risk:** Negligible. Any tampering of the SQL file requires a code rebuild.
- **Re-evaluate at:** Any phase that adds new migration files.

### T-01-03 — Information Disclosure (DATABASE_URL in go.mod)

- **Accepted by:** Plan author (01-01-PLAN.md threat model, disposition: accept)
- **Rationale:** go.mod declares module dependencies only — it contains no connection strings. DATABASE_URL is a runtime environment variable, handled separately in Plan 02. go.mod is not a secret.
- **Residual risk:** None for this phase. DATABASE_URL exposure risk is captured under T-01-04 (mitigated).
- **Re-evaluate at:** Not required.

### T-01-05 — Denial of Service (pgxpool connection exhaustion)

- **Accepted by:** Plan author (01-02-PLAN.md threat model, disposition: accept)
- **Rationale:** Default pgxpool configuration is sufficient for a low-traffic hobby pottery shop. The pool enforces built-in maximum connection limits and health checks. No additional env-var tuning is introduced.
- **Residual risk:** Low. Under sustained concurrent load, pool saturation could cause request queuing. Acceptable given usage profile.
- **Re-evaluate at:** Phase 2 if load characteristics change.

### T-01-07 — Elevation of Privilege (Dockerfile runs as root)

- **Accepted by:** Plan author (01-02-PLAN.md threat model, disposition: accept)
- **Rationale:** Pre-existing condition not introduced by this phase. The container runs a single process. Kubernetes `securityContext` can restrict privilege at deploy time and is scoped to Phase 2.
- **Residual risk:** Medium. Container process has root-equivalent inside its namespace. Mitigable via Kubernetes security policies in Phase 2.
- **Re-evaluate at:** Phase 2 (Helm/CNPG deployment).

### T-01-08 — Information Disclosure (testcontainers hardcoded credentials)

- **Accepted by:** Plan author (01-02-PLAN.md threat model, disposition: accept)
- **Rationale:** Testcontainers use hardcoded credentials (e.g., `postgres`/`postgres`) only in the test environment. These credentials are not used in production and are not committed to any production config.
- **Residual risk:** Low. Exposure is limited to the test/CI environment.
- **Re-evaluate at:** If test credentials ever appear in production configuration.

### T-01-09 — Denial of Service (testcontainers Docker dependency)

- **Accepted by:** Plan author (01-02-PLAN.md threat model, disposition: accept)
- **Rationale:** Testcontainers require a Docker daemon at test time. An unavailable Docker daemon fails the test run but has no impact on production availability.
- **Residual risk:** Low. CI pipeline may fail if Docker is unavailable; no production impact.
- **Re-evaluate at:** Not required.

## Unregistered Threat Flags

None. The `## Threat Flags` entries in 01-01-SUMMARY.md and 01-02-SUMMARY.md reference T-01-04 and T-01-06 as verified during implementation — both map to registered threats in this audit.
