# Phase 2: Helm + CI - Research

**Researched:** 2026-04-14
**Domain:** Helm subchart dependencies (CNPG), Kubernetes Deployment templating, GitHub Actions CI
**Confidence:** HIGH

## Summary

Phase 2 wires the clay Helm chart to CloudNative-PG as a subchart, injects `DATABASE_URL` correctly in two modes (managed vs external), and extends the CI pipeline to validate both modes. All major decisions are locked in CONTEXT.md, so research focuses on verifying the exact CNPG conventions the planner needs to produce correct YAML: secret naming, service naming, Cluster CRD field names, and the Chart.yaml dependency block syntax.

The Helm wiring follows CNPG conventions that are well-documented and stable across versions 1.24-1.29. The generated secret for the app user is named `{cluster-name}-app` and contains a `uri` key with the full connection string. The RW service is named `{cluster-name}-rw`. The init container pattern using `pg_isready` is a widely-used, no-RBAC approach that works reliably. CI changes are minor: drop the `apt-get install gcc` step, add `CGO_ENABLED=0`, run `helm dependency update` before lint, and add `helm template` validation steps.

**Primary recommendation:** Follow the locked CONTEXT.md decisions exactly. The only discretionary choices are the Cluster name helper (`{{ include "clay.fullname" . }}-postgres`), init container loop timing (sleep 2 between attempts), and whether to add a `helm-validate` Makefile target alongside `helm-lint`.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**CNPG Secret Timing Mitigation (HELM-06)**
- D-01: Use an init container with `postgres:16-alpine` that runs `pg_isready -h <cluster>-rw` in a retry loop. App container only starts when Postgres is actually accepting connections.
- D-02: No RBAC/kubectl approach — init container connects directly to Postgres (no k8s secret-read permissions needed).
- D-03: The CNPG RW service name is derived from the release name: `{{ include "clay.fullname" . }}-rw` (CNPG convention). Init container env reads `DATABASE_URL` or a derived host from the cluster name.
- D-04: The init container is only rendered when `postgres.managed: true` — not needed in external DSN mode.

**Deployment Strategy**
- D-05: Change `strategy: Recreate` to `strategy: RollingUpdate`. Remove "SQLite requires single-writer" comment.

**CNPG Subchart + Cluster Resource (HELM-01, HELM-02, HELM-03)**
- D-06: Add CNPG operator as Helm subchart dependency at version `0.28.0` with condition `cloudnative-pg.enabled`.
- D-07: Add a `postgres` block to `values.yaml` with `managed`, `cluster.instances`, `cluster.storage.size`, `cluster.storage.storageClass`, and `external.dsn`.
- D-08: Create `templates/cnpg-cluster.yaml` rendered only when `postgres.managed: true`.

**DATABASE_URL Injection (HELM-04, HELM-05)**
- D-09: Managed mode: inject `DATABASE_URL` via `env: valueFrom: secretKeyRef` pointing to `<cluster>-app`, key `uri`.
- D-10: External mode: inject `DATABASE_URL` from `postgres.external.dsn` as a plain env value.
- D-11: ConfigMap no longer includes `DATABASE_URL` — it comes from CNPG secret or separately managed secret.

**DB_PATH Cleanup (HELM-07)**
- D-12: Remove `DB_PATH: "/data/clay.db"` from `values.yaml` config block and any configmap templates.
- D-13: The `persistence` block remains for `/data/uploads` — update comments to clarify uploads only.

**CI Test Job (CI-01, CI-02)**
- D-14: Update existing `test` job in `.github/workflows/test.yml`: remove `apt-get install gcc`, add `CGO_ENABLED=0`.
- D-15: Testcontainers-go uses Docker daemon pre-installed on `ubuntu-latest` — no extra setup needed.
- D-16: Existing CMX integration test (`integration-test.yml`) left as-is in Phase 2.

**Helm Validation CI (CI-03)**
- D-17: Extend existing `helm-lint` job to also run `helm template` for both modes using `chart/clay/ci/` values files.
- D-18: Test values files live in `chart/clay/ci/`: `managed-values.yaml` and `external-values.yaml`.

