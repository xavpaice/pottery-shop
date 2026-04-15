# Clay.nz 🏺

A simple online shop for one-of-a-kind clay and sculpture pieces. Built with Go, PostgreSQL, and plain HTML/CSS.

## Features

- **Public gallery** — browse available pieces in a responsive grid
- **Product detail** — photo gallery with thumbnails, description, price
- **Shopping cart** — session-based, add/remove unique items
- **Order by email** — cart checkout sends order details to your inbox (no payment gateway)
- **Admin panel** — basic-auth protected; create/edit/delete listings, upload up to 5 photos, mark as sold
- **Auto thumbnails** — uploaded images get 400px thumbnails for fast gallery loading

## Prerequisites

- Go 1.26+
- Make
- Docker (required for integration tests — testcontainers-go spins up a Postgres container)
- For Kubernetes deployment: a Kubernetes cluster (v1.25+)

## Quick Start

The server requires a PostgreSQL database. The easiest way to run one locally is with Docker:

```bash
# Start a local Postgres instance
docker run -d --name clay-pg \
  -e POSTGRES_DB=clay \
  -e POSTGRES_USER=clay \
  -e POSTGRES_PASSWORD=dev-only \
  -p 5432:5432 \
  postgres:16-alpine

# Clone / navigate to the project
cd pottery-shop

# Copy env config and fill in required values
cp .env.example .env
# DATABASE_URL is pre-filled for the local Postgres above
# Set a real ADMIN_PASS and SESSION_SECRET

# Download dependencies
go mod tidy

# Run
go run ./cmd/server

# Visit http://localhost:8080
# Admin: http://localhost:8080/admin  (user: admin)
```

## Configuration

All config is via environment variables (or a `.env` file if you use something like `direnv`):

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | Server port |
| `BASE_URL` | `http://localhost:8080` | Public URL (used in order emails) |
| `ADMIN_USER` | `admin` | Admin username |
| `ADMIN_PASS` | — | Admin password (**required**) |
| `SESSION_SECRET` | — | Random string for cookie signing (**required**, ≥32 chars) |
| `DATABASE_URL` | — | Postgres connection string (**required**, e.g. `postgresql://user:pass@host:5432/clay`) |
| `UPLOAD_DIR` | `uploads` | Image upload directory |
| `SMTP_HOST` | (empty) | SMTP server — if empty, emails are logged to stdout |
| `SMTP_PORT` | `587` | SMTP port |
| `SMTP_USER` | (empty) | SMTP username |
| `SMTP_PASS` | (empty) | SMTP password |
| `SMTP_FROM` | (empty) | From address for emails |
| `ORDER_EMAIL` | `xavpaice@gmail.com` | Where order emails go |

## Deployment

### Docker

The image is published automatically to GitHub Container Registry on every merge to `main`:

```
ghcr.io/xavpaice/pottery-shop:latest
```

Images are also tagged with the git SHA (e.g. `ghcr.io/xavpaice/pottery-shop:sha-abc1234`).

The image is a two-stage build — Alpine with just the binary, templates, and static assets. It is a pure CGO-free Go binary (`CGO_ENABLED=0`). Uploaded images are stored at `/data/uploads`; the database is external (Postgres, required).

#### Full Docker testing workflow

The app needs a Postgres database. Use a Docker network to wire them together:

```bash
# 1. Create a shared network
docker network create clay-net

# 2. Start Postgres
docker run -d --name clay-pg --network clay-net \
  -e POSTGRES_DB=clay \
  -e POSTGRES_USER=clay \
  -e POSTGRES_PASSWORD=dev-only \
  postgres:16-alpine

# 3. Build the app image locally
make docker
# (builds ghcr.io/xavpaice/pottery-shop:latest — or use: docker build -t pottery-shop:local .)

# 4. Run the app
docker run -p 8080:8080 --network clay-net \
  -v clay-data:/data \
  -e DATABASE_URL=postgresql://clay:dev-only@clay-pg:5432/clay \
  -e ADMIN_PASS=changeme \
  -e SESSION_SECRET=$(openssl rand -hex 32) \
  ghcr.io/xavpaice/pottery-shop:latest

# Visit http://localhost:8080
# Admin: http://localhost:8080/admin  (user: admin, pass: changeme)

# 5. Cleanup when done
docker rm -f clay-pg && docker network rm clay-net
```

#### Pull and run the published image

