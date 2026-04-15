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

- [x] **Phase 6: Subchart Dependencies** — Wire cloudnative-pg and cert-manager as conditional Chart.yaml dependencies with values toggles and schema validation (completed 2026-04-15)
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
**Plans**: 3 plans

Plans:
- [x] 06-01-PLAN.md — Chart.yaml dependencies block + values.yaml toggle keys (CHART-01, CHART-02)
- [x] 06-02-PLAN.md — values.schema.json boolean enforcement + clay.validateDB helper (CHART-03)
- [x] 06-03-PLAN.md — helm dependency update, Chart.lock verification, full lint matrix (CHART-04)

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
| 6. Subchart Dependencies | v1.2 | 3/3 | Complete   | 2026-04-15 |
| 7. Webhook Readiness | v1.2 | 0/? | Not started | - |
| 8. Hook Weight Ordering | v1.2 | 0/? | Not started | - |
| 9. CI Test Matrix | v1.2 | 0/? | Not started | - |
| 10. Documentation | v1.2 | 0/? | Not started | - |
| 11. Seller Accounts and Auth | v1.3 | 0/3 | Not started | - |
| 12. Product Ownership | v1.3 | 0/2 | Planned    |  |
| 13. Gallery and Seller Profiles | v1.3 | 0/2 | Not started | - |
| 14. Firing Logs CRUD | v1.3 | 0/3 | Not started | - |
| 15. Temperature Graphs | v1.3 | 0/2 | Not started | - |
| 16. Admin Seller Oversight | v1.3 | 0/2 | Not started | - |

---

### 🚧 v1.3 Multi-Seller Accounts (Planned)

**Milestone Goal:** Transform the single-seller pottery shop into a multi-seller marketplace — sellers register and manage their own products, record private kiln firing logs with temperature graphs, and the public gallery shows seller attribution and profile pages. The site admin retains oversight of all sellers and products.

## Phase Checklist

- [ ] **Phase 11: Seller Accounts and Auth** — Add sellers table, bcrypt auth, session-based login/register/logout, seller dashboard, and approval flow with admin email notification
- [ ] **Phase 12: Product Ownership** — Add seller_id FK to products, scope seller dashboard to own products, route order emails to seller's order_email, admin sees all products with seller column
- [ ] **Phase 13: Gallery and Seller Profiles** — Add `/seller/{id}` profile pages, seller attribution on product cards in gallery and home, optional gallery filter by seller
- [ ] **Phase 14: Firing Logs CRUD** — Add firing_logs and firing_readings tables, private CRUD routes under /dashboard/firings, batch data entry with table-style form
- [ ] **Phase 15: Temperature Graphs** — JSON API endpoint for readings, Chart.js canvas on firing log detail page, X=elapsed minutes / Y=temperature with gas/flue annotations
- [ ] **Phase 16: Admin Seller Oversight** — Admin UI to list/approve/reject sellers, admin all-products view with seller name column

## Phase Details

### Phase 11: Seller Accounts and Auth
**Goal**: Sellers can register, log in, and access a dashboard; the approval flow gates account activation; the existing ADMIN_USER/ADMIN_PASS bootstrap creates the first admin seller from env vars
**Depends on**: Phase 10 (v1.2 complete) — no structural dependency, can start independently
**Requirements**: SELL-01, SELL-02, SELL-03, SELL-04
**Success Criteria** (what must be TRUE):
  1. `db/migrations/00002_add_sellers.sql` creates the `sellers` table with all required columns and `is_active=false` default
  2. `POST /register` creates a seller with `is_active=false`, generates an `approval_token`, and sends email to ORDER_EMAIL
  3. `POST /login` returns 200 with session cookie for active sellers, 403 with "pending approval" message for inactive sellers
  4. `GET /dashboard` requires a valid session with `SellerID > 0` and `is_active=true`; unauthenticated requests redirect to `/login`
  5. `GET /admin/sellers/approve?token=X` sets `is_active=true` and clears the token
  6. Startup creates an admin seller from ADMIN_USER/ADMIN_PASS if no admin seller exists
**Plans**: 3 plans

Plans:
- [ ] 11-01-PLAN.md — DB migration (sellers table) + SellerStore model (SELL-01)
- [ ] 11-02-PLAN.md — Auth handlers (login/register/logout) + session extension with SellerID (SELL-02, SELL-03)
- [ ] 11-03-PLAN.md — Approval flow (email + token endpoint) + bootstrap from env vars (SELL-04)

### Phase 12: Product Ownership
**Goal**: Every product is owned by a seller — existing products are migrated to the admin seller, sellers see only their own products, and order emails route to the seller's order_email with fallback to ORDER_EMAIL
**Depends on**: Phase 11
**Requirements**: SELL-05, SELL-06
**Success Criteria** (what must be TRUE):
  1. `db/migrations/00003_add_product_ownership.sql` adds `seller_id BIGINT REFERENCES sellers(id)` to products and assigns existing products to the admin seller
  2. Seller dashboard GET /dashboard/products lists only products where `seller_id = session.SellerID`
  3. Admin dashboard GET /admin shows all products with a "Seller" column
  4. Order placement emails use the seller's `order_email` if non-empty, falling back to the global `ORDER_EMAIL` env var
