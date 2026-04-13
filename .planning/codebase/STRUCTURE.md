# STRUCTURE.md — Directory Layout and Organization

## Top-Level Layout

```
pottery-shop/
├── cmd/server/         # Application entry point
├── internal/           # Private application packages
│   ├── handlers/       # HTTP request handlers
│   ├── middleware/     # HTTP middleware (auth, session)
│   └── models/         # Domain models and data access
├── templates/          # Go HTML templates
│   ├── partials/       # Shared template partials (header, footer)
│   └── admin/          # Admin-specific templates
├── static/             # Static assets (CSS, JS)
│   ├── css/
│   └── js/             # (empty)
├── uploads/            # User-uploaded product images (runtime)
│   └── thumbnails/     # Auto-generated 400px thumbnails
├── chart/clay/         # Helm chart for Kubernetes deployment
│   └── templates/      # Kubernetes manifest templates
├── k8s/                # Raw Kubernetes manifests (alternative to Helm)
├── .github/workflows/  # CI/CD pipelines
└── pottery.db          # SQLite database file (runtime, gitignored)
```

## Key File Locations

| File | Purpose |
|------|---------|
| `cmd/server/main.go` | Entry point: env config, DI, route registration, server start |
| `internal/handlers/public.go` | Shop-facing routes (home, gallery, cart, order) |
| `internal/handlers/admin.go` | Admin routes (CRUD products, image upload) |
| `internal/middleware/auth.go` | BasicAuth middleware for admin area |
| `internal/middleware/session.go` | HMAC-signed cookie session management |
| `internal/models/product.go` | ProductStore (SQLite queries), Product/Image structs |
| `internal/models/cart.go` | Cart struct, JSON serialization, CartItem |
| `templates/*.html` | Public page templates (home, gallery, product, cart, etc.) |
| `templates/partials/*.html` | Shared header/footer partials |
| `templates/admin/*.html` | Admin dashboard and product form |
| `static/css/style.css` | Public site stylesheet |
| `static/css/admin.css` | Admin panel stylesheet |
| `Makefile` | Dev commands (build, test, run, docker, deploy) |
| `Dockerfile` | Container image definition |
| `chart/clay/` | Helm chart for production Kubernetes deployment |
| `k8s/` | Raw Kubernetes manifests (alternative deployment path) |
| `.github/workflows/test.yml` | CI: unit tests + helm lint on PR/push to main |
| `.github/workflows/publish.yml` | CD: build + push image to GHCR on merge to main |
| `.github/workflows/integration-test.yml` | Integration: deploy to CMX cluster on PR |

## Test File Locations

Co-located with source:
- `internal/models/product_test.go` — ProductStore CRUD tests
- `internal/models/cart_test.go` — Cart serialization/manipulation tests
- `internal/middleware/auth_test.go` — BasicAuth middleware tests
- `internal/middleware/session_test.go` — Session signing/verification tests
- `internal/handlers/public_test.go` — HTTP handler integration tests

## Naming Conventions

- **Go packages:** lowercase, single word (`handlers`, `middleware`, `models`)
- **Go files:** lowercase with underscores where needed (`basic_auth.go`)
- **Test files:** `_test.go` suffix, same package as source
- **Templates:** `snake_case.html` (e.g., `product_form.html`, `order_confirmed.html`)
- **CSS files:** `snake_case.css` (e.g., `admin.css`, `style.css`)
- **Uploaded images:** `{product_id}_{timestamp}_{index}.{ext}` (e.g., `2_1774594352829686128_0.jpeg`)
- **Thumbnail files:** `thumb_{original_filename}` prefix

## Environment Configuration

All config via environment variables with defaults in `cmd/server/main.go:envOr()`:

| Var | Default | Purpose |
|-----|---------|---------|
| `PORT` | `8080` | HTTP listen port |
| `BASE_URL` | `http://localhost:8080` | Used in email links |
| `ADMIN_USER` | `admin` | BasicAuth username |
| `ADMIN_PASS` | `changeme` | BasicAuth password |
| `SESSION_SECRET` | `change-this-...` | HMAC signing key (32+ chars) |
| `DB_PATH` | `pottery.db` | SQLite database file path |
| `UPLOAD_DIR` | `uploads` | Directory for image uploads |
| `SMTP_HOST` | (empty) | SMTP server hostname |
| `SMTP_PORT` | `587` | SMTP port |
| `SMTP_USER` | (empty) | SMTP auth username |
| `SMTP_PASS` | (empty) | SMTP auth password |
| `SMTP_FROM` | (empty) | From address for order emails |
| `ORDER_EMAIL` | `xavpaice@gmail.com` | Destination for order notifications |

## Deployment Paths

**Helm (primary):** `chart/clay/` — full Kubernetes deployment with configmap, secret, PVC, ingress
**Raw k8s:** `k8s/` — equivalent manifests without Helm templating
**Docker:** `Dockerfile` — single-stage Go build, runs as pottery-server binary
**Local dev:** `make run` — build and start directly

## Binary Output

- Built binary: `pottery-server` (output of `make build`)
- Committed binary present at `/shared/pottery-shop/pottery-server` (should be gitignored)
