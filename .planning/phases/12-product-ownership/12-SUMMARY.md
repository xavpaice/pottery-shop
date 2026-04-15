---
phase: "12-product-ownership"
plan: "12-01 + 12-02"
subsystem: "product-ownership"
tags: ["product", "seller", "ownership", "migration", "dashboard"]
dependency_graph:
  requires:
    - "11-seller-accounts-and-auth"
  provides:
    - "products.seller_id FK column and data migration"
    - "ProductStore.ListBySeller / ListAllWithSeller"
    - "seller-scoped dashboard product CRUD"
    - "order email routing to seller.order_email"
  affects:
    - "internal/models/product.go"
    - "internal/handlers/auth.go"
    - "internal/handlers/admin.go"
    - "internal/handlers/public.go"
    - "cmd/server/main.go"
tech_stack:
  added: []
  patterns:
    - "package-level thumbnail helper shared across handlers"
    - "403 ownership enforcement before mutations"
    - "email routing: seller.order_email > global ORDER_EMAIL fallback"
key_files:
  created:
    - "internal/migrations/00003_add_product_ownership.sql"
    - "templates/dashboard_products.html"
    - "templates/dashboard_product_form.html"
  modified:
    - "internal/models/product.go"
    - "internal/handlers/admin.go"
    - "internal/handlers/auth.go"
    - "internal/handlers/public.go"
    - "cmd/server/main.go"
    - "templates/admin/dashboard.html"
    - "internal/models/product_test.go"
    - "internal/handlers/public_test.go"
decisions:
  - "admin-created products use sellerID=0 (TODO: wire admin seller identity once admin uses seller session)"
  - "email routing uses first cart item's seller; multi-seller carts are not yet addressed"
  - "generateThumbnail extracted to package-level function shared by Admin and Auth handlers"
  - "AuthHandler receives uploadDir/thumbDir to support seller image uploads"
metrics:
  duration: "~35 minutes"
  completed: "2026-04-15"
  tasks_completed: 5
  files_created: 3
  files_modified: 8
---

# Phase 12: Product Ownership Summary

**One-liner:** seller_id FK on products with scoped seller dashboard, 403 mutation guards, and order email routing to seller.order_email.

## What Was Implemented

### Plan 12-01 — DB migration + model update

**Migration** (`internal/migrations/00003_add_product_ownership.sql`):
- `ALTER TABLE products ADD COLUMN seller_id BIGINT REFERENCES sellers(id)`
- Data migration: assigns existing products to the first admin seller

**Product struct** (`internal/models/product.go`):
- `SellerID int64` and `SellerName string` (JOIN-populated) added to `Product`
- `Create(p *Product, sellerID int64)` — signature updated to include seller ownership
- `ListBySeller(ctx, sellerID)` — scoped query with LEFT JOIN sellers
- `ListAllWithSeller(ctx)` — full list with seller name for admin dashboard

**Admin.go fix:** `CreateProduct` passes `sellerID=0` with a TODO comment (admin Basic Auth has no seller session; to be addressed in a future phase).

### Plan 12-02 — Handler wiring

**PublicHandler** (`internal/handlers/public.go`):
- Added `Sellers *models.SellerStore` field
- `PlaceOrder` now fetches the first cart item's seller and routes the order email to `seller.OrderEmail` if set, falling back to global `Config.OrderEmail`
- `sendEmail` renamed to `sendEmailTo(to, subject, body)` for explicit recipient

**AdminHandler** (`internal/handlers/admin.go`):
- Added `Sellers *models.SellerStore` field (wired, available for future admin seller views)
- `Dashboard()` updated to call `ListAllWithSeller(ctx)` instead of `ListAll()`
- `generateThumbnail` method now delegates to package-level `generateThumbnail()`

