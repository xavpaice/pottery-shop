# CONCERNS.md — Technical Debt, Issues, and Fragile Areas

## Security

### HIGH: Weak Default Credentials
- `ADMIN_PASS` defaults to `"changeme"` in `cmd/server/main.go:26`
- `SESSION_SECRET` defaults to a hardcoded string
- `ORDER_EMAIL` defaults to `xavpaice@gmail.com` (personal email hardcoded)
- Risk: Deployments without explicit env vars are insecure out of the box

### HIGH: Uploaded Files Served Without Auth
- `/uploads/` is served via `http.FileServer` with no access control
- Any user can directly access `uploads/` URLs for any product, including draft/deleted items
- No path traversal protection beyond Go's stdlib FileServer

### MEDIUM: No CSRF Protection
- POST endpoints (`/cart/add`, `/cart/remove`, `/order`, all admin POSTs) have no CSRF tokens
- Session cookie uses `SameSite=Lax` which mitigates same-site attacks but not all cross-origin scenarios
- Admin actions (delete product, toggle sold) are vulnerable to CSRF

### MEDIUM: Static File Directory Listing
- `http.FileServer` for `static/` and `uploads/` enables directory listing by default
- `uploads/` directory listing would expose all product image filenames

### LOW: No Rate Limiting
- Login attempts against BasicAuth are unbounded
- Order placement (`/order`) has no rate limiting, could be abused for spam emails

### LOW: Email Enumeration / Order Spam
- `/order` sends emails to `ORDER_EMAIL` on any valid cart submission with no throttling

## Technical Debt

### Committed Binary in Repo
- `pottery-server` binary is present at root of repo (should be in `.gitignore`)
- Binary bloats repo history and poses supply chain risk if not rebuilt from source

### `pottery.db` in Repo
- Live database file with actual product data committed to repo
- May contain sensitive customer/order data, prices, etc.

### Hardcoded Personal Email
- `ORDER_EMAIL` defaults to `xavpaice@gmail.com` in source (`cmd/server/main.go:73`)
- Should be required env var with no default

### No Input Validation on Product Forms
- Price parsed with `strconv.ParseFloat` — errors silently produce 0.0
- No max length on title/description fields (only DB constraints)
- No sanitization of user-provided text before template rendering (though Go's `html/template` auto-escapes)

### Image Upload: Silent Failures
- Invalid content-type images are silently skipped (`internal/handlers/admin.go`)
- Disk write errors for image files are logged but don't fail the request
- No feedback to admin user that some images were skipped

### Admin Edit Route Fragility
- Admin product edit route uses path suffix matching (`strings.HasSuffix(r.URL.Path, "/edit")`) in `cmd/server/main.go:116-120`
- Unusual pattern that bypasses normal mux routing; fragile if path structure changes

### No Pagination
- `ProductStore.ListAll()` and `ListAvailable()` return all products
- Will degrade as product count grows; no cursor or limit/offset

### Cart Concurrency
- `Cart` uses `sync.Mutex` in `internal/models/cart.go`
- But cart is deserialized from cookie per-request — mutex protects in-process operations, not distributed concurrency (not an issue for single instance)

## Operational Concerns

### Single-Instance Only
- SQLite + local disk uploads = no horizontal scaling possible
- Kubernetes PVC mounts and single-replica deployment in `chart/clay/values.yaml` confirm this is known

### No Health Endpoint
- No `/healthz` or `/readyz` endpoint
- Kubernetes liveness/readiness checks rely on TCP connection only (if not configured otherwise)

### Template Parsing at Startup Only
- Templates parsed once at startup (`template.Must(...)` in `cmd/server/main.go`)
- Template errors cause fatal crash; changes require restart
- No hot-reload for development

### Uploads Not Backed Up
- Product images stored on local filesystem (`UPLOAD_DIR`)
- If PVC is lost, all images are lost with no recovery path

### Log Verbosity
- Standard `log` package with no structured logging, no log levels
- Debug-level info (e.g. template errors) mixed with operational info

## Performance

### N+1 Query Pattern
- `ProductStore.ListAll()` / `ListAvailable()` fetch products, then images are likely fetched per-product
- Review `internal/models/product.go` — if `GetImages()` is called in a loop this is a classic N+1

### Image Processing In-Request
- Thumbnail generation happens synchronously during product create/update
- Large images will slow the admin POST response; no async processing

### No Caching
- No HTTP cache headers on product pages or static assets (beyond Go's default behavior)
- Every page view hits SQLite

## Testing Gaps

### No End-to-End Tests for Order Flow
- Order placement (email sending) not unit tested
- SMTP integration not tested with a mock SMTP server

### Admin Handlers Untested
- `internal/handlers/admin.go` has no corresponding `admin_test.go`
- Image upload, product CRUD all untested at handler level

### Integration Test Scope
- Integration test (`make integration-test`) only verifies the pod starts and returns HTTP 200 on `/`
- Does not test actual shop functionality, admin, or order flow