### Claude's Discretion
- CNPG Cluster name: follow `{{ include "clay.fullname" . }}-postgres` pattern
- CNPG ownerReferences or explicit deletion policy: Claude's choice
- Init container retry interval: `until pg_isready ...; do sleep 2; done` — Claude decides exact loop
- Makefile targets for helm-lint and helm-validate (split or inline)

### Deferred Ideas (OUT OF SCOPE)
- CMX integration test update for CNPG
- CNPG backup configuration (WAL archiving)
- CNPG monitoring / Prometheus metrics
- CRD upgrade runbook for `helm upgrade`
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| HELM-01 | Add CNPG operator as Helm subchart dependency (chart 0.28.0, condition: `cloudnative-pg.enabled`) | Chart.yaml dependencies block syntax verified; repo URL confirmed as `https://cloudnative-pg.github.io/charts`; version 0.28.0 confirmed current (released 2026-04-01) |
| HELM-02 | Add `postgres` block to values.yaml | Exact structure locked in D-07 |
| HELM-03 | Create `templates/cnpg-cluster.yaml` rendered only when `postgres.managed: true` | CNPG Cluster CRD fields (`spec.instances`, `spec.storage.size`, `spec.storage.storageClass`) verified from official docs |
| HELM-04 | Inject `DATABASE_URL` from CNPG-generated Secret in managed mode | Secret naming convention `{cluster-name}-app` verified; key name `uri` verified from CNPG docs 1.27 |
| HELM-05 | Inject `DATABASE_URL` from `postgres.external.dsn` in external mode | Standard Helm `env.value` pattern; no CNPG dependency |
| HELM-06 | Add timing mitigation for CNPG Secret race (initContainer) | `pg_isready` init container pattern; RW service name `{cluster-name}-rw` verified from CNPG docs |
| HELM-07 | Remove `DB_PATH` SQLite artifact from values and configmap | `DB_PATH` found in values.yaml line 37; configmap uses `range` over `.Values.config` so removal from values auto-removes from configmap |
| CI-01 | Update build job: set `CGO_ENABLED=0`, remove CGO steps | Makefile already has `CGO_ENABLED=0`; test.yml still has `apt-get install gcc` to remove |
| CI-02 | Add test job: `go vet` + `golangci-lint` + `go test` (testcontainers-go) | Docker daemon confirmed available on `ubuntu-latest`; no extra setup required |
| CI-03 | Add Helm validation job: `helm lint` + `helm template` render check | `helm dependency update` must precede lint/template when Chart.lock present; `chart/clay/ci/` directory to create |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| cloudnative-pg Helm chart | 0.28.0 | CNPG operator subchart | Latest stable (2026-04-01); locked in CONTEXT.md D-06 |
| azure/setup-helm | v4 | Install Helm in GitHub Actions | Already used in existing `helm-lint` job |
| actions/setup-go | v5 | Install Go in GitHub Actions | Already used in existing `test` job |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| postgres:16-alpine | latest in range | Init container image for `pg_isready` | Managed mode only (D-04); lightweight, includes pg_isready binary |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| init container (`pg_isready`) | startupProbe retry | Init container chosen (D-01) — gives clear timing separation, no RBAC |
| init container (`pg_isready`) | CNPG `readyWhenNodeIsReady` | Not supported in CNPG without extra config; init container is simpler |
| `azure/setup-helm@v4` | `helm/chart-testing-action` | chart-testing-action adds overhead not needed for simple lint+template |

**Installation (CI — no local Helm required):**
```bash
# In Chart.yaml dependencies — downloaded by helm dependency update in CI
helm repo add cnpg https://cloudnative-pg.github.io/charts
helm dependency update chart/clay/
```

**Version verification:** [VERIFIED: cloudnative-pg.io/charts/index.yaml] — version 0.28.0, appVersion 1.29.0, released 2026-04-01.

## Architecture Patterns

### Recommended Project Structure (additions to existing chart)
```
chart/clay/
├── Chart.yaml              # Add dependencies: block for CNPG
├── Chart.lock              # Generated by helm dependency update — commit to repo
├── charts/                 # Downloaded subcharts — gitignore this dir
├── ci/                     # NEW: test values for helm lint/template CI
│   ├── managed-values.yaml
│   └── external-values.yaml
├── values.yaml             # Add postgres: block, remove DB_PATH
└── templates/
    ├── cnpg-cluster.yaml   # NEW: CNPG Cluster CRD — rendered when managed: true
    └── deployment.yaml     # Update: strategy, initContainer, DATABASE_URL env
```

