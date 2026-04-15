---
phase: quick
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - internal/handlers/health.go
  - internal/handlers/health_test.go
  - cmd/server/main.go
  - chart/clay/values.yaml
  - chart/clay/values.schema.json
  - chart/clay/templates/deployment.yaml
  - k8s/deployment.yaml
autonomous: true
must_haves:
  truths:
    - "GET /healthz returns 200 with {\"status\":\"ok\"} and no DB call"
    - "GET /readyz returns 200 with {\"status\":\"ready\"} when DB is reachable"
    - "GET /readyz returns 503 with {\"status\":\"not ready\"} when DB is unreachable"
    - "Helm chart uses /healthz for liveness, /readyz for readiness, and has startupProbe"
    - "k8s/deployment.yaml probes match the new endpoints"
  artifacts:
    - path: "internal/handlers/health.go"
      provides: "Health check HTTP handlers"
      exports: ["Healthz", "Readyz"]
    - path: "internal/handlers/health_test.go"
      provides: "Unit tests for health endpoints"
    - path: "chart/clay/values.yaml"
      provides: "Updated probe paths and startupProbe config"
      contains: "/healthz"
    - path: "chart/clay/templates/deployment.yaml"
      provides: "Templated startupProbe block"
      contains: "startupProbe"
  key_links:
    - from: "cmd/server/main.go"
      to: "internal/handlers/health.go"
      via: "route registration on mux"
      pattern: "mux.HandleFunc.*/healthz"
    - from: "internal/handlers/health.go"
      to: "database/sql.DB"
      via: "db.PingContext in Readyz handler"
      pattern: "PingContext"
---

<objective>
Add dedicated /healthz (liveness) and /readyz (readiness) health check endpoints to the Go
application, then update both the Helm chart and raw k8s manifests to use them for probes.

Purpose: Current probes hit `/` which renders the full home page with DB queries -- wasteful
and unable to distinguish "server alive" from "DB reachable". Dedicated endpoints fix both.

Output: health.go handler file, tests, updated probe configs across Helm and k8s manifests.
</objective>

<execution_context>
@/shared/pottery-shop/.claude/get-shit-done/workflows/execute-plan.md
@/shared/pottery-shop/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@cmd/server/main.go
@internal/handlers/public.go
@chart/clay/values.yaml
@chart/clay/values.schema.json
@chart/clay/templates/deployment.yaml
@k8s/deployment.yaml
</context>

<tasks>

<task type="auto" tdd="true">
  <name>Task 1: Create health check handlers with tests</name>
  <files>internal/handlers/health.go, internal/handlers/health_test.go, cmd/server/main.go</files>
  <behavior>
    - Test: GET /healthz returns 200 with Content-Type application/json and body {"status":"ok"}
    - Test: GET /readyz returns 200 with Content-Type application/json and body {"status":"ready"} when db.PingContext succeeds
    - Test: GET /readyz returns 503 with Content-Type application/json and body {"status":"not ready"} when db.PingContext fails
  </behavior>
  <action>
Create internal/handlers/health.go with two exported functions:

1. `Healthz(w http.ResponseWriter, r *http.Request)` -- a standalone function (not a method on
   any handler struct). Sets Content-Type to application/json, writes 200 with `{"status":"ok"}`.
   No DB interaction whatsoever.

2. `ReadyzHandler(db *sql.DB) http.HandlerFunc` -- a closure factory that captures `*sql.DB`.
   Calls `db.PingContext(r.Context())`. On success: 200 + `{"status":"ready"}`. On error:
   503 + `{"status":"not ready"}`. Always sets Content-Type application/json.

Use `encoding/json` to marshal response bodies (or just write the static strings directly --
they are simple enough). Import `database/sql` for the DB type.

Create internal/handlers/health_test.go with tests covering the three behaviors above. For
the Readyz failure case, use `sqlmock` or simply pass a closed `*sql.DB` so PingContext fails.
Prefer the simplest approach: open a sql.DB with a bogus DSN or call db.Close() before ping.

Register routes in cmd/server/main.go. Add these two lines right after the static file handlers
(line ~112) and BEFORE the `/` catch-all route (line 115), since Go's ServeMux matches `/`
as a catch-all:

    mux.HandleFunc("/healthz", handlers.Healthz)
    mux.HandleFunc("/readyz", handlers.ReadyzHandler(db))

