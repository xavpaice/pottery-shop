---
phase: 01-go-build
plan: 01
subsystem: database
tags: [pgx, goose, postgres, migrations, sql, cgo-free]

# Dependency graph
requires: []
provides:
  - pgx/v5 v5.9.1 and goose/v3 v3.27.0 in go.mod
  - internal/migrations package with embedded Goose SQL files
  - Postgres schema DDL (IDENTITY, BOOLEAN, NUMERIC, TIMESTAMPTZ)
  - product.go with all 12 query sites rewritten for Postgres dialect
  - RETURNING id pattern for Create and AddImage
  - Native bool scanning (no integer workaround)
  - Init() method removed from ProductStore
affects: [01-02, 01-03]

# Tech tracking
tech-stack:
  added:
    - "github.com/jackc/pgx/v5 v5.9.1 — pure Go Postgres driver"
    - "github.com/pressly/goose/v3 v3.27.0 — schema migration runner with go:embed support"
  patterns:
    - "RETURNING id pattern for INSERTs instead of LastInsertId()"
    - "Goose embedded migrations via //go:embed *.sql"
    - "Postgres $N numbered parameter placeholders"
    - "Native bool scanning for BOOLEAN columns"

key-files:
  created:
    - internal/migrations/migrations.go
    - internal/migrations/00001_initial_schema.sql
  modified:
    - go.mod
    - go.sum
    - internal/models/product.go

key-decisions:
  - "Keep go-sqlite3 in go.mod for this plan (product_test.go still imports it; removed in Plan 03)"
  - "pgx and goose listed as indirect deps until Plan 02 adds direct imports in main.go"
  - "Single migration file 00001_initial_schema.sql covers full schema (products + images)"

patterns-established:
  - "Goose Up/Down migration structure: Up creates tables, Down drops images before products (FK order)"
  - "BIGINT GENERATED ALWAYS AS IDENTITY replaces INTEGER PRIMARY KEY AUTOINCREMENT"
  - "NUMERIC(10,2) replaces REAL for price to preserve exact decimal values"

requirements-completed: [APP-01, APP-02, APP-03, APP-04, APP-05]

# Metrics
duration: 15min
completed: 2026-04-13
---

# Phase 01 Plan 01: Driver Swap + SQL Dialect Rewrite Summary

**pgx/v5 driver added, all 12 SQL query sites converted to Postgres dialect with $N params and RETURNING id, Goose migration file with Postgres DDL created**

## Performance

- **Duration:** ~15 min
- **Started:** 2026-04-13T07:12:00Z
- **Completed:** 2026-04-13T07:27:09Z
- **Tasks:** 2
- **Files modified:** 5

## Accomplishments

- Added pgx/v5 v5.9.1 and goose/v3 v3.27.0 as go.mod dependencies
- Created internal/migrations package with go:embed FS exporting all .sql files
- Created 00001_initial_schema.sql with Postgres DDL (IDENTITY, BOOLEAN, NUMERIC, TIMESTAMPTZ)
- Deleted Init() method from ProductStore (replaced by Goose)
- Rewrote all 12 query sites in product.go: ? -> $N numbered params
- Replaced Exec+LastInsertId with QueryRow+RETURNING id for Create() and AddImage()
- Removed isSold int workaround; bool fields scan natively from Postgres BOOLEAN columns
- Changed is_sold=0/1 to is_sold=false/true in ListAvailable and ListSold queries

## Task Commits

Each task was committed atomically:

1. **Task 1: Swap go.mod dependencies and create Goose migration package** - `01d18ff` (feat)
2. **Task 2: Rewrite product.go SQL for Postgres dialect** - `e480b11` (feat)

## Files Created/Modified

- `go.mod` - Added pgx/v5 v5.9.1, goose/v3 v3.27.0, and their transitive dependencies
- `go.sum` - Updated with new dependency hashes
- `internal/migrations/migrations.go` - New package; exports embed.FS for Goose SQL files
- `internal/migrations/00001_initial_schema.sql` - Postgres DDL for products and images tables with goose Up/Down markers
- `internal/models/product.go` - Rewrote all SQL queries for Postgres; removed Init(); 2 RETURNING id clauses; native bool scanning

## Decisions Made

- Kept go-sqlite3 in go.mod: product_test.go still calls store.Init() which is a compile error; but main code is correct. Test file is updated in Plan 03.
- pgx and goose appear as indirect dependencies until main.go (Plan 02) adds direct imports.
- Did not run `go mod tidy` after adding dependencies because it would remove them (nothing imports them directly yet in this plan).

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

- GOTOOLCHAIN=local incompatible with go.mod requiring 1.26; used GOTOOLCHAIN=auto for all go commands. No impact on output.
- `go mod tidy` after adding packages would remove them (nothing imports them yet). Skipped tidy as planned; packages remain in go.mod as indirect. This is expected and correct for this plan.
- product_test.go calls store.Init() which no longer exists; causes `go vet` error on the test file. This is a known intermediate state — Plan 03 rewrites the test file.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- go.mod has pgx/v5 and goose/v3 ready for Plan 02 (main.go rewiring)
- Migration package exports FS; Plan 02 will call goose.SetBaseFS(migrations.FS) and goose.Up(db, ".")
- product.go is fully Postgres-dialect; compiles correctly against database/sql
- Plan 03 can rewrite product_test.go to use testcontainers (removing the Init() call and sqlite3 import)

---
*Phase: 01-go-build*
*Completed: 2026-04-13*
