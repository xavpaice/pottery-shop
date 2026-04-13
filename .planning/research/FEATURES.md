# Features Research: Helm + CNPG Patterns

**Project:** pottery-shop clay chart
**Researched:** 2026-04-13
**Overall confidence:** HIGH (CNPG docs verified, Helm patterns from official docs)

---

## Values.yaml Structure

The canonical Helm pattern for dual-mode database configuration uses a top-level `postgres` block with a
`mode` discriminator (or a nested `external` block whose presence implies external mode). The clearest
approach for this chart is a boolean flag `postgres.managed` combined with an `external.dsn` field, because:

- Boolean flag is trivially evaluable in Helm `if` blocks
- `external.dsn` being empty ("") is an unambiguous no-op, and explicitly set means external mode
- Matches the Airflow chart convention of `postgresql.enabled: false` + `externalDatabase: {}` (widely
  understood by operators)

Proposed structure to add to `values.yaml`:

```yaml
postgres:
  # Set to true to deploy a CNPG-managed Postgres cluster inside this release.
  # Set to false and provide external.dsn to use an external Postgres instance.
  managed: true

  # Only used when managed: true
  cluster:
    instances: 1          # Set to 3 for HA
    imageName: "ghcr.io/cloudnative-pg/postgresql:16"
    storage:
      size: 8Gi
      storageClass: ""    # Empty = cluster default StorageClass

  # Only used when managed: false
  external:
    dsn: ""               # e.g. "postgres://user:pass@host:5432/db"

# cloudnative-pg subchart is gated by postgres.managed above
cloudnative-pg:
  enabled: true           # mirrors postgres.managed via Chart.yaml condition field
```

Note: `DB_PATH` in `config` is a SQLite artifact. It should be removed once the Postgres migration is
complete, since `DATABASE_URL` replaces it entirely.

Confidence: HIGH — matches official Helm subchart condition pattern and CNPG documentation.

---

## Conditional Resource Creation

Helm's `{{- if ... }}` block gates entire resource files. The standard idiom is:

```yaml
{{- if .Values.postgres.managed }}
apiVersion: postgresql.cnpg.io/v1
kind: Cluster
metadata:
  name: {{ include "clay.fullname" . }}-pg
  ...
{{- end }}
```

This means `templates/cnpg-cluster.yaml` renders nothing when `postgres.managed: false`. No partial
resources, no errors.

For the `env` block inside `deployment.yaml`, use an if/else to choose between a `secretKeyRef` into the
CNPG-generated secret vs. a plain `value` from the DSN string:

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

The existing deployment uses `envFrom.secretRef` for the app's own secret (ADMIN_PASS etc). Adding a
separate named `env` entry for DATABASE_URL alongside `envFrom` is valid Kubernetes — both are applied
to the container.

Confidence: HIGH — from official Helm flow control docs and Kubernetes env spec.

---

## Secret Mounting

### Managed mode: CNPG-generated Secret

CNPG automatically creates a Secret named `<cluster-name>-app` when a Cluster resource is created.
The confirmed key names inside this secret (from CNPG 1.27/1.28 docs) are:

| Key | Contents |
|-----|----------|
| `username` | Database username (app) |
| `password` | Plain password |
| `uri` | `postgresql://app:pass@cluster-rw.ns:5432/app` |
| `jdbc-uri` | JDBC connection URL |
| `pgpass` | `.pgpass` file format |
| `dbname` | Database name |
| `host` | RW service hostname |
| `port` | Port number |

The `uri` key is what should be mapped to `DATABASE_URL`. The secret name follows the pattern
`<cluster-metadata-name>-app`. If the Cluster metadata name is `{{ include "clay.fullname" . }}-pg`,
the secret name is `{{ include "clay.fullname" . }}-pg-app`.

This is a pre-existing secret created by the CNPG operator — the Helm chart does NOT create it.
The chart just references it via `secretKeyRef`.

### External mode: Plain env var injection