### Pattern 1: Chart.yaml Subchart Dependency
**What:** Declares CNPG operator as a conditional subchart dependency.
**When to use:** Single chart manages both operator and application resources.
**Example:**
```yaml
# chart/clay/Chart.yaml
# Source: https://helm.sh/docs/chart_best_practices/dependencies/
apiVersion: v2
name: clay
description: A Helm chart for the Clay pottery shop
type: application
version: 0.1.0
appVersion: "1.0.0"

dependencies:
  - name: cloudnative-pg
    repository: https://cloudnative-pg.github.io/charts
    version: "0.28.0"
    condition: cloudnative-pg.enabled
```

```yaml
# chart/clay/values.yaml — controlling the condition
cloudnative-pg:
  enabled: true
```

### Pattern 2: CNPG Cluster CRD Template
**What:** Minimal CNPG Cluster resource rendered conditionally.
**When to use:** `postgres.managed: true` (in-cluster managed Postgres).
**Example:**
```yaml
# chart/clay/templates/cnpg-cluster.yaml
# Source: cloudnative-pg.io/docs/1.27 Cluster spec
{{- if .Values.postgres.managed }}
apiVersion: postgresql.cnpg.io/v1
kind: Cluster
metadata:
  name: {{ include "clay.fullname" . }}-postgres
  labels:
    {{- include "clay.labels" . | nindent 4 }}
spec:
  instances: {{ .Values.postgres.cluster.instances }}
  storage:
    size: {{ .Values.postgres.cluster.storage.size }}
    {{- if .Values.postgres.cluster.storage.storageClass }}
    storageClassName: {{ .Values.postgres.cluster.storage.storageClass | quote }}
    {{- end }}
{{- end }}
```

### Pattern 3: DATABASE_URL Injection (managed mode)
**What:** Reads `uri` key from CNPG-generated secret via `secretKeyRef`.
**When to use:** `postgres.managed: true` — CNPG owns the secret lifecycle.

The CNPG operator generates a secret named `{cluster-name}-app`. For a cluster named `{fullname}-postgres`, the secret is `{fullname}-postgres-app`. The secret contains these keys: `username`, `password`, `host`, `port`, `dbname`, `pgpass`, `uri`, `jdbc-uri`, `fqdn-uri`, `fqdn-jdbc-uri`. [VERIFIED: cloudnative-pg.io/docs/1.27/applications/]

```yaml
# In deployment.yaml containers[0] — explicit env takes precedence over envFrom
# Source: https://cloudnative-pg.io/docs/1.27/applications/
{{- if .Values.postgres.managed }}
env:
  - name: DATABASE_URL
    valueFrom:
      secretKeyRef:
        name: {{ include "clay.fullname" . }}-postgres-app
        key: uri
{{- else if .Values.postgres.external.dsn }}
env:
  - name: DATABASE_URL
    value: {{ .Values.postgres.external.dsn | quote }}
{{- end }}
```

**Critical:** In Kubernetes, an explicit `env` entry takes precedence over the same key in `envFrom`. Since the existing `envFrom` reads from the clay Secret, adding an explicit `env.DATABASE_URL` entry correctly overrides any residual DATABASE_URL in the secret. [ASSUMED — standard Kubernetes env precedence behavior, well-documented but not re-verified this session]

### Pattern 4: Init Container for Postgres Readiness
**What:** Blocks the app container start until Postgres RW service accepts connections.
**When to use:** Managed mode only (D-04). The CNPG secret race condition means the secret may exist before Postgres is ready.

CNPG creates three services per cluster: `{cluster-name}-rw` (primary, read-write), `{cluster-name}-ro` (replicas), `{cluster-name}-r` (any instance). [VERIFIED: cloudnative-pg.io/documentation/1.24/service_management/]

