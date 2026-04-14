# Requirements: Pottery Shop — TLS (v1.1)

**Defined:** 2026-04-14
**Core Value:** The app runs reliably on Postgres with zero SQLite anywhere — CNPG manages the in-cluster database lifecycle, and the Go binary is a pure CGO-free build.

## v1.1 Requirements (this milestone)

### Ingress

- [x] **INGR-01**: User can expose the app via a Kubernetes Ingress — `ingressClassName: traefik`, `ingress.enabled` gate, `ingress.host` scalar in values.yaml
- [x] **INGR-02**: Ingress resource carries Traefik-specific annotations — `traefik.ingress.kubernetes.io/router.entrypoints: websecure` and `acme.cert-manager.io/http01-edit-in-place: "true"`
- [x] **INGR-03**: `clay.validateIngress` helper in `_helpers.tpl` fails fast at render time on missing required values (mirrors existing `clay.validateSecrets`)
- [x] **INGR-04**: Nginx-specific annotation (`nginx.ingress.kubernetes.io/proxy-body-size`) removed from Ingress defaults

### TLS

- [ ] **TLS-01**: User can enable Let's Encrypt mode (`ingress.tls.mode: letsencrypt`) — HTTP-01 ACME ClusterIssuer, staging ACME URL by default, production opt-in via `ingress.tls.acme.production: true`
- [ ] **TLS-02**: User can enable self-signed mode (`ingress.tls.mode: selfsigned`) — two-step CA bootstrap: SelfSigned ClusterIssuer → CA cert → CA ClusterIssuer → app cert
- [x] **TLS-03**: User can enable custom mode (`ingress.tls.mode: custom`) — chart references a user-provided TLS Secret by name; no cert-manager resources created

### CI

- [ ] **CI-04**: `chart/clay/ci/` contains three TLS values files (`tls-letsencrypt-values.yaml`, `tls-selfsigned-values.yaml`, `tls-custom-values.yaml`) for lint/template validation
- [ ] **CI-05**: `test.yml` Helm validation job extended with six steps (helm lint + helm template for each TLS mode)
- [ ] **CI-06**: `integration-test.yml` includes a cert-manager v1.20.2 pre-install step (mirrors existing CNPG pre-install pattern)

## Future Requirements

### Operations

- **OPS-01**: CNPG backup configuration (WAL archiving to object storage)
- **OPS-02**: CNPG monitoring integration (Prometheus metrics from CNPG operator)
- **OPS-03**: CRD upgrade runbook documented for `helm upgrade` scenarios

### Security

- **SEC-01**: CSRF protection for POST endpoints (pre-existing concern from CONCERNS.md)
- **SEC-03**: Directory listing disabled for `/uploads/` and `/static/`

### TLS (future)

- **TLS-04**: Postgres TLS — encrypt connections between the app pod and CNPG (client cert or `sslmode=verify-full`)
- **TLS-05**: DNS-01 ACME challenge support for wildcard certs or private clusters

## Out of Scope

| Feature | Reason |
|---------|--------|
| cert-manager as a Helm subchart | Official cert-manager docs prohibit embedding as subchart — cluster-scoped CRDs, webhook timing race; matches CNPG pre-install pattern |
| Postgres TLS | Separate concern from Ingress TLS; deferred to future milestone |
| DNS-01 ACME | Requires DNS provider credentials; HTTP-01 sufficient for this deployment |
| Ingress for admin on a separate subdomain | Single-host ingress is sufficient; split routing is future scope |
| Mobile app / frontend rewrite | Not relevant to infrastructure milestones |

## Previously Validated (v1.0 — Postgres Migration)

- ✓ Replace go-sqlite3 (CGO) with pgx/v5 (pure Go) — Phase 1
- ✓ SQL dialect ported to Postgres (types, placeholders, RETURNING) — Phase 1
- ✓ Goose v3 embedded schema migrations — Phase 1
- ✓ testcontainers-go integration tests — Phase 1
- ✓ CGO_ENABLED=0 pure Go Docker build — Phase 1
- ✓ CNPG operator pre-installed; DATABASE_URL injected via Secret — Phase 2
- ✓ External Postgres via postgres.external.dsn — Phase 2
- ✓ pg_isready init container (CNPG timing mitigation) — Phase 2
- ✓ CI: build, test, helm lint/template (managed + external modes) — Phase 2
- ✓ Secrets enforcement (ADMIN_PASS, SESSION_SECRET required at deploy) — Phase 2

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| INGR-01 | Phase 3 | Satisfied |
| INGR-02 | Phase 3 | Satisfied |
| INGR-03 | Phase 3 | Satisfied |
| INGR-04 | Phase 3 | Satisfied |
| TLS-01 | Phase 4 | Pending |
| TLS-02 | Phase 4 | Pending |
| TLS-03 | Phase 3 | Satisfied |
| CI-04 | Phase 5 | Pending |
| CI-05 | Phase 5 | Pending |
| CI-06 | Phase 4 | Pending |

**Coverage:**
- v1.1 requirements: 10 total
- Mapped to phases: 10
- Unmapped: 0 ✓

---
*Requirements defined: 2026-04-14*
*Last updated: 2026-04-14 after gap closure planning — INGR-01..04 and TLS-03 assigned to Phase 3.1 (verification closure); Phase 4 augmented with INTEGRATION-02; Phase 5 augmented with INTEGRATION-03*
