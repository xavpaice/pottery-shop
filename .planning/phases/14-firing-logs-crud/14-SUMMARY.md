---
phase: "14-firing-logs-crud"
plans: [1, 2, 3]
subsystem: seller-dashboard
tags: [firing-logs, crud, pgx, goose, templates]
dependency_graph:
  requires:
    - internal/models/seller.go (SellerStore pattern)
    - internal/handlers/auth.go (requireSeller guard)
    - internal/migrations/00003_add_product_ownership.sql
  provides:
    - internal/migrations/00004_add_firing_logs.sql
    - internal/models/firing_log.go (FiringLogStore)
    - internal/handlers/firing_log.go (FiringLogHandler)
    - templates/firings_list.html
    - templates/firings_form.html
    - templates/firings_detail.html
  affects:
    - cmd/server/main.go (new store + handler + routes)
    - internal/handlers/auth.go (RequireSeller exported)
    - templates/dashboard.html (nav updated)
tech_stack:
  added: []
  patterns:
    - pgx/v5 pool.QueryRow/Query/Exec with manual Scan
    - Goose Up/Down migration with BIGSERIAL + REFERENCES + ON DELETE CASCADE
    - DELETE+INSERT in pgx transaction for batch readings replacement
    - readings[N][field] form encoding parsed by iterating until key absent
    - Hidden template row with __IDX__ cloneNode JS pattern for dynamic table rows
    - canvas#firingChart placeholder for Phase 15 Chart.js integration
key_files:
  created:
    - internal/migrations/00004_add_firing_logs.sql
    - internal/models/firing_log.go
    - internal/handlers/firing_log.go
    - templates/firings_list.html
    - templates/firings_form.html
    - templates/firings_detail.html
  modified:
    - internal/handlers/auth.go
    - cmd/server/main.go
    - templates/dashboard.html
decisions:
  - "Exported RequireSeller on AuthHandler (wraps private requireSeller) so FiringLogHandler routes can be guarded at route registration in main.go without each method internally calling the guard"
  - "FiringDate stored as *string (nullable) — DATE scanned via ::text cast to avoid pgx time.Time timezone complications"
  - "GetReadingsForAPI uses a separate ownership COUNT query to avoid recursive call with GetByID"
  - "Route dispatch for /dashboard/firings/ uses suffix matching in a single catch-all handler, matching the existing /dashboard/products/ pattern"
metrics:
  duration: "~4 minutes"
  completed_date: "2026-04-15"
  tasks_completed: 10
  files_created: 6
  files_modified: 3
---

# Phase 14 Plans 1-3: Firing Logs CRUD Summary

**One-liner:** Seller firing log CRUD with ownership-enforced pgx model, seven guarded routes, and batch readings saved via DELETE+INSERT transaction.

## Plans Executed

| Plan | Name | Commit | Status |
|------|------|--------|--------|
| 14-01 | DB migration + FiringLogStore model | 8795887 | Complete |
| 14-02 | FiringLogHandler + route registration | a5a09eb | Complete |
| 14-03 | Firing log templates | 87415a8 | Complete |

## What Was Built

### Plan 1 — Migration and Model

`internal/migrations/00004_add_firing_logs.sql` creates:
- `firing_logs`: BIGSERIAL PK, seller_id FK → sellers(id), title, firing_date (nullable DATE), clay_body, glaze_notes, outcome, notes, timestamps
- `firing_readings`: BIGSERIAL PK, firing_log_id FK → firing_logs(id) ON DELETE CASCADE, elapsed_minutes, temperature NUMERIC(6,1), gas_setting, flue_setting, notes, created_at
- Composite index `idx_firing_readings_log ON firing_readings(firing_log_id, elapsed_minutes)`

`internal/models/firing_log.go` provides:
- `FiringLog` and `FiringReading` structs with PascalCase fields and `db:` tags
- `FiringLogStore` with `NewFiringLogStore(pool *pgxpool.Pool)`
- All CRUD methods enforce seller ownership via `AND seller_id=$N` in WHERE clauses
- `SaveReadings` verifies ownership then replaces all readings in a single pgx transaction
- `GetReadingsForAPI` uses a COUNT ownership check to avoid recursion with GetByID

### Plan 2 — Handler and Routes

`internal/handlers/firing_log.go` provides `FiringLogHandler` with:
- `List`, `New`, `Create`, `View`, `Edit`, `Update`, `Delete` methods
- `parseReadings` helper iterates `readings[N][field]` keys until absent, skips blank elapsed/temperature
- `extractFiringLogID` strips path prefix and suffix segments to parse the numeric ID

`internal/handlers/auth.go` — added `RequireSeller` (exported wrapper around `requireSeller`) to allow route-level guard application in main.go.

`cmd/server/main.go` — instantiates `FiringLogStore` and `FiringLogHandler`, registers 7 routes under `/dashboard/firings` all wrapped with `authHandler.RequireSeller`. The `/dashboard/firings/` catch-all dispatches on path suffix (`/edit`, `/update`, `/delete`, or view).

### Plan 3 — Templates

- `firings_list.html`: table with Title, Date, Outcome, Actions (View/Edit/Delete); empty state; dashboard nav
- `firings_form.html`: log fields + readings table with hidden `#reading-template` row; `addRow()` JS (~12 lines); gas select (low/medium/high); flue select (1/4, 1/2, 3/4, open); existing readings pre-populated by index
- `firings_detail.html`: read-only header fields + readings table; empty state; `<canvas id="firingChart" data-firing-id="{{.Log.ID}}">` placeholder for Phase 15
- `dashboard.html`: nav updated to include My Products and Firing Logs links

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical Functionality] Exported RequireSeller on AuthHandler**
- **Found during:** Plan 2, Task 2
- **Issue:** The plan said to wrap routes with `authHandler.requireSeller` at registration, but `requireSeller` is unexported. The FiringLogHandler has no access to it.
- **Fix:** Added `RequireSeller` as an exported method on `AuthHandler` that delegates to the private `requireSeller`. No behavioral change.
- **Files modified:** `internal/handlers/auth.go`
- **Commit:** a5a09eb

**2. [Rule 1 - Bug] GetReadingsForAPI avoids recursive call via COUNT ownership check**
- **Found during:** Plan 1, Task 3
- **Issue:** `GetByID` calls `GetReadingsForAPI` to populate `Readings`. If `GetReadingsForAPI` also called `GetByID` for ownership, it would recurse infinitely.
- **Fix:** `GetReadingsForAPI` does a lightweight `SELECT COUNT(*) FROM firing_logs WHERE id=$1 AND seller_id=$2` for the ownership check instead of calling `GetByID`.
- **Files modified:** `internal/models/firing_log.go`
- **Commit:** 8795887

## Known Stubs

None — all fields are wired from the database to templates. The `<canvas id="firingChart">` element is an intentional placeholder; Phase 15 will add the Chart.js script.

## Threat Flags

None — no new network endpoints beyond the guarded `/dashboard/firings/*` routes, which all require an active seller session. No new trust boundary crossings.

## Self-Check: PASSED
