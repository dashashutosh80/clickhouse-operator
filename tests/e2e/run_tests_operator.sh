#!/bin/bash
CUR_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
source "${CUR_DIR}/test_common.sh"

IMAGE_PULL_POLICY="${IMAGE_PULL_POLICY:-"Always"}"

common_install_pip_requirements
common_export_test_env

RUN_ALL_FLAG=$(common_convert_run_all)

RETRY_ARGS=()
if [[ -n "${RETRY_COUNT}" ]]; then
    RETRY_ARGS+=(--retry "/regression/e2e.test_operator/test_0:,${RETRY_COUNT},,${RETRY_DELAY:-30}")
fi

python3 "${COMMON_DIR}/../regression.py" \
    --only="/regression/e2e.test_operator/${ONLY}" \
    ${RUN_ALL_FLAG} \
    "${RETRY_ARGS[@]}" \
    -o short \
    --trim-results on \
    --debug \
    --native
