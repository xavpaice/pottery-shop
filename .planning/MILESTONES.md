# Milestones

## v1.1 TLS (Shipped: 2026-04-14)

**Phases completed:** 6 phases, 11 plans, 18 tasks

**Key accomplishments:**

- pgx/v5 driver added, all 12 SQL query sites converted to Postgres dialect with $N params and RETURNING id, Goose migration file with Postgres DDL created
- main.go rewired to Postgres via pgxpool/goose; Dockerfile and Makefile produce a pure CGO-free binary with no SQLite dependencies
- testcontainers-go postgres:16-alpine container replaces SQLite in-memory tests; go-sqlite3 fully removed; CGO_ENABLED=0 build and go vet pass clean
- CNPG operator wired as Helm subchart (0.28.0), Cluster CRD template with pg_isready init container, DATABASE_URL injection from CNPG secret in managed mode and plain DSN in external mode, RollingUpdate strategy, DB_PATH removed
- Dropped gcc from CI test job, added go vet, and extended helm-lint job with CNPG subchart dependency resolution and dual-mode lint+template validation using per-mode CI values files
- Helm chart ingress refactored from multi-host nginx array to single-host Traefik scalar with mode-driven TLS and fail-fast validation helpers
- Phase 3 certification gate closed: VERIFICATION.md created (status: verified, 6/6), SUMMARY frontmatter patched (requirements_completed), and REQUIREMENTS.md checkboxes checked — all three sources agree on INGR-01, INGR-02, INGR-03, INGR-04, TLS-03.
- ACME ClusterIssuer (staging default) + Certificate for letsencrypt mode, and four-resource SelfSigned CA bootstrap for selfsigned mode, both with Helm hook annotations for upgrade safety
- cert-manager v1.20.2 pre-install step added to integration-test.yml and 23-assertion helm-template-test.sh covering TLS-01/TLS-02 letsencrypt, selfsigned, and custom modes
- Three self-contained TLS CI values files added to chart/clay/ci/ covering letsencrypt, selfsigned, and custom modes, each passing helm lint and helm template independently
- helm-lint job extended with six TLS lint+template steps and one behavioral test step (23 assertions covering INGR-01..04, TLS-01..03), satisfying CI-05

---
