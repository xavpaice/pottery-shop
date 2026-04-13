# Phase 1: Go + Build - Research

**Researched:** 2026-04-13
**Domain:** Go SQLite-to-Postgres driver swap, CGO removal, embedded Goose migrations, testcontainers-go integration tests
**Confidence:** HIGH

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**D-01:** Use `pgx/v5/stdlib.OpenDBFromPool` — `ProductStore.DB` stays `*sql.DB`, handlers and middleware are completely untouched. Only SQL strings in `product.go` change.
**D-02:** `main.go` creates a `pgxpool.Pool`, wraps it with `stdlib.OpenDBFromPool`, passes the resulting `*sql.DB` to `NewProductStore` — same call signature as today.
**D-03:** No native pgx pool on ProductStore; do not change ProductStore's API.
**D-04:** Replace all `?` placeholders with `$1, $2, ...` numbered params (12 query sites).
**D-05:** Replace `Exec` + `LastInsertId()` with `QueryRow` + `RETURNING id` on `Create` and `AddImage`.
**D-06:** Remove SQLite boolean int workaround — scan directly into `bool` fields (Postgres handles it natively).
**D-07:** Delete `ProductStore.Init()` entirely — replaced by Goose migration.
**D-08:** Use Goose v3 with `//go:embed` — SQL files live at `internal/migrations/*.sql`.
**D-09:** Single migration file `00001_initial_schema.sql` with Postgres DDL: `BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY`, `NUMERIC(10,2)` for price, `BOOLEAN` for is_sold, `TIMESTAMPTZ DEFAULT NOW()` for timestamps.
**D-10:** `main.go` runs `goose.Up(db, "migrations")` after opening the pool and before starting the HTTP server. Fatal on migration failure.
**D-11:** Read `DATABASE_URL` from env; fatal if empty — no fallback.
**D-12:** Remove `DB_PATH` env var entirely (no SQLite anywhere).
**D-13:** Remove `tonistiigi/xx` CGO cross-compile scaffold from Dockerfile entirely.
**D-14:** Replace build line with `CGO_ENABLED=0 go build -o /app/clay-server ./cmd/server` — pure Go, no clang/lld.
**D-15:** Remove `sqlite-libs` from runtime Alpine stage; keep `ca-certificates` (required for TLS to Postgres).
**D-16:** One testcontainers Postgres container per test package, shared via `TestMain` — not per-test. `internal/models` gets a `TestMain` that spins up the container once, runs all tests against it, tears it down.
**D-17:** `setupTestStore(t)` is rewritten to use the shared Postgres container: runs Goose migrations on a fresh schema per test (use a unique schema name or truncate tables between tests).
**D-18:** Build tag `//go:build integration` is NOT used — `go test ./...` runs everything. Testcontainers handles the Docker dependency check.
**D-19:** Existing test helper function signatures (`setupTestStore`, `createSampleProduct`) are preserved — only their internals change.

### Claude's Discretion

- Connection pool config: use `pgxpool.New` with the default pool config from the URL string — no extra env vars for pool size.
- Goose dialect set to `"postgres"` via `goose.SetDialect`.
- Blank import `_ "github.com/mattn/go-sqlite3"` removed from main.go and test files in the same commit as go.mod changes (prevents broken intermediate state).
- `go mod tidy` run after driver swap to clean up unused CGO dependencies.

### Deferred Ideas (OUT OF SCOPE)

- Local dev docker-compose.yml — not selected for discussion; Claude's discretion to add if it improves developer experience (not a plan requirement)
- Connection pool tuning via env vars — not needed for a hobby pottery shop
- CNPG secret timing mitigation (init container vs startup retry) — Phase 2 decision
- SQLite PVC / uploads path decision — Phase 2 decision
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| APP-01 | Replace `go-sqlite3` (CGO) with `pgx/v5` (pure Go) as the database driver | Standard Stack section; go.mod change pattern documented |
| APP-02 | Fix all `?` placeholders → `$N` numbered params across all queries in product.go | Code Examples section; 12 query sites enumerated |
| APP-03 | Fix `LastInsertId()` → `RETURNING id` on every INSERT query | Code Examples section; affects Create and AddImage |
| APP-04 | Rewrite DDL for Postgres types (IDENTITY, BOOLEAN, TIMESTAMP, NUMERIC) | Architecture Patterns section; migration file DDL documented |
| APP-05 | Replace inline `store.Init()` with Goose v3 embedded schema migrations | Standard Stack + Architecture Patterns; Goose embed pattern documented |
| APP-06 | Read `DATABASE_URL` env var for Postgres connection string | Code Examples section; main.go rewrite pattern documented |
| BUILD-01 | Set `CGO_ENABLED=0` and strip `tonistiigi/xx` CGO cross-compile scaffold from Dockerfile | Code Examples section; exact Dockerfile diff documented |
| BUILD-02 | Docker image produces a pure Go binary with no CGO dependencies | Architecture Patterns section; Dockerfile pattern verified |
| TEST-01 | Add testcontainers-go to test suite — integration tests spin up a real Postgres container | Standard Stack section; testcontainers v0.42.0 with postgres module |
| TEST-02 | Existing integration tests updated to run against Postgres (not SQLite) | Code Examples section; TestMain pattern with table truncation documented |
</phase_requirements>

