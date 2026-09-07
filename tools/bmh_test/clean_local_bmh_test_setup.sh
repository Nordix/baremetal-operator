#!/usr/bin/env bash

set -ux

REPO_ROOT=$(realpath "$(dirname "${BASH_SOURCE[0]}")/../..")
VBMCTL="${REPO_ROOT}/bin/vbmctl"

BMH_NAME_REGEX="${1:-^bmh-test-}"
# Get a list of all virtual machines
VM_LIST=$(virsh -c qemu:///system list --all --name | grep "${BMH_NAME_REGEX}")

if [[ -n "${VM_LIST}" ]]; then
    # Loop through the list and delete each virtual machine (and its volumes)
    for vm_name in ${VM_LIST}; do
        "${VBMCTL}" delete vm "${vm_name}"
        # --wait=false: without a reachable Ironic endpoint the controller can
        # never finish deprovisioning, so the finalizer is never removed and a
        # blocking delete would hang forever.
        kubectl delete baremetalhost "${vm_name}" --wait=false
    done
else
    echo "No virtual machines found. Skipping..."
fi

# Clear the sushy-tools BMC emulator, the "baremetal-e2e" libvirt network, and
# the veth pair/Docker network bridging it to kind (see tools/bmh_test/vbmctl.yaml).
"${VBMCTL}" -c "${REPO_ROOT}/tools/bmh_test/vbmctl.yaml" delete bml

# Cleanup VM and volume qcow2
rm -rf /tmp/bmo-e2e-*.qcow2
rm -rf /tmp/pool_oo/bmo-e2e-*.qcow2