**Plans**: 2 plans

Plans:
- [ ] 12-01-PLAN.md — DB migration + ProductStore update (seller_id scoping, admin all-products) (SELL-05)
- [ ] 12-02-PLAN.md — Scoped dashboard handler + order email routing to seller (SELL-06)

### Phase 13: Gallery and Seller Profiles
**Goal**: The public gallery shows seller names on product cards and each seller has a profile page listing their available work and past sold items
**Depends on**: Phase 12
**Requirements**: SELL-07, SELL-08
**Success Criteria** (what must be TRUE):
  1. `GET /seller/{id}` returns 200 with seller name, bio, and grid of available products; 404 for unknown seller
  2. Product cards on `/` and `/gallery` display the seller's name linked to `/seller/{id}`
  3. Seller profile page includes a "past work" section of sold items
  4. Optional: `GET /gallery?seller={id}` filters gallery to one seller's products
**Plans**: 2 plans

Plans:
- [ ] 13-01-PLAN.md — SellerStore.GetByID + SellerHandler + seller profile template (SELL-07)
- [ ] 13-02-PLAN.md — Gallery/home attribution (product cards show seller name + link) (SELL-08)

### Phase 14: Firing Logs CRUD
**Goal**: Sellers have a private firing log section with structured kiln data entry — each log has batch-editable temperature readings; no public visibility; admin cannot access other sellers' logs
**Depends on**: Phase 11
**Requirements**: SELL-09, SELL-10, SELL-11
**Success Criteria** (what must be TRUE):
  1. `db/migrations/00004_add_firing_logs.sql` creates `firing_logs`, `firing_readings`, and the composite index
  2. All `/dashboard/firings/*` routes require a valid seller session; requests for another seller's log return 403
  3. `POST /dashboard/firings/create` creates a log and redirects to the detail page
  4. `POST /dashboard/firings/{id}/update` saves readings as a batch (insert-or-replace)
  5. `POST /dashboard/firings/{id}/delete` removes the log and all readings via CASCADE
**Plans**: 3 plans

Plans:
- [ ] 14-01-PLAN.md — DB migrations (firing_logs + firing_readings) + FiringLogStore model (SELL-09)
- [ ] 14-02-PLAN.md — Dashboard CRUD handlers for firing logs (list, create, view, edit, update, delete) (SELL-10)
- [ ] 14-03-PLAN.md — Templates: list, new/edit form (table rows + add-row), detail view (SELL-11)

### Phase 15: Temperature Graphs
**Goal**: The firing log detail page shows an interactive Chart.js temperature curve; the JSON API endpoint serves readings data; the JS footprint is under 50 lines with no build step
**Depends on**: Phase 14
**Requirements**: SELL-12, SELL-13
**Success Criteria** (what must be TRUE):
  1. `GET /api/firings/{id}/readings` returns `{"readings":[...]}` JSON for the owning seller; 403 for other sellers; 401 for unauthenticated
  2. The firing log detail page includes a `<canvas>` element and a `<script>` that loads Chart.js and renders the temperature curve
  3. X-axis is elapsed_minutes, Y-axis is temperature °C; gas/flue settings appear as annotations or point labels
  4. The graph renders on mobile (responsive canvas)
**Plans**: 2 plans

Plans:
- [ ] 15-01-PLAN.md — JSON API handler (GET /api/firings/{id}/readings) + auth guard (SELL-12)
- [ ] 15-02-PLAN.md — Chart.js integration: canvas on detail template + vanilla JS chart init (SELL-13)

### Phase 16: Admin Seller Oversight
**Goal**: The admin has a sellers management page with approve/reject buttons and sees all products across all sellers with a seller name column; firing logs remain private (admin cannot view them)
**Depends on**: Phase 12
**Requirements**: SELL-14, SELL-15, SELL-16
**Success Criteria** (what must be TRUE):
  1. `GET /admin/sellers` lists all sellers with name, email, status (active/pending), and approve/reject buttons
  2. `POST /admin/sellers/approve` sets `is_active=true` for the given seller ID
  3. `POST /admin/sellers/reject` deletes or deactivates the seller account
  4. Admin dashboard product list includes a "Seller" column showing each product's seller name
  5. No route under `/admin` reveals firing log data
**Plans**: 2 plans

Plans:
- [ ] 16-01-PLAN.md — Admin seller list/approve/reject handler + template (SELL-14, SELL-15)
- [ ] 16-02-PLAN.md — Admin all-products view with seller name column (SELL-16)