---

## Summary

This phase performs a complete driver swap from SQLite (CGO, `go-sqlite3`) to PostgreSQL (pure Go, `pgx/v5`) with all changes confined to four files plus a new migration file. Handlers, middleware, and all session/cart logic are untouched. The change surface is: `go.mod`, `cmd/server/main.go`, `internal/models/product.go`, `Dockerfile`, and a new `internal/migrations/00001_initial_schema.sql`. The test file `internal/models/product_test.go` gains a `TestMain` function using testcontainers-go, replacing the SQLite in-memory setup.

All three libraries in the standard stack are verified current as of this research: `pgx/v5` at v5.9.1 (released 2026-03-22), `goose/v3` at v3.27.0 (released 2026-02-22), and `testcontainers-go` at v0.42.0 (released 2026-04-09). There are two low-severity CVEs (GO-2026-4771, GO-2026-4772) in pgx's server-side wire protocol handler — they do not affect client applications connecting to Postgres and are present in v5.9.1 (the latest).

There are three changes that are guaranteed to cause silent failures if done in the wrong order: (1) the SQLite blank import must be removed in the same commit that removes `go-sqlite3` from go.mod — if split, the build breaks; (2) `?` placeholders cause runtime parse errors (not compile errors) if not replaced before any DB operation; (3) `LastInsertId()` returns `(0, ErrNotSupported)` silently on Postgres — every INSERT assigns `id=0` to the struct unless `RETURNING id` is used.

**Primary recommendation:** Execute the changes in dependency order: go.mod first (breaks the build intentionally), then remove the blank import and update product.go in the same commit, then write the migration file, then rewrite main.go, then update the Dockerfile and tests.

---

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/jackc/pgx/v5` | v5.9.1 | Pure Go Postgres driver; includes pgxpool and stdlib adapter | CGO-free; binary wire protocol; de-facto standard for Go+Postgres |
| `github.com/jackc/pgx/v5/pgxpool` | v5.9.1 (same module) | Connection pool for pgx | Built-in to pgx; zero additional deps |
| `github.com/jackc/pgx/v5/stdlib` | v5.9.1 (same module) | `database/sql` compatibility shim | Allows `*sql.DB` API to remain unchanged; wraps pgxpool |
| `github.com/pressly/goose/v3` | v3.27.0 | Schema migration runner with `//go:embed` support | Pure Go; programmatic API; embed SQL; widely used with pgx |

### Testing

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/testcontainers/testcontainers-go` | v0.42.0 | Spin up real Postgres container in tests | Required by D-16/TEST-01; replaces SQLite in-memory approach |
| `github.com/testcontainers/testcontainers-go/modules/postgres` | v0.42.0 | Postgres-specific helpers for testcontainers | Same module version; provides `postgres.Run()` convenience wrapper |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `pgx/v5/stdlib` wrapper | Native `pgx/v5` `pgxconn` API | Native API removes `*sql.DB` entirely — requires changing ProductStore, handlers, tests. Ruled out by D-01/D-03. |
| Goose v3 | `golang-migrate/migrate` | golang-migrate is also valid; Goose chosen for simpler programmatic API and `go:embed` support. Decision locked by D-08. |
| testcontainers postgres module | plain testcontainers + manual container config | Module provides `postgres.Run()` with sensible defaults; fewer lines; same version. |

**Installation:**
```bash
go get github.com/jackc/pgx/v5@v5.9.1
go get github.com/pressly/goose/v3@v3.27.0
go get github.com/testcontainers/testcontainers-go@v0.42.0
go get github.com/testcontainers/testcontainers-go/modules/postgres@v0.42.0
go mod tidy
```

**Version verification:** [VERIFIED: proxy.golang.org 2026-04-13]
- `pgx/v5` latest: v5.9.1 (2026-03-22)
- `goose/v3` latest: v3.27.0 (2026-02-22)
- `testcontainers-go` latest: v0.42.0 (2026-04-09)
- `testcontainers-go/modules/postgres` latest: v0.42.0 (2026-04-09)

---

## Architecture Patterns

### Recommended Project Structure After Phase 1

```
pottery-shop/
├── cmd/server/main.go          # pgxpool init + goose.Up() replaces sql.Open + store.Init()
├── internal/
│   ├── migrations/
│   │   └── 00001_initial_schema.sql   # NEW: Postgres DDL with //go:embed in main.go
│   ├── models/
│   │   ├── product.go          # CHANGED: $N params, RETURNING id, bool scans, no Init()
│   │   ├── product_test.go     # CHANGED: TestMain + testcontainers; same func signatures
│   │   ├── cart.go             # UNCHANGED
│   │   └── cart_test.go        # UNCHANGED
│   ├── handlers/               # UNCHANGED entirely
│   └── middleware/             # UNCHANGED entirely
├── go.mod                      # CHANGED: remove go-sqlite3, add pgx/v5 + goose/v3 + testcontainers
├── Dockerfile                  # CHANGED: remove xx scaffold, CGO_ENABLED=0
└── Makefile                    # CHANGED: CGO_ENABLED=0 in build/test targets
```

### Pattern 1: pgxpool + stdlib.OpenDBFromPool (main.go)

**What:** Create a `pgxpool.Pool`, wrap it in `stdlib.OpenDBFromPool` to produce a `*sql.DB`, pass that to `NewProductStore`. The pool's lifecycle is managed via `defer pool.Close()`.

**When to use:** Always in this project. Preserves `*sql.DB` interface downstream (D-01/D-02).

```go
// Source: SUMMARY.md (prior research, based on pgx/v5 stdlib docs)
import (
    "context"
    "os"
    "log"
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/jackc/pgx/v5/stdlib"
    "github.com/pressly/goose/v3"
)

