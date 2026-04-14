# Roadmap: Pottery Shop — Postgres Migration

## Milestones

- ✅ **v1.0 Postgres Migration** — Phases 1–2 (shipped 2026-04-13)
- ✅ **v1.1 TLS** — Phases 3–5 (shipped 2026-04-14)

## Phases

<details>
<summary>✅ v1.0 Postgres Migration (Phases 1–2) — SHIPPED 2026-04-13</summary>

- [x] **Phase 1: Go + Build** — Driver swap, SQL dialect fixes, Goose migrations, CGO removal, and local integration tests (completed 2026-04-13)
- [x] **Phase 2: Helm + CI** — CNPG subchart, Cluster resource, secret injection, timing fix, and CI pipeline (completed 2026-04-13)

</details>

<details>
<summary>✅ v1.1 TLS (Phases 3–5) — SHIPPED 2026-04-14</summary>

- [x] **Phase 3: Values and Ingress Refactor** — Restructure ingress values block, update Ingress template with Traefik annotations, add _helpers.tpl validation and TLS secret name helper (completed 2026-04-14)
- [x] **Phase 3.1: Phase 3 Verification Closure** *(INSERTED)* — Write VERIFICATION.md for Phase 3 with requirements_completed frontmatter; patch SUMMARY.md to satisfy the 3-source certification gate for INGR-01..04 and TLS-03 (completed 2026-04-14)
- [x] **Phase 4: cert-manager CR Templates** — ClusterIssuer and Certificate templates for letsencrypt and selfsigned modes; cert-manager pre-install step in integration-test.yml; add cert-manager.io/cluster-issuer annotation to Ingress for letsencrypt mode (completed 2026-04-14)
- [x] **Phase 5: CI Validation Extension** — Three TLS CI values files and six new lint/template steps in test.yml (completed 2026-04-14)

</details>

## Progress

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 1. Go + Build | v1.0 | 3/3 | Complete | 2026-04-13 |
| 2. Helm + CI | v1.0 | 2/2 | Complete | 2026-04-13 |
| 3. Values and Ingress Refactor | v1.1 | 1/1 | Complete | 2026-04-14 |
| 3.1. Phase 3 Verification Closure | v1.1 | 1/1 | Complete | 2026-04-14 |
| 4. cert-manager CR Templates | v1.1 | 2/2 | Complete | 2026-04-14 |
| 5. CI Validation Extension | v1.1 | 2/2 | Complete | 2026-04-14 |

---
*Full milestone details archived in `.planning/milestones/`*
*Requirements archived in `.planning/milestones/v1.1-REQUIREMENTS.md`*
