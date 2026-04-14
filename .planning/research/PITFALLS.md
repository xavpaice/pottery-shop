# Pitfalls Research: SQLite to Postgres + CNPG Migration

**Domain:** Go app SQLite-to-Postgres migration, CloudNative-PG on Kubernetes
**Researched:** 2026-04-13
**Overall confidence:** HIGH (code inspected directly; pitfalls verified against official docs and confirmed bugs)

---

## 1. `LastInsertId()` silently breaks on Postgres

**Risk:** HIGH
**Warning signs:** `AddImage` and `Create` both call `res.LastInsertId()` after `db.Exec(INSERT ...)`. On Postgres via pgx this returns `(0, ErrNotSupported)`. The error is currently swallowed in `AddImage` (return value ignored) and returned in `Create`. In production this means every inserted row gets `ID = 0` assigned back to the struct, breaking any logic that reads `p.ID` or `img.ID` after insert.
**Prevention:** Replace every `db.Exec(INSERT ...)` + `LastInsertId()` pair with `db.QueryRow(INSERT ... RETURNING id).Scan(&id)`. The `$N` placeholder syntax must be used simultaneously (see pitfall 2). Both `ProductStore.Create` and `ProductStore.AddImage` in `internal/models/product.go` need this change.
**Phase:** Core SQL migration (phase that rewrites queries)

---

## 2. SQLite `?` placeholder syntax rejected by Postgres

**Risk:** HIGH
**Warning signs:** Every query in `product.go` uses `?` as the placeholder (e.g. `VALUES (?, ?, ?, ?)`). Postgres requires positional placeholders `$1, $2, $3, $4`. When using `pgx/v5/stdlib` as a `database/sql` driver the `?` placeholders cause a parse error at runtime, not at compile time — the app compiles cleanly and the error only appears on the first DB operation.
**Prevention:** Mechanically replace all `?` with `$1`, `$2`, etc. in order of appearance within each query. Use a regex like `\?` to find every instance across the codebase before running tests.
**Phase:** Core SQL migration

---

## 3. SQLite `DATETIME` / `INTEGER` schema does not parse on Postgres

**Risk:** HIGH
**Warning signs:** The `Init()` method in `product.go` runs raw `CREATE TABLE` DDL using SQLite column types: `INTEGER PRIMARY KEY AUTOINCREMENT`, `REAL`, `INTEGER NOT NULL DEFAULT 0` (for booleans), and `DATETIME DEFAULT CURRENT_TIMESTAMP`. Postgres rejects `DATETIME` (not a valid type) and `AUTOINCREMENT` (not valid syntax). `REAL` is valid in Postgres (maps to `float4`) but `NUMERIC(10,2)` or `DOUBLE PRECISION` is safer for prices. The schema creation will fail hard on first startup.
**Prevention:**
- Replace `INTEGER PRIMARY KEY AUTOINCREMENT` with `BIGSERIAL PRIMARY KEY` or `BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY`.
- Replace `DATETIME` with `TIMESTAMPTZ`.
- Replace boolean-as-`INTEGER` with native `BOOLEAN`.
- Replace `REAL` price column with `NUMERIC(10,2)`.
- Replace `CURRENT_TIMESTAMP` string (already SQL-standard, works fine — no change needed here).
**Phase:** Core SQL migration

---

## 4. Boolean-as-integer scan pattern breaks on Postgres

**Risk:** HIGH
**Warning signs:** The codebase scans `is_sold` into a `var isSold int` and then manually converts `p.IsSold = isSold == 1`. This works in SQLite because booleans are stored as `0`/`1` integers. In Postgres, if the column is declared `BOOLEAN`, scanning into an `int` returns a type mismatch error. If the column is kept as `INTEGER` to avoid changing the scan, Postgres accepts it but you lose type safety. Both `GetByID` and `listProducts` have this pattern.
**Prevention:** Change the column to `BOOLEAN`. Update all `Scan` targets to `&p.IsSold` directly (Go `bool` scans from Postgres `BOOLEAN` natively via pgx). Remove the manual `isSold == 1` conversion everywhere.
**Phase:** Core SQL migration

