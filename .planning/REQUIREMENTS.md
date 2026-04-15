# Requirements: Clay.nz Pottery Shop

**Defined:** 2026-04-15
**Core Value:** The Helm chart deploys the full stack — app, database, and certificates — in a single `helm install`, with no operator pre-install required.

## v1.2 Requirements

Requirements for milestone v1.2 (Umbrella Chart). Each maps to roadmap phases.

### Subchart Dependencies (CHART)

- [ ] **CHART-01**: Operator can install CNPG as a subchart dependency controlled by `cloudnative-pg.enabled` toggle
- [ ] **CHART-02**: Operator can install cert-manager as a subchart dependency controlled by `cert-manager.enabled` toggle
- [ ] **CHART-03**: Subchart toggles are validated by values.schema.json (type-checked booleans with defaults)
- [x] **CHART-04**: `helm dependency update` produces Chart.lock and charts/ tarballs correctly

### Webhook Readiness (WBHK)

- [ ] **WBHK-01**: A Job blocks CNPG CR creation until the CNPG webhook endpoint is serving
- [ ] **WBHK-02**: A Job blocks cert-manager CR creation until the cert-manager webhook endpoint is serving
- [ ] **WBHK-03**: Webhook-wait Jobs have RBAC (ServiceAccount, ClusterRole, ClusterRoleBinding) to query CRDs and endpoints
- [ ] **WBHK-04**: Webhook-wait Jobs only render when the corresponding subchart toggle is enabled

### Hook Ordering (HOOK)

- [ ] **HOOK-01**: cnpg-cluster.yaml is a post-install/post-upgrade hook at weight -10, sequenced after webhook-wait Jobs
- [ ] **HOOK-02**: All hook weights are correct: RBAC at -25, webhook-wait Jobs at -20, CNPG Cluster at -10, cert-manager CRs at -10 to 5

### CI Coverage (CI)

- [ ] **CI-01**: CI values file covers "both operators bundled" — full umbrella default
- [ ] **CI-02**: CI values file covers "operators pre-installed" — both `enabled: false`, CRs still rendered
- [ ] **CI-03**: CI values file covers "external DB, no TLS" — both disabled, minimal install
- [ ] **CI-04**: CI values file covers "mixed" — one bundled, one pre-installed
- [ ] **CI-05**: helm-template-test.sh assertions cover all 4 toggle combinations

### Documentation (DOCS)

- [ ] **DOCS-01**: README explains that default install bundles both operators as subcharts
- [ ] **DOCS-02**: README explains `cloudnative-pg.enabled: false` / `cert-manager.enabled: false` for pre-installed operators
- [ ] **DOCS-03**: README notes ~30-60s first-install overhead from webhook-wait Jobs
- [ ] **DOCS-04**: README documents upgrade path for existing installs (must set `enabled: false` to avoid duplicate operators)

## Future Requirements

### Post-v1.2

- **CHART-F01**: Support air-gapped installs — replace bitnami/kubectl webhook-wait image with a pinned in-registry alternative
- **CHART-F02**: Option to disable webhook-wait Jobs entirely for fast-path installs where operators are guaranteed ready
- **CHART-F03**: Helm test (helm test) hook to verify deployed stack is functional end-to-end

## Out of Scope

| Feature | Reason |
|---------|--------|
| Multi-seller / multi-tenant support | Separate future milestone (spec exists in docs/MULTI-SELLER-SPEC.md) |
| Payment gateway | Order-by-email model is intentional; no payment processing planned |
| Mobile app | Web-first; mobile deferred indefinitely |
| Real-time features (websockets, live inventory) | Complexity not justified at current scale |
| Helm test (helm test hook) | Post-v1.2 — webhook-wait Jobs are the v1.2 readiness signal |

## Traceability

Roadmap created 2026-04-15 — all 19 v1.2 requirements mapped to phases 6–10.

| Requirement | Phase | Status |
|-------------|-------|--------|
| CHART-01 | Phase 6 | Pending |
| CHART-02 | Phase 6 | Pending |
| CHART-03 | Phase 6 | Pending |
| CHART-04 | Phase 6 | Complete |
| WBHK-01 | Phase 7 | Pending |
| WBHK-02 | Phase 7 | Pending |
| WBHK-03 | Phase 7 | Pending |
| WBHK-04 | Phase 7 | Pending |
| HOOK-01 | Phase 8 | Pending |
| HOOK-02 | Phase 8 | Pending |
| CI-01 | Phase 9 | Pending |
| CI-02 | Phase 9 | Pending |
| CI-03 | Phase 9 | Pending |
| CI-04 | Phase 9 | Pending |
| CI-05 | Phase 9 | Pending |
| DOCS-01 | Phase 10 | Pending |
| DOCS-02 | Phase 10 | Pending |
| DOCS-03 | Phase 10 | Pending |
| DOCS-04 | Phase 10 | Pending |

**Coverage:**
- v1.2 requirements: 19 total
- Mapped to phases: 19
- Unmapped: 0 ✓

---
*Requirements defined: 2026-04-15*
*Last updated: 2026-04-15 — roadmap created, traceability confirmed*
