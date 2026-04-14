---
phase: 4
slug: cert-manager-cr-templates
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-14
---

# Phase 4 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | helm template / helm lint / bash |
| **Config file** | chart/clay/Chart.yaml |
| **Quick run command** | `helm template chart/clay --set ingress.enabled=true --set ingress.host=shop.example.com --set ingress.tls.mode=letsencrypt --set ingress.tls.acme.email=admin@example.com \| grep -c "kind:"` |
| **Full suite command** | `helm lint chart/clay -f chart/clay/ci/managed-values.yaml && helm lint chart/clay -f chart/clay/ci/external-values.yaml` |
| **Estimated runtime** | ~5 seconds |

---

## Sampling Rate

- **After every task commit:** Run `helm template chart/clay ... | grep "kind:"` per mode
- **After every plan wave:** Run `helm lint chart/clay -f chart/clay/ci/managed-values.yaml`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 10 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 4-01-01 | 01 | 1 | TLS-01 | — | N/A | render | `helm template chart/clay --set ingress.enabled=true --set ingress.host=shop.example.com --set ingress.tls.mode=letsencrypt --set ingress.tls.acme.email=admin@example.com \| grep "kind: ClusterIssuer"` | ✅ | ⬜ pending |
| 4-01-02 | 01 | 1 | TLS-01 | — | N/A | render | `helm template ... --set tls.mode=letsencrypt ... \| grep "kind: Certificate"` | ✅ | ⬜ pending |
| 4-01-03 | 01 | 1 | TLS-01 | — | N/A | render | `helm template ... --set tls.mode=letsencrypt ... \| grep "helm.sh/hook: post-install,post-upgrade"` | ✅ | ⬜ pending |
| 4-02-01 | 02 | 1 | TLS-02 | — | N/A | render | `helm template ... --set tls.mode=selfsigned ... \| grep "kind: ClusterIssuer" \| wc -l` (expect 2) | ✅ | ⬜ pending |
| 4-02-02 | 02 | 1 | TLS-02 | — | N/A | render | `helm template ... --set tls.mode=custom --set tls.secretName=my-tls ... \| grep -c "kind: ClusterIssuer"` (expect 0) | ✅ | ⬜ pending |
| 4-03-01 | 03 | 2 | CI-06 | — | N/A | file | `grep "cert-manager" .github/workflows/integration-test.yml` | ✅ | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

*Existing infrastructure covers all phase requirements.* (`helm template` and `helm lint` are pre-installed in CI and locally.)

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| cert-manager controller reconciles Certificate → TLS secret created | TLS-01 | Requires live cluster with cert-manager installed | Install cert-manager, deploy chart with letsencrypt mode, wait for Certificate Ready condition |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 10s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
