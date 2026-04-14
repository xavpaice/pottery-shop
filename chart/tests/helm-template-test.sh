#!/usr/bin/env sh
# Behavioral tests for chart/clay Helm template rendering (Phase 3: values-and-ingress-refactor)
# Requirements: INGR-01, INGR-02, INGR-03, INGR-04, TLS-03, SC-5
# Run from any directory; CHART_DIR is resolved relative to this script's location.
set -u

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
CHART_DIR="${SCRIPT_DIR}/../clay"

# Resolve helm binary: prefer system helm, fall back to /tmp/helm
if command -v helm >/dev/null 2>&1; then
    HELM="helm"
elif [ -x "/tmp/helm" ]; then
    HELM="/tmp/helm"
else
    echo "FAIL: helm binary not found in PATH or /tmp/helm" >&2
    exit 1
fi

PASS=0
FAIL=0

pass() {
    echo "PASS: $1"
    PASS=$((PASS + 1))
}

fail() {
    echo "FAIL: $1"
    echo "      $2"
    FAIL=$((FAIL + 1))
}

# ---------------------------------------------------------------------------
# Common --set flags required for a valid render (secrets validation guard)
REQUIRED="--set secrets.ADMIN_PASS=x --set secrets.SESSION_SECRET=x"

# Common --set flags for a fully valid custom-mode ingress render
CUSTOM_INGRESS="--set ingress.enabled=true \
  --set ingress.host=shop.example.com \
  --set ingress.tls.mode=custom \
  --set ingress.tls.secretName=my-tls"

# ---------------------------------------------------------------------------
# G-01 / INGR-01: custom mode renders ingressClassName: traefik
# ---------------------------------------------------------------------------
OUTPUT=$(${HELM} template release-test "${CHART_DIR}" \
  ${REQUIRED} \
  ${CUSTOM_INGRESS} 2>&1)

if echo "${OUTPUT}" | grep -q "ingressClassName: traefik"; then
    pass "G-01 INGR-01: custom mode renders ingressClassName: traefik"
else
    fail "G-01 INGR-01: custom mode renders ingressClassName: traefik" \
         "Expected 'ingressClassName: traefik' in helm template output. Got: $(echo "${OUTPUT}" | grep -i ingressClassName || echo '<not found>')"
fi

# ---------------------------------------------------------------------------
# G-02 / INGR-02: custom mode renders traefik router.entrypoints annotation
# ---------------------------------------------------------------------------
# Note: the acme.cert-manager.io/http01-edit-in-place annotation is gated on
# tls.mode=letsencrypt in the implementation, so we test each annotation
# separately under the conditions that actually trigger them.

OUTPUT_CUSTOM=$(${HELM} template release-test "${CHART_DIR}" \
  ${REQUIRED} \
  ${CUSTOM_INGRESS} 2>&1)

if echo "${OUTPUT_CUSTOM}" | grep -q "traefik.ingress.kubernetes.io/router.entrypoints: websecure"; then
    pass "G-02a INGR-02: custom mode renders traefik router.entrypoints: websecure annotation"
else
    fail "G-02a INGR-02: custom mode renders traefik router.entrypoints: websecure annotation" \
         "Expected 'traefik.ingress.kubernetes.io/router.entrypoints: websecure' in output"
fi

OUTPUT_LE=$(${HELM} template release-test "${CHART_DIR}" \
  ${REQUIRED} \
  --set ingress.enabled=true \
  --set ingress.host=shop.example.com \
  --set ingress.tls.mode=letsencrypt \
  --set ingress.tls.acme.email=test@example.com 2>&1)

if echo "${OUTPUT_LE}" | grep -q 'acme.cert-manager.io/http01-edit-in-place: "true"'; then
    pass "G-02b INGR-02: letsencrypt mode renders acme.cert-manager.io/http01-edit-in-place: \"true\" annotation"
else
    fail "G-02b INGR-02: letsencrypt mode renders acme.cert-manager.io/http01-edit-in-place: \"true\" annotation" \
         "Expected 'acme.cert-manager.io/http01-edit-in-place: \"true\"' in letsencrypt mode output"
fi

# ---------------------------------------------------------------------------
# G-03 / INGR-03: ingress.enabled=true but no host fails with expected error
# ---------------------------------------------------------------------------
ERR_NO_HOST=$(${HELM} template release-test "${CHART_DIR}" \
  ${REQUIRED} \
  --set ingress.enabled=true 2>&1)

if echo "${ERR_NO_HOST}" | grep -q "ingress.host must be set"; then
    pass "G-03 INGR-03: ingress.enabled=true with no host fails with 'ingress.host must be set'"
else
    fail "G-03 INGR-03: ingress.enabled=true with no host fails with 'ingress.host must be set'" \
         "Expected error 'ingress.host must be set'. Output: ${ERR_NO_HOST}"
