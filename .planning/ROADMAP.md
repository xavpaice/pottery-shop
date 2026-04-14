# Roadmap: Pottery Shop — Postgres Migration

## Overview

A brownfield Go pottery shop migrates from SQLite (CGO) to PostgreSQL (pure Go). Phase 1 completes all code-level changes — driver swap, SQL dialect fixes, schema migration via Goose, CGO removal from the build, and testcontainers-go integration tests — so the app can be validated locally against a real Postgres container before any cluster work begins. Phase 2 wires the Kubernetes delivery layer: CNPG operator as a Helm subchart, the Cluster resource, secret injection, timing mitigation, and CI pipeline jobs.

Milestone v1.1 (TLS) begins at Phase 3. Three phases expose the app over HTTPS via Kubernetes Ingress with cert-manager-managed certificates: Phase 3 restructures the values.yaml ingress block and updates the Ingress template and helpers for all three TLS modes; Phase 4 adds the ClusterIssuer and Certificate templates for Let's Encrypt and self-signed modes plus the cert-manager integration-test pre-install step; Phase 5 creates the three CI values files and extends the test pipeline to lint and template all TLS mode variants.

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3, 4, 5): Planned milestone work
- Decimal phases (1.1, 1.2): Urgent insertions (marked with INSERTED)

- [x] **Phase 1: Go + Build** — Driver swap, SQL dialect fixes, Goose migrations, CGO removal, and local integration tests (completed 2026-04-13)
- [ ] **Phase 2: Helm + CI** — CNPG subchart, Cluster resource, secret injection, timing fix, and CI pipeline
- [ ] **Phase 3: Values and Ingress Refactor** — Restructure ingress values block, update Ingress template with Traefik annotations, add _helpers.tpl validation and TLS secret name helper
- [ ] **Phase 3.1: Phase 3 Verification Closure** *(INSERTED)* — Write VERIFICATION.md for Phase 3 with requirements_completed frontmatter; patch SUMMARY.md to satisfy the 3-source certification gate for INGR-01..04 and TLS-03
- [ ] **Phase 4: cert-manager CR Templates** — ClusterIssuer and Certificate templates for letsencrypt and selfsigned modes; cert-manager pre-install step in integration-test.yml; add cert-manager.io/cluster-issuer annotation to Ingress for letsencrypt mode
- [ ] **Phase 5: CI Validation Extension** — Three TLS CI values files and six new lint/template steps in test.yml

## Phase Details

### Phase 1: Go + Build
**Goal**: The Go application connects to Postgres, all SQL is Postgres-compatible, the binary builds without CGO, and integration tests pass against a real Postgres container — all verifiable without a Kubernetes cluster.
**Depends on**: Nothing (first phase)
**Requirements**: APP-01, APP-02, APP-03, APP-04, APP-05, APP-06, BUILD-01, BUILD-02, TEST-01, TEST-02
**Success Criteria** (what must be TRUE):
  1. `go build ./...` succeeds with `CGO_ENABLED=0` and no reference to go-sqlite3 anywhere in the module
  2. The app starts, runs Goose migrations, and serves requests when `DATABASE_URL` points to a local Postgres instance
  3. All INSERT operations return the correct generated `id` (not zero) and product CRUD works end-to-end
  4. Integration tests pass with `go test ./...` using a testcontainers-go Postgres container — no SQLite, no mocks
  5. Docker `docker build` produces a working image with no CGO dependencies and no cross-compile scaffold
**Plans:** 3/3 plans complete

Plans:
- [x] 01-01-PLAN.md — Driver swap, SQL dialect fixes in product.go, Goose migration file
- [x] 01-02-PLAN.md — main.go pgxpool/Goose wiring, Dockerfile and Makefile CGO removal
- [x] 01-03-PLAN.md — testcontainers-go integration tests, go-sqlite3 removal, end-to-end verification

---

