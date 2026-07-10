#!/usr/bin/env bash

# Helper script to compute the SHA-256 of sysrescue-customize at a given commit.
# Usage:
#   ./hack/e2e/get-sysrescue-customize-sha256.sh [COMMIT]
#
# If COMMIT is omitted, the latest commit touching the script on main is used.

set -eu -o pipefail

SCRIPT_PATH="airootfs/usr/share/sysrescue/bin/sysrescue-customize"
PROJECT_URL="https://gitlab.com/systemrescue/systemrescue-sources"

COMMIT="${1:-}"

if [[ -z "${COMMIT}" ]]; then
    echo "No commit specified, fetching latest from main..." >&2
    COMMIT="$(curl --proto '=https' --tlsv1.3 -sSf \
        "https://gitlab.com/api/v4/projects/systemrescue%2Fsystemrescue-sources/repository/files/${SCRIPT_PATH//\//%2F}?ref=main" \
        | python3 -c "import json,sys; print(json.load(sys.stdin)['last_commit_id'])")"
    echo "Latest commit: ${COMMIT}" >&2
fi

echo "Downloading sysrescue-customize at commit ${COMMIT}..." >&2
SHA256="$(curl --proto '=https' --tlsv1.3 -sSfL \
    "${PROJECT_URL}/-/raw/${COMMIT}/${SCRIPT_PATH}" \
    | sha256sum | awk '{print $1;}')"

echo "" >&2
echo "Update hack/ci-e2e.sh with:" >&2
echo "SYSRESCUE_CUSTOMIZE_COMMIT=\"${COMMIT}\""
echo "SYSRESCUE_CUSTOMIZE_SHA256=\"${SHA256}\""
