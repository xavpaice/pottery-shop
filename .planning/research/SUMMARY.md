# Research Summary: Pottery Shop — SQLite to Postgres + CNPG

**Project:** pottery-shop
**Domain:** Go web app database migration — SQLite to PostgreSQL via CloudNative-PG on Kubernetes
**Researched:** 2026-04-13
**Confidence:** HIGH

---

## Executive Summary

This project migrates a single-binary Go pottery shop from a local SQLite file to a PostgreSQL database managed by the CloudNative-PG (CNPG) operator on Kubernetes. The migration has three separable concerns that must be completed in strict dependency order: the Go source code changes (driver, SQL dialect, schema), the Helm chart changes (CNPG subchart, conditional Cluster resource, secret injection), and the Docker build simplification (CGO removal). Each concern is well-understood with official documentation; the overall pattern is a standard Go app Postgres migration with a Kubernetes delivery layer.

The recommended approach uses pgx v5.9.1 with the `database/sql` stdlib adapter (`pgxpool` + `stdlib.OpenDBFromPool`) to minimize the diff — all SQL strings change but method signatures and handler code do not. Goose v3 replaces the SQLite `Init()` startup DDL with embedded, versioned migrations. On the Kubernetes side, the CNPG operator is added as a Helm subchart and a `Cluster` resource template is added to `chart/clay/templates/`, both gated by a `postgres.managed` boolean in `values.yaml`.

The three failure modes that will occur without explicit prevention are: (1) every INSERT silently returns `id=0` due to `LastInsertId()` being unsupported on Postgres — the fix is `RETURNING id`; (2) the app pod enters `CreateContainerConfigError` on first `helm install` because the CNPG-generated secret does not exist yet — the fix is an init container or startup retry; and (3) the startup DDL schema contains `AUTOINCREMENT` and `DATETIME` which Postgres rejects — the fix is replacing `Init()` with a proper Goose migration. None of these are novel problems; all have standard solutions documented in the research.

---

## Recommended Stack

| Component | Version | Rationale |
|-----------|---------|-----------|
| pgx | v5.9.1 | Pure Go Postgres driver; no CGO; binary wire protocol; includes pgxpool |
| pgx stdlib adapter | v5.9.1 (same module) | Keeps `*sql.DB` interface in ProductStore — zero handler changes |
| Goose | v3 (latest) | Pure Go migration library; `//go:embed` support; programmatic API |
| CNPG operator | 1.29.0 | Latest stable as of April 2026; manages Cluster CRD and credentials |
| CNPG Helm chart | 0.28.0 | chart for operator 1.29.0; repo: cloudnative-pg.github.io/charts |
| Kubernetes minimum | 1.29.0 | Required by CNPG chart 0.28.0 |

**go.mod changes:**
- Remove: `github.com/mattn/go-sqlite3`
- Add: `github.com/jackc/pgx/v5 v5.9.1`
- Add: `github.com/pressly/goose/v3`

**Security note:** Two low-severity pgx CVEs (GO-2026-4771, GO-2026-4772) exist in `pgproto3`'s server-side wire protocol handler. They do not affect Go apps connecting *to* Postgres (only apps acting *as* a Postgres server). Use v5.9.1 and monitor for a patch.

---

## Critical Code Changes

All changes are confined to two files plus a new migration file. Handlers are untouched.

### 1. `cmd/server/main.go`

Replace the SQLite open block:
```go
// Remove:
import _ "github.com/mattn/go-sqlite3"
dbPath := envOr("DB_PATH", "pottery.db")
db, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=on")

// Replace with:
import (
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/jackc/pgx/v5/stdlib"
    "github.com/pressly/goose/v3"
)
databaseURL := os.Getenv("DATABASE_URL")
if databaseURL == "" { log.Fatal("DATABASE_URL must be set") }
pool, err := pgxpool.New(context.Background(), databaseURL)
defer pool.Close()
db := stdlib.OpenDBFromPool(pool)

// After db is ready, run migrations:
goose.SetBaseFS(migrations)  // //go:embed migrations/*.sql
goose.SetDialect("postgres")
if err := goose.Up(db, "migrations"); err != nil { log.Fatalf(...) }
// Remove: store.Init()
```

### 2. `internal/models/product.go`

Four mechanical changes across all query methods:

**a) Replace all `?` with `$1`, `$2`, ... (12 queries)**
```go
// Before: WHERE id=?
// After:  WHERE id=$1
```

**b) Replace `Exec+LastInsertId` with `QueryRow+RETURNING id` (Create and AddImage)**
```go
// Before:
res, err := s.DB.Exec(`INSERT INTO products (...) VALUES (?, ?, ?, ?)`, ...)
p.ID, err = res.LastInsertId()

// After:
err := s.DB.QueryRow(
    `INSERT INTO products (title, description, price, is_sold)
     VALUES ($1, $2, $3, $4) RETURNING id`,
    p.Title, p.Description, p.Price, p.IsSold,
).Scan(&p.ID)
```

**c) Remove boolean int workaround (GetByID and listProducts)**
```go
// Before: var isSold int ... p.IsSold = isSold == 1
// After:  scan directly into &p.IsSold (bool)
```

**d) Remove `Init()` method entirely** — replaced by Goose migration.

### 3. `internal/migrations/00001_initial_schema.sql` (new file)

```sql
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

Key DDL changes from SQLite:
- `INTEGER PRIMARY KEY AUTOINCREMENT` → `BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY`
- `REAL` price column → `NUMERIC(10,2)` (exact decimal; avoids float4 precision loss)
- `INTEGER NOT NULL DEFAULT 0` booleans → `BOOLEAN NOT NULL DEFAULT FALSE`
- `DATETIME DEFAULT CURRENT_TIMESTAMP` → `TIMESTAMPTZ DEFAULT NOW()`

### 4. `Dockerfile`

```dockerfile
# Remove from builder stage:
#   tonistiigi/xx cross-compilation scaffold
#   apk add clang lld
#   CGO_ENABLED=1

# Replace build line with:
RUN CGO_ENABLED=0 go build -o /app ./cmd/server

# Remove from runtime stage:
#   sqlite-libs apk package
# Keep: ca-certificates (required for TLS to Postgres)
```

---

## Helm / Kubernetes Changes

### `chart/clay/Chart.yaml` — add subchart dependency

```yaml
dependencies:
  - name: cloudnative-pg
    version: "0.28.0"
    repository: "https://cloudnative-pg.github.io/charts"
    condition: cloudnative-pg.enabled
```

Run `helm dependency update chart/clay` after editing.

### `chart/clay/values.yaml` — add postgres block, remove SQLite artifacts

```yaml
postgres:
  managed: true                  # false = use external.dsn instead
  cluster:
    instances: 1                 # set to 3 for HA
    imageName: "ghcr.io/cloudnative-pg/postgresql:16"
    storage:
      size: 8Gi
      storageClass: ""           # empty = cluster default
    database: "app"
  external:
    dsn: ""                      # used only when managed: false

cloudnative-pg:
  enabled: true                  # must mirror postgres.managed

# Remove:
# config:
#   DB_PATH: "/data/clay.db"
```

### `chart/clay/templates/cnpg-cluster.yaml` (new file)

```yaml
{{- if .Values.postgres.managed }}
apiVersion: postgresql.cnpg.io/v1
kind: Cluster
metadata:
  name: {{ include "clay.fullname" . }}-pg
  namespace: {{ .Release.Namespace }}
  labels:
    {{- include "clay.labels" . | nindent 4 }}
spec:
  instances: {{ .Values.postgres.cluster.instances }}
  imageName: {{ .Values.postgres.cluster.imageName | quote }}
  storage:
    size: {{ .Values.postgres.cluster.storage.size }}
    {{- if .Values.postgres.cluster.storage.storageClass }}
    storageClass: {{ .Values.postgres.cluster.storage.storageClass }}
    {{- end }}
  bootstrap:
    initdb:
      database: {{ .Values.postgres.cluster.database | default "app" }}
      owner: {{ .Values.postgres.cluster.database | default "app" }}
{{- end }}
```

### `chart/clay/templates/deployment.yaml` — replace DB_PATH with DATABASE_URL

```yaml
env:
  {{- if .Values.postgres.managed }}
  - name: DATABASE_URL
    valueFrom:
      secretKeyRef:
        name: {{ include "clay.fullname" . }}-pg-app
        key: uri
  {{- else }}
  - name: DATABASE_URL
    value: {{ .Values.postgres.external.dsn | quote }}
  {{- end }}
