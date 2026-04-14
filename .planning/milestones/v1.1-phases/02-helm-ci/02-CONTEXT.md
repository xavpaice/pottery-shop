# Phase 2: Helm + CI - Context

**Gathered:** 2026-04-14
**Status:** Ready for planning

<domain>
## Phase Boundary

The Helm chart supports both managed CNPG (in-cluster Postgres via CNPG operator as subchart) and external Postgres (plain DSN injected as `DATABASE_URL`). The app pod starts reliably on first `helm install` — no manual intervention required. The CI pipeline validates build (CGO-free), tests (testcontainers-go Postgres), and Helm rendering (both modes) on every push.

Scope: Helm chart wiring (CNPG subchart, Cluster template, secret injection, timing mitigation, DB_PATH cleanup) + CI workflow updates (remove CGO, testcontainers, Helm validation).
Kubernetes operator internals, data migration, and the existing CMX integration test are out of scope.

</domain>

<decisions>
## Implementation Decisions

### CNPG Secret Timing Mitigation (HELM-06)
- **D-01:** Use an init container with `postgres:16-alpine` that runs `pg_isready -h <cluster>-rw` in a retry loop. App container only starts when Postgres is actually accepting connections.
- **D-02:** No RBAC/kubectl approach — init container connects directly to Postgres (no k8s secret-read permissions needed).
- **D-03:** The CNPG RW service name is derived from the release name: `{{ include "clay.fullname" . }}-rw` (CNPG convention). Init container env reads `DATABASE_URL` or a derived host from the cluster name.
- **D-04:** The init container is only rendered when `postgres.managed: true` — not needed in external DSN mode.

### Deployment Strategy
- **D-05:** Change `strategy: Recreate` to `strategy: RollingUpdate`. The Recreate strategy was required for SQLite single-writer; with Postgres, multiple replicas can share the database. Remove the "SQLite requires single-writer" comment.

### CNPG Subchart + Cluster Resource (HELM-01, HELM-02, HELM-03)
- **D-06:** Add CNPG operator as a Helm subchart dependency at version `0.28.0` with condition `cloudnative-pg.enabled`.
- **D-07:** Add a `postgres` block to `values.yaml`:
  ```yaml
  postgres:
    managed: true
    cluster:
      instances: 1
      storage:
        size: 5Gi
        storageClass: ""
  external:
    dsn: ""
  ```
- **D-08:** Create `templates/cnpg-cluster.yaml` rendered only when `postgres.managed: true` (guarded by `{{- if .Values.postgres.managed }}`).

### DATABASE_URL Injection (HELM-04, HELM-05)
- **D-09:** In managed mode: inject `DATABASE_URL` via `env: valueFrom: secretKeyRef` pointing to `<cluster>-app`, key `uri` (CNPG-generated secret name convention).
- **D-10:** In external mode: inject `DATABASE_URL` from `postgres.external.dsn` as a plain env value (or via a Kubernetes Secret for the release).
- **D-11:** The `config` ConfigMap no longer includes `DATABASE_URL` — it comes from the CNPG secret or a separately managed secret.

### DB_PATH Cleanup (HELM-07)
- **D-12:** Remove `DB_PATH: "/data/clay.db"` from `values.yaml` config block and any configmap templates.
- **D-13:** The `persistence` block remains for `/data/uploads` — rename/update comments to clarify it covers uploads only (not SQLite).

### CI Test Job (CI-01, CI-02)
- **D-14:** Update the existing `test` job in `.github/workflows/test.yml`: remove `apt-get install gcc`, add `CGO_ENABLED=0` to `make test` and `make build` steps.
- **D-15:** Testcontainers-go uses the Docker daemon pre-installed on `ubuntu-latest` runners — no extra setup needed. `go test ./...` runs integration tests inline.
- **D-16:** The existing CMX integration test (`integration-test.yml`) is left as-is in Phase 2 — it will need a separate update for CNPG, but that is out of scope here.

### Helm Validation CI (CI-03)
- **D-17:** Extend the existing `helm-lint` job in `test.yml` to also run `helm template` for both modes:
  - `helm lint chart/clay/ --values chart/clay/ci/managed-values.yaml`
  - `helm lint chart/clay/ --values chart/clay/ci/external-values.yaml`
  - `helm template clay chart/clay/ --values chart/clay/ci/managed-values.yaml`
  - `helm template clay chart/clay/ --values chart/clay/ci/external-values.yaml`
