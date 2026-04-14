# Phase 3: Values and Ingress Refactor - Research

**Researched:** 2026-04-14
**Domain:** Helm chart templating — values.yaml refactor, Ingress template rewrite, _helpers.tpl validation helpers
**Confidence:** HIGH

## Summary

This phase is a Helm chart authoring task with all major decisions pre-locked in CONTEXT.md. The work is entirely within `chart/clay/`: replace the multi-host array ingress shape in `values.yaml`, rewrite `templates/ingress.yaml` from a range-over-hosts loop to a single-host pattern, and add two new helpers (`clay.validateIngress`, `clay.tlsSecretName`) to `_helpers.tpl`. No cert-manager CRDs are created in this phase.

The codebase already contains a working validation helper (`clay.validateSecrets`) that is the exact pattern to follow for `clay.validateIngress`. The call pattern — `{{- include "clay.validateSecrets" . }}` at the top of `deployment.yaml` — works because the helper outputs an empty string on success, and Helm suppresses empty output from named templates. The `fail` function causes Helm to abort with the supplied message.

The existing CI values files (`ci/managed-values.yaml`, `ci/external-values.yaml`) must not be modified. After this phase, both must still pass `helm lint` and `helm template` without setting `ingress.enabled: true`, because `ingress.enabled` defaults to `false` and no host/mode is required when the ingress is disabled.

**Primary recommendation:** Follow the `clay.validateSecrets` pattern precisely — named template in `_helpers.tpl`, invoked with `{{- include "clay.validateIngress" . }}` at the top of `ingress.yaml` inside the `{{- if .Values.ingress.enabled }}` guard.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- **D-01:** Hard break — remove old `ingress.hosts[]`, `ingress.tls[]` (array), `ingress.className`, and `ingress.annotations` keys entirely from `values.yaml`.
- **D-02:** Add a comment block in `values.yaml` near the new ingress section documenting the shape change (old vs. new), so operators can see what to migrate. No dual-path template support.
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
- **D-05:** `ingress.className` in `values.yaml` with default `traefik`. Template renders `ingressClassName: {{ .Values.ingress.className }}` only when value is non-empty.
- **D-06:** Template renders a single `rules` entry from `ingress.host` (not a range over an array).
- **D-07:** Traefik annotations hardcoded in template metadata (not in `values.yaml`), applied whenever `ingress.enabled: true`:
  - `traefik.ingress.kubernetes.io/router.entrypoints: websecure`
  - `acme.cert-manager.io/http01-edit-in-place: "true"`
- **D-08:** TLS block in spec always references the secret name from the `clay.tlsSecretName` helper. Present for all three modes.
- **D-09:** `nginx.ingress.kubernetes.io/proxy-body-size` annotation removed — not present in new defaults.
- **D-10:** `clay.validateIngress` validates:
  1. `ingress.host` is non-empty → `"ingress.host must be set"`
  2. `ingress.tls.mode` is non-empty → `"ingress.tls.mode must be set (letsencrypt|selfsigned|custom)"`
  3. If `tls.mode == letsencrypt`: `ingress.tls.acme.email` non-empty → `"ingress.tls.acme.email required for letsencrypt mode"`
  4. If `tls.mode == custom`: `ingress.tls.secretName` non-empty → `"ingress.tls.secretName required for custom mode"`
- **D-11:** No default TLS mode — omitting always fails at render time.
- **D-12:** `clay.tlsSecretName` returns:
  - `custom` mode → `.Values.ingress.tls.secretName`
  - `letsencrypt` and `selfsigned` → `{{ include "clay.fullname" . }}-tls`
- **D-13:** Both Ingress `spec.tls[0].secretName` and (Phase 4) Certificate `spec.secretName` use this helper.

### Claude's Discretion

- Exact Helm template conditionals and whitespace formatting
- Whether `clay.validateIngress` is a named template called via `include` or inlined at the top of `ingress.yaml`
- Comment style in `values.yaml` for the migration note

### Deferred Ideas (OUT OF SCOPE)

