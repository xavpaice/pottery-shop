# Architecture

**Analysis Date:** 2026-04-13

## Pattern Overview

**Overall:** Layered monolithic web application with separation of concerns into HTTP handlers, domain models, and middleware.

**Key Characteristics:**
- Single Go binary with embedded templates and static assets
- SQLite database with schema initialization at startup
- Template-driven HTML rendering with no client-side framework
- Session-based cart management (cookie serialization)
- Basic HTTP auth for admin area

## Layers

**HTTP Handler Layer:**
- Purpose: Accept requests, coordinate business logic, render responses
- Location: `internal/handlers/`
- Contains: PublicHandler (shop functionality) and AdminHandler (management)
- Depends on: Models (data access), Middleware (session/auth), Templates
- Used by: Main entry point configures and registers handlers

**Middleware Layer:**
- Purpose: Cross-cutting concerns - authentication, session management
- Location: `internal/middleware/`
- Contains: BasicAuth (HTTP 401 challenge), SessionManager (cookie signing/verification)
- Depends on: Standard library only (crypto, http)
- Used by: Main entry point wraps mux with middleware; handlers call session functions

**Models/Data Layer:**
- Purpose: Database access, domain entities, business logic
- Location: `internal/models/`
- Contains: ProductStore (SQLite queries), Product/Image entities, Cart (JSON serialization)
- Depends on: Standard library database/sql
- Used by: Handlers fetch/manipulate data through store interface

**Presentation Layer:**
- Purpose: HTML rendering and static asset serving
- Location: `templates/` and `static/`
- Contains: Go text/html templates (partials, pages), CSS stylesheets
- Depends on: Handler data structures
- Used by: Handlers execute templates to render responses

**Main/Configuration Layer:**
- Purpose: Application bootstrap, dependency injection, route registration
- Location: `cmd/server/main.go`
- Responsibilities: Load env config, initialize database, create handler instances, register routes, start HTTP server

## Data Flow

**Product Browse Flow:**

1. Request arrives at `/` or `/gallery`
2. Router dispatches to `PublicHandler.Home()` or `PublicHandler.Gallery()`
3. Handler queries `ProductStore.ListAvailable()` or `ProductStore.ListSold()` (SQLite)
4. Products loaded with associated images via `ProductStore.GetImages()`
5. Session middleware provides current cart count via `GetSession(r)`
6. Handler executes template with product/cart data
7. Template renders HTML with product grid, images, prices
8. Response returned to browser

**Add to Cart Flow:**

1. Form POST to `/cart/add` with product_id
2. Handler validates product exists and isn't sold
3. Handler retrieves current cart from session cookie via `GetSession(r)`
4. Deserializes JSON cart from session into Cart struct via `CartFromJSON()`
5. Handler adds CartItem to cart (prevents duplicates - unique pieces)
6. Handler serializes cart to JSON and stores in session
7. SessionManager signs and base64-encodes session data
8. Cookie written by sessionWriter on response
9. Redirect to referrer with flash message

**Admin Product Create/Edit Flow:**

