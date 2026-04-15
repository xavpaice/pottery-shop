# Roadmap: Clay.nz Pottery Shop

## Milestones

- ✅ **v1.0 Postgres Migration** — Phases 1–3 (shipped 2026-04-13)
- ✅ **v1.1 TLS** — Phases 4–5 (shipped 2026-04-14)
- 🚧 **v1.2 Umbrella Chart** — Phases 6–10 (in progress)

## Phases

<details>
<summary>✅ v1.0 Postgres Migration (Phases 1–3) — SHIPPED 2026-04-13</summary>

Phase 1: Go + Postgres driver migration
Phase 2: CNPG operator Helm integration
Phase 3: CI pipeline

</details>

<details>
<summary>✅ v1.1 TLS (Phases 4–5) — SHIPPED 2026-04-14</summary>

Phase 4: Ingress + cert-manager TLS refactor
Phase 5: CI extension and behavioral test harness

</details>

### 🚧 v1.2 Umbrella Chart (In Progress)

**Milestone Goal:** Make the chart self-sufficient — a single `helm install` bundles both the CNPG and cert-manager operators as optional subcharts, with webhook-readiness Jobs ensuring correct CRD ordering and CI proving all four toggle combinations work.

## Phase Checklist

- [ ] **Phase 6: Subchart Dependencies** — Wire cloudnative-pg and cert-manager as conditional Chart.yaml dependencies with values toggles and schema validation
- [ ] **Phase 7: Webhook Readiness** — Add Jobs and RBAC that block CR creation until each operator's webhook is serving
- [ ] **Phase 8: Hook Weight Ordering** — Convert cnpg-cluster.yaml to a post-install hook and enforce correct weight sequence across all hook resources
- [ ] **Phase 9: CI Test Matrix** — Extend CI with values files and assertions covering all four toggle combinations
- [ ] **Phase 10: Documentation** — Update README with subchart modes, pre-installed operator instructions, overhead warning, and upgrade path

## Phase Details

### Phase 6: Subchart Dependencies
**Goal**: Operators are wired as conditional subchart dependencies — toggling `cloudnative-pg.enabled` or `cert-manager.enabled` in values.yaml causes Helm to install or skip the bundled operator, and `helm dependency update` produces a verified Chart.lock
**Depends on**: Phase 5 (v1.1 complete)
**Requirements**: CHART-01, CHART-02, CHART-03, CHART-04
**Success Criteria** (what must be TRUE):
  1. `chart/clay/Chart.yaml` contains `cloudnative-pg` and `cert-manager` dependency entries, each with a `condition` field pointing to the toggle key
  2. `chart/clay/values.yaml` has top-level `cloudnative-pg.enabled` and `cert-manager.enabled` boolean keys with defaults
  3. `helm dependency update chart/clay` exits zero and produces `Chart.lock` plus tarballs in `charts/`
  4. `helm schema check` (or `helm lint`) rejects a values file that sets either toggle to a non-boolean
**Plans**: TBD

### Phase 7: Webhook Readiness
**Goal**: CNPG and cert-manager CRs can only be created after the corresponding operator webhooks are confirmed serving — Jobs with RBAC enforce the ordering, and the Jobs are skipped when the operator is pre-installed
**Depends on**: Phase 6
**Requirements**: WBHK-01, WBHK-02, WBHK-03, WBHK-04
**Success Criteria** (what must be TRUE):
  1. `helm template` with both operators enabled produces two webhook-wait Job manifests (one for CNPG, one for cert-manager), a ServiceAccount, a ClusterRole, and a ClusterRoleBinding
  2. `helm template` with `cloudnative-pg.enabled: false` produces no CNPG webhook-wait Job; with `cert-manager.enabled: false` produces no cert-manager webhook-wait Job
  3. Each webhook-wait Job carries `helm.sh/hook: post-install,post-upgrade` and a weight annotation placing it at -20
  4. The ClusterRole grants only the minimum verbs needed to query CRDs and endpoints (no wildcard rules)
**Plans**: TBD
**UI hint**: no

### Phase 8: Hook Weight Ordering
**Goal**: Every hook resource has the correct weight annotation, and the CNPG Cluster CR is a post-install/post-upgrade hook that runs after the webhook-wait Job — ensuring the full deployment sequence is RBAC → webhook-wait → CNPG Cluster → cert-manager CRs
**Depends on**: Phase 7
**Requirements**: HOOK-01, HOOK-02
**Success Criteria** (what must be TRUE):
  1. `helm template` shows `cnpg-cluster.yaml` carrying `helm.sh/hook: post-install,post-upgrade` and `helm.sh/hook-weight: "-10"`
  2. `helm template` shows hook weights in correct order: RBAC at -25, webhook-wait Jobs at -20, CNPG Cluster at -10, cert-manager CRs between -10 and 5
  3. A dry-run `helm install` against a real cluster applies resources in weight-ascending order with no CRD-not-found errors for CNPG or cert-manager resources
**Plans**: TBD

### Phase 9: CI Test Matrix
**Goal**: CI verifies all four operator toggle combinations — both bundled, both pre-installed, external DB only, and mixed — so no regression can ship undetected across any deployment mode
**Depends on**: Phase 8
**Requirements**: CI-01, CI-02, CI-03, CI-04, CI-05
**Success Criteria** (what must be TRUE):
  1. `chart/clay/ci/` contains four values files, each clearly named for its toggle combination, and each passes `helm lint` independently
  2. `chart/tests/helm-template-test.sh` contains named assertion groups for all four CI values files, with at least one positive assertion per group confirming subchart resources render (or are absent) correctly
  3. The CI workflow (`helm-lint` job or equivalent) runs all four lint + template + assertion steps and the job passes green in GitHub Actions
**Plans**: TBD

### Phase 10: Documentation
**Goal**: The README fully explains how to use the umbrella chart in every operator mode — a new user can install with bundled operators in one command, and an existing user knows exactly how to upgrade without duplicating operators
**Depends on**: Phase 9
**Requirements**: DOCS-01, DOCS-02, DOCS-03, DOCS-04
**Success Criteria** (what must be TRUE):
  1. README has a section (or table) showing the default `helm install` command that installs with both operators bundled
  2. README explains how to set `cloudnative-pg.enabled: false` and `cert-manager.enabled: false` when operators are pre-installed, with example values snippet
  3. README contains a note that first install with bundled operators takes ~30–60 seconds longer due to webhook-wait Jobs
  4. README has an upgrade path section instructing existing users to set both toggles to `false` when upgrading from a pre-umbrella chart to avoid duplicate operator installs
**Plans**: TBD

## Progress

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 6. Subchart Dependencies | v1.2 | 0/? | Not started | - |
| 7. Webhook Readiness | v1.2 | 0/? | Not started | - |
| 8. Hook Weight Ordering | v1.2 | 0/? | Not started | - |
| 9. CI Test Matrix | v1.2 | 0/? | Not started | - |
| 10. Documentation | v1.2 | 0/? | Not started | - |