When `postgres.managed: false`, the DSN from `postgres.external.dsn` is injected directly as a plain
environment variable. No secret object is created by the chart. If the caller wants to avoid putting
the DSN in values.yaml (which ends up in Helm release history), they can instead store it in a
pre-existing Secret and reference it via `secretKeyRef` with a configurable secret name — but that is
a "nice to have" enhancement, not table stakes for this milestone.

### Timing concern: CNPG Secret availability

The CNPG operator creates the `-app` Secret asynchronously after the Cluster resource is accepted.
The app Pod will fail `CreateContainerConfigError` if it starts before the Secret exists. Mitigation
options (in order of simplicity):

1. CNPG itself gates the Cluster readiness — once the Cluster reaches `Healthy` phase, the Secret
   exists. A Helm post-install hook or init container can wait for it, but for a single-instance hobby
   deployment a sufficiently high `initialDelaySeconds` on readinessProbe is often enough.
2. Use `optional: false` on the `secretKeyRef` (the default) — pod stays in
   `CreateContainerConfigError` and retries, which is acceptable since CNPG creates the secret quickly.

Confidence: HIGH for secret key names (verified from CNPG 1.27 docs). MEDIUM for timing — practical
experience, not from official docs.

---

## CNPG Subchart Configuration

### Chart.yaml dependency entry

```yaml
dependencies:
  - name: cloudnative-pg
    version: "0.28.0"
    repository: "https://cloudnative-pg.github.io/charts"
    condition: cloudnative-pg.enabled
```

The `condition` field points to a path in the parent chart's `values.yaml`. When `cloudnative-pg.enabled`
is `false`, Helm skips installing the subchart entirely (no CRDs, no operator deployment). This is the
standard Helm mechanism for optional subcharts — documented in `helm.sh/docs/topics/charts/`.

The condition value must live at top level of the parent `values.yaml` under the subchart alias key:

```yaml
# values.yaml
cloudnative-pg:
  enabled: true  # controlled by postgres.managed logic at install time
```

Since `cloudnative-pg.enabled` and `postgres.managed` express the same toggle, document clearly in
values.yaml that they must match. Alternatively, Helm's `condition` field accepts comma-separated
paths, so you could write `condition: postgres.managed,cloudnative-pg.enabled` — Helm evaluates the
first truthy path found.

After adding the dependency, run:

```bash
helm dependency update chart/clay
```

This downloads the CNPG operator chart into `chart/clay/charts/`.

Current CNPG operator Helm chart: `cloudnative-pg-v0.28.0` (released 2026-04-01).
Operator chart supports only the latest point release of the CNPG operator — pin to `"0.28.0"` or
use `"~0.28"` for patch-level auto-updates.

Confidence: HIGH — repo URL and version from official CNPG GitHub releases.

---

## Configurable CNPG Cluster Fields

These are the fields that belong in `values.yaml` because operators commonly need to tune them:

| Field | values.yaml path | Default | Rationale |
|-------|-----------------|---------|-----------|
| Instance count | `postgres.cluster.instances` | `1` | HA requires 3; hobby default is 1 |
| PostgreSQL image | `postgres.cluster.imageName` | `ghcr.io/cloudnative-pg/postgresql:16` | Pin major version; operator handles minor updates |
| Storage size | `postgres.cluster.storage.size` | `8Gi` | Per-instance PVC size |
| Storage class | `postgres.cluster.storage.storageClass` | `""` | Empty = cluster default; needed when cluster has multiple StorageClasses |
| Database name | `postgres.cluster.database` | `"app"` | CNPG defaults to "app"; expose for overriding |
| Superuser secret | `postgres.cluster.superuserSecret` | `""` | Optional; CNPG can manage it |

Fields to NOT expose (keep internal to the Cluster template, hardcoded or derived):

- `metadata.name` — derive from `{{ include "clay.fullname" . }}-pg` so the app secret name is
  predictable
- `bootstrap.initdb.database` / `owner` — hardcode to match the app user the Go binary uses
- Resource requests/limits on the Postgres pods — defer to a later phase; CNPG defaults are sensible

