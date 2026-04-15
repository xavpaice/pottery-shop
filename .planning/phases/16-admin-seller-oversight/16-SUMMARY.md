---
phase: "16-admin-seller-oversight"
plans: [1, 2]
subsystem: admin
tags: [admin, sellers, oversight, routing]
dependency_graph:
  requires: [11-01-PLAN.md, 11-02-PLAN.md, 12-01-PLAN.md]
  provides: [admin seller list UI, approve/reject actions, Sellers nav]
  affects: [internal/handlers/admin.go, cmd/server/main.go, templates/admin/]
tech_stack:
  added: []
  patterns: [POST-redirect-GET for admin mutations, method-specific routing to avoid token/admin collision]
key_files:
  created:
    - templates/admin/sellers.html
  modified:
    - internal/handlers/admin.go
    - cmd/server/main.go
    - templates/partials/admin_header.html
decisions:
  - "Changed GET /admin/sellers/approve registration to method-specific (GET only) so POST /admin/sellers/approve on adminMux resolves correctly without conflicting with token-based email approval"
  - "Firing log routes already wired in prior phase commit (a5a09eb); no change needed"
metrics:
  duration: "3 minutes"
  completed: "2026-04-15"
  tasks_completed: 4
  files_modified: 4
---

# Phase 16: Admin Seller Oversight Summary

**One-liner:** Admin seller list page with approve/deactivate actions, Sellers nav link, and confirmed no firing log leakage under /admin.

## Plans Executed

| Plan | Name | Commit | Status |
|------|------|--------|--------|
| 16-01 | Admin seller list, approve, and reject | d1438df | Complete |
| 16-02 | Admin seller column verified, no firing log leakage | ddcc18c | Complete |

## Tasks Completed

### Plan 16-01

**Task 1: Admin seller handler methods**
Added `SellerList`, `ApproveSeller`, and `RejectSeller` to `AdminHandler`. The `Sellers *models.SellerStore` field was already present from Phase 12 — not re-added.

**Task 2: Register admin seller routes**
Added `GET /admin/sellers`, `POST /admin/sellers/approve`, `POST /admin/sellers/reject` to `adminMux` (behind Basic Auth). Changed the email token route from unqualified `/admin/sellers/approve` to `GET /admin/sellers/approve` on the main mux to prevent it from intercepting the admin POST.

**Task 3: Admin sellers template**
Created `templates/admin/sellers.html` with Name/Email/Status/Registered/Actions table. Status renders "Active" (available badge) or "Pending Approval" (pending badge). Pending sellers get Approve + Reject buttons; active sellers get a Deactivate button. Empty state shows "No sellers registered yet."

### Plan 16-02

**Task 1: Verify admin dashboard Seller column**
Confirmed `Dashboard()` already calls `ListAllWithSeller` and `dashboard.html` already renders the Seller column (both done in Phase 12). Added "Sellers" nav link to `templates/partials/admin_header.html`.

**Task 2: Confirm no firing log routes in admin**
Scanned `cmd/server/main.go` — no `FiringLog` references anywhere under `/admin/*`. Firing log routes are strictly under `/dashboard/firings/*` (seller-owned). Clean.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Token route method collision**
- **Found during:** Task 2 (16-01)
- **Issue:** `mux.HandleFunc("/admin/sellers/approve", authHandler.ApproveSellerByToken)` registered without method qualifier would catch all methods, preventing the admin `POST /admin/sellers/approve` on `adminMux` from ever being reached.
- **Fix:** Changed to `mux.HandleFunc("GET /admin/sellers/approve", ...)` — token approval is always GET (email link), admin approval is always POST.
- **Files modified:** `cmd/server/main.go`
- **Commit:** d1438df

**2. [Rule 3 - Blocking] Unused variable build error**
- **Found during:** Task 1 (16-02) — build verification
- **Issue:** A prior-phase linter had injected `firingLogStore` and `firingLogHandler` declarations into `main.go` but had not yet wired the routes, causing `declared and not used` compile error.
- **Fix:** A separate prior-phase commit (a5a09eb, `feat(14-02): FiringLogHandler CRUD routes`) had already wired those routes. After re-reading `main.go` the file was current and compiled cleanly — no additional change needed.
- **Files modified:** None (already resolved by prior commit)

## Known Stubs

None. All rendered data is sourced from `SellerStore.ListAll()`.

## Threat Flags

None. New routes are behind existing Basic Auth middleware. No new attack surface introduced.

## Self-Check

Files created/modified:
- `internal/handlers/admin.go` — SellerList, ApproveSeller, RejectSeller added
- `templates/admin/sellers.html` — new template
- `cmd/server/main.go` — token route made GET-only, three admin seller routes added
- `templates/partials/admin_header.html` — Sellers nav link added

Commits:
- d1438df — feat(16-01): admin seller list, approve, and reject
- ddcc18c — feat(16-02): admin seller column verified, no firing log leakage confirmed

## Self-Check: PASSED
