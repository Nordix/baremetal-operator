#!/usr/bin/env bash

# Copyright 2021 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -eu -o pipefail

USR_LOCAL_BIN="/usr/local/bin"
MINIMUM_KUBECTL_VERSION=v1.34.1
KUBECTL_DOWNLOAD_URL="https://dl.k8s.io/release"

# Verify mode turned off by default
VERIFY_ONLY="${VERIFY_ONLY:-false}"

# Ensure the kubectl tool exists and is a viable version, or installs it
verify_kubectl_version() {
    # If kubectl is not available on the path, get it
    KUBECTL="$(command -v kubectl || true)"
    if ! [[ -x "${KUBECTL}" ]]; then
        if [[ "${VERIFY_ONLY}" != "false" ]]; then
            echo "kubectl is not in PATH"
            return 0
        fi
        if [[ "${OSTYPE}" == "linux-gnu" ]]; then
            if ! command -v sha256sum &>/dev/null; then
                echo "ERROR: sha256sum not found" >&2
                return 1
            fi

            echo "kubectl not found, installing"
            local tmp_dir
            tmp_dir="$(mktemp -d)"
            trap 'rm -rf "${tmp_dir}"; trap - RETURN' RETURN

            local BINARY_FILE="kubectl"
            local URL="${KUBECTL_DOWNLOAD_URL}/${MINIMUM_KUBECTL_VERSION}/bin/linux/amd64/${BINARY_FILE}"
            local CHECKSUMS_URL="${URL}.sha256"

            # Download checksum file (Kubernetes publishes .sha256 alongside binaries)
            curl --proto '=https' --tlsv1.3 -sSfL \
                --retry 3 --retry-delay 5 --max-time 120 \
                -o "${tmp_dir}/${BINARY_FILE}.sha256" "${CHECKSUMS_URL}"

            # Extract expected checksum (single hash in file)
            local expected_checksum
            expected_checksum="$(cat "${tmp_dir}/${BINARY_FILE}.sha256")"
            if [[ -z "${expected_checksum}" ]]; then
                echo >&2 "fatal: could not find checksum at ${CHECKSUMS_URL}"
                return 1
            fi

            # Download binary with security flags
            curl --proto '=https' --tlsv1.3 -sSfL \
                --retry 3 --retry-delay 5 --max-time 120 \
                -o "${tmp_dir}/${BINARY_FILE}" "${URL}"

            # Verify checksum before installation
            local checksum
            checksum="$(sha256sum "${tmp_dir}/${BINARY_FILE}" | awk '{print $1;}')"
            if [[ "${checksum}" != "${expected_checksum}" ]]; then
                echo >&2 "fatal: ${URL} checksum '${checksum}' differs from expected '${expected_checksum}'"
                return 1
            fi

            sudo install "${tmp_dir}/${BINARY_FILE}" "${USR_LOCAL_BIN}/kubectl"
            KUBECTL="$(command -v kubectl)"
        else
            echo "ERROR: Missing required binary in path: kubectl"
            return 2
        fi
    fi

    local kubectl_version
    IFS=" " read -ra kubectl_version <<< "$("${KUBECTL}" version --client)"
    if [[ "${MINIMUM_KUBECTL_VERSION}" != $(
        echo -e "${MINIMUM_KUBECTL_VERSION}\n${kubectl_version[2]}" \
        | sort -s -t. -k 1,1 -k 2,2n -k 3,3n \
        | head -n1
    ) ]]; then
        cat << EOF
Detected kubectl version: ${kubectl_version[2]}.
Requires ${MINIMUM_KUBECTL_VERSION} or greater.
Please install ${MINIMUM_KUBECTL_VERSION} or later.
EOF
        return 2
    else
        echo "kubectl version ${kubectl_version[2]} is installed at ${KUBECTL}"
    fi
}

verify_kubectl_version
