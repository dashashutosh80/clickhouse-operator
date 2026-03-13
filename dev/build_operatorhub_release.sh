#!/bin/bash

# Source configuration
CUR_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
source "${CUR_DIR}/go_build_config.sh"

##
## Section 1: Build manifests locally
##

if [[ -z "${PREVIOUS_VERSION}" ]]; then
    echo "PREVIOUS_VERSION is not explicitly specified"
    echo "Trying to figure out PREVIOUS_VERSION from releases"
    PREVIOUS_VERSION=$(head -n1 "${SRC_ROOT}/releases")
    echo "Found: PREVIOUS_VERSION=${PREVIOUS_VERSION}"
else
    echo "PREVIOUS_VERSION=${PREVIOUS_VERSION} (explicitly specified)"
fi

if [[ -z "${PREVIOUS_VERSION}" ]]; then
    echo "No PREVIOUS_VERSION available."
    echo "Please specify PREVIOUS_VERSION used in previous release, like:"
    echo "  PREVIOUS_VERSION=0.25.6 $0"
    exit 1
fi

echo "=================================================================================="
echo ""
echo "VERSION=${VERSION}"
echo "PREVIOUS_VERSION=${PREVIOUS_VERSION}"
echo ""
echo "=================================================================================="
read -n 1 -r -s -p $'Please verify VERSION and PREVIOUS_VERSION. Press enter to build...\n'

# Build manifests
PREVIOUS_VERSION="${PREVIOUS_VERSION}" "${SRC_ROOT}/deploy/builder/operatorhub.sh"

OPERATORHUB_DIR="${SRC_ROOT}/deploy/operatorhub"

##
## Section 2: Destinations
##

REPO_ROOTS=(
    "${CO_REPO_PATH:-${HOME}/dev/community-operators}"
    "${OCP_REPO_PATH:-${HOME}/dev/community-operators-prod}"
)

DESTINATIONS=()
for REPO_ROOT in "${REPO_ROOTS[@]}"; do
    DESTINATIONS+=("${REPO_ROOT}/operators/clickhouse")
done

# Name of the git remote pointing to the canonical upstream in each catalog repo
UPSTREAM_REMOTE="${UPSTREAM_REMOTE:-community}"

##
## Section 3: Sync destination repos
##

function prepare_destination_repo() {
    # $1 REPO_ROOT     - path to the local clone of the catalog repo
    #                    e.g. ~/dev/community-operators
    # $2 UPSTREAM      - name of the git remote pointing to the canonical upstream
    #                    e.g. upstream
    local REPO_ROOT="$1"
    local UPSTREAM="$2"

    echo ""
    echo "Syncing ${REPO_ROOT} from ${UPSTREAM}/main ..."

    git -C "${REPO_ROOT}" fetch --all || { echo "  [ERROR] git fetch failed in ${REPO_ROOT}"; return 1; }

    local upstream_sha
    upstream_sha=$(git -C "${REPO_ROOT}" rev-parse "${UPSTREAM}/main") || {
        echo "  [ERROR] Cannot resolve ${UPSTREAM}/main in ${REPO_ROOT}"
        return 1
    }

    git -C "${REPO_ROOT}" checkout main || { echo "  [ERROR] git checkout main failed in ${REPO_ROOT}"; return 1; }
    git -C "${REPO_ROOT}" reset --hard "${upstream_sha}" || { echo "  [ERROR] git reset --hard failed in ${REPO_ROOT}"; return 1; }

    echo "  [OK]   ${REPO_ROOT} synced to ${UPSTREAM}/main (${upstream_sha})"
}

for REPO_ROOT in "${REPO_ROOTS[@]}"; do
    prepare_destination_repo "${REPO_ROOT}" "${UPSTREAM_REMOTE}"
done

##
## Section 4: Copy to each destination
##

function copy_to_destination() {
    # $1 DST_FOLDER      - path to the operator's root folder in the target catalog repo
    #                      e.g. ~/dev/community-operators/operators/clickhouse
    # $2 VERSION         - operator version being published, e.g. 0.26.0
    # $3 SRC_BUNDLE_DIR  - path to locally generated bundle, e.g. deploy/operatorhub
    local DST_FOLDER="$1"
    local VERSION="$2"
    local SRC_BUNDLE_DIR="$3"

    local DST_MANIFESTS="${DST_FOLDER}/${VERSION}/manifests/"
    local DST_METADATA="${DST_FOLDER}/${VERSION}/metadata/"

    # Count pre-existing versions before we create anything
    local existing
    existing=$(ls -d "${DST_FOLDER}"/[0-9]*/ 2>/dev/null | wc -l)

    # Ensure target and copy
    mkdir -p "${DST_MANIFESTS}" "${DST_METADATA}"
    cp -r "${SRC_BUNDLE_DIR}/${VERSION}/"* "${DST_MANIFESTS}"
    cp -r "${SRC_BUNDLE_DIR}/metadata/"*   "${DST_METADATA}"

    # First version in this destination — no upgrade path exists yet, drop spec.replaces
    if [[ "${existing}" -eq 0 ]]; then
        local csv_file
        csv_file=$(ls "${DST_MANIFESTS}"*.clusterserviceversion.yaml 2>/dev/null | head -1)
        if [[ -n "${csv_file}" ]]; then
            yq -i 'del(.spec.replaces)' "${csv_file}"
            echo "  [INFO] First version in destination — removed spec.replaces from CSV"
        fi
    fi

    echo "  [OK]   Copied to: ${DST_FOLDER}"
}

echo ""
echo "Bundle v${VERSION} will be copied to the following destinations:"
for DST in "${DESTINATIONS[@]}"; do
    if [[ -d "${DST}" ]]; then
        echo "  [FOUND]       ${DST}"
    else
        echo "  [WILL CREATE] ${DST}"
    fi
done
echo ""
echo "=================================================================================="
read -n 1 -r -s -p $'Press enter to copy...\n'

for DST in "${DESTINATIONS[@]}"; do
    copy_to_destination "${DST}" "${VERSION}" "${OPERATORHUB_DIR}"
done

##
## Section 5: Commit each destination repo
##

function commit_destination_repo() {
    # $1 REPO_ROOT     - path to the local clone of the catalog repo
    #                    e.g. ~/dev/community-operators
    # $2 VERSION       - operator version being published, e.g. 0.26.0
    local REPO_ROOT="$1"
    local VERSION="$2"

    git -C "${REPO_ROOT}" add "operators/clickhouse/${VERSION}" || { echo "  [ERROR] git add failed in ${REPO_ROOT}"; return 1; }
    git -C "${REPO_ROOT}" commit -s -m "operator clickhouse (${VERSION})" || { echo "  [ERROR] git commit failed in ${REPO_ROOT}"; return 1; }
    git -C "${REPO_ROOT}" push --force || { echo "  [ERROR] git push failed in ${REPO_ROOT}"; return 1; }

    echo "  [OK]   Committed in ${REPO_ROOT}"
}

echo ""
for i in "${!REPO_ROOTS[@]}"; do
    commit_destination_repo "${REPO_ROOTS[$i]}" "${VERSION}"
done

echo ""
echo "DONE"