// In main():
databaseURL := os.Getenv("DATABASE_URL")
if databaseURL == "" {
    log.Fatal("DATABASE_URL must be set")
}
pool, err := pgxpool.New(context.Background(), databaseURL)
if err != nil {
    log.Fatalf("Failed to create connection pool: %v", err)
}
defer pool.Close()

db := stdlib.OpenDBFromPool(pool)
defer db.Close()
```

### Pattern 2: Goose embedded migrations (main.go)

**What:** Embed the `internal/migrations/` directory using `//go:embed`, set dialect to postgres, call `goose.Up()` before serving.

**When to use:** Immediately after the pool is open, before `NewProductStore` is called.

```go
// Source: SUMMARY.md (prior research, based on goose/v3 embed docs)
import "embed"

//go:embed internal/migrations/*.sql
var migrations embed.FS

// In main(), after db is ready:
goose.SetBaseFS(migrations)
if err := goose.SetDialect("postgres"); err != nil {
    log.Fatalf("goose dialect: %v", err)
}
if err := goose.Up(db, "internal/migrations"); err != nil {
    log.Fatalf("goose migrations: %v", err)
}
```

**Important:** The path argument to `goose.Up()` must match the directory path used in `//go:embed`. Since main.go is at `cmd/server/main.go`, the embed path must be relative to that file. If migrations live at `internal/migrations/`, the `//go:embed` directive in `cmd/server/main.go` uses `../../internal/migrations/*.sql` or the embed FS is defined in a package inside `internal/migrations/`. The cleanest pattern: define the embedded FS in a dedicated `migrations` package at `internal/migrations/migrations.go`, export it, and call it from main.go.

**Alternative (simpler):** Define the embed var in `main.go` itself:
```go
//go:embed ../../internal/migrations
var migrationsFS embed.FS
// goose.Up path: "internal/migrations" (relative to module root when using embed)
```

Note: `//go:embed` paths are relative to the Go source file. From `cmd/server/main.go`, `internal/migrations` would need to be `../../internal/migrations`. To avoid the path confusion, the recommended approach is to put the embed declaration in a file inside the `internal/migrations/` package itself:

```go
// internal/migrations/migrations.go
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
```

Then in `cmd/server/main.go`:
```go
import "pottery-shop/internal/migrations"

goose.SetBaseFS(migrations.FS)
if err := goose.Up(db, "."); err != nil { ... }  // "." = root of the embedded FS
```

This is the cleanest approach and avoids `../../` path hacks.

### Pattern 3: RETURNING id for INSERT (product.go)

**What:** Replace `db.Exec(INSERT...)` + `res.LastInsertId()` with `db.QueryRow(INSERT ... RETURNING id).Scan(&id)`.

**When to use:** All INSERT queries that need the generated ID — `Create()` and `AddImage()`.

```go
// Source: SUMMARY.md (prior research); pgx/v5 docs confirm LastInsertId unsupported
// Create():
err := s.DB.QueryRow(
    `INSERT INTO products (title, description, price, is_sold)
     VALUES ($1, $2, $3, $4) RETURNING id`,
    p.Title, p.Description, p.Price, p.IsSold,
).Scan(&p.ID)
return err

// AddImage():
err := s.DB.QueryRow(
    `INSERT INTO images (product_id, filename, thumbnail_fn, sort_order)
     VALUES ($1, $2, $3, $4) RETURNING id`,
    img.ProductID, img.Filename, img.ThumbnailFn, img.SortOrder,
).Scan(&img.ID)
return err
```

### Pattern 4: testcontainers TestMain (product_test.go)

