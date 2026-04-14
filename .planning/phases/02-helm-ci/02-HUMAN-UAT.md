---
status: passed
phase: 02-helm-ci
source: [02-VERIFICATION.md]
started: 2026-04-14T00:00:00Z
updated: 2026-04-14T02:48:00Z
---

## Current Test

Completed — all tests passed via CMX k3s 1.35 cluster (cluster ID: bb36b87e).

## Tests

### 1. App pod reaches Running on helm install (Managed CNPG mode)
expected: Install chart on a cluster with CNPG CRDs installed; verify pod transitions through `Init:0/1` to `Running` and readiness probe passes at `/`. No `CreateContainerConfigError`, CNPG secret referenced correctly, pg_isready init container completes.
result: PASSED — pod clay-55b5858655-xl27l reached 1/1 Running. CNPG Cluster `clay-postgres` reached healthy state. Secret `clay-postgres-app` created with `uri` key. Init container `wait-for-postgres` completed (pg_isready on clay-postgres-rw). `DATABASE_URL` injected via `secretKeyRef: clay-postgres-app/uri`. Readiness probe: Ready=True. No DB_PATH in env.

### 2. App pod reaches Running on helm install (External DSN mode)
expected: Install with `postgres.managed=false` and a real DSN; verify `DATABASE_URL` is injected at runtime and no CNPG Cluster resource is created. Pod starts without init container; env var populated from `postgres.external.dsn`.
result: PASSED — pod clay-86ddf79dcf-qtt2m reached 1/1 Running. DATABASE_URL injected as plain value from postgres.external.dsn (confirmed contains DSN host). No CNPG Cluster resource created in clay namespace. No init containers on pod spec.

## Summary

total: 2
passed: 2
issues: 0
pending: 0
skipped: 0
blocked: 0

## Gaps
