# Architecture Research: SQLite to Postgres Migration

**Project:** pottery-shop
**Researched:** 2026-04-13
**Overall Confidence:** HIGH (pgx/v5 official docs + multiple corroborating sources)

---

## Driver Change

### Import Removal

Remove from `go.mod` and all imports:
```
github.com/mattn/go-sqlite3 v1.14.22   // CGO, gone entirely
```

Add to `go.mod`:
```
github.com/jackc/pgx/v5
```

### Two Integration Paths

**Path A: pgxpool + stdlib adapter (recommended)**

This keeps the existing `*sql.DB` interface in `ProductStore` intact, meaning `models/product.go` does not need to change its method signatures — only its SQL strings and scanning logic. The pool provides connection management; the stdlib adapter presents a `*sql.DB` to existing code.

```go
// cmd/server/main.go
import (
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/jackc/pgx/v5/stdlib"
)

pool, err := pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))
if err != nil {
    log.Fatalf("Failed to connect to database: %v", err)
}
defer pool.Close()

db := stdlib.OpenDBFromPool(pool)
// db is *sql.DB — pass to models.NewProductStore(db) unchanged
```

**Path B: native pgx (pgxpool.Pool directly)**

Replace `*sql.DB` with `*pgxpool.Pool` throughout the models layer. Gives access to pgx-native features (batch queries, COPY, etc.) but requires changing method signatures and all call sites. This app has no use case for those features — unnecessary churn.

**Verdict: Use Path A.** It isolates all changes to `cmd/server/main.go` (connection init) and `internal/models/product.go` (SQL strings + scan fixes). The `handlers/` layer and `*sql.DB` usage in `ProductStore` remain structurally unchanged.

### Connection String Format

pgx accepts standard PostgreSQL DSN in either form. The CNPG-generated secret uses the URL form:

```
postgres://user:password@host:5432/dbname?sslmode=require
```

Or keyword/value form:
```
user=appuser password=secret host=db-rw port=5432 dbname=pottery sslmode=require
```

Environment variable name: `DATABASE_URL` (matches CNPG secret key `uri`).

In `main.go`, replace:
```go
dbPath := envOr("DB_PATH", "pottery.db")
db, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=on")
```
With:
```go
databaseURL := os.Getenv("DATABASE_URL")
if databaseURL == "" {
    log.Fatal("DATABASE_URL must be set")
}
pool, err := pgxpool.New(context.Background(), databaseURL)
// ...
db := stdlib.OpenDBFromPool(pool)
```

The `DB_PATH` env var is deleted. No fallback default — Postgres is required, so a missing `DATABASE_URL` is a fatal startup error.

---

## SQL Dialect Differences

Every difference below has a concrete impact on the existing `internal/models/product.go` code.

### Parameter Placeholders

| | SQLite (current) | Postgres (target) |
|---|---|---|
| Placeholder style | `?` (positional, unnamed) | `$1`, `$2`, ... (positional, numbered) |
| Example | `WHERE id=?` | `WHERE id=$1` |

All 12 queries in `product.go` use `?`. Every one must be renumbered to `$1`, `$2`, etc. This is mechanical — no logic changes.

### Auto-Increment Primary Keys

| | SQLite (current) | Postgres (target) |
|---|---|---|
| Syntax | `INTEGER PRIMARY KEY AUTOINCREMENT` | `BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY` |
| Alternate | — | `SERIAL PRIMARY KEY` (older, still valid) |
| Recommendation | — | Use `GENERATED ALWAYS AS IDENTITY` (SQL standard, Postgres 10+) |

`GENERATED ALWAYS AS IDENTITY` is the current Postgres standard. `SERIAL` is a legacy shorthand that still works but is not recommended for new schemas.

### LastInsertId Replacement