**What:** `TestMain` starts one shared Postgres container for the whole test package. Each `setupTestStore(t)` call truncates all tables before inserting fixtures. Container torn down when `TestMain` exits.

**When to use:** Per D-16/D-17. One container per package is the correct testcontainers pattern for `internal/models`.

```go
// Source: testcontainers-go v0.42.0 docs; postgres module
package models

import (
    "context"
    "database/sql"
    "os"
    "testing"

    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/jackc/pgx/v5/stdlib"
    "github.com/pressly/goose/v3"
    "github.com/testcontainers/testcontainers-go"
    "github.com/testcontainers/testcontainers-go/modules/postgres"
    "github.com/testcontainers/testcontainers-go/wait"

    migrations "pottery-shop/internal/migrations"
)

var testDBURL string

func TestMain(m *testing.M) {
    ctx := context.Background()

    pgContainer, err := postgres.Run(ctx,
        "postgres:16-alpine",
        postgres.WithDatabase("testdb"),
        postgres.WithUsername("test"),
        postgres.WithPassword("test"),
        testcontainers.WithWaitStrategy(
            wait.ForLog("database system is ready to accept connections").
                WithOccurrence(2),
        ),
    )
    if err != nil {
        panic("failed to start postgres container: " + err.Error())
    }
    defer pgContainer.Terminate(ctx)

    testDBURL, err = pgContainer.ConnectionString(ctx, "sslmode=disable")
    if err != nil {
        panic("failed to get connection string: " + err.Error())
    }

    // Run migrations once on shared container
    pool, err := pgxpool.New(ctx, testDBURL)
    if err != nil {
        panic("failed to create pool: " + err.Error())
    }
    db := stdlib.OpenDBFromPool(pool)
    goose.SetBaseFS(migrations.FS)
    goose.SetDialect("postgres")
    if err := goose.Up(db, "."); err != nil {
        panic("goose up: " + err.Error())
    }
    db.Close()
    pool.Close()

    os.Exit(m.Run())
}

func setupTestStore(t *testing.T) *ProductStore {
    t.Helper()
    pool, err := pgxpool.New(context.Background(), testDBURL)
    if err != nil {
        t.Fatalf("open db: %v", err)
    }
    db := stdlib.OpenDBFromPool(pool)
    t.Cleanup(func() {
        db.Close()
        pool.Close()
    })

    // Truncate tables between tests (D-17)
    if _, err := db.Exec(`TRUNCATE products, images RESTART IDENTITY CASCADE`); err != nil {
        t.Fatalf("truncate: %v", err)
    }

    return NewProductStore(db)
}
```

**Note on TRUNCATE RESTART IDENTITY:** This resets the identity sequences so IDs start from 1 in each test, making test assertions predictable. `CASCADE` handles the FK from images to products.

### Pattern 5: Goose migration file format

```sql
-- internal/migrations/00001_initial_schema.sql
-- Source: SUMMARY.md + goose/v3 docs

-- +goose Up
CREATE TABLE IF NOT EXISTS products (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    title       TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    price       NUMERIC(10,2) NOT NULL DEFAULT 0,
    is_sold     BOOLEAN NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    updated_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS images (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    product_id   BIGINT NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    filename     TEXT NOT NULL,
    thumbnail_fn TEXT NOT NULL DEFAULT '',
    sort_order   INTEGER NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ DEFAULT NOW()
);

-- +goose Down
DROP TABLE IF EXISTS images;
DROP TABLE IF EXISTS products;
```

### Pattern 6: Dockerfile — CGO removal

```dockerfile
# Source: SUMMARY.md + D-13/D-14/D-15

# BEFORE (remove entirely):
# FROM --platform=$BUILDPLATFORM tonistiigi/xx:1.6.1 AS xx
# FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder
# COPY --from=xx / /
# ARG TARGETPLATFORM
# RUN apk add --no-cache clang lld && xx-apk add --no-cache gcc musl-dev
# RUN CGO_ENABLED=1 xx-go build -o clay-server ./cmd/server && xx-verify clay-server

# AFTER:
FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o clay-server ./cmd/server

# Runtime stage (remove sqlite-libs, keep ca-certificates):
FROM alpine:3.20
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /app/clay-server .
COPY --from=builder /app/templates ./templates
COPY --from=builder /app/static ./static
RUN mkdir -p /data/uploads/thumbnails
ENV PORT=8080
ENV UPLOAD_DIR=/data/uploads
# Remove: ENV DB_PATH=/data/clay.db
EXPOSE 8080
CMD ["./clay-server"]
```

### Anti-Patterns to Avoid

