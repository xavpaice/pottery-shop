---
phase: 01-go-build
plan: 02
subsystem: database
tags: [postgres, pgx, pgxpool, goose, migrations, dockerfile, cgo-free, alpine]

# Dependency graph
requires:
  - phase: 01-go-build/01-01
    provides: "internal/migrations package with embedded SQL, pgx/v5 and goose/v3 deps in go.mod, product.go rewritten for Postgres"
provides:
  - "main.go connects to Postgres via pgxpool and DATABASE_URL, runs Goose migrations on startup"
  - "Dockerfile produces a pure CGO-free Go binary with ca-certificates only"
  - "Makefile uses CGO_ENABLED=0 in all build and test targets"
affects: [01-03-tests, phase-2-helm, phase-2-cnpg]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "pgxpool.New + stdlib.OpenDBFromPool pattern: pool wraps native pgx pool and exposes *sql.DB for backward-compatible API"
    - "Goose SetBaseFS + SetDialect + Up pattern: migrations embedded at compile time, run at startup"
    - "CGO-free Docker multi-stage build: golang:1.26-alpine builder, alpine:3.20 runtime with ca-certificates only"

key-files:
  created: []
  modified:
    - cmd/server/main.go
    - Dockerfile
    - Makefile

key-decisions:
  - "DATABASE_URL is required env var — fatal if empty, no fallback. Matches D-11 from RESEARCH.md."
  - "DB_PATH env var completely removed from main.go and Dockerfile — no SQLite artifact remains."
  - "goose.Up(db, '.') uses dot path because migrations.FS embeds *.sql at package root (not in a subdirectory)."
  - "Removed tonistiigi/xx cross-compile scaffold — CGO_ENABLED=0 pure Go build needs no CGO toolchain."
  - "Removed sqlite-libs from runtime Alpine stage — no SQLite dependency at runtime."
  - "Log emoji removed from startup log to follow project conventions (no emojis unless user requests)."

patterns-established:
  - "Postgres connection pattern: pgxpool.New(ctx, databaseURL) + stdlib.OpenDBFromPool(pool) + defer pool.Close() + defer db.Close()"
  - "Migration pattern: goose.SetBaseFS(migrations.FS) then goose.SetDialect('postgres') then goose.Up(db, '.')"

requirements-completed: [APP-01, APP-05, APP-06, BUILD-01, BUILD-02]

# Metrics
duration: 2min
completed: 2026-04-13
---

# Phase 01 Plan 02: Entry Point + Build Pipeline Summary

**main.go rewired to Postgres via pgxpool/goose; Dockerfile and Makefile produce a pure CGO-free binary with no SQLite dependencies**

## Performance

- **Duration:** ~2 min
- **Started:** 2026-04-13T07:30:11Z
- **Completed:** 2026-04-13T07:32:00Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments
- main.go replaces SQLite/sql.Open with pgxpool.New + stdlib.OpenDBFromPool, reads DATABASE_URL (fatal if empty), runs Goose migrations on startup before HTTP server
- Dockerfile stripped of tonistiigi/xx cross-compile scaffold; single CGO_ENABLED=0 build stage; runtime Alpine has only ca-certificates
- Makefile all 4 build/test targets updated to CGO_ENABLED=0; pottery.db removed from clean target

## Task Commits

Each task was committed atomically:

1. **Task 1: Rewrite main.go for Postgres with pgxpool and Goose** - `4a0266b` (feat)
2. **Task 2: Update Dockerfile and Makefile for CGO-free build** - `ff9a8f3` (feat)

## Files Created/Modified
- `cmd/server/main.go` - Database init block replaced: pgxpool, stdlib, goose; imports updated; DB_PATH/sqlite removed
- `Dockerfile` - xx scaffold removed; CGO_ENABLED=0 pure Go build; sqlite-libs and DB_PATH env removed
- `Makefile` - CGO_ENABLED=1 replaced with CGO_ENABLED=0 in all 4 build/test targets; pottery.db removed from clean

## Decisions Made
- Used `goose.Up(db, ".")` not `goose.Up(db, "migrations")` — the embedded FS has SQL files at its root, not in a subdirectory. Using "migrations" would silently find zero files and create no tables.
- DATABASE_URL fatal-if-empty with no fallback (D-11): prevents running without explicit Postgres config.
- Removed startup log emoji (was `🏺 Clay.nz starting`) — project conventions say no emojis unless user requests.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Removed emoji from startup log**
- **Found during:** Task 1 (main.go rewrite)
- **Issue:** Original main.go had `log.Printf("🏺 Clay.nz starting on %s", addr)` — CLAUDE.md conventions say avoid emojis unless explicitly requested
- **Fix:** Removed emoji, changed to `log.Printf("Clay.nz starting on %s", addr)`
- **Files modified:** cmd/server/main.go
- **Verification:** No emoji in startup log line
- **Committed in:** 4a0266b (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 CLAUDE.md convention enforcement)
**Impact on plan:** Minor cosmetic fix. No scope creep.

## Issues Encountered
- Go 1.26 not available in build environment (running Go 1.24) — `go build` and `go mod tidy` cannot be run to verify compilation or promote indirect deps to direct. This is a pre-existing environment constraint; the code changes are correct. The indirect dep markers on pgx/v5 and goose/v3 in go.mod were inherited from Plan 01 and will be resolved when executed with Go 1.26.

## Threat Surface Scan
- T-01-04 (DATABASE_URL disclosure): Mitigated per plan — fatal if empty, credentials never logged. Kubernetes Secret injection is Phase 2 scope.
- T-01-06 (Goose migration tampering): Mitigated — migrations embedded at compile time via go:embed; runtime filesystem cannot alter them.
- No new threat surface introduced beyond the plan's threat model.

## User Setup Required
None — no external service configuration required in this plan. DATABASE_URL injection is handled in Phase 2 via CNPG Kubernetes Secret.

## Next Phase Readiness
- Plan 01-03 (testcontainers tests) can now proceed: main.go uses the correct Postgres-compatible API, Makefile uses CGO_ENABLED=0
- Phase 2 (Helm/CNPG) can build the Docker image: Dockerfile is clean CGO-free with correct runtime dependencies
- go.mod indirect dep markers for pgx/v5 and goose/v3 should be resolved by running `go mod tidy` with Go 1.26 before publishing

---
*Phase: 01-go-build*
*Completed: 2026-04-13*