```

The CNPG secret name is `<cluster-metadata-name>-app`. With the cluster named `{{ include "clay.fullname" . }}-pg`, the secret is `{{ include "clay.fullname" . }}-pg-app`. The `uri` key contains a fully-formed libpq URI that pgxpool accepts directly.

---

## Top Pitfalls to Avoid

### 1. `LastInsertId()` silently returns 0 (CRITICAL)

Every INSERT in the app assigns `id=0` back to the struct. The app compiles and starts cleanly — the bug only surfaces when you inspect inserted records and find they all have `id=0`. Fix both `ProductStore.Create` and `ProductStore.AddImage` with `RETURNING id` before any testing.

### 2. SQLite `?` placeholders are a runtime parse error (CRITICAL)

Postgres rejects `?` — it requires `$1, $2, ...`. The app compiles successfully. The error appears only at runtime on the first database operation. Grep for `\?` in `product.go` and replace all 12 occurrences before running any tests against Postgres.

### 3. SQLite DDL crashes Postgres on startup (CRITICAL)

`store.Init()` contains `AUTOINCREMENT` and `DATETIME` — both invalid Postgres syntax. The app crashes before serving a single request. The fix is to delete `Init()` and replace it with a Goose migration. Do not attempt to patch `Init()` in place — use Goose from the start.

### 4. CNPG secret timing race on `helm install` (CRITICAL for first deploy)

The CNPG operator creates the `<cluster>-app` secret asynchronously after Postgres finishes bootstrapping. This takes tens of seconds to minutes. The app pod starts immediately after `helm install` and enters `CreateContainerConfigError` because the secret does not exist yet.

**Prevention options (pick one):**
- Add an init container that polls `pg_isready` before the main container starts
- Implement startup retry with exponential backoff in `main.go` after `pgxpool.New` fails

The pod will eventually become ready either way, but without mitigation the first `helm install` always looks broken. Document this in the chart README if not mitigating with an init container.

### 5. CGO dangling import breaks Docker build (CRITICAL for CI)

After removing `go-sqlite3` from `go.mod`, the blank import `_ "github.com/mattn/go-sqlite3"` in `main.go` causes a compile error. The Dockerfile's `tonistiigi/xx` cross-compilation scaffold becomes unreachable dead code. Do these in a single commit: delete the blank import, run `go mod tidy`, update the Dockerfile. Do not leave them split across commits or the build will be broken mid-migration.

---

## Suggested Build Order

The dependency graph from ARCHITECTURE.md and PITFALLS.md converges on this sequence:

```
Phase 1: Go changes (no K8s needed — test with local postgres)
  1a. Remove go-sqlite3, add pgx/v5 + goose/v3 to go.mod
  1b. Write internal/migrations/00001_initial_schema.sql (Postgres DDL)
  1c. Rewrite internal/models/product.go (placeholders, RETURNING id, bool scans, remove Init)
  1d. Rewrite cmd/server/main.go (pgxpool init, goose runner, remove DB_PATH)
  1e. Update Dockerfile (CGO_ENABLED=0, remove cross-compile scaffold)
  → Validate: go build ./... succeeds; app connects to a local Postgres and serves requests

Phase 2: Helm chart wiring (K8s required)
  2a. Add CNPG subchart to Chart.yaml, run helm dependency update
  2b. Add postgres block to values.yaml, remove DB_PATH config key
  2c. Add templates/cnpg-cluster.yaml with managed/external conditional
  2d. Update templates/deployment.yaml — DATABASE_URL from secretKeyRef or plain value
  2e. Add init container or startup retry for CNPG secret timing
  2f. Update strategy comment in deployment.yaml
  → Validate: helm template renders correctly; helm lint passes

Phase 3: Integration (target cluster)
  3a. helm install on staging namespace
  3b. Verify Cluster reaches Healthy, secret exists
  3c. Verify app pod starts and readiness probe passes
  3d. Exercise product CRUD and image upload
  3e. Document CRD upgrade runbook entry