The `db` variable (stdlib *sql.DB from stdlib.OpenDBFromPool) is already available at that
scope. These routes must NOT be wrapped in session middleware -- but since they are registered
on `mux` directly and `mux` is then wrapped, they will go through session middleware. That is
acceptable because session middleware is lightweight (just cookie read). They must NOT go
through BasicAuth.
  </action>
  <verify>
    <automated>cd /Users/xavpaice/shared/pottery-shop && go test ./internal/handlers/ -run TestHealth -v</automated>
  </verify>
  <done>All three health test cases pass. /healthz and /readyz routes registered in main.go before the / catch-all.</done>
</task>

<task type="auto">
  <name>Task 2: Update Helm chart and k8s manifests for health probes</name>
  <files>chart/clay/values.yaml, chart/clay/values.schema.json, chart/clay/templates/deployment.yaml, k8s/deployment.yaml</files>
  <action>
**chart/clay/values.yaml** -- Update the three probe sections:

```yaml
readinessProbe:
  httpGet:
    path: /readyz
    port: 8080
  initialDelaySeconds: 2
  periodSeconds: 10

livenessProbe:
  httpGet:
    path: /healthz
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 30

startupProbe:
  httpGet:
    path: /healthz
    port: 8080
  failureThreshold: 30
  periodSeconds: 2
```

The startupProbe allows up to 60s for the app to start (30 * 2s). During startup, liveness
and readiness probes are suspended by kubelet, preventing premature restarts during migration
runs.

**chart/clay/values.schema.json** -- Add a `startupProbe` schema entry as a sibling of
`readinessProbe` and `livenessProbe`. Use the same shape:

```json
"startupProbe": {
  "type": "object",
  "properties": {
    "httpGet": {
      "type": "object",
      "properties": {
        "path": { "type": "string" },
        "port": { "type": "integer" }
      }
    },
    "failureThreshold": { "type": "integer", "minimum": 1 },
    "periodSeconds": { "type": "integer", "minimum": 1 }
  }
}
```

**chart/clay/templates/deployment.yaml** -- Add the startupProbe block right after the
livenessProbe block (after line 74):

```yaml
          {{- with .Values.startupProbe }}
          startupProbe:
            {{- toYaml . | nindent 12 }}
          {{- end }}
```

**k8s/deployment.yaml** -- Update the existing probes and add startupProbe to match:

```yaml
          readinessProbe:
            httpGet:
              path: /readyz
              port: 8080
            initialDelaySeconds: 2
            periodSeconds: 10
          livenessProbe:
            httpGet:
              path: /healthz
              port: 8080
            initialDelaySeconds: 5
            periodSeconds: 30
          startupProbe:
            httpGet:
              path: /healthz
              port: 8080
            failureThreshold: 30
            periodSeconds: 2
```
  </action>
  <verify>
    <automated>cd /Users/xavpaice/shared/pottery-shop && helm template test chart/clay/ --set secrets.ADMIN_PASS=x --set secrets.SESSION_SECRET=x 2>&1 | grep -A3 -E '(readinessProbe|livenessProbe|startupProbe):' | head -20</automated>
  </verify>
  <done>Helm template renders all three probes with /healthz, /readyz, /healthz paths respectively. startupProbe present in rendered output. k8s/deployment.yaml has matching probe config. values.schema.json validates without error.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| client -> /healthz | Unauthenticated, returns static response |
| client -> /readyz | Unauthenticated, triggers DB ping |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-health-01 | D (Denial of Service) | /readyz | accept | DB ping is lightweight (no query); rate-limiting is out of scope for internal k8s probes. In production these endpoints are not exposed via ingress. |
| T-health-02 | I (Information Disclosure) | /healthz, /readyz | mitigate | Response bodies contain only "ok"/"ready"/"not ready" -- no version info, no error details, no stack traces. Readyz must NOT include the PingContext error message in the response body. |
</threat_model>

<verification>
1. `go test ./internal/handlers/ -run TestHealth -v` -- all health tests pass
2. `go build ./cmd/server/` -- binary compiles cleanly
3. `helm template test chart/clay/ --set secrets.ADMIN_PASS=x --set secrets.SESSION_SECRET=x` -- renders without error, contains all three probes with correct paths
4. `grep -c startupProbe k8s/deployment.yaml` -- returns 1
</verification>

<success_criteria>
- /healthz returns 200 {"status":"ok"} with no DB dependency
- /readyz returns 200 {"status":"ready"} when DB reachable, 503 {"status":"not ready"} when not
- Helm chart liveness uses /healthz, readiness uses /readyz, startupProbe uses /healthz
- k8s/deployment.yaml probes match Helm defaults
- All tests pass, binary compiles
</success_criteria>

<output>
After completion, create `.planning/quick/260415-ixx-implement-healthz-and-readyz-health-chec/260415-ixx-SUMMARY.md`
</output>
