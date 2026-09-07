#!/usr/bin/env bash

# -------------------------------------------------------------------------------------------
# Description:    This script creates a virtual machine via vbmctl and registers it as a
#                 BareMetalHost using the sushy-tools BMC emulator (Redfish virtual media).
#
# Usage:          make tilt-up -> press button in the right upper corner to create bmhs
#                 tools/bmh_test/create_bmh.sh <NAME> [CONSUMER] [CONSUMER_NAMESPACE]
#
# Prerequisites:  a "baremetal-e2e" libvirt network and a sushy-tools BMC emulator running
#                 (see tools/bmh_test/run_local_bmh_test_setup.sh)
# -------------------------------------------------------------------------------------------

set -euxo pipefail

REPO_ROOT=$(realpath "$(dirname "${BASH_SOURCE[0]}")/../..")
VBMCTL="${REPO_ROOT}/bin/vbmctl"

cd "${REPO_ROOT}" || exit 1

# Set default values
NAME="bmh-test-${1:?}"
CONSUMER="${2:-}"
CONSUMER_NAMESPACE="${3:-}"

# Address of the sushy-tools BMC emulator, matching vbmctl's defaults as used
# by run_local_bmh_test_setup.sh (create bmc-emulator --emulator-type sushy-tools).
SUSHY_ADDRESS="${SUSHY_ADDRESS:-192.168.222.1}"
SUSHY_PORT="${SUSHY_PORT:-8000}"

# Generate a random MAC address for the VM's network interface
MAC_ADDRESS="$(printf '00:60:2F:%02X:%02X:%02X\n' $((RANDOM%256)) $((RANDOM%256)) $((RANDOM%256)))"

# Create a virtual machine, attached to the "baremetal-e2e" libvirt network
# (see run_local_bmh_test_setup.sh). sushy-tools identifies systems by their
# libvirt domain UUID, so look that up once the domain has been defined.
"${VBMCTL}" create vm --name "${NAME}" --network baremetal-e2e --mac-address "${MAC_ADDRESS}"
VM_UUID="$(virsh -c qemu:///system domuuid "${NAME}")"

BMC_ADDRESS="redfish-virtualmedia+http://${SUSHY_ADDRESS}:${SUSHY_PORT}/redfish/v1/Systems/${VM_UUID}"

# Create a YAML file to generate Kubernetes configuration for the VM
# Apply the generated YAML file to the cluster
if [[ -n "${CONSUMER}" ]] && [[ -n "${CONSUMER_NAMESPACE}" ]]; then
  echo "Applying YAML for controlplane host..."
  cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Secret
metadata:
  name: ${NAME}-bmc-secret
type: Opaque
stringData:
  username: admin
  password: password

---
apiVersion: metal3.io/v1alpha1
kind: BareMetalHost
metadata:
  name: ${NAME}
spec:
  online: true
  bmc:
    address: ${BMC_ADDRESS}
    credentialsName: ${NAME}-bmc-secret
  bootMACAddress: ${MAC_ADDRESS}
  consumerRef:
    name: ${CONSUMER}
    namespace: ${CONSUMER_NAMESPACE}
EOF
else
  echo "Applying YAML for host..."
  cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Secret
metadata:
  name: ${NAME}-bmc-secret
type: Opaque
stringData:
  username: admin
  password: password

---
apiVersion: metal3.io/v1alpha1
kind: BareMetalHost
metadata:
  name: ${NAME}
spec:
  online: true
  bmc:
    address: ${BMC_ADDRESS}
    credentialsName: ${NAME}-bmc-secret
  bootMACAddress: ${MAC_ADDRESS}
EOF
fi
