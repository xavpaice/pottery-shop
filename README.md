# Earth & Fire Pottery Shop 🏺

A simple online shop for one-of-a-kind pottery and sculpture pieces. Built with Go, SQLite, and plain HTML/CSS.

## Features

- **Public gallery** — browse available pieces in a responsive grid
- **Product detail** — photo gallery with thumbnails, description, price
- **Shopping cart** — session-based, add/remove unique items
- **Order by email** — cart checkout sends order details to your inbox (no payment gateway)
- **Admin panel** — basic-auth protected; create/edit/delete listings, upload up to 5 photos, mark as sold
- **Auto thumbnails** — uploaded images get 400px thumbnails for fast gallery loading

## Prerequisites

- Go 1.21+
- GCC (for SQLite via cgo) — on Debian/Ubuntu: `apt install build-essential`

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
| `DB_PATH` | `pottery.db` | SQLite database file path |
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
go build -o pottery-server ./cmd/server

# Run
PORT=8080 BASE_URL=https://yoursite.com ./pottery-server
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

## Project Structure

```
pottery-shop/
├── cmd/server/main.go          # Entry point
├── internal/
│   ├── handlers/
│   │   ├── public.go           # Public routes (home, product, cart, order)
│   │   └── admin.go            # Admin routes (CRUD, image upload)
│   ├── middleware/
│   │   ├── auth.go             # Basic auth
│   │   └── session.go          # Cookie-based sessions
│   └── models/
│       ├── product.go          # Product & Image DB models
│       └── cart.go             # Cart (JSON in session cookie)
├── templates/
│   ├── partials/               # Header/footer partials
│   ├── admin/                  # Admin templates
│   ├── home.html               # Product gallery
│   ├── product.html            # Product detail
│   ├── cart.html               # Cart + order form
│   └── order_confirmed.html    # Order confirmation
├── static/css/                 # Stylesheets
├── uploads/                    # Image uploads (created at runtime)
├── .env.example                # Config template
└── go.mod
```
