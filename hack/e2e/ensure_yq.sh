#!/usr/bin/env bash

set -eu -o pipefail

USR_LOCAL_BIN="/usr/local/bin"
YQ_VERSION="v4.40.5"
YQ_DOWNLOAD_URL="https://github.com/mikefarah/yq/releases/download"

# Verify mode turned off by default
VERIFY_ONLY="${VERIFY_ONLY:-false}"

# Check if yq tool is installed and install it if not
verify_yq()
{
    YQ="$(command -v yq || true)"
    if ! [[ -x "${YQ}" ]]; then
        if [[ "${VERIFY_ONLY}" != "false" ]]; then
            echo "yq is not in PATH"
            return 0
        fi
        if [[ "${OSTYPE}" == "linux-gnu" ]]; then
            if ! command -v sha256sum &>/dev/null; then
                echo "ERROR: sha256sum not found" >&2
                return 1
            fi

            echo "yq not found, installing"
            local tmp_dir
            tmp_dir="$(mktemp -d)"
            trap 'rm -rf "${tmp_dir}"; trap - RETURN' RETURN

            local BINARY_FILE="yq_linux_amd64.tar.gz"
            local RELEASE_URL="${YQ_DOWNLOAD_URL}/${YQ_VERSION}"
            local URL="${RELEASE_URL}/${BINARY_FILE}"

            # Download the checksums_hashes_order file to determine SHA-256 column
            curl --proto '=https' --tlsv1.3 -sSfL \
                --retry 3 --retry-delay 5 --max-time 120 \
                -o "${tmp_dir}/checksums_hashes_order" \
                "${RELEASE_URL}/checksums_hashes_order"

            # Find which line SHA-256 is on (= column offset in checksums file)
            local sha256_line
            sha256_line="$(grep -n "^SHA-256$" "${tmp_dir}/checksums_hashes_order" | cut -d: -f1)"
            if [[ -z "${sha256_line}" ]]; then
                echo >&2 "fatal: SHA-256 not found in checksums_hashes_order"
                return 1
            fi
            # awk column = line number + 1 (column 1 is the filename in checksums file)
            local sha256_col=$((sha256_line + 1))

            # Download the checksums file
            curl --proto '=https' --tlsv1.3 -sSfL \
                --retry 3 --retry-delay 5 --max-time 120 \
                -o "${tmp_dir}/checksums" \
                "${RELEASE_URL}/checksums"

            # Extract the SHA-256 for our binary
            local expected_checksum
            expected_checksum="$(grep -F -- "${BINARY_FILE}" "${tmp_dir}/checksums" | awk "{print \$${sha256_col}}")"
            if [[ -z "${expected_checksum}" ]]; then
                echo >&2 "fatal: could not find checksum for ${BINARY_FILE} in ${RELEASE_URL}/checksums"
                return 1
            fi

            # Download binary with security flags
            curl --proto '=https' --tlsv1.3 -sSfL \
                --retry 3 --retry-delay 5 --max-time 120 \
                -o "${tmp_dir}/${BINARY_FILE}" "${URL}"

            # Verify checksum before extraction
            local checksum
            checksum="$(sha256sum "${tmp_dir}/${BINARY_FILE}" | awk '{print $1;}')"
            if [[ "${checksum}" != "${expected_checksum}" ]]; then
                echo >&2 "fatal: ${URL} checksum '${checksum}' differs from expected '${expected_checksum}'"
                return 1
            fi

            tar -xvf "${tmp_dir}/${BINARY_FILE}" -C "${tmp_dir}"
            sudo install "${tmp_dir}/yq_linux_amd64" "${USR_LOCAL_BIN}/yq"
        else
            echo "ERROR: Missing required binary in path: yq"
            return 2
        fi
    else
        echo "$(yq --version) is installed at ${YQ}"
    fi
}

verify_yq