```yaml
# In deployment.yaml spec.template.spec — before containers:
# Source: CONTEXT.md D-01, CNPG service naming convention
{{- if .Values.postgres.managed }}
initContainers:
  - name: wait-for-postgres
    image: postgres:16-alpine
    command:
      - sh
      - -c
      - |
        until pg_isready -h {{ include "clay.fullname" . }}-postgres-rw; do
          echo "Waiting for Postgres..."
          sleep 2
        done
        echo "Postgres is ready."
{{- end }}
```

**Note on cluster name vs service name:** The Cluster is named `{fullname}-postgres`, so the RW service is `{fullname}-postgres-rw`. This aligns with CNPG naming: `{cluster-name}-rw`. [VERIFIED: CNPG service naming from docs]

### Pattern 5: Helm Dependency in CI
**What:** `helm dependency update` downloads subcharts before lint/template in CI.
**When to use:** Any CI job that runs `helm lint` or `helm template` when Chart.yaml has a `dependencies:` block.

```yaml
# In .github/workflows/test.yml helm-lint job
- name: Add CNPG Helm repo
  run: helm repo add cnpg https://cloudnative-pg.github.io/charts

- name: Update Helm dependencies
  run: helm dependency update chart/clay/

- name: Lint Helm chart (managed mode)
  run: helm lint chart/clay/ --values chart/clay/ci/managed-values.yaml

- name: Lint Helm chart (external mode)
  run: helm lint chart/clay/ --values chart/clay/ci/external-values.yaml

- name: Template Helm chart (managed mode)
  run: helm template clay chart/clay/ --values chart/clay/ci/managed-values.yaml

- name: Template Helm chart (external mode)
  run: helm template clay chart/clay/ --values chart/clay/ci/external-values.yaml
```

**Chart.lock:** Commit `Chart.lock` to the repo. In CI, use `helm dependency build` (uses lock) for reproducible builds, or `helm dependency update` (re-resolves) when you want to pick up updates. For this phase, `helm dependency update` is correct since Chart.lock doesn't exist yet. [VERIFIED: helm.sh/docs/helm/helm_dependency_update/]

### Pattern 6: CI Values Test Files
**What:** Minimal values files that exercise each mode's render path.
```yaml
# chart/clay/ci/managed-values.yaml
postgres:
  managed: true
  cluster:
    instances: 1
    storage:
      size: 1Gi
      storageClass: ""
cloudnative-pg:
  enabled: true
```

```yaml
# chart/clay/ci/external-values.yaml
postgres:
  managed: false
  external:
    dsn: "postgresql://user:pass@external-host:5432/clay"
cloudnative-pg:
  enabled: false
```

### Anti-Patterns to Avoid
- **Putting DATABASE_URL in the clay ConfigMap:** The configmap uses `range .Values.config` — removing `DB_PATH` from values.yaml is sufficient. Never add DATABASE_URL to `.Values.config` — it should come only from the CNPG secret or explicit env.
- **Rendering the init container in external mode:** The init container must be wrapped in `{{- if .Values.postgres.managed }}` — external mode has no CNPG service to wait for.
- **Running `helm lint` without `helm dependency update` first:** Once Chart.yaml has a `dependencies:` block, `helm lint` fails with "found in Chart.yaml, but missing in charts/ directory" unless subcharts are downloaded first.
- **Committing `charts/` directory:** The `charts/` subdirectory (downloaded subcharts) should be in `.gitignore`. Commit `Chart.lock` instead.
- **Using `{{ .Values.fullnameOverride }}` directly instead of the helper:** Always use `{{ include "clay.fullname" . }}` for consistent name derivation including release prefix logic.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Postgres readiness check | Custom TCP wait script | `pg_isready` from `postgres:16-alpine` | pg_isready understands Postgres protocol; TCP check passes before Postgres accepts queries |
| Connection string injection | Manual secret mounting + env parsing | CNPG-generated secret + `secretKeyRef` | CNPG rotates credentials; manual management breaks rotation |
| Subchart lifecycle | Separate operator install step | `dependencies:` in Chart.yaml | Helm manages installation order and lifecycle |
| Helm CI validation | Custom YAML parser | `helm lint` + `helm template` | Helm knows chart semantics; catches template errors grep cannot |

