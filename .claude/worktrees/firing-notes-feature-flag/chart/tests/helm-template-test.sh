#!/usr/bin/env bash
# Behavioral tests for chart/clay Helm template rendering (Phase 3: values-and-ingress-refactor, Phase 4: cert-manager-cr-templates, Phase 7: webhook-readiness, Phase 8: hook-weight-ordering, Phase 9: ci-test-matrix)
# Requirements: INGR-01, INGR-02, INGR-03, INGR-04, TLS-01, TLS-02, TLS-03, SC-5, WBHK-01, WBHK-02, WBHK-03, WBHK-04, HOOK-01, HOOK-02, CI-01, CI-02, CI-03, CI-04, CI-05
# Run from any directory; CHART_DIR is resolved relative to this script's location.
set -euo pipefail

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
REQUIRED=(--set secrets.ADMIN_PASS=x --set secrets.SESSION_SECRET=x)

# Common --set flags for a fully valid custom-mode ingress render
CUSTOM_INGRESS=(
  --set ingress.enabled=true
  --set ingress.host=shop.example.com
  --set ingress.tls.mode=custom
  --set ingress.tls.secretName=my-tls
)

# Common --set flags for a fully valid letsencrypt-mode ingress render
LETSENCRYPT_INGRESS=(
  --set ingress.enabled=true
  --set ingress.host=shop.example.com
  --set ingress.tls.mode=letsencrypt
  --set ingress.tls.acme.email=admin@example.com
)

# Common --set flags for a fully valid selfsigned-mode ingress render
SELFSIGNED_INGRESS=(
  --set ingress.enabled=true
  --set ingress.host=shop.example.com
  --set ingress.tls.mode=selfsigned
)

# ---------------------------------------------------------------------------
# G-01 / INGR-01: custom mode renders ingressClassName: traefik
# ---------------------------------------------------------------------------
OUTPUT=$("${HELM}" template release-test "${CHART_DIR}" \
  "${REQUIRED[@]}" \
  "${CUSTOM_INGRESS[@]}" 2>&1)

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

OUTPUT_CUSTOM=$("${HELM}" template release-test "${CHART_DIR}" \
  "${REQUIRED[@]}" \
  "${CUSTOM_INGRESS[@]}" 2>&1)

if echo "${OUTPUT_CUSTOM}" | grep -q "traefik.ingress.kubernetes.io/router.entrypoints: websecure"; then
    pass "G-02a INGR-02: custom mode renders traefik router.entrypoints: websecure annotation"
else
    fail "G-02a INGR-02: custom mode renders traefik router.entrypoints: websecure annotation" \
         "Expected 'traefik.ingress.kubernetes.io/router.entrypoints: websecure' in output"
fi

