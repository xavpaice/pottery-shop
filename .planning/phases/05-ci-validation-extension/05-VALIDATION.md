---
phase: 5
slug: ci-validation-extension
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-14
---

# Phase 5 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | helm lint + helm template (shell) |
| **Config file** | none — CI values files created in Wave 1 |
| **Quick run command** | `helm lint chart/clay --values chart/clay/ci/tls-letsencrypt-values.yaml` |
| **Full suite command** | `chart/tests/helm-template-test.sh` |
| **Estimated runtime** | ~10 seconds |

---

## Sampling Rate

- **After every task commit:** Run `helm lint chart/clay --values chart/clay/ci/tls-letsencrypt-values.yaml`
- **After every plan wave:** Run `chart/tests/helm-template-test.sh`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 10 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 5-01-01 | 01 | 1 | CI-04 | — | N/A | helm lint | `helm lint chart/clay --values chart/clay/ci/tls-letsencrypt-values.yaml` | ❌ W0 | ⬜ pending |
| 5-01-02 | 01 | 1 | CI-04 | — | N/A | helm lint | `helm lint chart/clay --values chart/clay/ci/tls-selfsigned-values.yaml` | ❌ W0 | ⬜ pending |
| 5-01-03 | 01 | 1 | CI-04 | — | N/A | helm lint | `helm lint chart/clay --values chart/clay/ci/tls-custom-values.yaml` | ❌ W0 | ⬜ pending |
| 5-02-01 | 02 | 2 | CI-05 | — | N/A | CI yaml check | `yq '.jobs.helm-lint.steps' .github/workflows/test.yml` | ✅ | ⬜ pending |
| 5-02-02 | 02 | 2 | CI-05 | — | N/A | behavioral | `chart/tests/helm-template-test.sh` | ✅ | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `chart/clay/ci/tls-letsencrypt-values.yaml` — CI values for Let's Encrypt mode (CI-04)
- [ ] `chart/clay/ci/tls-selfsigned-values.yaml` — CI values for self-signed mode (CI-04)
- [ ] `chart/clay/ci/tls-custom-values.yaml` — CI values for custom cert mode (CI-04)

*Wave 0 creates the CI values files before test.yml steps are added.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| GitHub Actions workflow runs all 6 lint+template steps on push | CI-05 | Requires live GitHub push to verify | Push to branch, observe Actions tab — all 6 steps must pass |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 10s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