```bash
docker pull ghcr.io/xavpaice/pottery-shop:latest

docker network create clay-net
docker run -d --name clay-pg --network clay-net \
  -e POSTGRES_DB=clay -e POSTGRES_USER=clay -e POSTGRES_PASSWORD=dev-only \
  postgres:16-alpine
docker run -p 8080:8080 --network clay-net \
  -v clay-data:/data \
  -e DATABASE_URL=postgresql://clay:dev-only@clay-pg:5432/clay \
  -e ADMIN_PASS=changeme \
  -e SESSION_SECRET=$(openssl rand -hex 32) \
  ghcr.io/xavpaice/pottery-shop:latest
```

### Kubernetes (Helm)

A Helm chart is provided in `chart/clay/`. This is the recommended way to deploy.

#### Default install (operators bundled)

Both the CloudNative-PG and cert-manager operators install automatically as Helm subcharts. No separate operator setup is needed.

```bash
# Install with bundled operators (default)
helm install clay ./chart/clay -n clay --create-namespace \
  --set secrets.ADMIN_PASS=your-secure-password \
  --set secrets.SESSION_SECRET=$(openssl rand -hex 32)
```

> **Note:** First install with bundled operators takes ~30-60 seconds longer than usual while webhook-wait Jobs confirm the operators are ready.

#### Pre-installed operators

If the CNPG operator and/or cert-manager are already running in your cluster, disable the bundled subcharts to avoid installing duplicate operator instances:

```bash
# Inline flags
helm install clay ./chart/clay -n clay --create-namespace \
  --set cloudnative-pg.enabled=false \
  --set cert-manager.enabled=false \
  --set secrets.ADMIN_PASS=your-secure-password \
  --set secrets.SESSION_SECRET=$(openssl rand -hex 32)
```

Or in a values file:

```yaml
cloudnative-pg:
  enabled: false
cert-manager:
  enabled: false
```

#### External Postgres (no CNPG)

```bash
# Install with an external Postgres (CNPG not required)
helm install clay ./chart/clay -n clay --create-namespace \
  --set postgres.managed=false \
  --set postgres.external.dsn=postgresql://user:pass@host:5432/clay \
  --set secrets.ADMIN_PASS=your-secure-password \
  --set secrets.SESSION_SECRET=$(openssl rand -hex 32)
```

```bash
# Upgrade after changes
helm upgrade clay ./chart/clay -n clay
```

Key `values.yaml` settings to customise:

| Value | Default | Notes |
|---|---|---|
| `image.repository` | `ghcr.io/xavpaice/pottery-shop` | Container image |
| `image.tag` | `latest` | Pin to a SHA tag for production |
| `secrets.ADMIN_PASS` | — | **Required** — helm render fails if empty |
| `secrets.SESSION_SECRET` | — | **Required** — helm render fails if empty |
| `postgres.managed` | `true` | `true` = CNPG cluster; `false` = external DSN |
| `postgres.external.dsn` | `""` | Required when `managed: false` |
| `postgres.cluster.instances` | `1` | CNPG cluster replica count |
| `postgres.cluster.storage.size` | `5Gi` | CNPG PVC size |
| `cloudnative-pg.enabled` | `true` | `true` = bundle CNPG operator as subchart; `false` = use pre-installed operator |
| `cert-manager.enabled` | `true` | `true` = bundle cert-manager as subchart; `false` = use pre-installed cert-manager |
| `ingress.hosts[0].host` | `clay.nz` | Your domain |
| `persistence.size` | `5Gi` | PVC for uploaded images |
| `imagePullSecrets` | `[]` | Required for private images (see below) |

#### Private image registry

The container image is private (matching the repo visibility). To pull it from your cluster, create an image pull secret:

```bash
kubectl create secret docker-registry ghcr-creds \
  -n clay \
  --docker-server=ghcr.io \
  --docker-username=xavpaice \
  --docker-password=<PAT with read:packages scope>
```

Then set it in your values:

```yaml
imagePullSecrets:
  - name: ghcr-creds
```

Or via the command line:

```bash
helm install clay ./chart/clay -n clay --create-namespace \
  --set imagePullSecrets[0].name=ghcr-creds
```

#### Upgrading from a pre-umbrella chart

If you are upgrading from a version of this chart that did not bundle operators as subcharts, you must disable the bundled operators before upgrading. Otherwise Helm will install a second copy of each operator, which conflicts with your existing installations.

Add these overrides to your values file before running `helm upgrade`:

```yaml
cloudnative-pg:
  enabled: false
cert-manager:
  enabled: false
```

Then upgrade as normal:

```bash
helm upgrade clay ./chart/clay -n clay
```

### Kubernetes (raw manifests)

Raw manifests are also available in `k8s/` if you prefer not to use Helm:

1. **Edit secrets** in `k8s/secret.yaml` — set a real `ADMIN_PASS` and `SESSION_SECRET`
2. **Update** `k8s/configmap.yaml` and `k8s/ingress.yaml` with your domain/SMTP settings
3. **Apply:**
   ```bash
   make deploy
   # or: kubectl apply -f k8s/
   ```

### Bare metal / VPS

Build a static binary and run behind nginx with Let's Encrypt:

```bash
make build

# Run
PORT=8080 BASE_URL=https://clay.nz ./pottery-server
```

Nginx reverse proxy config:
```nginx
server {
    server_name clay.nz;
    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        client_max_body_size 50M;
    }
}
```

## Development

### Make targets

```bash
make build          # Compile the server binary
make test           # Run all tests
make test-verbose   # Run tests with verbose output
make test-coverage  # Generate coverage report (coverage.html)
make run            # Build + run the server
make tidy           # go mod tidy + verify
make clean          # Remove build artifacts
make helm-lint      # Lint the Helm chart
make lint           # Run all linters
```

### Running locally

```bash
cp .env.example .env     # Configure admin password, SMTP, etc.
make run                 # Builds and starts on :8080
```

The server reads templates and static files from the working directory, so run from the project root.

### Testing

Integration tests use [testcontainers-go](https://testcontainers.com/guides/getting-started-with-testcontainers-for-go/) to spin up a real Postgres container — Docker must be running locally.

```bash
make test                # Quick pass/fail
make test-verbose        # See individual test results
make test-coverage       # HTML coverage report
```

### Contributing

1. Create a feature branch from `main`
2. Make your changes
3. Run `make test` locally to verify
4. Open a PR — GitHub Actions will run tests automatically
5. Merge once CI is green

### CI/CD

Three GitHub Actions workflows run on PRs and pushes to `main`:

**`test.yml`** — runs on every PR and push:
- `go vet` + `make test` (testcontainers-go Postgres) + `make build` with `CGO_ENABLED=0`
- Helm lint + `helm template` render check in both managed and external modes

**`integration-test.yml`** — runs on PRs (non-fork only):
- Builds and pushes a PR-tagged image to GHCR
- Installs the CNPG operator on a live k3s cluster (via CMX)
- Installs the clay chart and verifies the pod reaches Running

**`publish.yml`** — runs on merge to `main`:
- Builds and pushes the Docker image to `ghcr.io/xavpaice/pottery-shop` with `latest` and SHA tags

### Adding new features

- **New pages**: add a handler in `internal/handlers/`, create a template in `templates/`, register the route in `cmd/server/main.go`
- **New model fields**: update the struct + `Init()` schema in `internal/models/product.go`
- **Static assets**: add to `static/` — served directly, no build step
- **Templates**: use `{{define "name.html"}}...{{end}}` and reference partials with `{{template "header" .}}`

## Project Structure

```
clay.nz/
├── cmd/server/main.go              # Entry point
├── internal/
│   ├── handlers/
│   │   ├── public.go               # Public routes (home, gallery, product, cart, order)
│   │   ├── public_test.go          # Handler tests
│   │   └── admin.go                # Admin routes (CRUD, image upload)
│   ├── middleware/
│   │   ├── auth.go                 # Basic auth
│   │   ├── auth_test.go
│   │   ├── session.go              # Cookie-based sessions
│   │   └── session_test.go
│   └── models/
│       ├── product.go              # Product & Image DB models
│       ├── product_test.go
│       ├── cart.go                  # Cart (JSON in session cookie)
│       └── cart_test.go
├── templates/
│   ├── partials/                   # Header/footer partials
│   ├── admin/                      # Admin templates
│   ├── home.html                   # Shop (available items)
│   ├── gallery.html                # Gallery (sold items)
│   ├── product.html                # Product detail
│   ├── cart.html                   # Cart + order form
│   └── order_confirmed.html        # Order confirmation
├── static/css/                     # Stylesheets
├── uploads/                        # Image uploads (created at runtime)
├── k8s/                            # Kubernetes manifests
│   ├── namespace.yaml
│   ├── configmap.yaml
│   ├── secret.yaml
│   ├── pvc.yaml
│   ├── deployment.yaml
│   ├── service.yaml
│   └── ingress.yaml
├── chart/clay/                      # Helm chart
│   ├── Chart.yaml
│   ├── values.yaml
│   └── templates/                  # K8s resource templates
├── .github/workflows/test.yml      # CI pipeline (tests + lint)
├── .github/workflows/publish.yml   # Image publish to ghcr.io
├── Dockerfile                      # Multi-stage container build
├── .dockerignore
├── Makefile                        # Build/test/run/deploy targets
├── .env.example                    # Config template
└── go.mod
```