- None — discussion stayed within phase scope
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| INGR-01 | User can expose the app via a Kubernetes Ingress — `ingressClassName: traefik`, `ingress.enabled` gate, `ingress.host` scalar in values.yaml | D-03 through D-06 define exact values shape and template behavior |
| INGR-02 | Ingress resource carries Traefik-specific annotations — `traefik.ingress.kubernetes.io/router.entrypoints: websecure` and `acme.cert-manager.io/http01-edit-in-place: "true"` | D-07 hardcodes these in template metadata |
| INGR-03 | `clay.validateIngress` helper in `_helpers.tpl` fails fast at render time on missing required values | D-10, D-11 define exact validation checks; `clay.validateSecrets` is the pattern |
| INGR-04 | Nginx-specific annotation (`nginx.ingress.kubernetes.io/proxy-body-size`) removed from Ingress defaults | D-09 removes it; confirmed in current values.yaml line 22 |
| TLS-03 | User can enable custom mode — chart references user-provided TLS Secret by name; no cert-manager resources created | D-08, D-12 define `clay.tlsSecretName` behavior |
</phase_requirements>

## Standard Stack

### Core

| Tool | Version | Purpose | Why Standard |
|------|---------|---------|--------------|
| Helm | 3.x (CI uses `azure/setup-helm@v4`) | Chart rendering, linting, templating | Project standard — all chart work uses Helm 3 [VERIFIED: .github/workflows/test.yml] |
| Kubernetes networking.k8s.io/v1 | v1 (GA since 1.19) | Ingress apiVersion | Stable API; project targets Kubernetes 1.35.0+ [VERIFIED: CLAUDE.md] |
| Traefik | k3s bundled | Ingress controller | Project deploys to k3s; Traefik is k3s default [VERIFIED: STATE.md, CONTEXT.md] |

### No External Dependencies

This phase is entirely Helm chart authoring — no new Go packages, no new Kubernetes operators, no new Helm subcharts. All work is in:
- `chart/clay/values.yaml`
- `chart/clay/templates/ingress.yaml`
- `chart/clay/templates/_helpers.tpl`

## Architecture Patterns

### Recommended Project Structure

No structural changes. Files modified in place:

```
chart/clay/
├── values.yaml              # ingress block replaced entirely
├── templates/
│   ├── _helpers.tpl         # two new helpers added
│   └── ingress.yaml         # complete rewrite
└── ci/
    ├── managed-values.yaml  # DO NOT MODIFY
    └── external-values.yaml # DO NOT MODIFY
```

### Pattern 1: Named Template Validation Helper (clay.validateSecrets pattern)

**What:** A named template in `_helpers.tpl` that uses `{{- fail "message" }}` conditionally. Called at the top of the template that needs validation via `{{- include "clay.validateIngress" . }}`. Returns empty string on success — Helm suppresses empty output.

**When to use:** Any time a template needs values that must be non-empty before rendering a Kubernetes resource.

**Existing example (verbatim from `_helpers.tpl` lines 54-61):**
```
// Source: chart/clay/templates/_helpers.tpl
{{- define "clay.validateSecrets" -}}
{{- if not .Values.secrets.ADMIN_PASS }}
  {{- fail "secrets.ADMIN_PASS must be set" }}
{{- end }}
{{- if not .Values.secrets.SESSION_SECRET }}
  {{- fail "secrets.SESSION_SECRET must be set" }}
{{- end }}
{{- end }}
```

**How it is called (verbatim from `deployment.yaml` line 1):**
```
{{- include "clay.validateSecrets" . }}
```

**clay.validateIngress will follow the same structure:** same `{{- define ... -}}` / `{{- end }}` wrapper, same `{{- if not .Values.X }}` / `{{- fail "..." }}` checks, same `{{- include "clay.validateIngress" . }}` call at the top of `ingress.yaml` (inside the `{{- if .Values.ingress.enabled }}` guard).

**Recommendation for "Claude's Discretion" item:** Use named template (`{{- include "clay.validateIngress" . }}`), not inline. Reasons:
1. Phase 4 adds cert-manager templates that may benefit from calling the same validation
2. Consistent with the only other validation pattern in the codebase (`clay.validateSecrets`)
3. Named templates are independently testable via `helm template --show-only`

### Pattern 2: clay.tlsSecretName Helper