1. GET `/admin/products/new` or `/admin/products/{id}/edit`
2. BasicAuth middleware validates credentials, allows access or returns 401
3. Handler renders product form template (new or pre-filled)
4. User uploads form + up to 5 images
5. POST to `/admin/products/create` or `/admin/products/update`
6. Handler parses multipart form
7. Handler creates/updates Product via `ProductStore.Create()`/`ProductStore.Update()`
8. Handler calls `handleImageUploads()`:
   - Validates content type (image/*)
   - Saves original to `uploads/` directory
   - Generates 400px thumbnail via disintegration/imaging library
   - Saves thumbnail to `uploads/thumbnails/`
   - Inserts Image records with sort order
9. Redirect to `/admin` dashboard

**State Management:**

- **Product state:** Persisted in SQLite (products, images tables)
- **Session/Cart state:** In signed HTTP-only cookie (serialized JSON)
- **File state:** Uploaded images stored on disk (uploads/ directory)
- **Configuration state:** Environment variables at startup (no reload)

## Key Abstractions

**ProductStore:**
- Purpose: Single data access object for products and images
- Examples: `internal/models/product.go`
- Pattern: Receiver methods on store struct that execute SQL queries
- Query patterns: Raw SQL with parameterized queries, row scanning into structs

**Handler Structs:**
- Purpose: Encapsulate handler functions and their dependencies
- Examples: PublicHandler in `internal/handlers/public.go`, AdminHandler in `internal/handlers/admin.go`
- Pattern: Receiver methods on handler struct (net/http.Handler not implemented, explicit registration)
- Dependency injection: Handlers receive store, templates, session manager, config via struct fields

**SessionManager:**
- Purpose: Cryptographic signing and verification of cookie data
- Examples: `internal/middleware/session.go`
- Pattern: Wraps http.ResponseWriter to intercept and persist session after handler completes
- Signing: HMAC-SHA256 base64, verified on each request

**Cart:**
- Purpose: Client-side cart abstraction that lives in session cookie
- Examples: `internal/models/cart.go`
- Pattern: JSON serialization of items array, prevents duplicate product IDs (each piece unique)
- Thread-safety: Uses sync.Mutex for concurrent operations

## Entry Points

**HTTP Server Start:**
- Location: `cmd/server/main.go:main()`
- Triggers: Process execution
- Responsibilities: Parse env config, initialize database schema, load templates, create handler instances, register routes, start listener on PORT

**HTTP Routes (Public):**
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

**HTTP Routes (Admin - Protected by BasicAuth):**
- `/admin` → `AdminHandler.Dashboard()` - Product management list
- `/admin/products/new` → `AdminHandler.NewProduct()` - New product form
- `/admin/products/create` → `AdminHandler.CreateProduct()` - POST to create
- `/admin/products/{id}/edit` → `AdminHandler.EditProduct()` - Edit form
- `/admin/products/update` → `AdminHandler.UpdateProduct()` - POST to update
- `/admin/products/delete` → `AdminHandler.DeleteProduct()` - POST to delete
- `/admin/products/toggle-sold` → `AdminHandler.ToggleSold()` - POST to toggle sold status
- `/admin/images/delete` → `AdminHandler.DeleteImage()` - POST to delete single image

## Error Handling

**Strategy:** Logging and HTTP error responses; graceful degradation where possible

**Patterns:**
- Missing resources return 404 with `http.NotFound()`
- Invalid input returns 400 with `http.Error()`
- Database/template errors return 500 with `http.Error()`
- Email failures logged but don't block order - user informed via session flash message
- Image upload failures (invalid type, disk error) silently skip file, continue with remaining
- Template execution errors logged; page returns 500

## Cross-Cutting Concerns

**Logging:** Using standard Go `log` package
- Database errors logged before returning 500
- Template rendering errors logged before returning 500
- SMTP failures logged if email fails
- Image processing errors logged if thumbnail generation fails
- SMTP not configured: logged instead of erroring (development mode)

**Validation:**
- Admin: Image content-type checked (must start with "image/")
- Admin: Max 5 images per product enforced at upload time
- Public: Product existence validated before adding to cart
- Public: Sold status checked before add-to-cart
- Forms: No client validation, server-side only

**Authentication:** BasicAuth middleware
- Uses `http.Request.BasicAuth()` to extract credentials
- Constant-time comparison with `subtle.ConstantTimeCompare()`
- 401 Unauthorized response with WWW-Authenticate header on failure
- Guards entire `/admin/*` path

**Session:** Cookie-based HMAC signing
- Session data (cart JSON, flash message) stored in cookie
- Signed with HMAC-SHA256 derived from SESSION_SECRET env var
- Verified on each request; tampered cookies rejected
- HttpOnly flag prevents JavaScript access
- SameSite=Lax prevents CSRF
- 7-day expiration

**Cart:** Session middleware provides session context to handlers
- Handlers call `middleware.GetSession(r)` to get session data
- Cart deserialized from JSON on each request
- Cart serialized to JSON and stored in session on each modification
- Changes persisted via SessionManager.Save() after handler completes

## Database Schema

**products table:**
```sql
id INTEGER PRIMARY KEY AUTOINCREMENT
title TEXT NOT NULL
description TEXT NOT NULL DEFAULT ''
price REAL NOT NULL DEFAULT 0
is_sold INTEGER NOT NULL DEFAULT 0
created_at DATETIME DEFAULT CURRENT_TIMESTAMP
updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
```

**images table:**
```sql
id INTEGER PRIMARY KEY AUTOINCREMENT
product_id INTEGER NOT NULL (FOREIGN KEY references products)
filename TEXT NOT NULL
thumbnail_fn TEXT NOT NULL DEFAULT ''
sort_order INTEGER NOT NULL DEFAULT 0
created_at DATETIME DEFAULT CURRENT_TIMESTAMP
```

Foreign key constraints enabled with `?_foreign_keys=on` connection parameter.

---

*Architecture analysis: 2026-04-13*
