# External Integrations

**Analysis Date:** 2026-04-13

## APIs & External Services

**Email Service (Optional):**
- SMTP - Email notifications for pottery orders
  - SDK/Client: Go standard `net/smtp`
  - Auth: Environment variables (`SMTP_USER`, `SMTP_PASS`)
  - Configuration: `SMTP_HOST`, `SMTP_PORT`, `SMTP_FROM`, `ORDER_EMAIL`
  - Implementation location: `internal/handlers/public.go` - `sendEmail()` method
  - Fallback: When SMTP_HOST is empty, emails are logged to stdout instead of sent

## Data Storage

**Databases:**
- SQLite 3 (local file-based)
  - Connection: File-based at path specified by `DB_PATH` env var (default: `pottery.db`)
  - Client: `github.com/mattn/go-sqlite3` v1.14.22
  - Initialization: Located in `cmd/server/main.go` - `sql.Open("sqlite3", dbPath+"?_foreign_keys=on")`
  - Schema: Auto-created on first run via `ProductStore.Init()` in `internal/models/product.go`
  - Tables: `products` (product metadata) and `images` (product images with EXIF-aware thumbnails)

**File Storage:**
- Local filesystem only
  - Upload directory: `uploads/` (configurable via `UPLOAD_DIR` env var)
  - Thumbnails: `uploads/thumbnails/` subdirectory
  - Served via: HTTP static file server at `/uploads/` endpoint
  - Images processed via `github.com/disintegration/imaging` with automatic EXIF orientation

**Caching:**
- None - No explicit caching layer

## Authentication & Identity

**Auth Provider:**
- Custom HTTP Basic Authentication
  - Implementation: `internal/middleware/auth.go` - `BasicAuth()` function
  - Admin panel protected with username/password from `ADMIN_USER` and `ADMIN_PASS` env vars
  - Public pages use session-based shopping cart

**Session Management:**
- Custom session implementation in `internal/middleware/session.go`
  - Session store: In-memory with HTTP-only cookies
  - Secret: `SESSION_SECRET` env var (minimum 32 characters recommended)
  - Usage: Shopping cart state persistence

## Monitoring & Observability

**Error Tracking:**
- None - No external error tracking service

**Logs:**
- Standard Go `log` package to stdout
  - Errors logged when: database failures, template errors, image processing errors, email failures
  - Location: `cmd/server/main.go`, `internal/handlers/public.go`, `internal/handlers/admin.go`

## CI/CD & Deployment

**Hosting:**
- Docker containers deployed to Kubernetes
- Container registry: GitHub Container Registry (ghcr.io)
- Image repository: `ghcr.io/xavpaice/pottery-shop`

**CI Pipeline:**
- GitHub Actions

**Workflows:**

1. **Test Pipeline** (`.github/workflows/test.yml`)
   - Trigger: Push to main, pull requests
   - Runs: Go tests via `make test`
   - Builds: Binary via `make build`
   - Lints: Helm chart via `make helm-lint`

2. **Publish Pipeline** (`.github/workflows/publish.yml`)
   - Trigger: Push to main only
   - Action: Builds Docker image via `docker/build-push-action@v6`
   - Publishes to: GitHub Container Registry
   - Tags: `sha` and `latest` (on default branch)
   - Auth: Uses `GITHUB_TOKEN` secret

3. **Integration Test Pipeline** (`.github/workflows/integration-test.yml`)
   - Trigger: Manual or scheduled
   - Process: Builds image → creates K3s cluster via Replicated CLI → deploys Helm chart → verifies installation → cleanup
   - Requires: `REPLICATED_API_TOKEN` and `GHCR_TOKEN` secrets
   - Cluster: Temporary K3s cluster (30 minute TTL)

## Environment Configuration

**Required env vars for operations:**
- Production: `PORT`, `BASE_URL`, `ADMIN_USER`, `ADMIN_PASS`, `SESSION_SECRET`, `DB_PATH`, `UPLOAD_DIR`
- Email: `SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_PASS`, `SMTP_FROM`, `ORDER_EMAIL` (optional)

**Secrets location:**
- Development: `.env` file (not committed)
- Kubernetes: `k8s/secret.yaml` (must be updated at deploy time) and helm values

## Webhooks & Callbacks

**Incoming:**
- None

**Outgoing:**
- SMTP email notifications to `ORDER_EMAIL` when orders are placed
  - Endpoint: Configured SMTP server (`SMTP_HOST:SMTP_PORT`)
  - Trigger: User completes checkout via `/order` endpoint
  - Contents: Order details, buyer name, email, message, item list, total price

## Container Image Details

**Build Process:**
- Multi-stage build via Dockerfile
- Stage 1: Builder - golang:1.26-alpine with CGO enabled for cross-compilation
- Stage 2: Runtime - alpine:3.20 minimal image
- Cross-platform support: Uses tonistiigi/xx for `linux/amd64` and `linux/arm64` builds

**Runtime Environment Variables:**
- `PORT=8080`
- `DB_PATH=/data/clay.db`
- `UPLOAD_DIR=/data/uploads`
- All other vars must be injected via ConfigMap and Secret

**Persistence:**
- SQLite database: `/data/clay.db` (must be PersistentVolume)
- Image uploads: `/data/uploads/` (must be PersistentVolume)

## Kubernetes Deployment

**Chart Location:** `chart/clay/`
- Helm chart for packaging and deploying to Kubernetes
- ConfigMap: `k8s/configmap.yaml` - Non-secret environment variables
- Secret: `k8s/secret.yaml` - Sensitive credentials
- Deployment: `k8s/deployment.yaml` - Single replica (SQLite limitation), Recreate strategy
- Service: `k8s/service.yaml` - Exposes port 8080
- Ingress: `k8s/ingress.yaml` - HTTP routing
- PersistentVolumeClaim: `k8s/pvc.yaml` - Storage for database and uploads

---

*Integration audit: 2026-04-13*
