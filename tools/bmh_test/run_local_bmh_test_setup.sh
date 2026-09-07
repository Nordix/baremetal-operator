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

# vbmctl needs CAP_NET_ADMIN to create the veth pair that bridges the libvirt
# network to the "kind" Docker network (see tools/bmh_test/vbmctl.yaml).
sudo setcap cap_net_admin+eip "${VBMCTL}"

# Create the "baremetal-e2e" libvirt network (bridge "metal3" at 192.168.222.1/24,
# matching hack/e2e/net.xml), bridge it to a Docker network named "kind" so that
# a kind cluster ends up able to reach it, and start a sushy-tools BMC emulator
# container that serves Redfish for VMs on the host via the local libvirt socket.
# See tools/bmh_test/vbmctl.yaml.
#
# NOTE: this must run *before* `kind create cluster`, so kind can detect and
# reuse the pre-created "kind" Docker network instead of creating its own.
"${VBMCTL}" -c "${REPO_ROOT}/tools/bmh_test/vbmctl.yaml" create bml
