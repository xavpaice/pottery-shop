# Stack Research: CNPG + pgx Migration

**Researched:** 2026-04-13
**Overall confidence:** HIGH (all findings verified against official sources)

---

## Go Database Driver

### pgx v5 — Confirmed Choice

**Latest stable version:** v5.9.1 (released 2026-03-22)
**Import path:** `github.com/jackc/pgx/v5`

pgx is a pure Go driver — no CGO, no C dependencies. This is the primary reason to choose it over `lib/pq` for this migration.

**Version history (recent):**
- v5.9.1 — 2026-03-22 (latest stable)
- v5.9.0 — 2026-03-21
- v5.8.0 — 2025-12-26
- v5.7.6 — 2025-09-08

**Security note (LOW severity, LOW urgency for this app):**
Two unpatched memory-safety vulnerabilities exist in `pgproto3` as of 2026-04-07: GO-2026-4771 (CVE-2026-33815) and GO-2026-4772 (CVE-2026-33816). Both affect `Backend.Receive` — the server-side wire protocol handler. This code path is only exercised when pgx acts as a Postgres server (unusual), not as a client. A standard Go app connecting *to* Postgres is not exposed to these. Use v5.9.1 anyway; monitor for a patch release.

Source: https://pkg.go.dev/vuln/GO-2026-4771, https://pkg.go.dev/vuln/GO-2026-4772

### Native pgx API vs database/sql — Use Native

**Recommendation: native pgx API via `pgxpool`** (not `database/sql`)

Rationale:
- This app is PostgreSQL-only — no multi-database portability needed
- Native API uses the binary wire protocol, which is faster
- `pgxpool` is built-in and production-ready; no extra dependency
- `database/sql` adds an abstraction layer with no benefit here
- `lib/pq` is in maintenance mode; pgx is the actively maintained successor

Use `database/sql` only if you add a library that requires it (e.g., sqlx, GORM). You can always drop to the native pgx connection from a `database/sql` driver via a type assertion if needed.

**Key imports:**

```go
// Native pgx with connection pool (recommended)
import (
    "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/pgxpool"
)

// database/sql compatibility (only if required)
import "github.com/jackc/pgx/v5/stdlib"
```

**Connection pool setup:**

```go
pool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
if err != nil {
    log.Fatal(err)
}
defer pool.Close()
```

The connection string format pgxpool accepts:
```
postgres://app:password@cluster-rw.namespace:5432/app?sslmode=require
```

**Query pattern:**

```go
var name string
err = pool.QueryRow(ctx, "SELECT name FROM products WHERE id=$1", id).Scan(&name)
```

Sources: https://pkg.go.dev/github.com/jackc/pgx/v5, https://pkg.go.dev/github.com/jackc/pgx/v5/pgxpool

---

## CloudNative-PG Operator

### Versions

| Component | Version | Notes |
|-----------|---------|-------|
| CNPG operator | 1.29.0 | Latest stable as of April 2026 |
| Helm chart | 0.28.0 | chart version — `appVersion: 1.29.0` |
| Kubernetes minimum | 1.29.0 | Required by chart |

1.29.0 is the latest stable operator release (announced on postgresql.org). 1.28.x EOL is June 30, 2026. 1.27.4 was the final 1.27.x release.

### Helm Chart

**Repository URL:** `https://cloudnative-pg.github.io/charts`
**Chart name:** `cloudnative-pg`

**Chart.yaml dependency block:**

```yaml
dependencies:
  - name: cloudnative-pg
    repository: https://cloudnative-pg.github.io/charts
    version: "0.28.0"
    condition: cnpg.enabled
```

The `condition` field is essential — it lets `values.yaml` skip installing the operator when `postgres.external.dsn` is set and CNPG is not needed (though typically the operator is always installed separately from the Cluster resource).

**To fetch the dependency after editing Chart.yaml:**

```bash
helm dependency update chart/clay/
```

Sources: https://cloudnative-pg.io/charts/, https://github.com/cloudnative-pg/charts

---

## CNPG Cluster Resource

The `Cluster` CRD is in `apiVersion: postgresql.cnpg.io/v1`. Key spec fields:

```yaml
apiVersion: postgresql.cnpg.io/v1
kind: Cluster
metadata:
  name: {{ .Release.Name }}-pg   # e.g. "pottery-shop-pg"
spec:
  instances: 1                   # configurable via values.yaml (3 for HA)
  storage:
    size: 1Gi                    # configurable via values.yaml
  bootstrap:
    initdb:
      database: app              # the database name created on init
      owner: app                 # the user that owns it
      # No secret.name needed — CNPG generates credentials automatically
  # superuserSecret is optional; omit to disable superuser access
```

**Configurable instances example in values.yaml:**

```yaml
postgres:
  instances: 1        # set to 3 for HA
  storage: 1Gi
  external:
    dsn: ""           # if set, skip Cluster creation entirely
```

**Helm template conditional (skip Cluster if external DSN is set):**

```yaml
{{- if not .Values.postgres.external.dsn }}
apiVersion: postgresql.cnpg.io/v1
kind: Cluster
...
{{- end }}
```

Sources: https://cloudnative-pg.io/docs/1.25/applications/, https://github.com/cloudnative-pg/cloudnative-pg/blob/main/docs/src/samples/cluster-example-full.yaml

---

## CNPG Credentials Secret

### Naming Convention

CNPG automatically generates two Kubernetes Secrets per Cluster:

| Secret name | Purpose |
|-------------|---------|
| `<cluster-name>-app` | Application credentials — use this |
| `<cluster-name>-superuser` | DBA/admin credentials — optional, can disable |

For a cluster named `pottery-shop-pg`, the app secret is `pottery-shop-pg-app`.

### Keys in the App Secret

| Key | Contents |
|-----|----------|
| `username` | `app` (the Postgres user) |
| `password` | generated password |
| `host` | RW service hostname (e.g. `pottery-shop-pg-rw.default`) |
| `port` | `5432` |
| `dbname` | `app` |
| `.pgpass` | pgpass-format connection string |
| `uri` | Full libpq URI — **use this as DATABASE_URL** |
| `jdbc-uri` | JDBC format (irrelevant for Go) |
| `fqdn-uri` | FQDN variant of `uri` |
| `fqdn-jdbc-uri` | FQDN variant of `jdbc-uri` |

**URI format example:**
```
postgresql://app:generated-password@pottery-shop-pg-rw.default:5432/app
```

### Using the Secret in a Pod

Reference the `uri` key directly as `DATABASE_URL`:

```yaml
env:
  - name: DATABASE_URL
    valueFrom:
      secretKeyRef:
        name: {{ .Release.Name }}-pg-app
        key: uri
```

When `postgres.external.dsn` is set, skip the secretKeyRef and inject the DSN directly:

```yaml
{{- if .Values.postgres.external.dsn }}
env:
  - name: DATABASE_URL
    value: {{ .Values.postgres.external.dsn | quote }}
{{- else }}
env:
  - name: DATABASE_URL
    valueFrom:
      secretKeyRef:
        name: {{ .Release.Name }}-pg-app
        key: uri
{{- end }}
```

Sources: https://cloudnative-pg.io/docs/1.25/applications/, https://www.gabrielebartolini.it/articles/2024/03/cloudnativepg-recipe-2-inspecting-default-resources-in-a-cloudnativepg-cluster/

---

## Docker Build

### CGO situation

**Current:** `go-sqlite3` requires `CGO_ENABLED=1` and a C compiler (GCC/Clang) at build time plus sqlite-libs at runtime.

**After migration to pgx:** pgx is pure Go. `CGO_ENABLED=0` works out of the box.

**Build change:**

```dockerfile
# Before (sqlite3):
RUN CGO_ENABLED=1 go build -o /app ./...

# After (pgx):
RUN CGO_ENABLED=0 go build -o /app ./...
```

### Multi-stage implications

With `CGO_ENABLED=0` the compiled binary is statically linked. This means:
- No need for `sqlite-libs` in the runtime image
- No need for GCC/Clang in the build stage (the `tonistiigi/xx` cross-compilation helper can be simplified or removed entirely)
- The runtime image can be a minimal Alpine or even `scratch`
- `ca-certificates` is still needed for TLS connections to Postgres

**Dockerfile runtime stage — remove:**
- `sqlite-libs` apk package
- Any CGO-related build args or toolchain setup

**Dockerfile runtime stage — keep:**
- `ca-certificates` (TLS to Postgres requires it)

**Makefile/CI change:** Remove or set `CGO_ENABLED=0`; remove `GCC_ENABLED` or similar flags if present.

Source: https://github.com/jackc/pgx#readme (explicitly: "pgx is a pure Go driver and toolkit for PostgreSQL")

---

## Recommendations

### Definitive version pins

| Component | Version | Confidence |
|-----------|---------|------------|
| pgx | v5.9.1 | HIGH — verified on pkg.go.dev |
| CNPG operator | 1.29.0 | HIGH — verified on cloudnative-pg.io and postgresql.org |
| CNPG Helm chart | 0.28.0 | HIGH — verified in GitHub Chart.yaml |

### Key choices

1. **Use pgx native API, not database/sql.** This app is Postgres-only. Native API is faster, simpler, and the pool (`pgxpool`) is included.

2. **Use `pgxpool.New()` at application startup.** One pool, injected into handlers. Close it on shutdown. Do not open per-request connections.

3. **Read `DATABASE_URL` from environment.** The CNPG `uri` secret key is a fully-formed libpq URI — pgxpool accepts it directly with no parsing needed.

4. **Set `CGO_ENABLED=0` in Dockerfile.** Remove all CGO build infrastructure. Simplifies the build and eliminates the sqlite/CGO runtime dependency.

5. **Name the CNPG Cluster `{{ .Release.Name }}-pg`.** This produces a predictable secret name `{{ .Release.Name }}-pg-app` that the Helm template can reference without hardcoding.

6. **Keep operator install separate from Cluster creation.** The CNPG operator (the subchart) is a cluster-level resource. The `Cluster` CR is namespace-scoped. Use `condition: cnpg.enabled` to allow installations where the operator is pre-installed.

7. **SQL dialect changes required:** Replace `INTEGER PRIMARY KEY` with `SERIAL` or `GENERATED ALWAYS AS IDENTITY`. Replace SQLite date/time functions. Replace `?` placeholders with `$1, $2, ...`.

### go.mod changes

```bash
# Remove:
go get -u github.com/mattn/go-sqlite3  # then manually delete from go.mod

# Add:
go get github.com/jackc/pgx/v5@v5.9.1
```

**Resulting require block:**
```
require (
    github.com/disintegration/imaging v1.6.2
    github.com/jackc/pgx/v5 v5.9.1
)
```