**Key insight:** CNPG's generated secrets are the authoritative source for connection strings. Any chart that hardcodes credentials or constructs DSNs from individual secret fields will break on credential rotation.

## Common Pitfalls

### Pitfall 1: CNPG Secret Race (timing)
**What goes wrong:** App pod starts before CNPG has generated the `{cluster}-app` secret; app crashes with "secret not found" or "connection refused."
**Why it happens:** CNPG generates the secret asynchronously after the Cluster resource is created. Secret creation takes 5-30 seconds; app deployment can start before that.
**How to avoid:** Init container with `pg_isready` (D-01). The init container blocks app container start until Postgres is accepting connections, which also implies the secret exists.
**Warning signs:** Pod stuck in `Init:0/1` for >60 seconds — investigate CNPG operator logs and Cluster CR status.

### Pitfall 2: Wrong Secret Name
**What goes wrong:** `secretKeyRef.name` references `{fullname}-app` instead of `{fullname}-postgres-app`, causing pod startup failure with "secret not found."
**Why it happens:** CNPG secret naming is `{cluster-name}-app` where cluster-name is the Cluster CR metadata.name — NOT the Helm release name directly. If the Cluster is named `{fullname}-postgres`, the secret is `{fullname}-postgres-app`.
**How to avoid:** Confirm Cluster metadata.name matches the `-app` suffix used in secretKeyRef. Use the same `{{ include "clay.fullname" . }}-postgres` expression in both places.
**Warning signs:** Pod in `CreateContainerConfigError` state; `kubectl describe pod` shows "secret not found."

### Pitfall 3: helm lint fails without dependency update
**What goes wrong:** CI `helm lint chart/clay/` fails with error: "found in Chart.yaml, but missing in charts/ directory" after adding the `dependencies:` block.
**Why it happens:** `helm lint` validates subchart presence; charts/ is gitignored.
**How to avoid:** Add `helm repo add cnpg ... && helm dependency update chart/clay/` step before any `helm lint` or `helm template` in CI. [VERIFIED: helm.sh/docs/helm/helm_dependency/]
**Warning signs:** CI fails on lint immediately after Chart.yaml edit; local `helm lint` also fails unless charts/ is populated.

### Pitfall 4: DB_PATH leaks in environment
**What goes wrong:** App receives `DB_PATH=/data/clay.db` from ConfigMap, causing confusion or log noise even though the Go binary ignores it.
**Why it happens:** The configmap uses `range .Values.config` — any key left in `.Values.config` is rendered.
**How to avoid:** Remove `DB_PATH` from `values.yaml` config block (D-12). Removal from values.yaml causes automatic removal from the rendered ConfigMap.
**Warning signs:** `kubectl exec -- env | grep DB_PATH` returns a value in a deployed pod.

### Pitfall 5: CGO step left in CI
**What goes wrong:** Build step in CI installs `gcc` then sets `CGO_ENABLED=0`, wasting time and creating a contradictory signal.
**Why it happens:** The existing `test.yml` has `apt-get install -y gcc` that predates Phase 1 CGO removal.
**How to avoid:** Remove the "Install dependencies" step entirely (D-14). `CGO_ENABLED=0` is already set in the Makefile for `make test` and `make build`.
**Warning signs:** CI log shows "Install dependencies" step that installs gcc.

### Pitfall 6: Cluster name derivation breaks on release name
**What goes wrong:** If the Helm release name already contains "clay", `clay.fullname` returns just the release name (not `release-clay`). The CNPG service and secret names must be derived from the same helper as the Cluster CR name.
**Why it happens:** The `clay.fullname` helper has deduplication logic: if the release name contains the chart name, it returns just the release name.
**How to avoid:** Use `{{ include "clay.fullname" . }}-postgres` consistently in the Cluster CR name, the init container host, and the secretKeyRef name. Never hardcode.
**Warning signs:** `helm template` output shows mismatched names between Cluster metadata.name and secretKeyRef.name.

## Code Examples

