---
phase: 03
slug: values-and-ingress-refactor
status: complete
nyquist_compliant: true
wave_0_complete: true
created: 2026-04-14
---

# Phase 03 — Validation Strategy

> Per-phase validation contract. Phase 3 is a Helm chart-only change (zero Go source modifications).

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | helm CLI (template + lint) |
| **Config file** | `chart/tests/helm-template-test.sh` |
| **Quick run command** | `bash chart/tests/helm-template-test.sh` |
| **Full suite command** | `make lint` |
| **Estimated runtime** | ~5 seconds |

---

## Sampling Rate

- **After every task commit:** `bash chart/tests/helm-template-test.sh`
- **After every plan wave:** `make lint`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** ~5 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 03-01-G01 | 01 | 1 | INGR-01 | T-03-04 | ingressClassName: traefik renders in custom mode | smoke | `bash chart/tests/helm-template-test.sh` | ✅ | ✅ green |
| 03-01-G02a | 01 | 1 | INGR-02 | T-03-04 | Traefik `router.entrypoints: websecure` annotation renders (custom mode) | smoke | `bash chart/tests/helm-template-test.sh` | ✅ | ✅ green |
| 03-01-G02b | 01 | 1 | INGR-02 | T-03-04 | `acme.cert-manager.io/http01-edit-in-place: "true"` renders (letsencrypt mode) | smoke | `bash chart/tests/helm-template-test.sh` | ✅ | ✅ green |
| 03-01-G03 | 01 | 1 | INGR-03 | T-03-01 | Missing `ingress.host` fails with "ingress.host must be set" | smoke | `bash chart/tests/helm-template-test.sh` | ✅ | ✅ green |
| 03-01-G04 | 01 | 1 | INGR-03 | T-03-02 | Missing `acme.email` (letsencrypt) fails with correct error | smoke | `bash chart/tests/helm-template-test.sh` | ✅ | ✅ green |
| 03-01-G05 | 01 | 1 | INGR-03 | T-03-01 | Missing `secretName` (custom mode) fails with correct error | smoke | `bash chart/tests/helm-template-test.sh` | ✅ | ✅ green |
| 03-01-G06 | 01 | 1 | INGR-04 | T-03-04 | Zero `nginx` strings in rendered output (custom mode) | smoke | `bash chart/tests/helm-template-test.sh` | ✅ | ✅ green |
| 03-01-G07 | 01 | 1 | TLS-03 | T-03-01 | TLS block renders user-provided `secretName: my-tls` (custom mode) | smoke | `bash chart/tests/helm-template-test.sh` | ✅ | ✅ green |
| 03-01-G08a | 01 | 1 | SC-5 | — | `helm lint -f managed-values.yaml` exits 0 | smoke | `make lint` | ✅ | ✅ green |
| 03-01-G08b | 01 | 1 | SC-5 | — | `helm lint -f external-values.yaml` exits 0 | smoke | `make lint` | ✅ | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

Existing infrastructure covers all phase requirements.

`chart/tests/helm-template-test.sh` was created to fill all gaps (no Wave 0 install step needed — helm CLI was already available).

---

## Manual-Only Verifications

All phase behaviors have automated verification.

---

## Implementation Note (G-02)

The PLAN specified both Traefik annotations as "hardcoded." The implementation conditionalizes them:
- `traefik.ingress.kubernetes.io/router.entrypoints: websecure` — rendered when `className=traefik` (which is the default)
- `acme.cert-manager.io/http01-edit-in-place: "true"` — rendered when `tls.mode=letsencrypt`

Both conditions are satisfied at runtime by expected values. Tests G-02a and G-02b verify each annotation under the mode that produces it. Requirement INGR-02 is fully met.

---

## Validation Sign-Off

- [x] All tasks have automated verify commands
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] No Wave 0 stubs — all tests implemented and green
- [x] No watch-mode flags
- [x] Feedback latency < 5s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-04-14

---

## Validation Audit 2026-04-14

| Metric | Count |
|--------|-------|
| Gaps found | 8 (7 MISSING + 1 PARTIAL) |
| Resolved | 8 |
| Escalated | 0 |

## Validation Audit 2026-04-14 (re-run)

| Metric | Count |
|--------|-------|
| Gaps found | 0 |
| Resolved | 0 |
| Escalated | 0 |
| Tests run | 10 passed, 0 failed |
