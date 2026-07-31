#!/usr/bin/env bash

# -----------------------------------------------------------------------------
# Description: IP-addresses used in the e2e tests collected into single file.
# Usage: The file can be executed or sourced.
# -----------------------------------------------------------------------------

set -eu

if [[ "${USE_IPV6}" = true ]]; then
	export IP_ADDRESS="fd55::1"
	export HOST_ADDRESS="[fd55::1]"
	export SUBNET_MASK="64"
	export UPGRADE_IRONIC_PROVISIONING_IP="fd55::3"
	# IP address where Ironic is available
	export IRONIC_PROVISIONING_IP="fd55::2"
	export IRONIC_PROVISIONING_HOST="[fd55::2]"
	# Values for IrSO Ironic
	export IRONIC_DHCP_RANGE_BEGIN="fd55::100"
	export IRONIC_DHCP_RANGE_END="fd55::200"
	export IRONIC_NETWORK_CIDR="fd55::/64"
	# IP addresses for the VMs
	# export VM_HOST1_IP="fd55::120, fd55::121, fd55::122, fd55::123"
	# export VM_HOST2_IP="fd55::124, fd55::125, fd55::126, fd55::127"
	# export VM_HOST1_HOST="[fd55::120/126]"
	# export VM_HOST2_HOST="[fd55::124/126]"
	export VM_HOST1_IP="fd55::123"
	export VM_HOST2_IP="fd55::126"
	export VM_HOST1_HOST="[fd55::122]"
	export VM_HOST2_HOST="[fd55::126]"
	export IP_FAMILY="ipv6"
else
	# Sushy-tools / image server endpoint, not ironic
	export IP_ADDRESS="192.168.222.1"
	export HOST_ADDRESS="192.168.222.1"
	export SUBNET_MASK="24"
	export UPGRADE_IRONIC_PROVISIONING_IP="192.168.222.3"
	# IP address where Ironic is available
	export IRONIC_PROVISIONING_IP="192.168.222.2"
	export IRONIC_PROVISIONING_HOST="192.168.222.2"
	# Values for IrSO Ironic
	export IRONIC_DHCP_RANGE_BEGIN="192.168.222.100"
	export IRONIC_DHCP_RANGE_END="192.168.222.200"
	export IRONIC_NETWORK_CIDR="192.168.222.0/24"
	# IP addresses for the VMs
	export VM_HOST1_IP="192.168.222.122"
	export VM_HOST2_IP="192.168.222.124"
	export VM_HOST1_HOST="192.168.222.122"
	export VM_HOST2_HOST="192.168.222.123"
	export IP_FAMILY="ipv4"
fi