### Phase 2: Helm + CI
**Goal**: The Helm chart supports both managed CNPG and external Postgres, the app pod starts reliably on first `helm install`, and the CI pipeline validates build, tests, and Helm rendering on every push.
**Depends on**: Phase 1
**Requirements**: HELM-01, HELM-02, HELM-03, HELM-04, HELM-05, HELM-06, HELM-07, CI-01, CI-02, CI-03
**Success Criteria** (what must be TRUE):
  1. `helm lint chart/clay` and `helm template chart/clay` both pass with no errors in managed mode and in external-DSN mode
  2. After `helm install` on a cluster with the CNPG operator, the app pod reaches `Running` and its readiness probe passes without manual intervention
  3. `DATABASE_URL` is injected from the CNPG-generated Secret in managed mode and from `postgres.external.dsn` in external mode — no `DB_PATH` reference remains anywhere in the chart
  4. The CI pipeline runs build, lint/test, and Helm validation jobs on every push and reports failures before merge
**Plans:** 2 plans

Plans:
- [x] 02-01-PLAN.md — Helm chart wiring: CNPG subchart dependency, values.yaml postgres block, Cluster CRD template, deployment.yaml updates (strategy, init container, DATABASE_URL injection), DB_PATH removal
- [x] 02-02-PLAN.md — CI pipeline: remove gcc from test job, add go vet, extend helm-lint job with dependency resolution and dual-mode lint+template validation

---

### Phase 3: Values and Ingress Refactor
**Goal**: The chart's ingress values block uses a single-host, mode-driven shape; the Ingress template renders correctly for all three TLS modes; and the helpers layer validates required values at render time.
**Depends on**: Phase 2
**Requirements**: INGR-01, INGR-02, INGR-03, INGR-04, TLS-03
**Success Criteria** (what must be TRUE):
  1. `helm template chart/clay --set ingress.enabled=true --set ingress.host=shop.example.com --set ingress.tls.mode=custom --set ingress.tls.secretName=my-tls` renders an Ingress resource with `ingressClassName: traefik`, the Traefik websecure annotation, and a TLS block pointing at `my-tls` — no ClusterIssuer or Certificate rendered
  2. `helm template chart/clay --set ingress.enabled=true` fails at render time with a clear error message about missing `ingress.host` (clay.validateIngress fires)
  3. `helm template chart/clay --set ingress.enabled=true --set ingress.tls.mode=letsencrypt --set ingress.host=shop.example.com` fails at render time with a clear error about missing `ingress.tls.acme.email`
  4. The nginx proxy-body-size annotation is absent from the default Ingress output; no nginx-specific annotation keys appear in the default values
  5. `helm lint chart/clay` passes cleanly with the refactored values.yaml
**Plans:** 1 plan

Plans:
- [x] 03-01-PLAN.md — Values.yaml ingress block replacement, clay.validateIngress + clay.tlsSecretName helpers, ingress.yaml template rewrite

---

### Phase 3.1: Phase 3 Verification Closure *(INSERTED)*
**Goal**: The formal verification artifacts for Phase 3 exist and are correctly populated — VERIFICATION.md confirms INGR-01..04 and TLS-03 are satisfied, and SUMMARY.md carries the requirements_completed frontmatter field needed by the 3-source certification gate.
**Depends on**: Phase 3 (implementation already complete)
**Requirements**: INGR-01, INGR-02, INGR-03, INGR-04, TLS-03
**Gap Closure**: Closes INGR-01..04 and TLS-03 requirement gaps from v1.1 audit
**Success Criteria** (what must be TRUE):
  1. `phases/03-values-and-ingress-refactor/VERIFICATION.md` exists with `requirements_completed: [INGR-01, INGR-02, INGR-03, INGR-04, TLS-03]` and `status: verified`
  2. `03-01-SUMMARY.md` frontmatter contains `requirements_completed` field listing the same five requirement IDs
  3. The 3-source cross-reference for INGR-01..04 and TLS-03 shows all three sources satisfied (VERIFICATION.md ✓, SUMMARY frontmatter ✓, REQUIREMENTS.md checkbox ✓)
