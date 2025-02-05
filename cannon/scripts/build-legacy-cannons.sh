#!/bin/bash
set -euo pipefail
SCRIPTS_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)

# This script builds a version of the cannon executable that includes support for both current and legacy state versions.
# Each cannon release is built separately.

TMP_DIR=$(mktemp -d)
function cleanup() {
    rm -rf "${TMP_DIR}"
}
trap cleanup EXIT
echo "Using temp dir: ${TMP_DIR}"
cd "${TMP_DIR}"

# Need to check out a fresh copy of the monorepo so we can switch to specific tags without it also affecting the
# contents of this script (which is checked into the repo).
git clone https://github.com/ethereum-optimism/optimism --recurse-submodules

CANNON_DIR="${SCRIPTS_DIR}/../"
EMBEDS_DIR="${CANNON_DIR}/multicannon/embeds"
LOGS_DIR="${CANNON_DIR}/temp/logs"
REPO_DIR="${TMP_DIR}/optimism"
BIN_DIR="${REPO_DIR}/cannon/multicannon/embeds"

mkdir -p "${LOGS_DIR}"

cd "${REPO_DIR}"

function buildVersion() {
  TAG=${1}
  LOG_FILE="${LOGS_DIR}/build-$(echo "${TAG}" | cut -c 8-).txt"
  echo "Building Version: ${TAG} Logs: ${LOG_FILE}"
  git checkout "${TAG}" > "${LOG_FILE}" 2>&1
  git submodule update --init --recursive >> "${LOG_FILE}" 2>&1
  rm -rf "${BIN_DIR}/cannon-"*
  make -C "${REPO_DIR}/cannon" cannon-embeds >> "${LOG_FILE}" 2>&1
  cp "${BIN_DIR}/cannon-"* "${EMBEDS_DIR}/"
  echo "Built ${TAG} with versions:"
  (cd "${BIN_DIR}" && ls cannon-*)
}

# Build each release of cannon from earliest to latest. Releases with qualifiers (e.g. `-rc`, `-alpha` etc are skipped.
# If the same state version is supported by multiple version tags built, the build from the last tag listed will be used
# The currently checked out code is built after this list to include the currently supported state versions and
# build the final cannon executable.
VERSIONS=$(git tag --list 'cannon/v*' --sort taggerdate | grep -v -- '-')
for VERSION in ${VERSIONS}
do
  buildVersion "$VERSION"
done

cd "${CANNON_DIR}"
LOG_FILE="${LOGS_DIR}/build-current.txt"
echo "Building current version of cannon Logs: ${LOG_FILE}"
make cannon > "${LOG_FILE}" 2>&1

echo "All cannon versions successfully built and available in ${EMBEDS_DIR}"
"${CANNON_DIR}/bin/cannon" list
