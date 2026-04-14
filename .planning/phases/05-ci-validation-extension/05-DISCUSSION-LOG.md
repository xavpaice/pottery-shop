# Phase 5: CI Validation Extension - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions captured in CONTEXT.md — this log preserves the discussion.

**Date:** 2026-04-14
**Phase:** 05-ci-validation-extension
**Mode:** discuss
**Areas analyzed:** CI values file Postgres baseline, test script CI placement

## Gray Areas Presented

### CI Values File Postgres Baseline
| Area | Options | User Choice |
|------|---------|-------------|
| Postgres baseline in TLS CI files | External DSN (simpler, no CNPG resources) vs Managed mode (mirrors primary deployment) | **Managed mode** |

**Rationale:** User selected managed Postgres to mirror the primary deployment mode. TLS validation output will include CNPG Cluster alongside TLS resources — acceptable for CI.

### helm-template-test.sh CI Placement
| Area | Options | User Choice |
|------|---------|-------------|
| Where to invoke test script | Step in helm-lint job vs new helm-behavioral-tests job | **Step in helm-lint job** |

**Rationale:** Helm is already set up in helm-lint job — no extra runner cost. Keeps all Helm validation in one place.

## Corrections Made

No corrections — the two areas were the only genuine user-facing decisions. All other phase details (six-step structure, --validate-less helm template, secretName=my-tls for custom mode) were derived from existing patterns and ROADMAP success criteria.

## Prior Decisions Applied

From Phase 4 CONTEXT.md (D-16): "clay chart install step keeps `--set ingress.enabled=false` — TLS mode validation is handled by `helm template` in Phase 5" → Phase 5 CI values files enable ingress (they ARE the TLS validation mechanism).

From ROADMAP STATE.md: "helm template without --validate in CI — avoids cert-manager CRD absence failures" → `helm template` steps run without `--validate` flag.
