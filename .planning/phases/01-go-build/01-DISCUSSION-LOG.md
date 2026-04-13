# Phase 1: Go + Build - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions captured in CONTEXT.md — this log preserves the discussion.

**Date:** 2026-04-13
**Phase:** 01-go-build
**Mode:** discuss
**Areas discussed:** pgx API style, Existing test migration

## Gray Areas Presented

| Area | Description |
|------|-------------|
| pgx API style | database/sql wrapper vs native pgx pool — affects how much code beyond product.go needs changing |
| Existing test migration | All 5 test files use setupTestStore() → :memory: SQLite; these break when go-sqlite3 is removed |
| Local dev Postgres | No docker-compose.yml exists; local dev needs Postgres after migration |

**Selected for discussion:** pgx API style, Existing test migration
**Not selected:** Local dev Postgres (Claude's discretion)

## Discussion

### pgx API style

| Option | Description |
|--------|-------------|
| stdlib.OpenDBFromPool ✓ | pgx/v5/stdlib wraps pgxpool in *sql.DB — ProductStore unchanged, only SQL strings change |
| Native pgx pool | ProductStore holds *pgxpool.Pool — cleaner pgx but more files touched |

**User chose:** stdlib.OpenDBFromPool (Recommended)
**Rationale:** Minimal diff — handlers and middleware untouched, only SQL strings in product.go change

### Existing test migration

| Option | Description |
|--------|-------------|
| Shared container via TestMain ✓ | One testcontainers Postgres per package, shared across all tests |
| Per-test container | Each test gets its own container — simple but slow (30–60s test suite) |
| Mock unit tests + separate integration | StoreInterface + mocks for fast tests, separate integration layer |

**User chose:** Shared container via TestMain (Recommended)
**Rationale:** Fastest — single Docker container startup (~3–5s) shared across all model tests; setupTestStore rewritten to use shared DB

## Decisions Not Discussed (Claude's Discretion)

- Goose migrations in `internal/migrations/` (embedded) — research recommendation
- Build tags: not used, `go test ./...` runs everything
- Connection pool: default config from DATABASE_URL string
- Dockerfile: remove tonistiigi/xx, use `CGO_ENABLED=0 go build`
- Local dev Postgres: user chose not to discuss — left to Claude during planning

## No Corrections, No Deferred Ideas