---

## 5. CGO removal leaves dangling import and breaks the Docker build

**Risk:** HIGH
**Warning signs:** `cmd/server/main.go` has `_ "github.com/mattn/go-sqlite3"` as a blank import. `go.mod` lists `go-sqlite3` as a direct dependency. The current `Dockerfile` uses the `tonistiigi/xx` cross-compilation toolchain with `CGO_ENABLED=1` specifically for sqlite3. After removing go-sqlite3 and switching to pgx (pure Go), the blank import must be deleted, the `go.mod` dependency removed via `go mod tidy`, and the Dockerfile simplified — the entire `xx` cross-compilation scaffold (`tonistiigi/xx`, `clang`, `lld`, `xx-apk`, `xx-go`, `xx-verify`) can be replaced with a standard `CGO_ENABLED=0 GOOS=linux go build`. Leaving any of these in place will either cause a compile error (import with no package) or a ghost CGO build that still pulls in `sqlite-libs` in the runtime image.
**Prevention:** In the same commit that removes go-sqlite3: delete the blank import, run `go mod tidy`, rewrite the builder stage to `CGO_ENABLED=0 go build -o clay-server ./cmd/server`, remove the `apk add clang lld` line, and remove `sqlite-libs` from the runtime stage.
**Phase:** Go driver swap (first phase of migration)

---

## 6. CNPG `<cluster>-app` Secret does not exist when the Deployment first applies

**Risk:** HIGH
**Warning signs:** The app Deployment uses `envFrom.secretRef` to inject environment variables. The plan is to mount the CNPG-generated Secret (`<cluster-name>-app`) this way. That secret is created asynchronously by the CNPG operator after the Cluster resource is reconciled and Postgres finishes bootstrapping — this takes tens of seconds to several minutes on a cold cluster. If the Deployment and the Cluster resource are applied in the same `helm install` invocation, Kubernetes will try to start the app pod immediately. Because the secret does not yet exist, every pod enters `CreateContainerConfigError` and stays there until the secret appears. On a fresh install this is a guaranteed failure.
**Prevention:** Use an `initContainer` in the Deployment that polls for the secret (or for Postgres connectivity) before the main container starts. A minimal init container can run `until pg_isready -h $HOST; do sleep 2; done` using the `postgres:16-alpine` image, or use `kubectl wait`. Alternatively set `optional: true` on the `secretRef` and have the app itself retry the DB connection with exponential backoff on startup. The retry-on-startup path is more robust for rolling restarts too.
**Phase:** Helm chart + CNPG wiring phase

---

## 7. CNPG Cluster CRD not installed when chart first deploys

**Risk:** HIGH
**Warning signs:** The CNPG operator Helm chart (from `https://cloudnative-pg.github.io/charts`) places its CRDs in the `crds/` directory with `crds.create: true` by default. When used as a **subchart dependency** in `chart/clay/Chart.yaml`, Helm does install those CRDs on `helm install` — the `crds/` directory in a subchart is honoured. However there is a critical gap on **`helm upgrade`**: Helm's documented limitation is that CRDs in `crds/` directories are never updated or deleted by `helm upgrade`, only by `helm install`. If the CNPG operator Helm chart releases a new CRD version (e.g. adding a field to the `Cluster` CRD schema), a `helm upgrade` of the parent chart will not update the CRD in the cluster. The Cluster resource may then fail validation against the old CRD schema.
**Prevention:**
- For initial install this is not a problem — Helm will install the CRD before creating the Cluster CR.
- For upgrades: document that CRD upgrades require a manual `kubectl apply -f` of the new CRD manifests, or a `helm upgrade` of the CNPG subchart independently before upgrading the parent. Add this to your runbook.
- Do NOT set `crds.create: false` thinking you will manage them separately unless you have a concrete plan — an absent CRD causes an immediate "no kind Cluster registered" error.
**Phase:** Helm chart + CNPG wiring phase

---

## 8. CNPG operator and Kubernetes version mismatch

