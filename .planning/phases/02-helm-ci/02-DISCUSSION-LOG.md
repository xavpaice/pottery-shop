# Phase 2: Helm + CI - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions captured in CONTEXT.md — this log preserves the discussion.

**Date:** 2026-04-14
**Phase:** 02-helm-ci
**Mode:** discuss
**Areas analyzed:** CNPG Secret timing, Deployment strategy, CI test job structure, Helm template validation

## Assumptions Presented

### CNPG Secret Timing (HELM-06)
| Assumption | Confidence | Evidence |
|------------|-----------|----------|
| Init container pg_isready is the right mitigation | Confident | Phase 1 CONTEXT.md flagged this as open; pg_isready avoids RBAC complexity |
| postgres:16-alpine image for init container | Confident | Standard image with pg_isready built in; small footprint |
| Init container only in managed mode | Confident | External DSN mode has no CNPG race condition |

### Deployment Strategy
| Assumption | Confident | Evidence |
|------------|-----------|----------|
| Recreate → RollingUpdate | Confident | deployment.yaml comment "SQLite requires single-writer" — constraint removed with Postgres |

### CI Test Job Structure
| Assumption | Confidence | Evidence |
|------------|-----------|----------|
| Single job update (not split) | Confident | test.yml has one test job; removing gcc + adding CGO_ENABLED=0 is minimal change |
| Docker available on ubuntu-latest | Confident | GitHub Actions ubuntu-latest has Docker pre-installed; testcontainers standard approach |
| CMX test left as-is | Confident | Complex infra; CI-01/02/03 requirements don't mention CMX |

### Helm Template Validation
| Assumption | Confidence | Evidence |
|------------|-----------|----------|
| Validate both modes | Confident | Two conditional code paths (managed vs external) — both need rendering validated |
| chart/clay/ci/ for test values | Confident | Helm chart-testing convention; discoverable location |

## Corrections Made

No corrections — all areas confirmed by user on first pass.

## Discussions

### CNPG timing
- Q: "How should the app pod wait for the CNPG-generated Secret?" → User selected: "Init container with pg_isready (Recommended)"
- Q: "What image for the init container?" → User selected: "postgres:16-alpine (Recommended)"

### Deployment strategy
- Q: "What deployment strategy after SQLite removal?" → User selected: "RollingUpdate (Recommended)"

### CI test job structure
- Q: "How to restructure test job?" → User selected: "Single job — update existing test job (Recommended)"
- Q: "What to do with CMX integration test?" → User selected: "Leave it as-is for now (Recommended)"

### Helm template validation
- Q: "Validate both modes or just one?" → User selected: "Validate both modes (Recommended)"
- Q: "Where should test values files live?" → User selected: "chart/clay/ci/ directory (Recommended)"
