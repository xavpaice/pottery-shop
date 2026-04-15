---
phase: "13-gallery-and-seller-profiles"
plan: "13-01 + 13-02"
subsystem: "public-seller-profiles"
tags: ["seller", "gallery", "profile", "attribution", "public"]
dependency_graph:
  requires:
    - "12-product-ownership"
    - "11-seller-accounts-and-auth"
  provides:
    - "GET /seller/{id} public seller profile page"
    - "Seller attribution on home, gallery, and product detail pages"
    - "Optional gallery seller filter: GET /gallery?seller={id}"
    - "ProductStore.ListAvailableWithSeller / ListSoldWithSeller / GetByIDWithSeller"
  affects:
    - "internal/handlers/public.go"
    - "internal/models/product.go"
    - "templates/seller_profile.html"
    - "templates/home.html"
    - "templates/gallery.html"
    - "templates/product.html"
    - "cmd/server/main.go"
tech_stack:
  added: []
  patterns:
    - "r.PathValue for Go 1.22+ pattern-routing path params"
    - "LEFT JOIN sellers in product queries to populate SellerName without N+1"
    - "{{if .SellerName}} guard on all seller attribution links"
key_files:
  created:
    - "templates/seller_profile.html"
  modified:
    - "internal/handlers/public.go"
    - "internal/models/product.go"
    - "cmd/server/main.go"
    - "templates/home.html"
    - "templates/gallery.html"
    - "templates/product.html"
decisions:
  - "Added ListAvailableWithSeller and ListSoldWithSeller (separate from ListAllWithSeller) to keep Home and Gallery semantics correct without post-filter slicing"
  - "GetByIDWithSeller uses QueryRowContext JOIN instead of two separate queries"
  - "Gallery ?seller filter uses ListBySeller then filters to is_sold=true (avoids a new method)"
  - "Worktree branched from main pre-phase-12; merged umbrella to incorporate phases 11-12 before starting"
metrics:
  duration: "~20 minutes"
  completed: "2026-04-15"
  tasks_completed: 4
  files_created: 1
  files_modified: 6
---

# Phase 13: Gallery and Seller Profiles Summary

**One-liner:** Public seller profile page at `/seller/{id}` with available/past-work grids, plus seller attribution links on home, gallery, and product detail pages via LEFT JOIN queries.

## What Was Implemented

### Plan 13-01 — Seller profile page

**SellerProfile handler** (`internal/handlers/public.go`):
- Parses `{id}` with `r.PathValue("id")` (Go 1.22+ stdlib routing)
- Fetches seller via `h.Sellers.GetByID(ctx, id)` — returns 404 if nil or error
- Fetches all seller products via `h.Store.ListBySeller(ctx, id)` — splits into `Available` (IsSold=false) and `PastWork` (IsSold=true)
- Renders `seller_profile.html`

**Route registered** (`cmd/server/main.go`):
- `GET /seller/{id}` → `publicHandler.SellerProfile`

**Template** (`templates/seller_profile.html`):
- Hero section: seller name (h1) and bio (guarded with `{{if .Seller.Bio}}`)
- "Available Work" section: product grid using same card markup as gallery.html; shows empty-state message if no available products
- "Past Work" section: sold product grid with sold badge; entire section hidden if PastWork is empty
- Product cards link to `/product/{id}` as normal

### Plan 13-02 — Seller attribution

**New ProductStore methods** (`internal/models/product.go`):
- `ListAvailableWithSeller(ctx)` — unsold products with LEFT JOIN sellers
- `ListSoldWithSeller(ctx)` — sold products with LEFT JOIN sellers
- `GetByIDWithSeller(ctx, id)` — single product with LEFT JOIN sellers

**Handler updates** (`internal/handlers/public.go`):
- `Home()` now calls `ListAvailableWithSeller` so `SellerName`/`SellerID` are populated
- `Gallery()` now calls `ListSoldWithSeller`; if `?seller={id}` is a valid int64, calls `ListBySeller` then filters to sold items only
- `ProductDetail()` now calls `GetByIDWithSeller` for seller attribution

**Template updates:**
- `templates/home.html`: adds `{{if .SellerName}}<p class="seller-name">by <a href="/seller/{{.SellerID}}">{{.SellerName}}</a></p>{{end}}` inside product card
- `templates/gallery.html`: same attribution pattern
- `templates/product.html`: `{{if .SellerName}}<p class="seller-attribution">by <a href="/seller/{{.SellerID}}">{{.SellerName}}</a></p>{{end}}` below product title

## Deviations from Plan

### Pre-execution fix

**[Rule 3 - Blocker] Worktree branched from main, missing phases 11 and 12 code**
- **Found during:** Pre-execution read of internal/models/ (no seller.go present, no seller fields on Product)
- **Issue:** This worktree (`worktree-agent-af53a6be`) branched from `main` at commit `ea3cb1e`, before phases 11 and 12 were merged into `umbrella`. The plan depends on `SellerStore`, `Product.SellerID/SellerName`, `ListBySeller`, and `PublicHandler.Sellers` — none of which existed in this worktree.
- **Fix:** Ran `git merge umbrella` (fast-forward merge incorporating all umbrella phase work including 06, 07, 11, 12, and planning artifacts)
- **Impact:** No code conflicts; clean fast-forward

### Plan 13-02 implementation approach

**[Deviation] Added ListAvailableWithSeller and ListSoldWithSeller instead of reusing ListAllWithSeller**
- **Reason:** `ListAllWithSeller` returns all products regardless of sold status, but `Home()` needs only available and `Gallery()` needs only sold. Post-filtering in the handler would work but is wasteful. Separate query methods are cleaner and match the existing `ListAvailable`/`ListSold` pattern.
- **Impact:** Two additional methods in product.go (well-scoped, follow existing patterns)

**[Deviation] Gallery seller filter filters ListBySeller results to sold items**
- **Reason:** `ListBySeller` returns all products (available + sold) for a seller. When filtering gallery by seller, only sold items should appear (gallery = sold items). Rather than add yet another method (`ListSoldBySeller`), the handler filters in-memory after the query — the result set per seller is small.

## Known Stubs

None — all seller attribution data flows from real database JOINs. No hardcoded placeholders.

## Threat Flags

None — `GET /seller/{id}` is a fully public read-only endpoint. No new auth paths or trust boundary crossings introduced.

## Self-Check: PASSED

- `templates/seller_profile.html`: FOUND
- Commit `01c3136` (13-01): FOUND
- Commit `a88338d` (13-02): FOUND
- `go build ./...`: PASSED
- `go vet ./...`: PASSED