- **D-18:** Test values files live in `chart/clay/ci/` (Helm chart-testing convention):
  - `chart/clay/ci/managed-values.yaml` — sets `postgres.managed: true`
  - `chart/clay/ci/external-values.yaml` — sets `postgres.managed: false`, `postgres.external.dsn: "postgresql://user:pass@host:5432/db"`

### Claude's Discretion
- CNPG Cluster name: follow `{{ include "clay.fullname" . }}-postgres` pattern — predictable, derived from release name
- CNPG `ownerReferences` or explicit deletion policy: Claude's choice for lifecycle management
- Init container retry interval and timeout: `until pg_isready ...; do sleep 2; done` pattern — Claude decides exact loop
- Makefile targets for `helm-lint` and `helm-validate` (if splitting into separate targets vs inline in workflow)

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements
- `.planning/REQUIREMENTS.md` — Full requirement list; Phase 2 requirements are HELM-01 through HELM-07, CI-01 through CI-03

### Helm Chart (current state — read before modifying)
- `chart/clay/Chart.yaml` — Current chart metadata; subchart dependency block goes here
- `chart/clay/values.yaml` — Current values structure; postgres block and DB_PATH removal targets
- `chart/clay/templates/deployment.yaml` — Deployment template; strategy change + init container + DATABASE_URL injection go here
- `chart/clay/templates/configmap.yaml` — ConfigMap template; DB_PATH removal target

### CI (current state — read before modifying)
- `.github/workflows/test.yml` — Existing test + helm-lint jobs; CI-01, CI-02, CI-03 targets
- `.github/workflows/build.yml` — Build/push workflow (reference only — minimal changes expected)
- `.github/workflows/integration-test.yml` — CMX cluster test (read to understand scope; do NOT modify in this phase)

### Phase 1 Context (cross-phase consistency)
- `.planning/phases/01-go-build/01-CONTEXT.md` — DATABASE_URL decisions (D-11, D-12), CGO removal decisions (D-13, D-14)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `chart/clay/templates/_helpers.tpl` — `clay.fullname`, `clay.labels`, `clay.selectorLabels` helpers; use these for CNPG Cluster name derivation and init container labels
- `envFrom: configMapRef + secretRef` pattern in deployment.yaml — `DATABASE_URL` will augment this with an explicit `env: valueFrom: secretKeyRef` (takes precedence over envFrom in Kubernetes)

### Established Patterns
- All Helm values accessed as `.Values.*` — postgres block follows same pattern as existing `config.*` and `persistence.*` blocks
- Conditional rendering already used in `deployment.yaml` (`{{- with .Values.readinessProbe }}` etc.) — use same pattern for init container and CNPG Cluster conditional
- `ConfigMap` renders `config.*` as a range loop — removing `DB_PATH` means removing it from `values.yaml`, it disappears from configmap automatically

### Integration Points
- `chart/clay/templates/deployment.yaml` — primary change target: strategy, init container, DATABASE_URL env
- `chart/clay/Chart.yaml` — add `dependencies:` block for CNPG subchart
- `chart/clay/values.yaml` — add `postgres` block, remove `config.DB_PATH`
- `.github/workflows/test.yml` — update `test` job + extend `helm-lint` job

</code_context>

<specifics>
## Specific Ideas

- CNPG chart version pinned to `0.28.0` per HELM-01 requirement
- Init container pattern: `until pg_isready -h {{ include "clay.fullname" . }}-rw; do sleep 2; done` — simple, no RBAC
- CI values files in `chart/clay/ci/` follow the [helm/chart-testing](https://github.com/helm/chart-testing) convention for per-mode test values
- The existing `helm-lint` Makefile target can be extended to cover both modes without adding new targets

</specifics>

<deferred>
## Deferred Ideas

- CMX integration test update for CNPG — acknowledged as needed, deferred to a follow-up phase or ad-hoc work
- CNPG backup configuration (WAL archiving) — v2 requirement OPS-01
- CNPG monitoring / Prometheus metrics — v2 requirement OPS-02
- CRD upgrade runbook for `helm upgrade` — v2 requirement OPS-03

</deferred>

---

*Phase: 02-helm-ci*
*Context gathered: 2026-04-14*
