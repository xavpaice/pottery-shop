# Technology Stack

**Analysis Date:** 2026-04-13

## Languages

**Primary:**
- Go 1.26 - Server application and CLI tools

## Runtime

**Environment:**
- Go 1.26

**Package Manager:**
- Go modules (go.mod/go.sum)
- Lockfile: `go.sum` (present)

## Frameworks

**Core:**
- Standard Go `net/http` - HTTP server and routing
- `html/template` - Server-side HTML templating

**Testing:**
- Go built-in `testing` package - Unit and integration testing

**Build/Dev:**
- Docker with multi-stage builds - Container image creation
- Alpine Linux 3.20 - Runtime container base image
- Helm 3.x - Kubernetes package management
- kubectl - Kubernetes deployment

## Key Dependencies

**Critical:**
- `github.com/mattn/go-sqlite3 v1.14.22` - SQLite3 database driver with CGO support
- `github.com/disintegration/imaging v1.6.2` - Image processing and thumbnail generation with EXIF support
- `golang.org/x/image v0.0.0-20191009234506-e7c1f5e7dbb8` - Standard image manipulation library (indirect dependency)

**Infrastructure:**
- None - Standard library HTTP server only

## Configuration

**Environment:**
- Environment variables for configuration (see .env.example)
- Key configs required:
  - `PORT`: Server port (default: 8080)
  - `BASE_URL`: Public base URL for order emails and links
  - `ADMIN_USER`: Basic auth username for admin panel
  - `ADMIN_PASS`: Basic auth password for admin panel
  - `SESSION_SECRET`: Session encryption secret (minimum 32 characters)
  - `DB_PATH`: SQLite database file path (default: pottery.db)
  - `UPLOAD_DIR`: Directory for product image uploads (default: uploads)
  - `SMTP_HOST`: SMTP server hostname (optional, empty disables email)
  - `SMTP_PORT`: SMTP server port (default: 587)
  - `SMTP_USER`: SMTP authentication username
  - `SMTP_PASS`: SMTP authentication password
  - `SMTP_FROM`: Email sender address
  - `ORDER_EMAIL`: Destination email for order notifications

**Build:**
- `Makefile` - Build tasks (build, test, clean, run, docker, deploy)
- `Dockerfile` - Multi-stage Docker build with cross-compilation support via tonistiigi/xx

## Platform Requirements

**Development:**
- Go 1.26
- CGO_ENABLED=1 (required for SQLite)
- GCC/Clang compiler
- Make

**Production:**
- Alpine Linux 3.20 (via Docker)
- ca-certificates (for HTTPS)
- sqlite-libs (for SQLite runtime)
- Kubernetes 1.35.0+ (for k8s deployment)
- Helm 3.x (for chart deployments)

---

*Stack analysis: 2026-04-13*