**What:** A named template that returns a string — the TLS secret name derived from either `ingress.tls.secretName` (custom mode) or `{{ include "clay.fullname" . }}-tls` (letsencrypt/selfsigned).

**Key technique:** Named templates that return a value use the `{{- define "name" -}}` / `{{- end }}` trimming convention. The caller uses `{{ include "clay.tlsSecretName" . }}` inline in the template field where the value is needed.

**Example pattern (string-returning helper):**
```
// Pattern: string-returning named template
{{- define "clay.tlsSecretName" -}}
{{- if eq .Values.ingress.tls.mode "custom" -}}
  {{- .Values.ingress.tls.secretName -}}
{{- else -}}
  {{- printf "%s-tls" (include "clay.fullname" .) -}}
{{- end -}}
{{- end }}
```

**Usage in ingress.yaml spec.tls block:**
```yaml
  tls:
    - secretName: {{ include "clay.tlsSecretName" . }}
      hosts:
        - {{ .Values.ingress.host | quote }}
```

**Important:** Both `-` trimming on `define` and inside the `if`/`else`/`end` blocks are needed to avoid injecting unexpected whitespace when this value is inlined into YAML. [VERIFIED: helm.sh named templates docs — `include` preferred over `template` for pipeline compatibility; whitespace trimming via `-`]

### Pattern 3: Single-Host Ingress Template Structure

**What:** Replaces the `{{- range .Values.ingress.hosts }}` loop with a direct reference to `ingress.host` scalar.

**Current shape (multi-host range — lines 17-29 of ingress.yaml):**
```
{{- range .Values.ingress.hosts }}
- host: {{ .host | quote }}
  http:
    paths:
      {{- range .paths }}
      - path: {{ .path }}
        pathType: {{ .pathType }}
        backend:
          service:
            name: {{ include "clay.fullname" $ }}
            port:
              number: {{ $.Values.service.port }}
      {{- end }}
{{- end }}
```

**New shape (single host — no range):**
```
  rules:
    - host: {{ .Values.ingress.host | quote }}
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: {{ include "clay.fullname" . }}
                port:
                  number: {{ .Values.service.port }}
```

Note: With single-host, no `$` root-scope workaround is needed — `include "clay.fullname" .` works directly (no range scope to escape).

### Pattern 4: Hardcoded Annotations in Template Metadata

**What:** D-07 requires Traefik annotations always present when `ingress.enabled: true`. These are hardcoded in the template, not sourced from `values.yaml`.

**Shape:**
```yaml
metadata:
  name: {{ include "clay.fullname" . }}
  labels:
    {{- include "clay.labels" . | nindent 4 }}
  annotations:
    traefik.ingress.kubernetes.io/router.entrypoints: websecure
    acme.cert-manager.io/http01-edit-in-place: "true"
```

**Why hardcoded vs. values.yaml:** D-07 (locked). The old template used `{{- with .Values.ingress.annotations }}` / `{{- toYaml . | nindent 4 }}` — that pattern is removed. [VERIFIED: CONTEXT.md D-07, current ingress.yaml lines 8-11]

### Pattern 5: ingressClassName Conditional

**Current pattern (ingress.yaml lines 13-15) — KEEP:**
```
{{- if .Values.ingress.className }}
ingressClassName: {{ .Values.ingress.className }}
{{- end }}
```

This pattern is explicitly called out in CONTEXT.md as an established pattern to preserve (Existing Code Insights section). With the new default `className: traefik` in values.yaml, this field will render for all default deployments unless the operator explicitly sets `className: ""`.

### Anti-Patterns to Avoid