```

**Why this order:**
- Go changes have zero Kubernetes dependencies and can be validated in isolation with `docker run postgres`.
- The Helm chart cannot be meaningfully validated until the binary accepts `DATABASE_URL` without panicking.
- Goose migrations (step 1b) are written before models (step 1c) because the models layer depends on the schema being correct.
- CGO removal (step 1e) belongs in Phase 1 because it is tightly coupled to the driver removal.
- The CNPG secret timing mitigation belongs in Phase 2, not Phase 3, because it blocks integration testing entirely on a cold cluster.

---

## Open Questions

1. **Init container vs. startup retry for secret timing:** Both are valid. Init container (`pg_isready` loop) is operationally transparent. Go retry loop is more robust for rolling restarts. Pick one before Phase 2 starts — do not implement both.

2. **SQLite PVC / uploads after migration:** The existing `persistence` PVC mounts at `/data` and currently holds `clay.db`. After migration, if `UPLOAD_DIR` still maps to `/data`, the PVC should be retained (but its purpose updated in comments). If uploads move to object storage (a future milestone), the PVC can be removed. Needs a decision before Phase 2 begins to avoid deploying storage that will be immediately removed.

3. **Goose vs. `CREATE TABLE IF NOT EXISTS` on startup:** PITFALLS.md notes that bare `CREATE TABLE IF NOT EXISTS` is technically workable for the initial deployment. ARCHITECTURE.md recommends Goose upfront while the schema is still simple. This summary recommends Goose — confirm before Phase 1 begins.

4. **CNPG CRD upgrade on `helm upgrade`:** Helm's documented behaviour is that CRDs in `crds/` directories are installed on `helm install` but never updated by `helm upgrade`. If CNPG releases a new CRD schema version, a `helm upgrade` of the parent chart will not update the CRD. Add a runbook entry in Phase 3: CRD upgrades require a manual `kubectl apply -f` of the new CRD manifests before running `helm upgrade`.

5. **`postgres.managed` vs `cloudnative-pg.enabled` dual-toggle:** Both flags in `values.yaml` express the same toggle. Use `condition: postgres.managed,cloudnative-pg.enabled` in Chart.yaml (Helm evaluates the first truthy path), or document clearly that they must be set together. Consider a `helm lint` check or a notes template warning if they diverge.

---

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | All versions verified on pkg.go.dev, cloudnative-pg.io, and GitHub releases. |
| Features (Helm) | HIGH | CNPG secret key names from 1.27/1.28 docs. Subchart condition pattern from official Helm docs. Secret timing mitigation rated MEDIUM by researcher (practical, not doc-sourced). |
| Architecture | HIGH | Code paths inspected directly at the file level. All 12 queries enumerated. Six architectural decisions with explicit rationale. |
| Pitfalls | HIGH | 15 pitfalls identified with risk ratings. All three critical pitfalls are independently verifiable from Postgres and pgx documentation. |

**Overall confidence:** HIGH

---

## Sources

- `pkg.go.dev/github.com/jackc/pgx/v5` — pgx v5.9.1 version, native API
- `pkg.go.dev/github.com/jackc/pgx/v5/pgxpool` — pool configuration, `OpenDBFromPool`
- `pkg.go.dev/github.com/jackc/pgx/v5/stdlib` — `database/sql` compatibility layer
- `github.com/jackc/pgx/issues/1483` — confirms `LastInsertId` not supported
- `cloudnative-pg.io/docs/1.25/applications/` — Cluster bootstrap spec, secret key names
- `cloudnative-pg.github.io/charts` — Helm chart 0.28.0, repository URL
- `postgresql.org` — CNPG 1.29.0 release announcement
- `helm.sh/docs/topics/charts/` — subchart conditions, `crds/` directory install/upgrade behaviour
- `helm.sh/docs/chart_template_guide/control_structures/` — `{{- if }}` gating
- `pressly.github.io/goose/blog/2021/embed-sql-migrations/` — Goose `//go:embed` pattern
- `pkg.go.dev/github.com/pressly/goose/v3` — Goose v3 programmatic API
- `postgresql.org/docs/current/ddl-identity-columns.html` — `GENERATED ALWAYS AS IDENTITY`
- `cloudnative-pg.io/docs/1.24/supported_releases/` — CNPG/Kubernetes version matrix
- `gabrielebartolini.it/articles/2024/03/cloudnativepg-recipe-2-inspecting-default-resources-in-a-cloudnativepg-cluster/` — secret naming convention
- `pkg.go.dev/vuln/GO-2026-4771`, `GO-2026-4772` — pgx CVEs (low severity, client unaffected)

---
*Research completed: 2026-04-13*
*Ready for roadmap: yes*