OUTPUT_LE=$("${HELM}" template release-test "${CHART_DIR}" \
  "${REQUIRED[@]}" \
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
ERR_NO_HOST=$("${HELM}" template release-test "${CHART_DIR}" \
  "${REQUIRED[@]}" \
  --set ingress.enabled=true 2>&1 || true)

if echo "${ERR_NO_HOST}" | grep -q "ingress.host must be set"; then
    pass "G-03 INGR-03: ingress.enabled=true with no host fails with 'ingress.host must be set'"
else
    fail "G-03 INGR-03: ingress.enabled=true with no host fails with 'ingress.host must be set'" \
         "Expected error 'ingress.host must be set'. Output: ${ERR_NO_HOST}"
fi

# ---------------------------------------------------------------------------
# G-04 / INGR-03: tls.mode=letsencrypt but no acme.email fails with expected error
# ---------------------------------------------------------------------------
ERR_NO_EMAIL=$("${HELM}" template release-test "${CHART_DIR}" \
  "${REQUIRED[@]}" \
  --set ingress.enabled=true \
  --set ingress.host=shop.example.com \
  --set ingress.tls.mode=letsencrypt 2>&1 || true)

if echo "${ERR_NO_EMAIL}" | grep -q "ingress.tls.acme.email required for letsencrypt mode"; then
    pass "G-04 INGR-03: letsencrypt mode with no acme.email fails with expected error"
else
    fail "G-04 INGR-03: letsencrypt mode with no acme.email fails with expected error" \
         "Expected 'ingress.tls.acme.email required for letsencrypt mode'. Output: ${ERR_NO_EMAIL}"
fi

# ---------------------------------------------------------------------------
# G-05 / INGR-03: tls.mode=custom but no secretName fails with expected error
# ---------------------------------------------------------------------------
ERR_NO_SECRET=$("${HELM}" template release-test "${CHART_DIR}" \
  "${REQUIRED[@]}" \
  --set ingress.enabled=true \
  --set ingress.host=shop.example.com \
  --set ingress.tls.mode=custom 2>&1 || true)

if echo "${ERR_NO_SECRET}" | grep -q "ingress.tls.secretName required for custom mode"; then
    pass "G-05 INGR-03: custom mode with no secretName fails with expected error"
else
    fail "G-05 INGR-03: custom mode with no secretName fails with expected error" \
         "Expected 'ingress.tls.secretName required for custom mode'. Output: ${ERR_NO_SECRET}"
fi

# ---------------------------------------------------------------------------
# G-06 / INGR-04: custom mode output contains zero 'nginx' strings
# ---------------------------------------------------------------------------
OUTPUT_NGINX=$("${HELM}" template release-test "${CHART_DIR}" \
  "${REQUIRED[@]}" \
  "${CUSTOM_INGRESS[@]}" 2>&1)

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
OUTPUT_TLS=$("${HELM}" template release-test "${CHART_DIR}" \
  "${REQUIRED[@]}" \
  "${CUSTOM_INGRESS[@]}" 2>&1)

if echo "${OUTPUT_TLS}" | grep -q "secretName: my-tls"; then
    pass "G-07 TLS-03: custom mode TLS block renders secretName: my-tls (user-provided)"
else
    fail "G-07 TLS-03: custom mode TLS block renders secretName: my-tls (user-provided)" \
         "Expected 'secretName: my-tls' in TLS block. Output: $(echo "${OUTPUT_TLS}" | grep -i secretName || echo '<not found>')"
fi

# ---------------------------------------------------------------------------
# G-08 / CI-01..CI-04: helm lint with CI values files exits 0
# ---------------------------------------------------------------------------
LINT_BUNDLED=$("${HELM}" lint "${CHART_DIR}" -f "${CHART_DIR}/ci/ci-bundled-values.yaml" 2>&1 || true)
LINT_BUNDLED_EXIT=$?

if [ ${LINT_BUNDLED_EXIT} -eq 0 ]; then
    pass "G-08a CI-01: helm lint with ci-bundled-values.yaml exits 0"
else
    fail "G-08a CI-01: helm lint with ci-bundled-values.yaml exits 0" \
         "helm lint exited ${LINT_BUNDLED_EXIT}. Output: ${LINT_BUNDLED}"
fi

LINT_PREINSTALLED=$("${HELM}" lint "${CHART_DIR}" -f "${CHART_DIR}/ci/ci-preinstalled-values.yaml" 2>&1 || true)
LINT_PREINSTALLED_EXIT=$?

if [ ${LINT_PREINSTALLED_EXIT} -eq 0 ]; then
    pass "G-08b CI-02: helm lint with ci-preinstalled-values.yaml exits 0"
else
    fail "G-08b CI-02: helm lint with ci-preinstalled-values.yaml exits 0" \
         "helm lint exited ${LINT_PREINSTALLED_EXIT}. Output: ${LINT_PREINSTALLED}"
fi

LINT_EXTERNALDB=$("${HELM}" lint "${CHART_DIR}" -f "${CHART_DIR}/ci/ci-external-db-values.yaml" 2>&1 || true)
LINT_EXTERNALDB_EXIT=$?

if [ ${LINT_EXTERNALDB_EXIT} -eq 0 ]; then
    pass "G-08c CI-03: helm lint with ci-external-db-values.yaml exits 0"
else
    fail "G-08c CI-03: helm lint with ci-external-db-values.yaml exits 0" \
         "helm lint exited ${LINT_EXTERNALDB_EXIT}. Output: ${LINT_EXTERNALDB}"
fi

LINT_MIXED=$("${HELM}" lint "${CHART_DIR}" -f "${CHART_DIR}/ci/ci-mixed-values.yaml" 2>&1 || true)
LINT_MIXED_EXIT=$?

if [ ${LINT_MIXED_EXIT} -eq 0 ]; then
    pass "G-08d CI-04: helm lint with ci-mixed-values.yaml exits 0"
else
    fail "G-08d CI-04: helm lint with ci-mixed-values.yaml exits 0" \
         "helm lint exited ${LINT_MIXED_EXIT}. Output: ${LINT_MIXED}"
fi

# ---------------------------------------------------------------------------
# G-09 / TLS-01: letsencrypt mode renders ClusterIssuer with ACME staging endpoint and hook annotations
# ---------------------------------------------------------------------------
OUTPUT_LE_09=$("${HELM}" template release-test "${CHART_DIR}" \
  "${REQUIRED[@]}" \
  "${LETSENCRYPT_INGRESS[@]}" 2>&1)

if echo "${OUTPUT_LE_09}" | grep -q "^kind: ClusterIssuer"; then
    pass "G-09a TLS-01: letsencrypt mode renders ClusterIssuer resource"
else
    fail "G-09a TLS-01: letsencrypt mode renders ClusterIssuer resource" \
         "Expected 'kind: ClusterIssuer' in letsencrypt mode output"
fi

if echo "${OUTPUT_LE_09}" | grep -q "acme-staging-v02.api.letsencrypt.org/directory"; then
    pass "G-09b TLS-01: letsencrypt mode ClusterIssuer uses ACME staging endpoint"
else
    fail "G-09b TLS-01: letsencrypt mode ClusterIssuer uses ACME staging endpoint" \
         "Expected 'acme-staging-v02.api.letsencrypt.org/directory' in output"
fi

if echo "${OUTPUT_LE_09}" | grep -q "helm.sh/hook: post-install,post-upgrade"; then
    pass "G-09c TLS-01: letsencrypt mode ClusterIssuer carries helm.sh/hook: post-install,post-upgrade annotation"
else
    fail "G-09c TLS-01: letsencrypt mode ClusterIssuer carries helm.sh/hook: post-install,post-upgrade annotation" \
         "Expected 'helm.sh/hook: post-install,post-upgrade' in letsencrypt mode output"
fi

# ---------------------------------------------------------------------------
# G-10 / TLS-01: letsencrypt mode renders Certificate CR with hook annotations
# ---------------------------------------------------------------------------
OUTPUT_LE_10=$("${HELM}" template release-test "${CHART_DIR}" \
  "${REQUIRED[@]}" \
  "${LETSENCRYPT_INGRESS[@]}" 2>&1)

if echo "${OUTPUT_LE_10}" | grep -q "^kind: Certificate"; then
    pass "G-10a TLS-01: letsencrypt mode renders Certificate resource"
else
    fail "G-10a TLS-01: letsencrypt mode renders Certificate resource" \
         "Expected 'kind: Certificate' in letsencrypt mode output"
fi

if echo "${OUTPUT_LE_10}" | grep -q "helm.sh/hook: post-install,post-upgrade"; then
    pass "G-10b TLS-01: letsencrypt mode Certificate carries helm.sh/hook: post-install,post-upgrade annotation"
else
    fail "G-10b TLS-01: letsencrypt mode Certificate carries helm.sh/hook: post-install,post-upgrade annotation" \
         "Expected 'helm.sh/hook: post-install,post-upgrade' in letsencrypt Certificate output"
fi

# ---------------------------------------------------------------------------
# G-11 / TLS-01: letsencrypt mode Ingress carries cert-manager.io/cluster-issuer annotation
# ---------------------------------------------------------------------------
OUTPUT_LE_11=$("${HELM}" template release-test "${CHART_DIR}" \
  "${REQUIRED[@]}" \
  "${LETSENCRYPT_INGRESS[@]}" 2>&1)

if echo "${OUTPUT_LE_11}" | grep -q "cert-manager.io/cluster-issuer: release-test-clay-letsencrypt"; then
    pass "G-11 TLS-01: letsencrypt mode Ingress carries cert-manager.io/cluster-issuer annotation with release-derived name"
else
    fail "G-11 TLS-01: letsencrypt mode Ingress carries cert-manager.io/cluster-issuer annotation with release-derived name" \
         "Expected 'cert-manager.io/cluster-issuer: release-test-clay-letsencrypt'. Got: $(echo "${OUTPUT_LE_11}" | grep "cluster-issuer" || echo '<not found>')"
fi

# ---------------------------------------------------------------------------
# G-12 / TLS-02: selfsigned mode renders two ClusterIssuers and two Certificates (four-resource CA bootstrap)
# ---------------------------------------------------------------------------
OUTPUT_SS_12=$("${HELM}" template release-test "${CHART_DIR}" \
  "${REQUIRED[@]}" \
  "${SELFSIGNED_INGRESS[@]}" 2>&1)

CI_COUNT=$(echo "${OUTPUT_SS_12}" | grep -c "^kind: ClusterIssuer" || true)
CERT_COUNT=$(echo "${OUTPUT_SS_12}" | grep -c "^kind: Certificate" || true)

if [ "${CI_COUNT}" -eq 2 ]; then
    pass "G-12a TLS-02: selfsigned mode renders exactly 2 ClusterIssuer resources (got ${CI_COUNT})"
else
    fail "G-12a TLS-02: selfsigned mode renders exactly 2 ClusterIssuer resources" \
         "Expected 2 ClusterIssuer resources, got ${CI_COUNT}"
fi

if [ "${CERT_COUNT}" -eq 2 ]; then
    pass "G-12b TLS-02: selfsigned mode renders exactly 2 Certificate resources (got ${CERT_COUNT})"
else
    fail "G-12b TLS-02: selfsigned mode renders exactly 2 Certificate resources" \
         "Expected 2 Certificate resources, got ${CERT_COUNT}"
fi

if echo "${OUTPUT_SS_12}" | grep -q "selfSigned: {}"; then
    pass "G-12c TLS-02: selfsigned mode renders SelfSigned root ClusterIssuer (selfSigned: {})"
else
    fail "G-12c TLS-02: selfsigned mode renders SelfSigned root ClusterIssuer (selfSigned: {})" \
         "Expected 'selfSigned: {}' in selfsigned mode output"
fi

if echo "${OUTPUT_SS_12}" | grep -q "isCA: true"; then
    pass "G-12d TLS-02: selfsigned mode renders CA Certificate with isCA: true"
else
    fail "G-12d TLS-02: selfsigned mode renders CA Certificate with isCA: true" \
         "Expected 'isCA: true' in selfsigned mode output"
fi

# ---------------------------------------------------------------------------
# G-13 / TLS-02: selfsigned mode renders zero ACME resources
# ---------------------------------------------------------------------------
OUTPUT_SS_13=$("${HELM}" template release-test "${CHART_DIR}" \
  "${REQUIRED[@]}" \
  "${SELFSIGNED_INGRESS[@]}" 2>&1)

if echo "${OUTPUT_SS_13}" | grep -q "acme-staging-v02"; then
    fail "G-13 TLS-02: selfsigned mode renders no ACME staging URL" \
         "Found 'acme-staging-v02' in selfsigned mode output — should not appear"
else
    pass "G-13 TLS-02: selfsigned mode renders no ACME staging URL (acme-staging-v02 absent)"
fi

# ---------------------------------------------------------------------------
# G-14 / TLS-01+TLS-02: custom mode renders zero ClusterIssuer and zero Certificate resources
# ---------------------------------------------------------------------------
OUTPUT_CU_14=$("${HELM}" template release-test "${CHART_DIR}" \
  "${REQUIRED[@]}" \
  "${CUSTOM_INGRESS[@]}" 2>&1)

CI_CUSTOM_COUNT=$(echo "${OUTPUT_CU_14}" | grep -c "^kind: ClusterIssuer" || true)
CERT_CUSTOM_COUNT=$(echo "${OUTPUT_CU_14}" | grep -c "^kind: Certificate" || true)

if [ "${CI_CUSTOM_COUNT}" -eq 0 ]; then
    pass "G-14a TLS-01+TLS-02: custom mode renders zero ClusterIssuer resources (got ${CI_CUSTOM_COUNT})"
else
    fail "G-14a TLS-01+TLS-02: custom mode renders zero ClusterIssuer resources" \
         "Expected 0 ClusterIssuer resources, got ${CI_CUSTOM_COUNT}"
fi

if [ "${CERT_CUSTOM_COUNT}" -eq 0 ]; then
    pass "G-14b TLS-01+TLS-02: custom mode renders zero Certificate resources (got ${CERT_CUSTOM_COUNT})"
else
    fail "G-14b TLS-01+TLS-02: custom mode renders zero Certificate resources" \
         "Expected 0 Certificate resources, got ${CERT_CUSTOM_COUNT}"
fi

# ---------------------------------------------------------------------------
# G-15 / WBHK-01+WBHK-02+WBHK-03: both operators enabled -> 2 Jobs, shared RBAC
# ---------------------------------------------------------------------------
OUTPUT_WH_15=$("${HELM}" template release-test "${CHART_DIR}" \
  "${REQUIRED[@]}" 2>&1)

# Count only the clay webhook-wait Jobs (cnpg + cert-manager); subchart Jobs (e.g. cert-manager-startupapicheck) are excluded
# Anchor to "^  name:" (2 spaces = metadata.name) to avoid matching template.metadata.name (6 spaces) inside the Job spec
JOB_COUNT=$(echo "${OUTPUT_WH_15}" | grep -cE "^  name: release-test-clay-(cnpg|cert-manager)-webhook-wait" || true)

if [ "${JOB_COUNT}" -eq 2 ]; then
    pass "G-15a WBHK-01+WBHK-02: both operators enabled renders exactly 2 Job resources (got ${JOB_COUNT})"
else
    fail "G-15a WBHK-01+WBHK-02: both operators enabled renders exactly 2 Job resources" \
         "Expected 2 Job resources, got ${JOB_COUNT}"
fi

if echo "${OUTPUT_WH_15}" | grep -q "name: release-test-clay-webhook-wait"; then
    pass "G-15b WBHK-03: ServiceAccount named release-test-clay-webhook-wait present"
else
    fail "G-15b WBHK-03: ServiceAccount named release-test-clay-webhook-wait present" \
         "Expected 'name: release-test-clay-webhook-wait' in output"
fi

# Extract only the webhook-wait-rbac.yaml section to scope the SA count check
RBAC_SECTION=$(echo "${OUTPUT_WH_15}" | awk '/# Source: clay\/templates\/webhook-wait-rbac\.yaml/{p=1} /^# Source:/{if(p && !/webhook-wait-rbac/){p=0}} p')
SA_COUNT=$(echo "${RBAC_SECTION}" | grep -c "^kind: ServiceAccount" || true)

if [ "${SA_COUNT}" -ge 1 ]; then
    pass "G-15c WBHK-03: at least 1 ServiceAccount for webhook-wait"
else
    fail "G-15c WBHK-03: at least 1 ServiceAccount for webhook-wait" \
         "Expected ServiceAccount with webhook-wait name, got ${SA_COUNT}"
fi

if echo "${OUTPUT_WH_15}" | grep -q "^kind: ClusterRole$"; then
    pass "G-15d WBHK-03: ClusterRole resource present"
else
    fail "G-15d WBHK-03: ClusterRole resource present" \
         "Expected 'kind: ClusterRole' (not ClusterRoleBinding) in output"
fi

if echo "${OUTPUT_WH_15}" | grep -q "^kind: ClusterRoleBinding"; then
    pass "G-15e WBHK-03: ClusterRoleBinding resource present"
else
    fail "G-15e WBHK-03: ClusterRoleBinding resource present" \
         "Expected 'kind: ClusterRoleBinding' in output"
fi

# ---------------------------------------------------------------------------
# G-16 / WBHK-04: cloudnative-pg.enabled=false -> no CNPG webhook-wait Job
# ---------------------------------------------------------------------------
OUTPUT_WH_16=$("${HELM}" template release-test "${CHART_DIR}" \
  "${REQUIRED[@]}" \
  --set 'cloudnative-pg.enabled=false' 2>&1)

if echo "${OUTPUT_WH_16}" | grep -q "cnpg-webhook-wait"; then
    fail "G-16 WBHK-04: cloudnative-pg disabled renders no CNPG webhook-wait Job" \
         "Found 'cnpg-webhook-wait' in output -- should not appear when cloudnative-pg.enabled=false"
else
    pass "G-16 WBHK-04: cloudnative-pg disabled renders no CNPG webhook-wait Job (cnpg-webhook-wait absent)"
fi

# ---------------------------------------------------------------------------
# G-17 / WBHK-04: cert-manager.enabled=false -> no cert-manager webhook-wait Job
# ---------------------------------------------------------------------------
OUTPUT_WH_17=$("${HELM}" template release-test "${CHART_DIR}" \
  "${REQUIRED[@]}" \
  --set 'cert-manager.enabled=false' 2>&1)

if echo "${OUTPUT_WH_17}" | grep -q "cert-manager-webhook-wait"; then
    fail "G-17 WBHK-04: cert-manager disabled renders no cert-manager webhook-wait Job" \
         "Found 'cert-manager-webhook-wait' in output -- should not appear when cert-manager.enabled=false"
else
    pass "G-17 WBHK-04: cert-manager disabled renders no cert-manager webhook-wait Job (cert-manager-webhook-wait absent)"
fi

# ---------------------------------------------------------------------------
# G-18 / WBHK-01+WBHK-02: webhook-wait Jobs carry correct hook annotations
# ---------------------------------------------------------------------------
OUTPUT_WH_18=$("${HELM}" template release-test "${CHART_DIR}" \
  "${REQUIRED[@]}" 2>&1)

HOOK_COUNT=$(echo "${OUTPUT_WH_18}" | grep -c '"helm.sh/hook": post-install,post-upgrade' || true)

if [ "${HOOK_COUNT}" -ge 2 ]; then
    pass "G-18a WBHK-01+WBHK-02: at least 2 resources carry helm.sh/hook: post-install,post-upgrade (got ${HOOK_COUNT})"
else
    fail "G-18a WBHK-01+WBHK-02: at least 2 resources carry helm.sh/hook: post-install,post-upgrade" \
         "Expected at least 2 occurrences, got ${HOOK_COUNT}"
fi

WEIGHT_COUNT=$(echo "${OUTPUT_WH_18}" | grep -c '"helm.sh/hook-weight": "-20"' || true)

if [ "${WEIGHT_COUNT}" -ge 2 ]; then
    pass "G-18b WBHK-01+WBHK-02: at least 2 resources carry hook-weight -20 (got ${WEIGHT_COUNT})"
else
    fail "G-18b WBHK-01+WBHK-02: at least 2 resources carry hook-weight -20" \
         "Expected at least 2 occurrences of hook-weight -20, got ${WEIGHT_COUNT}"
fi

# ---------------------------------------------------------------------------
# G-19 / WBHK-03: ClusterRole has no wildcard rules
# (scoped to webhook-wait-rbac.yaml section only -- subchart ClusterRoles are out of scope)
# ---------------------------------------------------------------------------
OUTPUT_WH_19=$("${HELM}" template release-test "${CHART_DIR}" \
  "${REQUIRED[@]}" 2>&1)

# Extract only the webhook-wait-rbac.yaml section to avoid false positives from subcharts
RBAC_SECTION_19=$(echo "${OUTPUT_WH_19}" | awk '/# Source: clay\/templates\/webhook-wait-rbac\.yaml/{p=1} /^# Source:/{if(p && !/webhook-wait-rbac/){p=0}} p')

if echo "${RBAC_SECTION_19}" | grep -qF '"*"'; then
    fail "G-19 WBHK-03: ClusterRole contains no wildcard rules" \
         "Found '\"*\"' in webhook-wait-rbac output -- ClusterRole must not grant wildcard permissions"
else
    pass "G-19 WBHK-03: ClusterRole contains no wildcard rules (no '\"*\"' found in webhook-wait-rbac section)"
fi

# ---------------------------------------------------------------------------
# G-20 / HOOK-01: cnpg-cluster.yaml carries post-install,post-upgrade hook at weight -10
# ---------------------------------------------------------------------------
OUTPUT_G20=$("${HELM}" template release-test "${CHART_DIR}" \
  "${REQUIRED[@]}" 2>&1)

CLUSTER_SECTION=$(echo "${OUTPUT_G20}" | awk \
  '/# Source: clay\/templates\/cnpg-cluster\.yaml/{p=1} \
   /^# Source:/{if(p && !/cnpg-cluster/){p=0}} p')

if echo "${CLUSTER_SECTION}" | grep -q '"helm.sh/hook": post-install,post-upgrade'; then
    pass "G-20a HOOK-01: cnpg-cluster carries helm.sh/hook: post-install,post-upgrade"
else
    fail "G-20a HOOK-01: cnpg-cluster carries helm.sh/hook: post-install,post-upgrade" \
         "Expected hook annotation in cnpg-cluster section"
fi

if echo "${CLUSTER_SECTION}" | grep -q '"helm.sh/hook-weight": "-10"'; then
    pass "G-20b HOOK-01: cnpg-cluster carries hook-weight -10"
else
    fail "G-20b HOOK-01: cnpg-cluster carries hook-weight -10" \
         "Expected weight -10 in cnpg-cluster section"
fi

if echo "${CLUSTER_SECTION}" | grep -q '"helm.sh/hook-delete-policy": before-hook-creation'; then
    pass "G-20c HOOK-01: cnpg-cluster carries hook-delete-policy before-hook-creation"
else
    fail "G-20c HOOK-01: cnpg-cluster carries hook-delete-policy before-hook-creation" \
         "Expected before-hook-creation delete-policy in cnpg-cluster section"
fi

# ---------------------------------------------------------------------------
# G-21 / HOOK-02: webhook-wait-rbac carries hook-weight -25 (RBAC tier)
# ---------------------------------------------------------------------------
RBAC_SECTION_21=$(echo "${OUTPUT_G20}" | awk \
  '/# Source: clay\/templates\/webhook-wait-rbac\.yaml/{p=1} \
   /^# Source:/{if(p && !/webhook-wait-rbac/){p=0}} p')

RBAC_W25_COUNT=$(echo "${RBAC_SECTION_21}" | grep -c '"helm.sh/hook-weight": "-25"' || true)

if [ "${RBAC_W25_COUNT}" -ge 1 ]; then
    pass "G-21 HOOK-02: webhook-wait-rbac carries hook-weight -25 (got ${RBAC_W25_COUNT} resources)"
else
    fail "G-21 HOOK-02: webhook-wait-rbac carries hook-weight -25" \
         "Expected at least 1 occurrence of hook-weight -25 in webhook-wait-rbac section, got ${RBAC_W25_COUNT}"
fi

# ---------------------------------------------------------------------------
# G-22 / HOOK-02: webhook-wait-jobs carry hook-weight -20 (Job tier)
# ---------------------------------------------------------------------------
JOBS_SECTION_22=$(echo "${OUTPUT_G20}" | awk \
  '/# Source: clay\/templates\/webhook-wait-jobs\.yaml/{p=1} \
   /^# Source:/{if(p && !/webhook-wait-jobs/){p=0}} p')

JOBS_W20_COUNT=$(echo "${JOBS_SECTION_22}" | grep -c '"helm.sh/hook-weight": "-20"' || true)

if [ "${JOBS_W20_COUNT}" -ge 2 ]; then
    pass "G-22 HOOK-02: webhook-wait-jobs carry hook-weight -20 (got ${JOBS_W20_COUNT} resources)"
else
    fail "G-22 HOOK-02: webhook-wait-jobs carry hook-weight -20" \
         "Expected at least 2 occurrences of hook-weight -20 in webhook-wait-jobs section, got ${JOBS_W20_COUNT}"
fi

# ---------------------------------------------------------------------------
# G-23 / HOOK-02: cert-manager-letsencrypt resources carry hook-weights in range [-10, 5]
# ---------------------------------------------------------------------------
OUTPUT_G23=$("${HELM}" template release-test "${CHART_DIR}" \
  "${REQUIRED[@]}" \
  "${LETSENCRYPT_INGRESS[@]}" 2>&1)

LE_SECTION=$(echo "${OUTPUT_G23}" | awk \
  '/# Source: clay\/templates\/cert-manager-letsencrypt\.yaml/{p=1} \
   /^# Source:/{if(p && !/cert-manager-letsencrypt/){p=0}} p')

if echo "${LE_SECTION}" | grep -q 'helm.sh/hook-weight: "-5"'; then
    pass "G-23a HOOK-02: letsencrypt ClusterIssuer carries hook-weight -5"
else
    fail "G-23a HOOK-02: letsencrypt ClusterIssuer carries hook-weight -5" \
         "Expected 'helm.sh/hook-weight: \"-5\"' in cert-manager-letsencrypt section"
fi

if echo "${LE_SECTION}" | grep -q 'helm.sh/hook-weight: "0"'; then
    pass "G-23b HOOK-02: letsencrypt Certificate carries hook-weight 0"
else
    fail "G-23b HOOK-02: letsencrypt Certificate carries hook-weight 0" \
         "Expected 'helm.sh/hook-weight: \"0\"' in cert-manager-letsencrypt section"
fi

# ---------------------------------------------------------------------------
# G-24 / CI-01: bundled mode — both operator Deployments render
# ---------------------------------------------------------------------------
OUTPUT_G24=$("${HELM}" template release-test "${CHART_DIR}" \
  "${REQUIRED[@]}" \
  --values "${CHART_DIR}/ci/ci-bundled-values.yaml" 2>&1)

if grep -q "name: release-test-cloudnative-pg" <<< "${OUTPUT_G24}"; then
    pass "G-24a CI-01: bundled mode renders CNPG operator Deployment"
else
    fail "G-24a CI-01: bundled mode renders CNPG operator Deployment" \
         "Expected 'name: release-test-cloudnative-pg' in output"
fi

if grep -qE "^  name: release-test-cert-manager$" <<< "${OUTPUT_G24}"; then
    pass "G-24b CI-01: bundled mode renders cert-manager operator Deployment"
else
    fail "G-24b CI-01: bundled mode renders cert-manager operator Deployment" \
         "Expected 'name: release-test-cert-manager' in output"
fi

# ---------------------------------------------------------------------------
# G-25 / CI-02: pre-installed mode — Cluster CR renders, operator Deployments absent
# ---------------------------------------------------------------------------
OUTPUT_G25=$("${HELM}" template release-test "${CHART_DIR}" \
  "${REQUIRED[@]}" \
  --values "${CHART_DIR}/ci/ci-preinstalled-values.yaml" 2>&1)

if grep -q "^kind: Cluster" <<< "${OUTPUT_G25}"; then
    pass "G-25a CI-02: pre-installed mode renders CNPG Cluster CR (postgres.managed=true)"
else
    fail "G-25a CI-02: pre-installed mode renders CNPG Cluster CR" \
         "Expected 'kind: Cluster' in output"
fi

if grep -q "release-test-cloudnative-pg" <<< "${OUTPUT_G25}"; then
    fail "G-25b CI-02: pre-installed mode renders no CNPG operator Deployment" \
         "Found 'release-test-cloudnative-pg' in output -- should not appear when cloudnative-pg.enabled=false"
else
    pass "G-25b CI-02: pre-installed mode renders no CNPG operator Deployment (cloudnative-pg absent)"
fi

# ---------------------------------------------------------------------------
# G-26 / CI-03: external-db mode — no Cluster CR, no webhook-wait Jobs
# ---------------------------------------------------------------------------
OUTPUT_G26=$("${HELM}" template release-test "${CHART_DIR}" \
  "${REQUIRED[@]}" \
  --values "${CHART_DIR}/ci/ci-external-db-values.yaml" 2>&1)

if grep -q "^kind: Cluster" <<< "${OUTPUT_G26}"; then
    fail "G-26a CI-03: external-db mode renders no CNPG Cluster CR" \
         "Found 'kind: Cluster' in output -- should not appear when postgres.managed=false"
else
    pass "G-26a CI-03: external-db mode renders no CNPG Cluster CR (postgres.managed=false)"
fi

if grep -q "webhook-wait" <<< "${OUTPUT_G26}"; then
    fail "G-26b CI-03: external-db mode renders no webhook-wait Jobs" \
         "Found 'webhook-wait' in output -- should not appear when both operators disabled"
else
    pass "G-26b CI-03: external-db mode renders no webhook-wait Jobs (all disabled)"
fi

# ---------------------------------------------------------------------------
# G-27 / CI-04: mixed mode — CNPG operator Deployment renders, cert-manager absent
# ---------------------------------------------------------------------------
OUTPUT_G27=$("${HELM}" template release-test "${CHART_DIR}" \
  "${REQUIRED[@]}" \
  --values "${CHART_DIR}/ci/ci-mixed-values.yaml" 2>&1)

if grep -q "name: release-test-cloudnative-pg" <<< "${OUTPUT_G27}"; then
    pass "G-27a CI-04: mixed mode renders CNPG operator Deployment (cloudnative-pg.enabled=true)"
else
    fail "G-27a CI-04: mixed mode renders CNPG operator Deployment" \
         "Expected 'name: release-test-cloudnative-pg' in output"
fi

if grep -q "release-test-cert-manager" <<< "${OUTPUT_G27}"; then
    fail "G-27b CI-04: mixed mode renders no cert-manager operator Deployment" \
         "Found 'release-test-cert-manager' in output -- should not appear when cert-manager.enabled=false"
else
    pass "G-27b CI-04: mixed mode renders no cert-manager operator Deployment (cert-manager absent)"
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