The minimal `templates/cnpg-cluster.yaml` that covers this chart's requirements:

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

Confidence: HIGH — Cluster spec fields from official CNPG quickstart and full-example YAML.

---

## Table Stakes vs Differentiators

### Table Stakes (must have for the milestone to be complete)

| Feature | Why Essential | Implementation |
|---------|--------------|----------------|
| `postgres.managed: true/false` toggle | Core requirement — dual-mode support | Boolean in values.yaml |
| Conditional CNPG Cluster resource | Without this, managed mode is broken | `{{- if .Values.postgres.managed }}` gate in template |
| `secretKeyRef` to CNPG app secret `uri` key | App must get DATABASE_URL in managed mode | `env[].valueFrom.secretKeyRef` in deployment |
| External DSN injection as DATABASE_URL | External mode requirement | `env[].value` in deployment when not managed |
| CNPG operator as subchart with `condition` | Single `helm install` must deploy operator + cluster | `Chart.yaml` dependency with `condition: cloudnative-pg.enabled` |
| Configurable `instances` | Explicitly required by PROJECT.md | `postgres.cluster.instances` in values.yaml |
| Configurable `storage.size` | Per-instance PVC is set at cluster creation — can't resize easily later | `postgres.cluster.storage.size` in values.yaml |
| Remove `DB_PATH` from ConfigMap | SQLite artifact — will confuse the app if left in | Delete from `config` block and ConfigMap template |
| `strategy: type` change in Deployment | SQLite comment says Recreate; Postgres allows RollingUpdate | Change strategy comment; Recreate is still safe |

### Differentiators (useful but not required for this milestone)

| Feature | Value | When to Add |
|---------|-------|-------------|
| External DSN from existing Secret (not plaintext in values) | Avoids DSN in Helm history | Later phase — security hardening |
| `postgres.cluster.imageName` override | Lets operator pin exact PG version | Include now (low cost) |
| `postgres.cluster.storage.storageClass` override | Needed in multi-SC clusters | Include now (low cost) |
| CNPG backup config (barman/S3) | Production readiness | Separate milestone |
| Pod anti-affinity for HA (3-instance) | Prevents all instances on one node | When instances > 1 |
| PodDisruptionBudget for Postgres pods | Prevents simultaneous draining | Separate milestone |
| Wait init container for CNPG Secret | Prevents race condition on first deploy | Nice to have; not required |

### Anti-Features (explicitly do not build)

| Anti-Feature | Why Avoid |
|--------------|-----------|
| Bundling both `DB_PATH` and `DATABASE_URL` simultaneously | Ambiguity in app config; clean cut is simpler |
| Custom CNPG user/password management | CNPG owns credentials; fighting it creates drift |
| SQLite fallback mode | PROJECT.md explicitly out of scope |
| Exposing all CNPG Cluster spec fields as values | Over-engineering; expose only what this app needs |

---

## Sources

- Helm official docs — subchart conditions: https://helm.sh/docs/topics/charts/
- Helm flow control: https://helm.sh/docs/chart_template_guide/control_structures/
- CNPG app secret keys: https://cloudnative-pg.io/docs/1.27/applications/
- CNPG external secrets (key names confirmed): https://cloudnative-pg.io/docs/1.28/cncf-projects/external-secrets/
- CNPG Cluster spec (instances, storage, imageName): https://cloudnative-pg.io/documentation/1.20/quickstart/ and https://github.com/cloudnative-pg/cloudnative-pg/blob/main/docs/src/samples/cluster-example-full.yaml
- CNPG Helm chart repo: https://cloudnative-pg.github.io/charts (v0.28.0 as of 2026-04-01)
- Airflow chart external DB pattern: https://github.com/airflow-helm/charts/blob/main/charts/airflow/docs/faq/database/external-database.md
- CNPG secret naming (recipe article): https://www.gabrielebartolini.it/articles/2024/03/cloudnativepg-recipe-2-inspecting-default-resources-in-a-cloudnativepg-cluster/
