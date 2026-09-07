#!/usr/bin/env bash

set -eux

REPO_ROOT=$(realpath "$(dirname "${BASH_SOURCE[0]}")/../..")

cd "${REPO_ROOT}" || exit 1

# List of packages to check
commands=("virsh")

# Check each package
for cmd in "${commands[@]}"; do
    if ! command -v "${cmd}" &> /dev/null; then
        echo "ERROR: ${cmd} not found. Please install it."
        exit 1
    fi
done

VBMCTL="${REPO_ROOT}/bin/vbmctl"
if [[ ! -x "${VBMCTL}" ]]; then
    make -C "${REPO_ROOT}" build-vbmctl
fi

# Create the "baremetal-e2e" libvirt network (bridge "metal3" at 192.168.222.1/24,
# matching hack/e2e/net.xml) and start a sushy-tools BMC emulator container that
# serves Redfish for VMs on the host via the local libvirt socket. Both use
# vbmctl's built-in defaults, so no config file is required for this simple setup.
"${VBMCTL}" create network
"${VBMCTL}" create bmc-emulator --emulator-type sushy-tools
