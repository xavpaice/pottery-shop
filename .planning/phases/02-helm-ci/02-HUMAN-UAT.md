---
status: partial
phase: 02-helm-ci
source: [02-VERIFICATION.md]
started: 2026-04-14T00:00:00Z
updated: 2026-04-14T00:00:00Z
---

## Current Test

[awaiting human testing — requires live Kubernetes cluster with CNPG operator]

## Tests

### 1. App pod reaches Running on helm install (Managed CNPG mode)
expected: Install chart on a cluster with CNPG CRDs installed; verify pod transitions through `Init:0/1` to `Running` and readiness probe passes at `/`. No `CreateContainerConfigError`, CNPG secret referenced correctly, pg_isready init container completes.
result: [pending]

### 2. App pod reaches Running on helm install (External DSN mode)
expected: Install with `postgres.managed=false` and a real DSN; verify `DATABASE_URL` is injected at runtime and no CNPG Cluster resource is created. Pod starts without init container; env var populated from `postgres.external.dsn`.
result: [pending]

## Summary

total: 2
passed: 0
issues: 0
pending: 2
skipped: 0
blocked: 0

## Gaps