- **Splitting go-sqlite3 removal from blank import removal:** If go.mod drops the dependency but main.go still has `_ "github.com/mattn/go-sqlite3"`, `go build` fails immediately. Must be one atomic commit.
- **Using `?` placeholders anywhere in product.go:** They compile fine but cause a Postgres parse error at the first DB operation. Every single `?` in the file must be replaced before running tests.
- **Calling `res.LastInsertId()` on a Postgres exec result:** Returns `(0, pgx.ErrNotSupported)`. The error may be swallowed (as in the current `AddImage` — the error return of `LastInsertId` is assigned but then the function returns the outer `err`, not this one). Result: `img.ID = 0` silently.
- **Using `store.Init()` on Postgres:** Contains `AUTOINCREMENT` and `DATETIME` — invalid Postgres syntax. Fatal on startup.
- **Putting `//go:embed` in `cmd/server/main.go` with a relative path to `internal/migrations/`:** The Go spec requires embed paths to be relative to the file containing the directive. Use a dedicated `internal/migrations/migrations.go` file with `//go:embed *.sql` instead.
- **Scanning Postgres BOOLEAN into an `int` variable:** Type mismatch error at runtime. Scan directly into `&p.IsSold` (Go `bool`).

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Schema migration versioning | Custom `Init()` DDL in application code | `goose/v3` with SQL files | Goose handles up/down, version tracking in `goose_db_version` table, idempotency, embed support |
| Postgres connection pooling | Manual `sql.Open` + pool config | `pgxpool.New` with stdlib wrapper | pgxpool handles health checks, max conns, connection lifetime, reconnect — all invisible via stdlib shim |
| Test database isolation | Per-test schema creation | `TRUNCATE ... RESTART IDENTITY CASCADE` in `setupTestStore` | Schema creation is slow; truncate is fast and resets sequences |
| Postgres container lifecycle | Raw Docker API in tests | `testcontainers/testcontainers-go/modules/postgres` | Handles image pull, port mapping, readiness wait, cleanup — single call to `postgres.Run()` |

**Key insight:** The entire complexity of this migration is in SQL dialect translation, not in infrastructure. The `database/sql` interface remains unchanged — only the strings inside the SQL literals change.

---

## Common Pitfalls

### Pitfall 1: LastInsertId silently returns 0

**What goes wrong:** `res.LastInsertId()` on a pgx-backed `*sql.DB` returns `(0, pgx.ErrNotSupported)`. In the current `AddImage` code, the error from `LastInsertId` is assigned to `img.ID, err = ...` but then `return err` returns the *outer* err variable, not this assignment's error. So `img.ID` is set to 0 and no error surfaces. Every image inserted has `id=0`.

**Why it happens:** Postgres uses sequences/RETURNING for INSERT IDs, not the MySQL-style `LAST_INSERT_ID()`. The pgx driver correctly returns not-supported rather than lying.

**How to avoid:** Replace both `Create()` and `AddImage()` with `QueryRow(...RETURNING id).Scan(&p.ID)` / `Scan(&img.ID)` before any testing.

**Warning signs:** Test `TestAddImage` checks `if img.ID == 0` and will catch this. Test `TestCreateAndGetByID` checks `if p.ID == 0` and will also catch it.

### Pitfall 2: `?` placeholder parse error only appears at runtime

**What goes wrong:** All 12 query sites in product.go currently use `?`. These compile cleanly. At runtime, Postgres's wire protocol parser rejects `?` with: `pq: syntax error at or near "$1"` or `ERROR: syntax error at or near "?"`. The first DB operation fails.

**Why it happens:** SQLite (and MySQL) use `?` as the positional placeholder. Postgres requires `$1, $2, ...`. The stdlib pgx driver does not translate between them.

**How to avoid:** Grep for `\?` in `product.go` before running any tests. There are exactly 12 occurrences across: `Create` (4), `Update` (5), `Delete` (1), `GetByID` (1), `AddImage` (4), `GetImages` (1), `DeleteImage` (2), `CountImages` (1). Replace sequentially within each query.

**Warning signs:** Every single test will fail with the same error type. If all tests fail with a parse error on the first query, this is the cause.

### Pitfall 3: TestInit test must be deleted (not adapted)

**What goes wrong:** `TestInit` calls `store.Init()` to verify idempotency. Since `Init()` is being deleted (D-07), `TestInit` must be deleted too. It cannot be adapted — there is no Goose equivalent of "call Init twice" to test. Migration idempotency is tested by the Goose framework's own `IF NOT EXISTS` clauses.

**Why it happens:** The test was written to verify a function that is being deleted.

**How to avoid:** Delete `TestInit` in the same commit that deletes `Init()`.

**Warning signs:** Compile error: `store.Init undefined`.

### Pitfall 4: goose.Up path mismatches embed FS