### Chart.yaml Dependencies Block
```yaml
# Source: https://helm.sh/docs/chart_best_practices/dependencies/ + cloudnative-pg.io/charts/
apiVersion: v2
name: clay
description: A Helm chart for the Clay pottery shop
type: application
version: 0.1.0
appVersion: "1.0.0"

dependencies:
  - name: cloudnative-pg
    repository: https://cloudnative-pg.github.io/charts
    version: "0.28.0"
    condition: cloudnative-pg.enabled
```

### values.yaml postgres block (new, from D-07)
```yaml
# Full postgres block to add to values.yaml
postgres:
  managed: true
  cluster:
    instances: 1
    storage:
      size: 5Gi
      storageClass: ""
  external:
    dsn: ""

cloudnative-pg:
  enabled: true
```

### Deployment strategy change (from D-05)
```yaml
# Before:
  strategy:
    type: Recreate  # SQLite requires single-writer

# After:
  strategy:
    type: RollingUpdate
```

### Updated test.yml test job (from D-14, CI-01, CI-02)
```yaml
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.26'

      # No longer needed: apt-get install gcc (CGO_ENABLED=0 in Makefile)

      - name: Verify dependencies
        run: go mod verify

      - name: Run vet
        run: go vet ./...

      - name: Run tests
        run: make test

      - name: Build
        run: make build
```