**Risk:** MEDIUM
**Warning signs:** CNPG version compatibility is narrow: CNPG 1.26.x supports Kubernetes 1.30–1.33; CNPG 1.25.x supports 1.29–1.32; CNPG 1.24.x (EOL March 2025) supported 1.28–1.31. Using a CNPG chart version that predates or postdates the cluster's Kubernetes version causes the operator to refuse to start or behave incorrectly.
**Prevention:** Pin the CNPG Helm chart version in `Chart.yaml` and verify it against the target Kubernetes version before deploying. Check the CNPG supported releases page at https://cloudnative-pg.io/documentation/1.24/supported_releases/ for the current matrix. The values.yaml for the subchart should expose the CNPG version as a pinned value, not `latest`.
**Phase:** Helm chart + CNPG wiring phase

---

## 9. pgxpool not closed on server shutdown

**Risk:** MEDIUM
**Warning signs:** The current code calls `defer db.Close()` on the `*sql.DB` handle and then `http.ListenAndServe` runs until killed — there is no graceful shutdown signal handler. When migrating to `pgxpool.Pool` (if using pgx natively) or even to `database/sql` backed by pgx, `pool.Close()` must be called before the process exits. Without it, Postgres-side connections stay in `idle` state until `tcp_keepalives_idle` fires, wasting connection slots. On a default CNPG cluster with 1 instance and the default `max_connections = 100` this matters.
**Prevention:** Add signal handling (`os.Signal`, `SIGTERM`, `SIGINT`) in `main.go` with a `context.WithTimeout` shutdown sequence: stop the HTTP server, then close the pool. Even if keeping `database/sql`, replace the bare `defer db.Close()` with a deferred close that runs only after the HTTP server has drained.
**Phase:** Go driver swap

---

## 10. Postgres requires explicit transaction rollback after error

**Risk:** MEDIUM
**Warning signs:** SQLite is forgiving about transactions: a failed statement inside an implicit transaction leaves the connection usable. Postgres marks the entire transaction as aborted on the first error; all subsequent commands on that connection return `ERROR: current transaction is aborted, commands ignored until end of transaction block`. The current `listProducts` function (and others) does not use explicit transactions, so this mainly matters if pgxpool is used and a connection is returned to the pool in an aborted transaction state. This can cause cascading failures where pooled connections become permanently broken.
**Prevention:** For every explicit `db.Begin()` / `tx.Exec()` block: always `defer tx.Rollback()` immediately after `Begin()`, then call `tx.Commit()` in the success path. `Rollback()` on an already-committed transaction returns a benign error. For `database/sql` the pattern is: `tx, err := db.Begin(); if err != nil { return err }; defer tx.Rollback(); ...; return tx.Commit()`.
**Phase:** Go driver swap

---

## 11. `DB_PATH` config key left in place after migration

**Risk:** MEDIUM
**Warning signs:** `values.yaml` has `DB_PATH: "/data/clay.db"` in the `config` section. The `configmap.yaml` template renders all `config` keys as environment variables. After migration, `DB_PATH` will still be injected into the pod as an env var and the `cmd/server/main.go` code reads it on startup (`dbPath := envOr("DB_PATH", "pottery.db")`). If any code path still references `DB_PATH` it will silently open a non-existent file instead of connecting to Postgres. The SQLite `persistence` PVC (`/data` mount) will still exist in the chart and be mounted, wasting storage.
**Prevention:** Remove `DB_PATH` from `configmap.yaml` / `values.yaml`. Replace with a `DATABASE_URL` env var sourced from the CNPG secret. Remove the `DB_PATH` read from `main.go`. Either remove the PVC (if uploads are being handled separately) or document that it is kept only for `UPLOAD_DIR`.
**Phase:** Helm chart + CNPG wiring phase

---

## 12. Helm subchart values passthrough: CNPG operator config not reachable by default