**AuthHandler** (`internal/handlers/auth.go`):
- Added `products *models.ProductStore`, `uploadDir`, `thumbDir` fields
- `NewAuthHandler` signature extended accordingly
- Added dashboard product handlers: `DashboardProducts`, `DashboardNewProduct`, `DashboardCreateProduct`, `DashboardEditProduct`, `DashboardUpdateProduct`, `DashboardDeleteProduct`, `DashboardToggleSold`
- All mutations enforce `product.SellerID == session.SellerID` (403 otherwise)
- `handleImageUploads` method added (mirrors admin logic using seller's upload dirs)
- Package-level `generateThumbnail()` added (shared with admin)

**Templates:**
- `templates/admin/dashboard.html`: Seller column added to products table
- `templates/dashboard_products.html`: new seller product list page
- `templates/dashboard_product_form.html`: new seller product create/edit form

**Routes registered** in `cmd/server/main.go`:
- `GET /dashboard/products` → `DashboardProducts`
- `GET /dashboard/products/new` → `DashboardNewProduct`
- `POST /dashboard/products/create` → `DashboardCreateProduct`
- `POST /dashboard/products/update` → `DashboardUpdateProduct`
- `POST /dashboard/products/delete` → `DashboardDeleteProduct`
- `POST /dashboard/products/toggle-sold` → `DashboardToggleSold`
- `GET /dashboard/products/{id}/edit` → `DashboardEditProduct`

**Tests fixed:** `product_test.go` and `public_test.go` updated for the new `Create(p, sellerID)` signature.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing critical functionality] Worktree lacked phase 11 code**
- **Found during:** Pre-execution check
- **Issue:** Worktree branch `worktree-agent-affc0974` branched from `ea3cb1e` before phase 11 commits; `internal/models/seller.go`, `internal/handlers/auth.go`, and seller migrations were absent
- **Fix:** Merged `umbrella` branch into worktree (fast-forward); all phase 11 work incorporated
- **Commit:** merge (fast-forward, no separate commit)

**2. [Rule 1 - Bug] Test files used old Create(p) signature**
- **Found during:** Task 12-02 (`go vet`)
- **Issue:** `product_test.go:91` and `public_test.go:127` called `store.Create(p)` without `sellerID`
- **Fix:** Updated both to `store.Create(p, 0)`
- **Files modified:** `internal/models/product_test.go`, `internal/handlers/public_test.go`

**3. [Rule 1 - Bug] `imaging` import became unused in admin.go**
- **Found during:** Build after extracting `generateThumbnail` to package level
- **Fix:** Removed `github.com/disintegration/imaging` import from `admin.go`

**4. [Deviation] `NewAuthHandler` signature extended beyond plan**
- **Reason:** Plan called for adding `products *models.ProductStore` to `AuthHandler`. The dashboard product handlers also need `uploadDir` and `thumbDir` to save images. Rather than a separate global or handler field populated later, these were added to `NewAuthHandler` at the same time.
- **Impact:** `main.go` wiring updated to pass `uploadDir` and `thumbDir`

**5. [Deviation] `sendEmail` renamed to `sendEmailTo`**
- **Reason:** The plan described routing email by fetching the seller, but the existing `sendEmail` always sent to `h.Config.OrderEmail`. Renaming to `sendEmailTo(to, subject, body)` makes the recipient explicit and eliminates duplication.

**6. [Deviation] Email routing uses first cart item only**
- **Reason:** The plan says "fetch its seller" (singular product), but a cart can contain items from multiple sellers. The implementation routes to the first item's seller. Multi-seller order splitting is a future enhancement.

## Known Stubs

- Admin-created products use `sellerID=0` (no seller association). The admin Basic Auth session carries no `SellerID`. A future plan should either: (a) detect admin seller by `is_admin=true` at create time, or (b) migrate admin to use the seller session flow. This is tracked with a `TODO` comment in `admin.go:CreateProduct`.

## Threat Flags

None — no new network endpoints or auth paths beyond those planned. The `/dashboard/products/*` routes are all guarded by `requireSeller` (cookie session check + active seller validation).

## Self-Check: PASSED

All key files confirmed present. Both task commits verified:
- `816b56e` — feat(12-01): seller_id on products
- `52126b0` — feat(12-02): product ownership wired into handlers