fi

# ---------------------------------------------------------------------------
# G-04 / INGR-03: tls.mode=letsencrypt but no acme.email fails with expected error
# ---------------------------------------------------------------------------
ERR_NO_EMAIL=$(${HELM} template release-test "${CHART_DIR}" \
  ${REQUIRED} \
  --set ingress.enabled=true \
  --set ingress.host=shop.example.com \
  --set ingress.tls.mode=letsencrypt 2>&1)

if echo "${ERR_NO_EMAIL}" | grep -q "ingress.tls.acme.email required for letsencrypt mode"; then
    pass "G-04 INGR-03: letsencrypt mode with no acme.email fails with expected error"
else
    fail "G-04 INGR-03: letsencrypt mode with no acme.email fails with expected error" \
         "Expected 'ingress.tls.acme.email required for letsencrypt mode'. Output: ${ERR_NO_EMAIL}"
fi

# ---------------------------------------------------------------------------
# G-05 / INGR-03: tls.mode=custom but no secretName fails with expected error
# ---------------------------------------------------------------------------
ERR_NO_SECRET=$(${HELM} template release-test "${CHART_DIR}" \
  ${REQUIRED} \
  --set ingress.enabled=true \
  --set ingress.host=shop.example.com \
  --set ingress.tls.mode=custom 2>&1)

if echo "${ERR_NO_SECRET}" | grep -q "ingress.tls.secretName required for custom mode"; then
    pass "G-05 INGR-03: custom mode with no secretName fails with expected error"
else
    fail "G-05 INGR-03: custom mode with no secretName fails with expected error" \
         "Expected 'ingress.tls.secretName required for custom mode'. Output: ${ERR_NO_SECRET}"
fi

# ---------------------------------------------------------------------------
# G-06 / INGR-04: custom mode output contains zero 'nginx' strings
# ---------------------------------------------------------------------------
OUTPUT_NGINX=$(${HELM} template release-test "${CHART_DIR}" \
  ${REQUIRED} \
  ${CUSTOM_INGRESS} 2>&1)

NGINX_COUNT=$(echo "${OUTPUT_NGINX}" | grep -c "nginx" || true)

if [ "${NGINX_COUNT}" -eq 0 ]; then
    pass "G-06 INGR-04: custom mode rendered output contains zero 'nginx' strings"
else
    fail "G-06 INGR-04: custom mode rendered output contains zero 'nginx' strings" \
         "Found ${NGINX_COUNT} occurrence(s) of 'nginx' in output: $(echo "${OUTPUT_NGINX}" | grep nginx)"
fi

# ---------------------------------------------------------------------------
# G-07 / TLS-03: custom mode renders TLS block with user-provided secretName: my-tls
# ---------------------------------------------------------------------------
OUTPUT_TLS=$(${HELM} template release-test "${CHART_DIR}" \
  ${REQUIRED} \
  ${CUSTOM_INGRESS} 2>&1)

if echo "${OUTPUT_TLS}" | grep -q "secretName: my-tls"; then
    pass "G-07 TLS-03: custom mode TLS block renders secretName: my-tls (user-provided)"
else
    fail "G-07 TLS-03: custom mode TLS block renders secretName: my-tls (user-provided)" \
         "Expected 'secretName: my-tls' in TLS block. Output: $(echo "${OUTPUT_TLS}" | grep -i secretName || echo '<not found>')"
fi

# ---------------------------------------------------------------------------
# G-08 / SC-5: helm lint with CI values files exits 0
# ---------------------------------------------------------------------------
LINT_MANAGED=$(${HELM} lint "${CHART_DIR}" -f "${CHART_DIR}/ci/managed-values.yaml" 2>&1)
LINT_MANAGED_EXIT=$?

if [ ${LINT_MANAGED_EXIT} -eq 0 ]; then
    pass "G-08a SC-5: helm lint with managed-values.yaml exits 0"
else
    fail "G-08a SC-5: helm lint with managed-values.yaml exits 0" \
         "helm lint exited ${LINT_MANAGED_EXIT}. Output: ${LINT_MANAGED}"
fi

LINT_EXTERNAL=$(${HELM} lint "${CHART_DIR}" -f "${CHART_DIR}/ci/external-values.yaml" 2>&1)
LINT_EXTERNAL_EXIT=$?

if [ ${LINT_EXTERNAL_EXIT} -eq 0 ]; then
    pass "G-08b SC-5: helm lint with external-values.yaml exits 0"
else
    fail "G-08b SC-5: helm lint with external-values.yaml exits 0" \
         "helm lint exited ${LINT_EXTERNAL_EXIT}. Output: ${LINT_EXTERNAL}"
fi

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
echo ""
echo "Results: ${PASS} passed, ${FAIL} failed"

if [ ${FAIL} -gt 0 ]; then
    exit 1
fi
exit 0