- **Do not use `{{- with .Values.ingress.annotations }}`:** The old template used this for a values-driven annotations block. D-07 removes this entirely — annotations are now hardcoded.
- **Do not use `{{- toYaml .Values.ingress.tls | nindent 4 }}`:** The old template dumped the entire `tls` array as YAML. The new shape constructs the tls block from individual scalar values.
- **Do not validate outside `{{- if .Values.ingress.enabled }}`:** `clay.validateIngress` must only fire when `ingress.enabled: true`. The existing CI values files do not set ingress fields, so validation must be gated on `enabled`.
- **Do not leave `ingress.enabled: true` in default values.yaml:** D-04 locks `enabled: false`. The old values.yaml has `enabled: true` (line 19) — this must be flipped.
- **Do not modify ci/managed-values.yaml or ci/external-values.yaml:** These are canonical CI fixtures. Both currently pass with `ingress.enabled` absent (defaults to false). After this phase they must still pass.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| TLS secret name derivation | Custom concatenation per-template | `clay.tlsSecretName` helper | D-13 explicitly states both Ingress and Phase 4 Certificate must use same helper to prevent name mismatch |
| Required-value enforcement | Runtime panic or silent misconfiguration | `{{- fail "message" }}` in named template | Helm's `fail` aborts at render time with user-visible error — no runtime surprises |
| Ingress host value | `range` loop over single-element arrays | `.Values.ingress.host` scalar | D-06 locked; single scalar eliminates scope escape complexity |

**Key insight:** Helm's `fail` function plus a named template is the correct tool for chart-level validation. The pattern is already proven in this codebase via `clay.validateSecrets`. No custom tooling needed.

## Common Pitfalls

### Pitfall 1: Validation Fires When Ingress Is Disabled

**What goes wrong:** `clay.validateIngress` checks for `ingress.host` and `ingress.tls.mode` — but these fields are empty by default (D-03). If the `include` call is placed outside the `{{- if .Values.ingress.enabled }}` guard, every `helm template` invocation (including CI runs with managed-values.yaml) fails with "ingress.host must be set".

**Why it happens:** The existing CI values files do not set `ingress.enabled`, so it defaults to `false`. Validation must be conditional on `enabled`.

**How to avoid:** Call `{{- include "clay.validateIngress" . }}` inside the `{{- if .Values.ingress.enabled }}` block — at the top, before any YAML is emitted.

**Warning signs:** `helm lint chart/clay/ --values chart/clay/ci/managed-values.yaml` fails with ingress.host error.

### Pitfall 2: String-Returning Helper Whitespace Contamination

**What goes wrong:** `clay.tlsSecretName` is used inline in a YAML field: `secretName: {{ include "clay.tlsSecretName" . }}`. If the helper does not use `{{- ... -}}` trimming consistently, leading/trailing newlines or spaces corrupt the YAML scalar value.

**Why it happens:** Helm template rendering preserves all whitespace, including newlines from `{{- define "..." -}}` blocks without `-` trimming.

**How to avoid:** Use `{{- define "clay.tlsSecretName" -}}` opening and `{{- end }}` closing. Use `-}}` on all internal `{{- if ... -}}`, `{{- else -}}`, `{{- end -}}` directives inside string-returning helpers.

**Warning signs:** `helm template` output shows `secretName: ` with extra whitespace or a blank line after it; `kubectl apply --dry-run` rejects the rendered YAML.

### Pitfall 3: `eq` Comparison with Empty String

**What goes wrong:** `{{- if eq .Values.ingress.tls.mode "letsencrypt" }}` when `mode` is unset (empty string `""`) — Go template `eq` with an empty string is `false`, which is correct, but `{{- if not .Values.ingress.tls.mode }}` is what catches the empty-string case.

**Why it happens:** Both checks are needed — first validate `mode` is non-empty, then branch on its value. Mixing them into one `if eq` misses the empty-string case.

**How to avoid:** In `clay.validateIngress`, check `{{- if not .Values.ingress.tls.mode }}` first (fail if empty), then use `{{- if eq .Values.ingress.tls.mode "letsencrypt" }}` for mode-specific checks. The ordering matters.

**Warning signs:** `helm template ... --set ingress.enabled=true --set ingress.host=shop.example.com` with no `tls.mode` set succeeds silently (renders without TLS) instead of failing.

### Pitfall 4: Old Values Keys Leaking Into Migration Comment

**What goes wrong:** D-02 requires a migration comment showing old vs. new shape. If the old keys are left as uncommented YAML (not as YAML comments), Helm will parse them and the old shape persists alongside the new shape.

**Why it happens:** Copy-paste of old block without prepending `#` to each line.

**How to avoid:** Old shape in the migration comment must be YAML-commented (`# `) lines, not live YAML. New shape is live YAML.