**Risk:** MEDIUM
**Warning signs:** Helm subchart values are namespaced under the subchart name. If `Chart.yaml` declares the CNPG subchart with `name: cloudnative-pg`, then any value override must be nested under `cloudnative-pg:` in the parent `values.yaml`. A common mistake is setting top-level keys expecting them to pass through, or referencing `{{ .Values.replicaCount }}` inside a subchart template thinking it refers to the parent value. Additionally, the CNPG `Cluster` CR is a separate resource from the operator — they ship as separate Helm charts (`cloudnative-pg` operator chart + `cluster` chart). Conflating them in one subchart entry causes the Cluster CR to be missing.
**Prevention:** Use two separate chart dependencies if using the official CNPG charts: one for the operator (`cloudnative-pg`) and one for the cluster (`cluster`). Or define the `Cluster` resource directly as a template in `chart/clay/templates/` controlled by a `{{- if .Values.postgres.managed }}` guard. Verify the values namespace with `helm template` before deploying.
**Phase:** Helm chart + CNPG wiring phase

---

## 13. `REAL` price column and float precision in Go scans

**Risk:** LOW
**Warning signs:** SQLite stores `REAL` as a 64-bit IEEE 754 float. Postgres `REAL` is a 32-bit float (`float4`). If the DDL is migrated literally (`REAL` → `REAL`), a price like `12.50` stored via Go `float64` and then scanned back will show floating-point artifacts (e.g. `12.500000186264515`) because Postgres demotes to 32-bit on write. The current `Product.Price float64` field will silently gain precision errors.
**Prevention:** Declare the price column as `NUMERIC(10,2)` or `DOUBLE PRECISION` in the Postgres schema. `NUMERIC(10,2)` is the safer choice for money. pgx scans `NUMERIC` to `float64` fine via `database/sql`. This only requires a DDL change — no application logic change beyond the column type in `Init()`.
**Phase:** Core SQL migration

---

## 14. Schema init runs on every startup — not safe for production

**Risk:** LOW (now) / MEDIUM (if multiple replicas or rolling restarts)
**Warning signs:** `store.Init()` runs `CREATE TABLE IF NOT EXISTS` on every startup. For SQLite this is safe because there is one writer. For Postgres with multiple app pods (even just a rolling restart), two pods can race on `CREATE TABLE IF NOT EXISTS` — Postgres serialises this via DDL locking but it can still cause transient errors or unexpected behaviour on `ALTER TABLE` style migrations. More critically, `CREATE TABLE IF NOT EXISTS` cannot add new columns to an existing table — schema changes require migrations.
**Prevention:** For the current migration scope `CREATE TABLE IF NOT EXISTS` is acceptable (fresh schema, no data to preserve). Flag it as a known limitation in a code comment and plan to replace with a proper migration tool (e.g. `golang-migrate`, `goose`) before the schema needs changes. Do not use `ALTER TABLE` inside `Init()`.
**Phase:** Core SQL migration (note for future milestone)

---

## 15. Deployment `strategy: Recreate` comment is now wrong — but keep it

**Risk:** LOW
**Warning signs:** `deployment.yaml` has `strategy: type: Recreate  # SQLite requires single-writer`. After migration to Postgres, multiple replicas are theoretically possible (uploads still require a shared PVC for images, which is a separate concern). The comment will be stale. The `Recreate` strategy itself is not harmful.
**Prevention:** Update the comment. Leave `Recreate` as the strategy until upload storage is addressed (shared PVC or object storage). This is documentation drift, not a runtime failure.
**Phase:** Helm chart + CNPG wiring phase (comment fix only)

---

## Critical Pitfalls Summary

**The three things that will definitely bite you on first deployment:**

1. **`LastInsertId()` + `?` placeholders** — Every INSERT in the app returns `id=0` and every query fails with a parse error. The app compiles but fails entirely on any database operation. These two issues appear in the same code locations and should be fixed together.

2. **CNPG Secret timing on `helm install`** — The app pod enters `CreateContainerConfigError` and stays there for minutes (or indefinitely if something goes wrong with Postgres bootstrap) because `envFrom.secretRef` references a Secret that doesn't exist yet. Without an `initContainer` or startup retry loop, the first `helm install` always looks broken even when CNPG is healthy.

3. **SQLite DDL runs against Postgres** — The `Init()` schema creation contains `AUTOINCREMENT`, `DATETIME`, and integer booleans that Postgres rejects outright. The app will crash on startup before it can serve a single request.