### Extended helm-lint job (from D-17, CI-03)
```yaml
  helm-lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Set up Helm
        uses: azure/setup-helm@v4

      - name: Add CNPG Helm repo
        run: helm repo add cnpg https://cloudnative-pg.github.io/charts

      - name: Update Helm dependencies
        run: helm dependency update chart/clay/

      - name: Lint (managed mode)
        run: helm lint chart/clay/ --values chart/clay/ci/managed-values.yaml

      - name: Lint (external mode)
        run: helm lint chart/clay/ --values chart/clay/ci/external-values.yaml

      - name: Template (managed mode)
        run: helm template clay chart/clay/ --values chart/clay/ci/managed-values.yaml

      - name: Template (external mode)
        run: helm template clay chart/clay/ --values chart/clay/ci/external-values.yaml
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `strategy: Recreate` (SQLite single-writer) | `strategy: RollingUpdate` (Postgres shared) | Phase 2 (this work) | Multiple replicas now possible |
| `DB_PATH` SQLite file path in ConfigMap | `DATABASE_URL` from CNPG secret or explicit env | Phase 2 (this work) | No SQLite refs in chart |
| `apt-get install gcc` in CI (CGO) | No compiler step; `CGO_ENABLED=0` | Phase 1 completed | Faster CI, smaller attack surface |
| Single `helm lint chart/clay/` | Lint + template for both modes | Phase 2 (this work) | Catches mode-specific template errors |

**Deprecated/outdated:**
- `strategy: Recreate` comment "SQLite requires single-writer": remove entirely
- `DB_PATH: "/data/clay.db"` in values.yaml: remove (Pitfall 4 if left)

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Explicit `env` entry takes precedence over `envFrom` for the same key in Kubernetes | Architecture Patterns (Pattern 3) | DATABASE_URL from CNPG secret may not override ConfigMap value — test with `kubectl exec -- env` |
| A2 | `postgres:16-alpine` image includes `pg_isready` binary | Architecture Patterns (Pattern 4) | Init container fails; use `postgres:16-alpine` confirmed to include pg client tools |
| A3 | CNPG 0.28.0 (appVersion 1.29.0) still uses `{cluster}-app` secret naming and `uri` key | Architecture Patterns (Pattern 3) | Secret key mismatch; verify against CNPG 1.29 docs if behavior changed |

## Open Questions

1. **Chart.lock in CI: build vs update**
   - What we know: `helm dependency update` re-resolves; `helm dependency build` uses existing lock.
   - What's unclear: Whether to commit Chart.lock and use `build` in CI, or always use `update`.
   - Recommendation: Use `helm dependency update` for now (no Chart.lock exists yet). After first run, commit Chart.lock and switch CI to `helm dependency build` for reproducibility. The planner should document this two-step approach.

2. **External DSN mode: Secret vs plain value**
   - What we know: D-10 says "inject as plain env value (or via a Kubernetes Secret for the release)."
   - What's unclear: The existing `secret.yaml` uses `range .Values.secrets` — adding `DATABASE_URL` to `.Values.secrets` would inject it via the existing secret. Or it could be a plain `env.value`.
   - Recommendation: Use plain `env.value` from `.Values.postgres.external.dsn` — simpler, consistent with D-10, avoids secrets sprawl. Note that plain env values in Deployment YAML are visible in `kubectl describe` — acceptable for a hobby shop.

3. **golangci-lint in CI**
   - What we know: D-14 says "update existing test job"; CI-02 mentions golangci-lint but the existing workflow only has `go vet`.
   - What's unclear: Whether to add golangci-lint or keep just `go vet`.
   - Recommendation: Add `go vet ./...` as an explicit step (D-14 says to add it). Skip golangci-lint for now — it requires an action install step and the phase scope is focused on Postgres/Helm. The planner should note this decision.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go | CI test job | ✓ (local) | 1.24.13 (local); 1.26 in CI via setup-go | — |
| Helm | helm-lint job | ✗ (local) | — (installed in CI via azure/setup-helm@v4) | CI only |
| Docker | testcontainers-go tests | ✗ (local) | — (available on ubuntu-latest GitHub Actions runner) | Tests run in CI |
| kubectl | CMX integration test | ✗ (local) | — (installed in CMX workflow) | CMX workflow out of scope |

**Missing dependencies with no fallback:**
- None that block CI execution — helm and docker are available in GitHub Actions runners.

**Missing dependencies with fallback:**
- Helm (local): Not available locally. All `helm lint` and `helm template` validation runs in CI. Makefile targets call `helm` directly — will fail locally if helm not installed, but CI is the canonical validation environment.

## Security Domain

The phase is infrastructure wiring (Helm templates, CI workflow YAML). No new application authentication, input validation, or cryptography is introduced.

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | — |
| V3 Session Management | no | — |
| V4 Access Control | no | — |
| V5 Input Validation | no | — |
| V6 Cryptography | no | — |

**Relevant security consideration (not ASVS-gated):** The CNPG-generated secret (`{cluster}-app`) contains the database password. The `secretKeyRef` injection means the password never appears in values.yaml or the Deployment spec — it stays in the Kubernetes Secret. External DSN mode uses a plain `env.value`, which is visible in `kubectl describe pod`. Acceptable for this hobby shop context; noted for awareness.

## Sources

### Primary (HIGH confidence)
- [cloudnative-pg.io/charts/index.yaml](https://cloudnative-pg.io/charts/) — version 0.28.0 confirmed current (released 2026-04-01, appVersion 1.29.0)
- [cloudnative-pg.io/docs/1.27/applications/](https://cloudnative-pg.io/docs/1.27/applications/) — secret naming `{cluster}-app`, key names including `uri`, RW service `{cluster}-rw`
- [cloudnative-pg.io/documentation/1.24/service_management/](https://cloudnative-pg.io/documentation/1.24/service_management/) — three service types: `-rw`, `-ro`, `-r`; RW service naming confirmed
- [helm.sh/docs/helm/helm_dependency_update/](https://helm.sh/docs/helm/helm_dependency_update/) — dependency update required before lint/template with subcharts
- Codebase read: `chart/clay/Chart.yaml`, `values.yaml`, `templates/deployment.yaml`, `templates/configmap.yaml`, `templates/_helpers.tpl`, `.github/workflows/test.yml`, `Makefile`

### Secondary (MEDIUM confidence)
- [cloudnative-pg.io/docs/1.28/cncf-projects/external-secrets/](https://cloudnative-pg.io/docs/1.28/cncf-projects/external-secrets/) — confirms secret field list including `uri`, `jdbc-uri`, `pgpass`
- WebSearch: Docker daemon available on `ubuntu-latest` GitHub Actions runners — no extra setup for testcontainers-go

### Tertiary (LOW confidence)
- None

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — versions verified from live registry; dependencies confirmed from codebase
- Architecture: HIGH — patterns derived from official CNPG docs and existing codebase structure
- Pitfalls: HIGH — derived from verified CNPG behavior (timing, naming) and Helm mechanics

**Research date:** 2026-04-14
**Valid until:** 2026-05-14 (CNPG chart versions update monthly; core behavior stable)
