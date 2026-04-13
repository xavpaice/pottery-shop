<!-- GSD:project-start source:PROJECT.md -->
## Project

**Pottery Shop — Postgres Migration**

An e-commerce pottery shop built in Go that is being migrated from SQLite to PostgreSQL. The app manages a product catalog with image uploads, a session-based shopping cart, order placement via email, and an admin area. It deploys to Kubernetes via Helm, and this work adds CloudNative-PG as the database layer — either managed in-cluster via the CNPG operator (installed as a Helm subchart) or pointed at an external Postgres via a DSN in values.yaml.

**Core Value:** The app runs reliably on Postgres with zero SQLite anywhere — CNPG manages the in-cluster database lifecycle, and the Go binary is a pure CGO-free build.

### Constraints

- **Tech stack**: Go + `pgx/v5` — no CGO, no go-sqlite3
- **Database**: PostgreSQL only — no SQLite anywhere
- **Kubernetes**: CloudNative-PG operator (CNPG) as Helm subchart
- **Helm**: Single chart manages both CNPG Cluster and app; values.yaml controls mode (managed vs external)
- **Compatibility**: Existing Helm values structure must remain backward-compatible where possible
<!-- GSD:project-end -->

<!-- GSD:stack-start source:codebase/STACK.md -->
## Technology Stack

## Languages
- Go 1.26 - Server application and CLI tools
## Runtime
- Go 1.26
- Go modules (go.mod/go.sum)
- Lockfile: `go.sum` (present)
## Frameworks
- Standard Go `net/http` - HTTP server and routing
- `html/template` - Server-side HTML templating
- Go built-in `testing` package - Unit and integration testing
- Docker with multi-stage builds - Container image creation
- Alpine Linux 3.20 - Runtime container base image
- Helm 3.x - Kubernetes package management
- kubectl - Kubernetes deployment
## Key Dependencies
- `github.com/mattn/go-sqlite3 v1.14.22` - SQLite3 database driver with CGO support
- `github.com/disintegration/imaging v1.6.2` - Image processing and thumbnail generation with EXIF support
- `golang.org/x/image v0.0.0-20191009234506-e7c1f5e7dbb8` - Standard image manipulation library (indirect dependency)
- None - Standard library HTTP server only
## Configuration
- Environment variables for configuration (see .env.example)
- Key configs required:
- `Makefile` - Build tasks (build, test, clean, run, docker, deploy)
- `Dockerfile` - Multi-stage Docker build with cross-compilation support via tonistiigi/xx
## Platform Requirements
- Go 1.26
- CGO_ENABLED=1 (required for SQLite)
- GCC/Clang compiler
- Make
- Alpine Linux 3.20 (via Docker)
- ca-certificates (for HTTPS)
- sqlite-libs (for SQLite runtime)
- Kubernetes 1.35.0+ (for k8s deployment)
- Helm 3.x (for chart deployments)
<!-- GSD:stack-end -->

<!-- GSD:conventions-start source:CONVENTIONS.md -->
## Conventions

## Language
## File Naming
- Source files: lowercase with underscores (e.g., `product.go`, `basic_auth.go`)
- Test files: `_test.go` suffix co-located with source (e.g., `product_test.go`)
## Naming Conventions
### Exported (public) identifiers
- PascalCase for functions, types, structs, interfaces, constants
- Examples: `CreateProduct`, `GetByID`, `ProductStore`, `Product`
### Unexported (private) identifiers
- camelCase for variables, functions
- Examples: `setupTestStore`, `createSampleProduct`
### Database schema
- snake_case for column names: `product_id`, `is_sold`, `created_at`
### Go struct fields
- PascalCase mirroring DB columns: `ProductID`, `Title`, `IsSold`, `CreatedAt`
## Package Organization
## Error Handling
- Explicit `(result, error)` return tuples throughout
- Errors propagated up to handlers for HTTP response formatting
- Fatal errors logged with `log.Fatalf()` at startup
## Logging
- Standard library `log` package
- `log.Printf()` for informational messages
- `log.Fatalf()` for fatal startup errors
## Import Order
## Database Field Mapping
- DB snake_case → Go struct PascalCase via struct tags (e.g., `` `db:"product_id"` ``)
<!-- GSD:conventions-end -->

