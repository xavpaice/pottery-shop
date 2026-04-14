# Phase 3: Values and Ingress Refactor - Context

**Gathered:** 2026-04-14
**Status:** Ready for planning

<domain>
## Phase Boundary

The chart's ingress values block is refactored from a multi-host array shape to a single-host, mode-driven shape. The Ingress template is updated to render correctly for all three TLS modes (`letsencrypt`, `selfsigned`, `custom`). Helpers validate required values at render time. No cert-manager CRs are created in this phase — that is Phase 4.

Scope: values.yaml ingress block refactor, ingress.yaml template rewrite, `clay.validateIngress` + `clay.tlsSecretName` helpers in `_helpers.tpl`, removal of nginx annotation.

</domain>

<decisions>
## Implementation Decisions

### Backward Compatibility
- **D-01:** Hard break — remove old `ingress.hosts[]`, `ingress.tls[]` (array), `ingress.className`, and `ingress.annotations` keys entirely from `values.yaml`.
- **D-02:** Add a comment block in `values.yaml` near the new ingress section documenting the shape change (old vs. new), so operators can see what to migrate. No dual-path template support.

### New values.yaml Ingress Shape
- **D-03:** Replace the entire ingress block with:
  ```yaml
  ingress:
    enabled: false
    className: traefik          # ingressClassName field — override for non-Traefik controllers
    host: ""                    # REQUIRED when enabled — e.g. shop.example.com
    tls:
      mode: ""                  # REQUIRED when enabled — letsencrypt | selfsigned | custom
      secretName: ""            # custom mode only — name of pre-existing TLS Secret
      acme:
        email: ""               # letsencrypt mode only — REQUIRED
        production: false       # letsencrypt: false = staging ACME endpoint
  ```
- **D-04:** `ingress.enabled` defaults to `false` — explicit opt-in required.

### ingressClassName Behavior
- **D-05:** `ingress.className` lives in `values.yaml` with default `traefik`. Template renders `ingressClassName: {{ .Values.ingress.className }}` only when the value is non-empty (same conditional pattern as current `className` rendering). Overridable for other controllers without touching the template.

### Ingress Template Rewrite
- **D-06:** Template renders a single `rules` entry from `ingress.host` (not a range over an array).
- **D-07:** Traefik annotations hardcoded in the template metadata (not in `values.yaml`), applied whenever `ingress.enabled: true`:
  - `traefik.ingress.kubernetes.io/router.entrypoints: websecure`
  - `acme.cert-manager.io/http01-edit-in-place: "true"`
- **D-08:** TLS block in spec always references the secret name from the `clay.tlsSecretName` helper. Present for `letsencrypt`, `selfsigned`, and `custom` modes (all three get TLS termination).
- **D-09:** `nginx.ingress.kubernetes.io/proxy-body-size` annotation removed — not present in new defaults.

### clay.validateIngress Helper (INGR-03)
- **D-10:** Add `clay.validateIngress` to `_helpers.tpl`, called from `ingress.yaml`. Validates:
  1. `ingress.host` is non-empty → fail: `"ingress.host must be set"`
  2. `ingress.tls.mode` is non-empty → fail: `"ingress.tls.mode must be set (letsencrypt|selfsigned|custom)"`
  3. If `tls.mode == letsencrypt`: `ingress.tls.acme.email` is non-empty → fail: `"ingress.tls.acme.email required for letsencrypt mode"`
  4. If `tls.mode == custom`: `ingress.tls.secretName` is non-empty → fail: `"ingress.tls.secretName required for custom mode"`
- **D-11:** No default TLS mode — omitting `ingress.tls.mode` always fails at render time with a clear error. Explicit choice required.

### clay.tlsSecretName Helper
- **D-12:** Add `clay.tlsSecretName` to `_helpers.tpl`. Returns:
  - `custom` mode → `.Values.ingress.tls.secretName`
  - `letsencrypt` and `selfsigned` modes → `{{ include "clay.fullname" . }}-tls` (derived from release name)
- **D-13:** Both the Ingress `spec.tls[0].secretName` and (in Phase 4) the Certificate `spec.secretName` use this helper — one definition prevents name mismatch.

### Claude's Discretion
- Exact Helm template conditionals and whitespace formatting
- Whether `clay.validateIngress` is a named template called via `include` or inlined at the top of `ingress.yaml`
- Comment style in `values.yaml` for the migration note

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements
- `.planning/REQUIREMENTS.md` — Phase 3 requirements: INGR-01, INGR-02, INGR-03, INGR-04, TLS-03; success criteria are the authoritative test for correctness

### Chart files (current state — read before modifying)
- `chart/clay/values.yaml` — Current ingress block shape (multi-host array); full replacement target
- `chart/clay/templates/ingress.yaml` — Current Ingress template; complete rewrite target
- `chart/clay/templates/_helpers.tpl` — Add `clay.validateIngress` and `clay.tlsSecretName` here; read existing `clay.validateSecrets` as the pattern to follow

### CI values files (existing — do not modify in Phase 3)
- `chart/clay/ci/managed-values.yaml` — Existing CI values for managed Postgres mode
- `chart/clay/ci/external-values.yaml` — Existing CI values for external Postgres mode
- *(TLS CI values files are Phase 5 — not Phase 3)*

### Phase 2 context (patterns to follow)
- `.planning/phases/02-helm-ci/02-CONTEXT.md` — `clay.validateSecrets` pattern, conditional rendering patterns, `clay.fullname` helper usage

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `clay.fullname` helper in `_helpers.tpl` — used to derive `clay.tlsSecretName` (`{{ include "clay.fullname" . }}-tls`)
- `clay.validateSecrets` in `_helpers.tpl` — exact pattern for `clay.validateIngress`; uses `{{- fail "message" }}` within a named template, called via `{{- include "clay.validateSecrets" . }}` at template top
- `chart/clay/ci/` directory — TLS CI values files (Phase 5) will live here alongside existing `managed-values.yaml` / `external-values.yaml`

### Established Patterns
- Conditional rendering: `{{- if .Values.ingress.className }}` wrapping `ingressClassName:` field — same pattern as current template; keep it
- `{{- with .Values.foo }}` / `{{- end }}` blocks for optional sections
- `{{- include "clay.fullname" $ }}` for cross-range scope access (if needed)

### Integration Points
- `chart/clay/templates/ingress.yaml` — calls `clay.validateIngress` and uses `clay.tlsSecretName`
- Phase 4 will add `templates/cert-manager-*.yaml` that also call `clay.tlsSecretName` — helper must be defined in Phase 3 so Phase 4 can use it without duplication
- `chart/clay/ci/` — Phase 5 adds three new values files here; Phase 3 must not break existing two

</code_context>

<specifics>
## Specific Ideas

- Migration comment in `values.yaml` should show the old shape (commented out) and the new shape side-by-side so operators can see exactly what changed
- `clay.validateIngress` validates all modes in Phase 3 (including letsencrypt email check) even though the cert-manager CRs come in Phase 4 — this way the helper is complete and Phase 4 doesn't need to touch `_helpers.tpl` for validation

</specifics>

<deferred>
## Deferred Ideas

- None — discussion stayed within phase scope

</deferred>

---

*Phase: 03-values-and-ingress-refactor*
*Context gathered: 2026-04-14*