**Plans:** 1 plan

Plans:
- [x] 03.1-01-PLAN.md — Create VERIFICATION.md, patch SUMMARY.md frontmatter, check REQUIREMENTS.md checkboxes

---

### Phase 4: cert-manager CR Templates
**Goal**: The chart renders ClusterIssuer and Certificate resources for letsencrypt and selfsigned modes, all cert-manager resources use post-install hook annotations to avoid webhook timing races, and the integration test workflow installs cert-manager before the clay chart.
**Depends on**: Phase 3
**Requirements**: TLS-01, TLS-02, CI-06
**Success Criteria** (what must be TRUE):
  1. `helm template chart/clay --set ingress.enabled=true --set ingress.host=shop.example.com --set ingress.tls.mode=letsencrypt --set ingress.tls.acme.email=admin@example.com` renders a ClusterIssuer (ACME HTTP-01, staging endpoint) and a Certificate CR referencing it — both carry `helm.sh/hook: post-install,post-upgrade` annotations — and the Ingress resource carries a `cert-manager.io/cluster-issuer` annotation referencing the letsencrypt ClusterIssuer
  2. `helm template chart/clay --set ingress.enabled=true --set ingress.host=shop.example.com --set ingress.tls.mode=selfsigned` renders a SelfSigned ClusterIssuer and a Certificate CR referencing a separate CA ClusterIssuer (two-step CA bootstrap) — no ACME resources present
  3. `helm template chart/clay --set ingress.enabled=true --set ingress.host=shop.example.com --set ingress.tls.mode=custom --set ingress.tls.secretName=my-tls` renders zero ClusterIssuer and zero Certificate resources
  4. The TLS secret name referenced in the Ingress `tls.secretName` field and in the Certificate `spec.secretName` field are identical — both derived from the same `clay.tlsSecretName` helper
  5. `integration-test.yml` installs cert-manager v1.20.2 via `helm install cert-manager jetstack/cert-manager --set crds.enabled=true` before the clay chart install step
**Plans**: TBD
**UI hint**: no

---

### Phase 5: CI Validation Extension
**Goal**: The CI pipeline validates all three TLS modes on every push using dedicated values files, and helm lint plus helm template pass for each mode without requiring cert-manager CRDs to be present.
**Depends on**: Phase 4
**Requirements**: CI-04, CI-05
**Success Criteria** (what must be TRUE):
  1. Three files exist under `chart/clay/ci/` — `tls-letsencrypt-values.yaml`, `tls-selfsigned-values.yaml`, `tls-custom-values.yaml` — each with the correct ingress and TLS values for its mode
  2. `helm lint chart/clay --values chart/clay/ci/tls-letsencrypt-values.yaml` passes (no errors, no cert-manager CRDs needed)
  3. `helm lint chart/clay --values chart/clay/ci/tls-selfsigned-values.yaml` passes
  4. `helm lint chart/clay --values chart/clay/ci/tls-custom-values.yaml` passes
  5. `test.yml` contains six new steps (lint + template for each of the three TLS modes) and all six pass on a push without cluster access
  6. `chart/tests/helm-template-test.sh` is invoked in `test.yml` — all 10 behavioral tests covering INGR-01..04 and TLS-03 pass in CI on every push
**Plans**: TBD
**UI hint**: no

---

## Progress

**Execution Order:**
Phases execute in numeric order: 1 → 2 → 3 → 3.1 → 4 → 5

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Go + Build | 3/3 | Complete    | 2026-04-13 |
| 2. Helm + CI | 0/2 | Not started | - |
| 3. Values and Ingress Refactor | 0/1 | Not started | - |
| 3.1. Phase 3 Verification Closure (INSERTED) | 0/1 | Not started | - |
| 4. cert-manager CR Templates | 0/0 | Not started | - |
| 5. CI Validation Extension | 0/0 | Not started | - |