<!-- GSD:architecture-start source:ARCHITECTURE.md -->
## Architecture

## Pattern Overview
- Single Go binary with embedded templates and static assets
- SQLite database with schema initialization at startup
- Template-driven HTML rendering with no client-side framework
- Session-based cart management (cookie serialization)
- Basic HTTP auth for admin area
## Layers
- Purpose: Accept requests, coordinate business logic, render responses
- Location: `internal/handlers/`
- Contains: PublicHandler (shop functionality) and AdminHandler (management)
- Depends on: Models (data access), Middleware (session/auth), Templates
- Used by: Main entry point configures and registers handlers
- Purpose: Cross-cutting concerns - authentication, session management
- Location: `internal/middleware/`
- Contains: BasicAuth (HTTP 401 challenge), SessionManager (cookie signing/verification)
- Depends on: Standard library only (crypto, http)
- Used by: Main entry point wraps mux with middleware; handlers call session functions
- Purpose: Database access, domain entities, business logic
- Location: `internal/models/`
- Contains: ProductStore (SQLite queries), Product/Image entities, Cart (JSON serialization)
- Depends on: Standard library database/sql
- Used by: Handlers fetch/manipulate data through store interface
- Purpose: HTML rendering and static asset serving
- Location: `templates/` and `static/`
- Contains: Go text/html templates (partials, pages), CSS stylesheets
- Depends on: Handler data structures
- Used by: Handlers execute templates to render responses
- Purpose: Application bootstrap, dependency injection, route registration
- Location: `cmd/server/main.go`
- Responsibilities: Load env config, initialize database, create handler instances, register routes, start HTTP server
## Data Flow
- **Product state:** Persisted in SQLite (products, images tables)
- **Session/Cart state:** In signed HTTP-only cookie (serialized JSON)
- **File state:** Uploaded images stored on disk (uploads/ directory)
- **Configuration state:** Environment variables at startup (no reload)
## Key Abstractions
- Purpose: Single data access object for products and images
- Examples: `internal/models/product.go`
- Pattern: Receiver methods on store struct that execute SQL queries
- Query patterns: Raw SQL with parameterized queries, row scanning into structs
- Purpose: Encapsulate handler functions and their dependencies
- Examples: PublicHandler in `internal/handlers/public.go`, AdminHandler in `internal/handlers/admin.go`
- Pattern: Receiver methods on handler struct (net/http.Handler not implemented, explicit registration)
- Dependency injection: Handlers receive store, templates, session manager, config via struct fields
- Purpose: Cryptographic signing and verification of cookie data
- Examples: `internal/middleware/session.go`
- Pattern: Wraps http.ResponseWriter to intercept and persist session after handler completes
- Signing: HMAC-SHA256 base64, verified on each request
- Purpose: Client-side cart abstraction that lives in session cookie
- Examples: `internal/models/cart.go`
- Pattern: JSON serialization of items array, prevents duplicate product IDs (each piece unique)
- Thread-safety: Uses sync.Mutex for concurrent operations
## Entry Points
- Location: `cmd/server/main.go:main()`
- Triggers: Process execution
- Responsibilities: Parse env config, initialize database schema, load templates, create handler instances, register routes, start listener on PORT
- `/` → `PublicHandler.Home()` - Browse available items
- `/gallery` → `PublicHandler.Gallery()` - View sold items
- `/product/{id}` → `PublicHandler.ProductDetail()` - Single item detail
- `/cart` → `PublicHandler.ViewCart()` - View cart contents
- `/cart/add` → `PublicHandler.AddToCart()` - POST to add item
- `/cart/remove` → `PublicHandler.RemoveFromCart()` - POST to remove item
- `/order` → `PublicHandler.PlaceOrder()` - POST to place order (sends email)
- `/order-confirmed` → `PublicHandler.OrderConfirmed()` - Confirmation page
- `/static/*` → FileServer for CSS/JS
- `/uploads/*` → FileServer for product images
- `/admin` → `AdminHandler.Dashboard()` - Product management list
- `/admin/products/new` → `AdminHandler.NewProduct()` - New product form
- `/admin/products/create` → `AdminHandler.CreateProduct()` - POST to create
- `/admin/products/{id}/edit` → `AdminHandler.EditProduct()` - Edit form
- `/admin/products/update` → `AdminHandler.UpdateProduct()` - POST to update
- `/admin/products/delete` → `AdminHandler.DeleteProduct()` - POST to delete
- `/admin/products/toggle-sold` → `AdminHandler.ToggleSold()` - POST to toggle sold status
- `/admin/images/delete` → `AdminHandler.DeleteImage()` - POST to delete single image
## Error Handling
- Missing resources return 404 with `http.NotFound()`
- Invalid input returns 400 with `http.Error()`
- Database/template errors return 500 with `http.Error()`
- Email failures logged but don't block order - user informed via session flash message
- Image upload failures (invalid type, disk error) silently skip file, continue with remaining
- Template execution errors logged; page returns 500
## Cross-Cutting Concerns
- Database errors logged before returning 500
- Template rendering errors logged before returning 500
- SMTP failures logged if email fails
- Image processing errors logged if thumbnail generation fails
- SMTP not configured: logged instead of erroring (development mode)
- Admin: Image content-type checked (must start with "image/")
- Admin: Max 5 images per product enforced at upload time
- Public: Product existence validated before adding to cart
- Public: Sold status checked before add-to-cart
- Forms: No client validation, server-side only
- Uses `http.Request.BasicAuth()` to extract credentials
- Constant-time comparison with `subtle.ConstantTimeCompare()`
- 401 Unauthorized response with WWW-Authenticate header on failure
- Guards entire `/admin/*` path
- Session data (cart JSON, flash message) stored in cookie
- Signed with HMAC-SHA256 derived from SESSION_SECRET env var
- Verified on each request; tampered cookies rejected
- HttpOnly flag prevents JavaScript access
- SameSite=Lax prevents CSRF
- 7-day expiration
- Handlers call `middleware.GetSession(r)` to get session data
- Cart deserialized from JSON on each request
- Cart serialized to JSON and stored in session on each modification
- Changes persisted via SessionManager.Save() after handler completes
## Database Schema
```sql
```
```sql
```
<!-- GSD:architecture-end -->

<!-- GSD:skills-start source:skills/ -->
## Project Skills

No project skills found. Add skills to any of: `.claude/skills/`, `.agents/skills/`, `.cursor/skills/`, or `.github/skills/` with a `SKILL.md` index file.
<!-- GSD:skills-end -->

<!-- GSD:workflow-start source:GSD defaults -->
## GSD Workflow Enforcement

Before using Edit, Write, or other file-changing tools, start work through a GSD command so planning artifacts and execution context stay in sync.

Use these entry points:
- `/gsd-quick` for small fixes, doc updates, and ad-hoc tasks
- `/gsd-debug` for investigation and bug fixing
- `/gsd-execute-phase` for planned phase work

Do not make direct repo edits outside a GSD workflow unless the user explicitly asks to bypass it.
<!-- GSD:workflow-end -->



<!-- GSD:profile-start -->
## Developer Profile

> Profile not yet configured. Run `/gsd-profile-user` to generate your developer profile.
> This section is managed by `generate-claude-profile` -- do not edit manually.
<!-- GSD:profile-end -->