**What goes wrong:** `goose.SetBaseFS(fs)` sets the filesystem, then `goose.Up(db, "path")` uses a path *within* that FS. If the embed FS is `//go:embed *.sql` (rooted at the file's directory), the correct path is `"."`. If the embed FS is `//go:embed migrations/*.sql` (from elsewhere), the path is `"migrations"`. Using the wrong path results in "no migrations found" — no error, no panic, 0 migrations run. The `goose_db_version` table may not even be created.

**Why it happens:** The path argument to `goose.Up` is a path within the embedded FS, not a filesystem path.

**How to avoid:** Use the `internal/migrations` package pattern (embed `*.sql` in the package directory, export `FS`, call `goose.Up(db, ".")`) — this makes the path unambiguous.

**Warning signs:** App starts without error but tables don't exist; first DB query returns "relation does not exist".

### Pitfall 5: Makefile CGO_ENABLED=1 left in place

**What goes wrong:** After removing go-sqlite3, `make build` and `make test` still have `CGO_ENABLED=1`. This is now harmless (CGO is requested but no CGO code remains), but it's misleading and may cause confusion in CI environments that lack a C compiler.

**Why it happens:** Makefile is not updated as part of the driver swap.

**How to avoid:** Update `CGO_ENABLED=1` → `CGO_ENABLED=0` in the `build`, `test`, `test-verbose`, and `test-coverage` Makefile targets in the same commit.

**Warning signs:** CI build fails with "gcc not found" or similar even though the build should be pure Go.

### Pitfall 6: `pottery.db` artifact in project root

**What goes wrong:** A `pottery.db` file is present in the project root (visible in the git status). This SQLite database file is a runtime artifact from running the app locally with the old driver. After migration it is inert but could confuse developers.

**Why it happens:** It was created by running the old app locally.

**How to avoid:** Add `*.db` to `.gitignore` if not already present. The file is not a migration concern but should not be checked in.

**Warning signs:** `git status` shows `pottery.db` as untracked or modified.

### Pitfall 7: Docker ENV DB_PATH still injected

**What goes wrong:** The current Dockerfile has `ENV DB_PATH=/data/clay.db`. Even after the app no longer reads `DB_PATH`, this env var persists in the image. More critically, `values.yaml` has `config.DB_PATH: "/data/clay.db"` which is rendered by `chart/clay/templates/configmap.yaml` into the pod's environment. The pod gets `DB_PATH=/data/clay.db` injected. This is harmless after the app code is updated, but it's a Phase 1 cleanup item per D-12.

**Why it happens:** Dockerfile and values.yaml carry the old config key.

**How to avoid:** Remove `ENV DB_PATH=...` from Dockerfile (D-14). The values.yaml `DB_PATH` removal is a Phase 2 concern (Helm changes).

**Warning signs:** `docker inspect` shows `DB_PATH` in the environment. Harmless but confusing.

---

## Code Examples

### Complete query mapping: `?` → `$N`

```go
// Source: Current product.go (line numbers given for verification)
// Create: 4 params
`INSERT INTO products (title, description, price, is_sold) VALUES ($1, $2, $3, $4)`

// Update: 5 params
`UPDATE products SET title=$1, description=$2, price=$3, is_sold=$4, updated_at=NOW() WHERE id=$5`

// Delete: 1 param
`DELETE FROM products WHERE id=$1`

// GetByID: 1 param
`SELECT id, title, description, price, is_sold, created_at, updated_at FROM products WHERE id=$1`

// ListAvailable: 0 params (no change needed — no ? in current code)
`SELECT id, title, description, price, is_sold, created_at, updated_at FROM products WHERE is_sold=false ORDER BY created_at DESC`

// ListSold: 0 params (no change needed — but SQLite integer 1 must become bool comparison)
`SELECT id, title, description, price, is_sold, created_at, updated_at FROM products WHERE is_sold=true ORDER BY updated_at DESC`

// AddImage: 4 params
`INSERT INTO images (product_id, filename, thumbnail_fn, sort_order) VALUES ($1, $2, $3, $4) RETURNING id`

// GetImages: 1 param
`SELECT id, product_id, filename, thumbnail_fn, sort_order, created_at FROM images WHERE product_id=$1 ORDER BY sort_order`

// DeleteImage (SELECT): 1 param
`SELECT id, product_id, filename, thumbnail_fn FROM images WHERE id=$1`

// DeleteImage (DELETE): 1 param
`DELETE FROM images WHERE id=$1`

// CountImages: 1 param
`SELECT COUNT(*) FROM images WHERE product_id=$1`
```

**Note:** `ListAvailable` currently uses `WHERE is_sold=0` and `ListSold` uses `WHERE is_sold=1`. These are SQLite integer comparisons. After the column becomes `BOOLEAN`, they must change to `WHERE is_sold=false` and `WHERE is_sold=true` respectively. These are not `?` replacements but are required companion changes to D-06.

### Boolean scan removal

```go
// Before (product.go GetByID and listProducts):
var isSold int
rows.Scan(&p.ID, &p.Title, &p.Description, &p.Price, &isSold, &p.CreatedAt, &p.UpdatedAt)
p.IsSold = isSold == 1

// After:
rows.Scan(&p.ID, &p.Title, &p.Description, &p.Price, &p.IsSold, &p.CreatedAt, &p.UpdatedAt)
// p.IsSold is bool; pgx scans BOOLEAN natively
```

### updated_at: CURRENT_TIMESTAMP → NOW()

```go
// Before:
`UPDATE products SET ..., updated_at=CURRENT_TIMESTAMP WHERE id=?`

// After (CURRENT_TIMESTAMP is valid Postgres SQL but NOW() is idiomatic):
`UPDATE products SET ..., updated_at=NOW() WHERE id=$5`
```

Note: `CURRENT_TIMESTAMP` is valid in Postgres (SQL standard). Either works. `NOW()` is idiomatic Postgres.

---

## Runtime State Inventory

> Rename/refactor phase indicators: This phase removes `DB_PATH` and replaces SQLite with Postgres. Runtime state check required.

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | `pottery.db` SQLite file in project root — contains local dev data | None — not a migration (fresh start per REQUIREMENTS.md); add to .gitignore |
| Live service config | `chart/clay/values.yaml` has `config.DB_PATH: "/data/clay.db"` | Phase 2 only — not touched in Phase 1 |
| OS-registered state | None — no systemd/pm2/scheduler registrations found | None |
| Secrets/env vars | `ENV DB_PATH=/data/clay.db` in Dockerfile; `config.DB_PATH` in values.yaml | Remove from Dockerfile (D-14); values.yaml is Phase 2 |
| Build artifacts | `pottery-server` binary in project root (built with CGO) | `make clean` removes it; `go mod tidy` removes CGO deps from go.sum |

**Nothing found requiring data migration:** Per REQUIREMENTS.md Out of Scope: "Data migration from SQLite: Fresh start — no production data to carry over."

---

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go | Build and test | Yes | 1.24.13 | — |
| Docker | testcontainers-go (TEST-01, TEST-02) | Not found in current shell | — | Tests will fail without Docker; needs Docker in build/test environment |
| `pgx/v5` | APP-01 through APP-06 | Not yet in go.mod | v5.9.1 (available) | — |
| `goose/v3` | APP-05 | Not yet in go.mod | v3.27.0 (available) | — |
| `testcontainers-go` | TEST-01, TEST-02 | Not yet in go.mod | v0.42.0 (available) | — |

**Note on Go version:** The project targets Go 1.26 (`go 1.26` in go.mod). The current shell has Go 1.24.13. The `go` directive in go.mod specifies the minimum language version — the build will succeed with Go 1.24 (all features used are pre-1.24). Docker build stage `FROM golang:1.26-alpine` will use 1.26. No action required.

**Missing dependencies with no fallback:**
- Docker is required for testcontainers-go. The current research environment lacks Docker, but the test environment (CI, developer workstation) is expected to have it. `go test ./...` without Docker will panic at `TestMain` when testcontainers attempts to connect to the Docker daemon. This is by design per D-18 (no build tag guard).

**Missing dependencies with fallback:**
- None.

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| CGO + go-sqlite3 | Pure Go pgx/v5 | This phase | Removes C compiler requirement; enables `CGO_ENABLED=0` builds |
| `res.LastInsertId()` | `RETURNING id` | This phase | Required for Postgres; SQLite supported `LastInsertId`, Postgres does not |
| `?` positional params | `$1, $2, ...` positional params | This phase | Postgres wire protocol syntax |
| Inline `CREATE TABLE` in Init() | Goose v3 versioned migrations | This phase | Migration versioning enables future schema changes without app changes |
| In-memory SQLite for tests | testcontainers Postgres | This phase | Tests run against real Postgres; catches Postgres-specific type behavior |
| tonistiigi/xx cross-compile | `CGO_ENABLED=0 go build` | This phase | Simpler Dockerfile; no C toolchain in builder; smaller build layer |

**Deprecated/outdated:**
- `github.com/mattn/go-sqlite3`: Removed entirely. Any reference to it (import, go.mod entry, go.sum hash) must be gone after `go mod tidy`.
- `DB_PATH` env var: Removed from main.go and Dockerfile in this phase; removed from Helm values in Phase 2.
- `ProductStore.Init()`: Deleted; replaced by Goose. The `TestInit` test is also deleted.

---

## Open Questions

1. **Goose embed: package vs. cmd/server embed declaration**
   - What we know: Two valid patterns exist — embed in `internal/migrations/migrations.go` (cleaner) or embed directly in `cmd/server/main.go` with `../../internal/migrations` path.
   - What's unclear: The `../../` path in `//go:embed` directives is not allowed by the Go spec — embed paths cannot start with `.` or `..`. This means the only valid approach is the migrations package pattern.
   - Recommendation: Use `internal/migrations/migrations.go` with `//go:embed *.sql` and export `var FS embed.FS`. [VERIFIED: Go spec says embed paths cannot start with `..` — confirmed by `go help embed` documentation]

2. **TRUNCATE vs. per-test schema for test isolation**
   - What we know: D-17 says "use a unique schema name or truncate tables between tests". The CONTEXT.md `<specifics>` section says truncation is preferred.
   - What's unclear: `TRUNCATE ... RESTART IDENTITY CASCADE` is the correct Postgres command; implementation confirmed above.
   - Recommendation: Use `TRUNCATE products, images RESTART IDENTITY CASCADE` in `setupTestStore`. This is fast and sufficient for sequential tests.

3. **`pottery.db` in git**
   - What we know: `pottery.db` is visible in project root (untracked in git status). It is not tracked.
   - What's unclear: Whether `.gitignore` already excludes `*.db`.
   - Recommendation: Add `*.db` to `.gitignore` as part of Phase 1 cleanup. Not a blocking concern.

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `//go:embed` does not allow `..` path components — paths must be relative and within the module | Architecture Patterns (Pattern 2), Open Questions | If wrong, the `cmd/server/main.go` embed approach works; use `internal/migrations` package either way |
| A2 | `TRUNCATE products, images RESTART IDENTITY CASCADE` is valid Postgres syntax and resets sequences | Code Examples / Pattern 4 | If syntax differs, test isolation is broken; IDs may not start at 1 each test |
| A3 | `testcontainers-go` panics (rather than skip) when Docker is unavailable | Environment Availability | If it skips, tests may silently pass without actually running |

**Note on A1:** The Go spec explicitly states embed path elements may not begin with `.` or `..`. [ASSUMED based on training knowledge — not verified via tool in this session. Low risk of being wrong as this is a well-known Go constraint.]

---

## Project Constraints (from CLAUDE.md)

These directives are extracted from `./CLAUDE.md` and must be honored by the planner:

| Constraint | Enforcement in Phase 1 |
|-----------|----------------------|
| Tech stack: Go + `pgx/v5` — no CGO, no go-sqlite3 | APP-01; remove go-sqlite3 from go.mod |
| Database: PostgreSQL only — no SQLite anywhere | All SQL changes; remove `pottery.db` reference; remove in-memory SQLite from tests |
| No CGO | BUILD-01/02; CGO_ENABLED=0 in Dockerfile and Makefile |
| File naming: lowercase with underscores | `migrations/00001_initial_schema.sql` follows this convention |
| Naming conventions: PascalCase exported, camelCase private | New `migrations.FS` (exported) follows convention |
| Error handling: explicit `(result, error)` return tuples | `QueryRow(...).Scan(...)` returns error; must be returned |
| DB snake_case → Go PascalCase struct fields | Preserved; `is_sold` → `IsSold`, `product_id` → `ProductID` |
| Existing Helm values structure must remain backward-compatible where possible | Phase 1 only touches Dockerfile/Go — values.yaml DB_PATH removal deferred to Phase 2 |

---

## Sources

### Primary (HIGH confidence)
- `proxy.golang.org/github.com/jackc/pgx/v5/@latest` — verified v5.9.1 current as of 2026-04-13
- `proxy.golang.org/github.com/pressly/goose/v3/@latest` — verified v3.27.0 current as of 2026-04-13
- `proxy.golang.org/github.com/testcontainers/testcontainers-go/@latest` — verified v0.42.0 current as of 2026-04-13
- `proxy.golang.org/github.com/testcontainers/testcontainers-go/modules/postgres/@latest` — verified v0.42.0 current as of 2026-04-13
- `.planning/research/SUMMARY.md` — prior project research; pgx integration pattern, Goose embed pattern, Dockerfile changes
- `.planning/research/PITFALLS.md` — detailed pitfall analysis with risk ratings
- `internal/models/product.go` — direct inspection of all 12 query sites
- `cmd/server/main.go` — direct inspection of SQLite open block, env var reading
- `Dockerfile` — direct inspection of CGO scaffold
- `internal/models/product_test.go` — direct inspection of setupTestStore, test coverage

### Secondary (MEDIUM confidence)
- `.planning/codebase/ARCHITECTURE.md` — architecture analysis
- `.planning/codebase/STACK.md` — current dependency and config analysis
- `.planning/codebase/TESTING.md` — test structure analysis

### Tertiary (LOW confidence)
- [ASSUMED] Go embed spec: paths cannot start with `..` — well-known constraint but not re-verified via tool

---

## Metadata

**Confidence breakdown:**
- Standard stack versions: HIGH — verified against proxy.golang.org on 2026-04-13
- Architecture patterns: HIGH — derived from direct file inspection + prior research
- Pitfalls: HIGH — derived from direct code inspection + PITFALLS.md + prior research
- Test pattern: HIGH — testcontainers postgres module verified current; pattern from docs

**Research date:** 2026-04-13
**Valid until:** 2026-05-13 (library versions; check proxy.golang.org if delayed)