### Pitfall 5: className Default Behavior Change

**What goes wrong:** Current values.yaml has `className: ""` (empty, line 20). New values.yaml has `className: traefik`. The conditional `{{- if .Values.ingress.className }}` will now ALWAYS render `ingressClassName: traefik` for any deployment using default values. This is intentional (D-05) but could be a surprise if an operator relied on the empty default.

**Why it happens:** The default changed from empty to `traefik`.

**How to avoid:** This is correct per D-05. The migration comment (D-02) should note this change. Not a bug, but worth calling out in the comment block so operators know to set `className: ""` if they need no ingressClassName.

## Code Examples

Verified patterns from existing codebase and official sources:

### clay.validateIngress — Full Helper

```
// Pattern source: chart/clay/templates/_helpers.tpl (clay.validateSecrets at lines 54-61)
// + CONTEXT.md D-10 for the specific validation checks
{{- define "clay.validateIngress" -}}
{{- if not .Values.ingress.host }}
  {{- fail "ingress.host must be set" }}
{{- end }}
{{- if not .Values.ingress.tls.mode }}
  {{- fail "ingress.tls.mode must be set (letsencrypt|selfsigned|custom)" }}
{{- end }}
{{- if eq .Values.ingress.tls.mode "letsencrypt" }}
  {{- if not .Values.ingress.tls.acme.email }}
    {{- fail "ingress.tls.acme.email required for letsencrypt mode" }}
  {{- end }}
{{- end }}
{{- if eq .Values.ingress.tls.mode "custom" }}
  {{- if not .Values.ingress.tls.secretName }}
    {{- fail "ingress.tls.secretName required for custom mode" }}
  {{- end }}
{{- end }}
{{- end }}
```

### clay.tlsSecretName — Full Helper

```
// Pattern source: CONTEXT.md D-12, clay.fullname usage in _helpers.tpl lines 11-22
{{- define "clay.tlsSecretName" -}}
{{- if eq .Values.ingress.tls.mode "custom" -}}
  {{- .Values.ingress.tls.secretName -}}
{{- else -}}
  {{- printf "%s-tls" (include "clay.fullname" .) -}}
{{- end -}}
{{- end }}
```

### ingress.yaml — Full Rewrite Structure

```
// Source: CONTEXT.md D-06 through D-08, current ingress.yaml structure [VERIFIED codebase]
{{- if .Values.ingress.enabled }}
{{- include "clay.validateIngress" . }}
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: {{ include "clay.fullname" . }}
  labels:
    {{- include "clay.labels" . | nindent 4 }}
  annotations:
    traefik.ingress.kubernetes.io/router.entrypoints: websecure
    acme.cert-manager.io/http01-edit-in-place: "true"
spec:
  {{- if .Values.ingress.className }}
  ingressClassName: {{ .Values.ingress.className }}
  {{- end }}
  rules:
    - host: {{ .Values.ingress.host | quote }}
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: {{ include "clay.fullname" . }}
                port:
                  number: {{ .Values.service.port }}
  tls:
    - secretName: {{ include "clay.tlsSecretName" . }}
      hosts:
        - {{ .Values.ingress.host | quote }}
{{- end }}
```

### values.yaml Migration Comment Block

```yaml
# -- Ingress configuration
#
# MIGRATION NOTE (v0.1.0 → v0.2.0): The ingress block shape has changed.
#
# OLD shape (removed):
#   ingress:
#     enabled: true
#     className: ""
#     annotations:
#       nginx.ingress.kubernetes.io/proxy-body-size: "50m"
#     hosts:
#       - host: clay.nz
#         paths:
#           - path: /
#             pathType: Prefix
#     tls:
#       - secretName: clay-tls
#         hosts:
#           - clay.nz
#
# NEW shape (below): single host scalar, mode-driven TLS, Traefik annotations hardcoded.
# Note: className now defaults to "traefik" (was ""). Set className: "" for other controllers.
#
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

### Helm Commands to Verify Success Criteria

```bash
# Success criterion 1: custom mode renders correctly
helm template chart/clay \
  --set ingress.enabled=true \
  --set ingress.host=shop.example.com \
  --set ingress.tls.mode=custom \
  --set ingress.tls.secretName=my-tls \
  --set secrets.ADMIN_PASS=x \
  --set secrets.SESSION_SECRET=x

