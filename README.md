# Clay.nz 🏺

A simple online shop for one-of-a-kind clay and sculpture pieces. Built with Go, SQLite, and plain HTML/CSS.

## Features

- **Public gallery** — browse available pieces in a responsive grid
- **Product detail** — photo gallery with thumbnails, description, price
- **Shopping cart** — session-based, add/remove unique items
- **Order by email** — cart checkout sends order details to your inbox (no payment gateway)
- **Admin panel** — basic-auth protected; create/edit/delete listings, upload up to 5 photos, mark as sold
- **Auto thumbnails** — uploaded images get 400px thumbnails for fast gallery loading

## Prerequisites

- Go 1.26+
- GCC (for SQLite via cgo) — on Debian/Ubuntu: `apt install build-essential`
- Make

## Quick Start

```bash
# Clone / navigate to the project
cd pottery-shop

# Copy env config
cp .env.example .env
# Edit .env with your admin password, SMTP settings, etc.

# Download dependencies
go mod tidy

# Run
go run ./cmd/server

# Visit http://localhost:8080
# Admin: http://localhost:8080/admin (default: admin / changeme)
# Or visit https://clay.nz once deployed
```

## Configuration

All config is via environment variables (or a `.env` file if you use something like `direnv`):

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | Server port |
| `BASE_URL` | `http://localhost:8080` | Public URL (used in order emails) |
| `ADMIN_USER` | `admin` | Admin username |
| `ADMIN_PASS` | `changeme` | Admin password |
| `SESSION_SECRET` | (default) | Random string for cookie signing (≥32 chars) |
| `DB_PATH` | `clay.db` | SQLite database file path |
| `UPLOAD_DIR` | `uploads` | Image upload directory |
| `SMTP_HOST` | (empty) | SMTP server — if empty, emails are logged to stdout |
| `SMTP_PORT` | `587` | SMTP port |
| `SMTP_USER` | (empty) | SMTP username |
| `SMTP_PASS` | (empty) | SMTP password |
| `SMTP_FROM` | (empty) | From address for emails |
| `ORDER_EMAIL` | `xavpaice@gmail.com` | Where order emails go |

## Production

Build a static binary and run behind nginx with Let's Encrypt:

```bash
go build -o clay-server ./cmd/server

# Run
PORT=8080 BASE_URL=https://clay.nz ./clay-server
```

Nginx reverse proxy config:
```nginx
server {
    server_name yoursite.com;
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
```

### Running locally

```bash
cp .env.example .env     # Configure admin password, SMTP, etc.
make run                 # Builds and starts on :8080
```

The server reads templates and static files from the working directory, so run from the project root.

### Testing

Tests use in-memory SQLite and real templates via `httptest` — no external services needed.

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

GitHub Actions runs on every PR and push to `main`:
- Sets up Go 1.26 + gcc
- Verifies module dependencies
- Runs `make test`
- Runs `make build`

See `.github/workflows/test.yml` for details.

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
├── .github/workflows/test.yml      # CI pipeline
├── Makefile                        # Build/test/run targets
├── .env.example                    # Config template
└── go.mod
```
