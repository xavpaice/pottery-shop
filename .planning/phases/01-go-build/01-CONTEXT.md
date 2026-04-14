# Phase 1: Go + Build - Context

**Gathered:** 2026-04-13
**Status:** Ready for planning

<domain>
## Phase Boundary

The Go application connects to Postgres, all SQL is Postgres-compatible, the binary builds without CGO, and integration tests pass against a real Postgres container — all verifiable without a Kubernetes cluster.

Scope: driver swap, SQL dialect fixes, Goose migrations, CGO removal from Dockerfile, testcontainers-go integration tests.
Kubernetes/Helm work is Phase 2 and must not be touched here.

</domain>

<decisions>
## Implementation Decisions

### pgx Integration Style
- **D-01:** Use `pgx/v5/stdlib.OpenDBFromPool` — `ProductStore.DB` stays `*sql.DB`, handlers and middleware are completely untouched. Only SQL strings in `product.go` change.
- **D-02:** `main.go` creates a `pgxpool.Pool`, wraps it with `stdlib.OpenDBFromPool`, passes the resulting `*sql.DB` to `NewProductStore` — same call signature as today.
- **D-03:** No native pgx pool on ProductStore; do not change ProductStore's API.

### SQL Dialect Fixes (all in product.go)
- **D-04:** Replace all `?` placeholders with `$1, $2, ...` numbered params (12 query sites).
- **D-05:** Replace `Exec` + `LastInsertId()` with `QueryRow` + `RETURNING id` on `Create` and `AddImage`.
- **D-06:** Remove SQLite boolean int workaround — scan directly into `bool` fields (Postgres handles it natively).
- **D-07:** Delete `ProductStore.Init()` entirely — replaced by Goose migration.

### Goose Migrations
- **D-08:** Use Goose v3 with `//go:embed` — SQL files live at `internal/migrations/*.sql`.
- **D-09:** Single migration file `00001_initial_schema.sql` with Postgres DDL: `BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY`, `NUMERIC(10,2)` for price, `BOOLEAN` for is_sold, `TIMESTAMPTZ DEFAULT NOW()` for timestamps.
- **D-10:** `main.go` runs `goose.Up(db, "migrations")` after opening the pool and before starting the HTTP server. Fatal on migration failure.

### DATABASE_URL Config
- **D-11:** Read `DATABASE_URL` from env; fatal if empty — no fallback.
- **D-12:** Remove `DB_PATH` env var entirely (no SQLite anywhere).

### Docker Build
- **D-13:** Remove `tonistiigi/xx` CGO cross-compile scaffold from Dockerfile entirely.
- **D-14:** Replace build line with `CGO_ENABLED=0 go build -o /app/clay-server ./cmd/server` — pure Go, no clang/lld.
- **D-15:** Remove `sqlite-libs` from runtime Alpine stage; keep `ca-certificates` (required for TLS to Postgres).

### Integration Tests
- **D-16:** One testcontainers Postgres container per test package, shared via `TestMain` — not per-test. `internal/models` gets a `TestMain` that spins up the container once, runs all tests against it, tears it down.
- **D-17:** `setupTestStore(t)` is rewritten to use the shared Postgres container: runs Goose migrations on a fresh schema per test (use a unique schema name or truncate tables between tests).
- **D-18:** Build tag `//go:build integration` is NOT used — `go test ./...` runs everything. Testcontainers handles the Docker dependency check.
- **D-19:** Existing test helper function signatures (`setupTestStore`, `createSampleProduct`) are preserved — only their internals change.

### Claude's Discretion
- Connection pool config: use `pgxpool.New` with the default pool config from the URL string — no extra env vars for pool size.
- Goose dialect set to `"postgres"` via `goose.SetDialect`.
- Blank import `_ "github.com/mattn/go-sqlite3"` removed from main.go and test files in the same commit as go.mod changes (prevents broken intermediate state).
- `go mod tidy` run after driver swap to clean up unused CGO dependencies.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements
- `.planning/REQUIREMENTS.md` — Full requirement list; Phase 1 requirements are APP-01 through APP-06, BUILD-01, BUILD-02, TEST-01, TEST-02

### Research
- `.planning/research/SUMMARY.md` — Recommended stack (pgx v5.9.1, goose v3), critical code changes with before/after examples, top 5 pitfalls to avoid (LastInsertId silent zero, ? placeholder runtime error, SQLite DDL crash, CNPG secret timing, CGO dangling import)
- `.planning/research/PITFALLS.md` — Detailed pitfall analysis with risk ratings

### Codebase
- `.planning/codebase/ARCHITECTURE.md` — Database schema (exact current SQLite DDL), ProductStore patterns, main.go flow
- `.planning/codebase/STACK.md` — Current dependencies, env vars, Dockerfile details
- `.planning/codebase/TESTING.md` — Existing test structure, helper functions, coverage areas

### Source files to read before implementing
- `internal/models/product.go` — All SQL queries that need dialect changes; Init() to delete
- `cmd/server/main.go` — DB open/init block to replace; env var reading
- `Dockerfile` — CGO scaffold to remove
- `internal/models/product_test.go` — setupTestStore() to rewrite with testcontainers

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `ProductStore` struct with `DB *sql.DB` field — unchanged after migration (stdlib.OpenDBFromPool preserves the interface)
- `NewProductStore(*sql.DB)` constructor — signature unchanged
- All handler files (`internal/handlers/public.go`, `internal/handlers/admin.go`) — untouched entirely
- Middleware layer (`internal/middleware/`) — untouched entirely
- `setupTestStore(t)` and `createSampleProduct(t, store, ...)` test helpers — signature preserved, internals replaced

### Established Patterns
- Raw SQL with named struct fields; scan order matters and must match column order in SELECT
- `database/sql` patterns: `db.QueryRow`, `db.Query`, `rows.Scan`, `db.Exec` — all preserved under stdlib wrapper
- `envOr(key, default)` helper in main.go for config — DATABASE_URL uses `os.Getenv` with fatal (no default makes sense)
- Error propagation: all store methods return `error`; callers check and return 500

### Integration Points
- `main.go:main()` — replace the SQLite open/init block with pgxpool + goose runner
- `internal/models/product.go` — all SQL strings need `?` → `$N` replacement; Init() deleted
- `Dockerfile` builder stage — swap CGO scaffold for simple `CGO_ENABLED=0 go build`
- `go.mod` — remove go-sqlite3, add pgx/v5 and goose/v3
- `internal/models/product_test.go` — rewrite setupTestStore() with testcontainers TestMain

</code_context>

<specifics>
## Specific Ideas

- Research gives exact before/after code for the main.go pool init block, RETURNING id pattern, and DDL — use it directly
- The blank import `_ "github.com/mattn/go-sqlite3"` appears in both main.go and product_test.go — must be removed from both in the same commit as go.mod change
- Test table truncation between tests is preferred over per-test schema creation for simplicity (Goose runs once in TestMain, each test truncates tables before inserting fixtures)

</specifics>

<deferred>
## Deferred Ideas

- Local dev docker-compose.yml — not selected for discussion; Claude's discretion to add if it improves developer experience (not a plan requirement)
- Connection pool tuning via env vars — not needed for a hobby pottery shop
- CNPG secret timing mitigation (init container vs startup retry) — Phase 2 decision (noted in SUMMARY.md open questions)
- SQLite PVC / uploads path decision — Phase 2 decision (noted in SUMMARY.md open questions)

</deferred>

---

*Phase: 01-go-build*
*Context gathered: 2026-04-13*