# Success criterion 2: missing host fails
helm template chart/clay \
  --set ingress.enabled=true \
  --set secrets.ADMIN_PASS=x \
  --set secrets.SESSION_SECRET=x

# Success criterion 3: letsencrypt missing email fails
helm template chart/clay \
  --set ingress.enabled=true \
  --set ingress.tls.mode=letsencrypt \
  --set ingress.host=shop.example.com \
  --set secrets.ADMIN_PASS=x \
  --set secrets.SESSION_SECRET=x

# Success criterion 5: helm lint passes
helm lint chart/clay \
  --values chart/clay/ci/managed-values.yaml
helm lint chart/clay \
  --values chart/clay/ci/external-values.yaml
```

Note: `secrets.ADMIN_PASS` and `secrets.SESSION_SECRET` must be set in every `helm template` call because `clay.validateSecrets` is called unconditionally from `deployment.yaml`. Omitting them causes a different validation failure before ingress validation fires.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `networking.k8s.io/v1beta1` Ingress | `networking.k8s.io/v1` Ingress | Kubernetes 1.19 GA, removed 1.22 | Project targets 1.35.0+ — v1 is the only option [VERIFIED: CLAUDE.md] |
| `kubernetes.io/ingress.class` annotation | `spec.ingressClassName` field | Kubernetes 1.18 (field), annotation deprecated 1.22 | Template must use `ingressClassName:` field, not annotation [VERIFIED: current ingress.yaml uses field already] |
| Multi-host array ingress | Single-host scalar | This phase | Simpler template, no range loop, no `$` scope workaround needed |

**Deprecated/outdated:**
- `kubernetes.io/ingress.class` annotation: replaced by `spec.ingressClassName` — already using the correct field
- `nginx.ingress.kubernetes.io/proxy-body-size`: Nginx-specific, being removed per INGR-04

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `helm template` with `--set secrets.ADMIN_PASS=x` is required in all test invocations because `clay.validateSecrets` fires unconditionally | Code Examples (helm commands) | If wrong: test commands in PLAN miss the flag, CI verification steps fail with secrets error not ingress error |
| A2 | `acme.cert-manager.io/http01-edit-in-place: "true"` annotation has no effect without cert-manager installed, and Traefik ignores it — safe to add in Phase 3 before Phase 4 cert-manager integration | Code Examples (ingress.yaml) | If wrong: annotation causes unexpected behavior when cert-manager is absent — unlikely, annotations are metadata only |

**A1 is verified:** `deployment.yaml` line 1 calls `{{- include "clay.validateSecrets" . }}` unconditionally. `clay.validateSecrets` fails unless both secrets are set. [VERIFIED: codebase]

**A2 is LOW confidence** — annotation behavior when cert-manager is absent is not verified against official cert-manager docs, but Kubernetes spec defines that unknown annotations are ignored by the API server.

## Open Questions

1. **`selfsigned` mode validation**
   - What we know: D-10 does not list a `selfsigned`-specific validation check (only `letsencrypt` requires email, only `custom` requires secretName)
   - What's unclear: Should `clay.validateIngress` validate that `mode` is one of the three valid strings? (i.e., fail on `mode: typo`)
   - Recommendation: Add `{{- if and (ne .Values.ingress.tls.mode "letsencrypt") (ne .Values.ingress.tls.mode "selfsigned") (ne .Values.ingress.tls.mode "custom") }}` check. This is within "exact Helm template conditionals" which is Claude's Discretion. Recommend adding it — the success criteria test only valid modes, but an invalid mode should produce a clear error rather than silently rendering.

2. **`helm lint` behavior with missing required values**
   - What we know: `helm lint` runs with the CI values files which have `ingress.enabled` absent (defaults false) — validation does not fire.
   - What's unclear: Whether `helm lint chart/clay` (no values file) triggers validation failures and whether that counts as "fails lint" for the phase gate.
   - Recommendation: Phase 5 will add TLS CI values files. For now, `helm lint` without a values file will hit `clay.validateSecrets` before ingress validation. Success criterion 5 uses existing CI files — those will pass.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Helm | All helm lint/template validation | ✗ (not on dev machine) | — | CI (GitHub Actions, `azure/setup-helm@v4`) |
| kubectl | Kubernetes API validation | ✗ (not on dev machine) | — | `helm template` without `--validate` (CI pattern already established) |

**Missing dependencies with no fallback:**
- None blocking. Chart authoring (editing YAML/template files) requires no local Helm. Verification requires Helm, which is available in CI.

**Missing dependencies with fallback:**
- Helm on dev machine: validation via CI (`helm-lint` job in test.yml). Pattern already established from Phase 2.

**Note on "helm not found" locally:** The CI workflow installs Helm via `azure/setup-helm@v4`. All helm lint/template verification should be framed as CI verification steps, not local commands, unless the planner adds a local Helm install step.

## Security Domain

`security_enforcement` key is absent from config.json — treated as enabled.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | Not applicable — Ingress config only |
| V3 Session Management | no | Not applicable — Ingress config only |
| V4 Access Control | no | Not applicable — Ingress config only |
| V5 Input Validation | yes (chart layer) | `{{- fail "..." }}` in `clay.validateIngress` — validates required values at render time |
| V6 Cryptography | no | TLS secret contents are external; chart only references the secret name |

### Known Threat Patterns for This Stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Misconfigured TLS (empty secretName in custom mode) | Elevation of Privilege | `clay.validateIngress` fails fast at render time |
| Wrong ACME email (silent Let's Encrypt cert failures) | Denial of Service | `clay.validateIngress` requires email for letsencrypt mode |
| Nginx annotation on Traefik cluster (misconfiguration) | Tampering (wrong routing) | INGR-04 removes nginx annotation entirely; Traefik annotations hardcoded |

**Key point:** This phase's security posture is "fail fast at deploy time." No secrets are created or managed — `clay.tlsSecretName` references an existing secret by name in custom mode. The chart does not create TLS secrets in Phase 3.

## Sources

### Primary (HIGH confidence)
- `chart/clay/templates/_helpers.tpl` (codebase, lines 54-61) — `clay.validateSecrets` pattern: exact model for `clay.validateIngress`
- `chart/clay/templates/ingress.yaml` (codebase) — current template structure being replaced
- `chart/clay/values.yaml` (codebase) — current ingress block shape, confirmed all old keys present
- `chart/clay/templates/deployment.yaml` (codebase, line 1) — confirms `include` call pattern for validation helpers
- `.planning/phases/03-values-and-ingress-refactor/03-CONTEXT.md` — all locked decisions (D-01 through D-13)
- [Helm Named Templates docs](https://helm.sh/docs/chart_template_guide/named_templates/) — `define`/`include` syntax, whitespace trimming with `-`
- [Helm Flow Control docs](https://helm.sh/docs/chart_template_guide/control_structures/) — `if`/`eq` syntax for string comparison
- [Helm Function List docs](https://helm.sh/docs/chart_template_guide/function_list/) — `fail` function: "unconditionally returns empty string and error"

### Secondary (MEDIUM confidence)
- [Helm Chart Tips and Tricks](https://helm.sh/docs/howto/charts_tips_and_tricks/) — `required` and `fail` validation patterns confirmed
- `chart/clay/ci/managed-values.yaml`, `chart/clay/ci/external-values.yaml` (codebase) — confirmed neither sets `ingress.*` keys; both must pass after refactor

### Tertiary (LOW confidence)
- A2 assumption: cert-manager annotation is safe as metadata when cert-manager is absent — based on Kubernetes annotation semantics (ignored by API server), not verified against cert-manager docs

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — pure Helm chart authoring, no new packages, all patterns verified in codebase
- Architecture: HIGH — exact patterns extracted from live codebase files (`clay.validateSecrets`, `clay.fullname`)
- Pitfalls: HIGH — derived from locked decisions and codebase inspection; Pitfall 1 (validation gating) is directly verifiable from existing CI values files

**Research date:** 2026-04-14
**Valid until:** 2026-06-14 (Helm 3 template syntax is stable; no fast-moving dependencies)
