---
phase: "11-seller-accounts-and-auth"
plans: [1, 2, 3]
subsystem: "auth"
tags: ["sellers", "auth", "bcrypt", "sessions", "approval-flow"]
dependency_graph:
  requires: ["postgres via pgxpool (phase 01)", "goose migrations (phase 01)"]
  provides: ["sellers table", "SellerStore", "session SellerID", "login/register/logout", "approval flow", "admin bootstrap"]
  affects: ["cmd/server/main.go", "internal/middleware/session.go"]
tech_stack:
  added: ["golang.org/x/crypto/bcrypt (direct)", "crypto/rand for tokens"]
  patterns: ["pgxpool direct queries (no database/sql wrapper)", "HMAC-signed cookie session with SellerID", "token-authenticated approval URL"]
key_files:
  created:
    - internal/migrations/00002_add_sellers.sql
    - internal/models/seller.go
    - internal/handlers/auth.go
    - templates/login.html
    - templates/register.html
    - templates/dashboard.html
  modified:
    - internal/middleware/session.go
    - cmd/server/main.go
    - go.mod
decisions:
  - "SellerStore uses pgxpool.Pool directly (not database/sql) — consistent with test infrastructure already using pgxpool"
  - "ApproveSellerByToken placed on AuthHandler to avoid a new file; comment notes token expiry as future enhancement"
  - "sendApprovalEmail included in auth.go (Plan 2) rather than waiting for Plan 3, since AuthHandler.Register already needed it"
  - "Admin bootstrap uses ADMIN_USER/ADMIN_PASS env vars already present in main.go; CreateAdmin sets name=email for simplicity"
  - "Approval route /admin/sellers/approve registered directly on main mux before /admin/ prefix — Go ServeMux longer-path precedence ensures it bypasses Basic Auth"
metrics:
  duration: "~45 minutes"
  completed: "2026-04-15"
  tasks_completed: 9
  files_created: 6
  files_modified: 3
---

# Phase 11 — Seller Accounts and Auth Summary

**One-liner:** Cookie-session seller auth with bcrypt passwords, token-based email approval flow, and env-var admin bootstrap using pgxpool and Goose migration.

## What Was Implemented

### Plan 1 — DB Migration + SellerStore (commit a271c17)
- `internal/migrations/00002_add_sellers.sql`: Goose up/down migration adding the `sellers` table with all required columns (`id`, `email`, `password_hash`, `name`, `bio`, `order_email`, `is_active`, `is_admin`, `approval_token`, `created_at`, `updated_at`)
- `internal/models/seller.go`: `Seller` struct and `SellerStore` with 10 methods:
  - `Create` — bcrypt cost 12, 32-byte crypto/rand hex approval token
  - `CreateAdmin` — sets is_active=true, is_admin=true (for bootstrap)
  - `GetByEmail`, `GetByID`, `GetByApprovalToken` — return nil, nil on not-found
  - `Approve` — sets is_active=true, clears approval_token
  - `CheckPassword` — bcrypt.CompareHashAndPassword
  - `UpdateProfile`, `ListAll`, `SetActive`, `AdminExists`

### Plan 2 — Auth Handlers + Session + Templates (commit 595a549)
- `internal/middleware/session.go`: Added `SellerID int64 \`json:"seller_id,omitempty"\`` to `SessionData`; zero = anonymous/buyer; existing sessions deserialize to SellerID=0 with no migration needed
- `internal/handlers/auth.go`: `AuthHandler` struct with:
  - `ShowLogin` / `Login` — pending sellers get "awaiting approval" message, no SellerID set
  - `ShowRegister` / `Register` — creates seller (is_active=false), calls sendApprovalEmail, redirects to /login with flash
  - `Logout` — clears SellerID, redirects to /
  - `Dashboard` — guarded by requireSeller middleware
  - `requireSeller` — redirects to /login if SellerID==0 or seller not active
  - `sendApprovalEmail` — SMTP failure is logged, does not block registration
  - `ApproveSellerByToken` — GET handler for token-based approval (also in Plan 3 scope, implemented here)
- Routes registered in `cmd/server/main.go`: GET+POST /login, GET+POST /register, POST /logout, GET /dashboard
- Templates: `login.html`, `register.html`, `dashboard.html` — all extend the existing header/footer partials

### Plan 3 — Approval Flow + Bootstrap (commit c538848)
- `GET /admin/sellers/approve?token=X` registered on main mux directly (not behind Basic Auth); token IS the auth credential
- Admin seller bootstrap: after migrations, calls `AdminExists`; if false and `ADMIN_USER`/`ADMIN_PASS` are set, calls `CreateAdmin`
- `golang.org/x/crypto` promoted to direct dependency in go.mod

## Deviations from Plan

**1. [Path correction] Migration in internal/migrations/ not db/migrations/**
- **Found during:** Pre-execution review
- **Issue:** Plans referenced `db/migrations/` but actual location is `internal/migrations/`
- **Fix:** Created `internal/migrations/00002_add_sellers.sql` (correct location)
- **Files modified:** N/A — correct path used from the start

**2. [Early implementation] ApproveSellerByToken and sendApprovalEmail in Plan 2**
- **Found during:** Task 2 (auth.go creation)
- **Issue:** AuthHandler.Register needed sendApprovalEmail immediately, and ApproveSellerByToken was a natural fit on the same struct
- **Fix:** Both implemented in auth.go during Plan 2; Plan 3 only needed route registration and bootstrap

**3. [go mod tidy not runnable] Manual go.mod edit instead of go mod tidy**
- **Found during:** Plan 3 execution
- **Issue:** `go mod tidy` requires Go 1.26 (matching go.mod), but environment has go 1.24.13
- **Fix:** Manually moved `golang.org/x/crypto` from indirect to direct require block in go.mod
- **Note:** The `go test ./...` verification is also blocked by this same toolchain mismatch — this is a pre-existing environment constraint, not a regression

## Known Stubs

None — all data flows are wired. The dashboard shows real seller data from the database.

## Threat Flags

| Flag | File | Description |
|------|------|-------------|
| threat_flag: auth-endpoint | internal/handlers/auth.go | New unauthenticated POST /register endpoint; no rate limiting |
| threat_flag: token-auth | internal/handlers/auth.go | Approval token has no expiry; comment notes this as future enhancement |

## Self-Check: PASSED

Files created:
- internal/migrations/00002_add_sellers.sql — FOUND
- internal/models/seller.go — FOUND
- internal/handlers/auth.go — FOUND
- templates/login.html — FOUND
- templates/register.html — FOUND
- templates/dashboard.html — FOUND

Commits:
- a271c17 feat(11-01) — FOUND
- 595a549 feat(11-02) — FOUND
- c538848 feat(11-03) — FOUND