`res.LastInsertId()` is not supported by any PostgreSQL driver (including pgx's stdlib adapter). The existing `Create()` and `AddImage()` methods both call it.

**SQLite pattern (current):**
```go
res, err := s.DB.Exec(`INSERT INTO products (...) VALUES (?, ?, ?, ?)`, ...)
p.ID, err = res.LastInsertId()
```

**Postgres pattern (required):**
```go
err := s.DB.QueryRow(
    `INSERT INTO products (title, description, price, is_sold)
     VALUES ($1, $2, $3, $4) RETURNING id`,
    p.Title, p.Description, p.Price, p.IsSold,
).Scan(&p.ID)
```

Both `Create()` and `AddImage()` must be converted from `Exec` + `LastInsertId` to `QueryRow` + `RETURNING id` + `Scan`.

### Boolean Columns

| | SQLite (current) | Postgres (target) |
|---|---|---|
| Column type | `INTEGER NOT NULL DEFAULT 0` | `BOOLEAN NOT NULL DEFAULT FALSE` |
| Go scan workaround | `var isSold int` then `p.IsSold = isSold == 1` | Scan directly into `bool`: no workaround needed |

The current code has an `int`-to-`bool` manual conversion in `GetByID()`, `listProducts()`, and any other scan sites. With a Postgres `BOOLEAN` column, pgx scans directly into a Go `bool`. The workaround code is deleted.

In `Update()`, the current query passes `p.IsSold` (a `bool`) as the value for `is_sold INTEGER`. SQLite accepts this via implicit conversion. Postgres `BOOLEAN` accepts Go `bool` natively — no change needed for the argument, just the column type in the schema.

### Date/Time Columns

| | SQLite (current) | Postgres (target) |
|---|---|---|
| Column type | `DATETIME DEFAULT CURRENT_TIMESTAMP` | `TIMESTAMPTZ DEFAULT NOW()` |
| Go type | `time.Time` (scanned via database/sql) | `time.Time` (scanned natively by pgx) |

The Go structs use `time.Time` already. Postgres `TIMESTAMPTZ` scans cleanly into `time.Time` with pgx's stdlib adapter — no code change in the scan calls. The schema DDL changes; the Go code does not.

`TIMESTAMPTZ` (with timezone) is preferred over `TIMESTAMP` (without) for new schemas. It stores UTC and converts on retrieval based on session timezone.

### PRAGMA Statements

`internal/models/product.go:Init()` does not use any PRAGMA statements directly. The foreign keys PRAGMA is in `main.go` as a connection parameter: `?_foreign_keys=on`. This connection parameter is SQLite-specific and is removed.

Postgres enforces foreign key constraints by default — no equivalent opt-in is needed.

### Foreign Key Constraint Syntax

The existing `FOREIGN KEY (product_id) REFERENCES products(id) ON DELETE CASCADE` syntax is standard SQL and works identically in Postgres. No change required for the constraint clause itself.

### JSON Handling

The `Cart` type uses `encoding/json` in Go and is stored in a signed cookie — it has no database column at all. There is no JSON stored in the products or images tables. JSON handling differences between SQLite and Postgres are not relevant to this app.

### Multi-Statement Exec

SQLite's `database/sql` driver accepts multiple statements separated by semicolons in a single `Exec()` call. The current `Init()` method does exactly this — both `CREATE TABLE` statements are in a single string. Postgres (via pgx stdlib) does not support multi-statement execution in a single `Exec` call.

This is a non-issue once `Init()` is replaced by a migration tool (see Schema Initialization below), but if schema-on-startup were kept, each `CREATE TABLE` would need to be a separate `Exec` call.

---

## Schema Initialization

### Current Approach

`store.Init()` runs raw `CREATE TABLE IF NOT EXISTS` DDL at startup. This works for SQLite because:
- The database file is local and always fresh-ish
- Multi-statement exec is allowed
- No versioning or rollback is needed

### Why Bare Schema-on-Startup Does Not Scale to Postgres

For a persistent Postgres database (CNPG-managed, survives pod restarts), `CREATE TABLE IF NOT EXISTS` at startup is safe for the initial create but provides no path for future schema changes (adding a column, changing a type, etc.) without manual intervention. Once the app is deployed to Kubernetes, you cannot just delete the database to apply changes.

### Recommendation: Goose v3 with Embedded Migrations

Use `github.com/pressly/goose/v3`. Reasons over golang-migrate:
- Pure Go library (embeds into the binary, no separate CLI binary needed in the container)
- Supports `//go:embed` for baking migration files into the binary at compile time
- Programmatic API: call `goose.Up(db, "migrations")` in `main.go` — same startup-run pattern the app already uses
- Active maintenance, widely used in the Go ecosystem
- Supports Postgres natively

**Migration file structure:**
```
internal/migrations/
  00001_initial_schema.sql
```

**00001_initial_schema.sql:**
```sql
-- +goose Up
CREATE TABLE IF NOT EXISTS products (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    price NUMERIC(10,2) NOT NULL DEFAULT 0,
    is_sold BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS images (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    product_id BIGINT NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    filename TEXT NOT NULL,
    thumbnail_fn TEXT NOT NULL DEFAULT '',
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- +goose Down
DROP TABLE IF EXISTS images;
DROP TABLE IF EXISTS products;
```

**main.go startup pattern:**
```go
import (
    "embed"
    "github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrations embed.FS

// In main(), after pool creation:
goose.SetBaseFS(migrations)
goose.SetDialect("postgres")
if err := goose.Up(db, "migrations"); err != nil {
    log.Fatalf("Failed to run migrations: %v", err)
}
```

Goose creates and manages a `goose_db_version` table to track applied migrations. Running `goose.Up()` on a database where migrations are already applied is a no-op — safe for multi-pod restarts.

**CNPG note:** With CNPG, the Postgres instance persists across pod restarts (it's a separate StatefulSet). The app pod may restart independently. Running `goose.Up()` at startup handles both the initial creation and future schema updates cleanly.

### Alternative: Schema-on-Startup (Simpler but Limited)

If migration versioning is deferred, the `Init()` method can be rewritten to run each `CREATE TABLE` as a separate `Exec` call with the corrected Postgres DDL. This works for the initial deployment but will require manual DDL work for any future schema change. Not recommended — add goose upfront while the schema is still simple.

---

## Connection Pooling

### Why pgxpool (Not a Single Connection)

The app uses `net/http` with Go's built-in concurrency model — each HTTP request runs in its own goroutine. Multiple concurrent requests mean multiple concurrent database calls. A single `*pgx.Conn` is not safe for concurrent use. `pgxpool.Pool` is.

When using `stdlib.OpenDBFromPool`, the pool handles connection acquisition and return. The `*sql.DB` wrapper's own idle connection settings are overridden to zero (the pool manages this), so configure everything on `pgxpool.Config`.

### Recommended Configuration for This App

This is a low-traffic pottery shop with one Kubernetes pod (default). CNPG defaults to a single Postgres instance. Conservative pool settings are appropriate:

```go
config, err := pgxpool.ParseConfig(os.Getenv("DATABASE_URL"))
if err != nil {
    log.Fatalf("Failed to parse database URL: %v", err)
}

config.MaxConns = 10                          // Well below Postgres default max_connections (100)
config.MinConns = 2                           // Keep 2 connections warm
config.MaxConnLifetime = 30 * time.Minute     // Recycle connections periodically
config.MaxConnIdleTime = 10 * time.Minute     // Release idle connections

pool, err := pgxpool.NewWithConfig(context.Background(), config)
```

For the simplest initial implementation, `pgxpool.New(ctx, databaseURL)` with no custom config is also acceptable. Default `MaxConns` is `max(4, runtime.NumCPU())`. For a small app, this is fine and can be tuned later via env var if needed.

### pgxpool DSN Pool Parameters (URL-encoded)

pgxpool accepts pool parameters in the connection string directly if you don't want to configure them in code:

```
postgres://user:pass@host/db?pool_max_conns=10&pool_min_conns=2&pool_max_conn_lifetime=30m&pool_max_conn_idle_time=10m
```

These are pgxpool-specific URL parameters and are not forwarded to Postgres itself.

---

## Component Build Order

The dependency graph dictates this order:

```
Schema (migrations/) → Models (internal/models/) → Main (cmd/server/main.go) → Handlers (internal/handlers/)
```

### Step 1: Schema Migration File

Write `internal/migrations/00001_initial_schema.sql` with Postgres DDL. This is a pure SQL file with no Go dependencies — it can be written and verified independently.

### Step 2: Models Layer (`internal/models/product.go`)

This is the only file with SQL query strings and scan logic. All dialect changes land here:
- Replace `?` with `$1`, `$2`, ... in all 12 queries
- Replace `INTEGER PRIMARY KEY AUTOINCREMENT` with `BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY` in schema (already moved to migration file)
- Replace `Exec` + `LastInsertId` pattern with `QueryRow` + `RETURNING id` + `Scan` in `Create()` and `AddImage()`
- Remove `var isSold int` workaround; scan `is_sold` directly into `p.IsSold bool`
- Remove `Init()` method entirely (replaced by goose)
- `ProductStore` struct field stays `*sql.DB` — no signature changes

`cart.go` is untouched — it has no database interaction.

### Step 3: Main (`cmd/server/main.go`)

Changes are confined to the database initialization block:
- Remove `import _ "github.com/mattn/go-sqlite3"`
- Remove `DB_PATH` env var
- Add `DATABASE_URL` env var (fatal if missing)
- Replace `sql.Open("sqlite3", ...)` with `pgxpool.New` + `stdlib.OpenDBFromPool`
- Add goose migration runner after pool creation
- Remove `store.Init()` call

All other code in `main.go` (templates, handlers, routes, session) is untouched.

### Step 4: Handlers (`internal/handlers/`)

**No changes expected.** Handlers receive `*ProductStore` (unchanged interface) and call the same methods. The handler layer is insulated from the database driver change by the models abstraction. Verify by running the app — any scan-related bugs surface as runtime errors in the models layer, not the handlers.

### Step 5: go.mod

```
Remove: github.com/mattn/go-sqlite3
Add:    github.com/jackc/pgx/v5
Add:    github.com/pressly/goose/v3
```

Run `go mod tidy` after changes.

---

## Key Architectural Decisions

### Decision 1: Keep `*sql.DB` Interface in ProductStore

**Rationale:** The models layer already depends only on `database/sql` (`*sql.DB`, `*sql.Rows`, `*sql.Row`). By routing pgxpool through `stdlib.OpenDBFromPool`, the `ProductStore` struct and `NewProductStore` signature remain unchanged. This minimizes the diff and reduces risk.

**Alternative rejected:** Switching to native `pgxpool.Pool` in `ProductStore` would require touching `handlers/` call sites and `main.go` initialization — more files, more risk, no benefit for this app's query patterns.

**Phase implication:** Models and main are the only files with changes. Handlers do not need a dedicated migration phase.

### Decision 2: Goose Over golang-migrate

**Rationale:** Goose is a Go library that can be imported and called programmatically. golang-migrate works the same way but goose has better embedding support and the `//go:embed` pattern is simpler. Both are valid; goose is the more idiomatic choice for Go-first projects.

**Phase implication:** Migration file is written in the schema phase, before models. The goose runner is wired into main.go in the main-changes phase.

### Decision 3: `GENERATED ALWAYS AS IDENTITY` Over `SERIAL`

**Rationale:** `SERIAL` is a Postgres-specific shorthand that creates a sequence implicitly; it is documented as legacy in Postgres 10+. `GENERATED ALWAYS AS IDENTITY` is the SQL standard (SQL:2003) equivalent. It prevents accidental manual ID insertion, which is correct for this app (fresh schema, no data import).

**Phase implication:** No code impact beyond the migration SQL file. The Go types (`int64`) are unchanged.

### Decision 4: `TIMESTAMPTZ` Over `TIMESTAMP`

**Rationale:** `TIMESTAMP` stores no timezone info — if the server timezone ever changes, all timestamps shift. `TIMESTAMPTZ` stores UTC internally and handles conversion. For a Kubernetes deployment where the container timezone could vary, `TIMESTAMPTZ` is the safe default.

**Code impact:** None. pgx scans `TIMESTAMPTZ` into `time.Time` with timezone information. Existing `time.Time` struct fields work without change.

### Decision 5: `NUMERIC(10,2)` Over `REAL` for Price

**Rationale:** The current schema uses `REAL` (floating-point). Postgres has a `REAL` type too, so this would work. However, financial amounts should use `NUMERIC` (exact decimal) to avoid floating-point rounding errors. `NUMERIC(10,2)` means up to 8 digits before decimal, 2 after — sufficient for pottery prices.

**Code impact:** The `Product.Price` field is `float64` in Go. Postgres `NUMERIC` scans into `float64` via pgx's stdlib adapter. No code change required; the schema change is in the migration file only. This is a correctness improvement over the SQLite schema.

### Decision 6: No CGO in Docker Build

**Rationale:** Removing `go-sqlite3` eliminates the only CGO dependency. The Dockerfile can be simplified to a pure Go build: `CGO_ENABLED=0 go build ./cmd/server`. This allows smaller base images (scratch or distroless) and eliminates the need for a C toolchain in the build stage.

**Phase implication:** The Dockerfile multi-stage build may need `CGO_ENABLED=0` added explicitly and the C build tools (gcc) removed from the build stage.

---

## Sources

- pgx/v5 stdlib package: https://pkg.go.dev/github.com/jackc/pgx/v5/stdlib
- pgxpool package: https://pkg.go.dev/github.com/jackc/pgx/v5/pgxpool
- pgx GitHub repo (canonical reference): https://github.com/jackc/pgx
- LastInsertId not supported in pgx: https://github.com/jackc/pgx/issues/1483
- Postgres GENERATED ALWAYS AS IDENTITY: https://www.postgresql.org/docs/current/ddl-identity-columns.html
- Goose embedded migrations: https://pressly.github.io/goose/blog/2021/embed-sql-migrations/
- Goose v3 package: https://pkg.go.dev/github.com/pressly/goose/v3
- pgx positional parameters and NamedArgs: https://ectobit.com/blog/pgx-v5-2/
