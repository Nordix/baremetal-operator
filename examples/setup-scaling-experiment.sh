#!/bin/bash

# Setup kind cluster and init metal3
kind create cluster --config examples/kind-config.yaml
kubectl apply -k config/crd
kubectl taint node kind-control-plane node-role.kubernetes.io/control-plane-
kubectl taint node kind-control-plane2 node-role.kubernetes.io/control-plane-
kubectl taint node kind-control-plane3 node-role.kubernetes.io/control-plane-
clusterctl init --infrastructure=metal3

# Deploy BMO in test-mode
kubectl create ns baremetal-operator-system
kubectl apply -k config/default
kubectl -n baremetal-operator-system wait --for=condition=Available deploy/baremetal-operator-controller-manager

# Create 1 BMH (can be applied in multiple namespaces)
./examples/produce-available-hosts.sh 1 > test-hosts.yaml

# Download cluster-template
cluster_template=/tmp/cluster-template.yaml
# https://github.com/metal3-io/cluster-api-provider-metal3/blob/main/examples/clusterctl-templates/clusterctl-cluster.yaml
cluster_template_url="https://raw.githubusercontent.com/metal3-io/cluster-api-provider-metal3/main/examples/clusterctl-templates/clusterctl-cluster.yaml"
wget -O "${cluster_template}" "${cluster_template_url}"

## Create new target cluster
## -------------------------

# Cluster template required variables
export CLUSTER_APIENDPOINT_HOST="test-kube-apiserver.metal3.svc.cluster.local"
export CLUSTER_APIENDPOINT_PORT="6443"
export CTLPLANE_KUBEADM_EXTRA_CONFIG="
    clusterConfiguration:
      controlPlaneEndpoint: test-kube-apiserver.metal3.svc.cluster.local:6443
      apiServer:
        certSANs:
        - localhost
        - 127.0.0.1
        - 0.0.0.0
        - test-kube-apiserver.metal3.svc.cluster.local
      etcd:
        local:
          serverCertSANs:
            - etcd-server.metal3.cluster.svc.local
          peerCertSANs:
            - etcd-0.etcd.metal3.cluster.svc.local"
export IMAGE_CHECKSUM="97830b21ed272a3d854615beb54cf004"
export IMAGE_CHECKSUM_TYPE="md5"
export IMAGE_FORMAT="raw"
export IMAGE_URL="http://172.22.0.1/images/rhcos-ootpa-latest.qcow2"
export KUBERNETES_VERSION="v1.25.3"
export WORKERS_KUBEADM_EXTRA_CONFIG=""

## Create first cluster and fake API
cluster="test"
namespace="metal3"
kubectl create namespace "${namespace}"

# It could be that the webhook is still not ready. Retry a few times
num=10
for (( i = 1; i <= num; ++i )); do
  if kubectl -n "${namespace}" apply -f test-hosts.yaml; then
    break
  fi
  sleep 2
done

# Create cluster!
clusterctl generate cluster "${cluster}" \
  --from "${cluster_template}" \
  --target-namespace "${namespace}" | kubectl apply -f -

## Generate certificates

# Kubeadm is configured to use /tmp/pki as certificate directory
rm --recursive /tmp/pki
mkdir -p /tmp/pki/etcd

# Wait for certificates
while ! kubectl -n "${namespace}" get secrets "${cluster}-etcd" "${cluster}-ca" &> /dev/null; do
  echo "Waiting for certificates to exist."
  sleep 1
done

# Get the etcd CA certificate and key.
# This is used by kubeadm to generate etcd peer, server and client certificates
kubectl -n "${namespace}" get secrets "${cluster}-etcd" -o jsonpath="{.data.tls\.crt}" | base64 -d > /tmp/pki/etcd/ca.crt
kubectl -n "${namespace}" get secrets "${cluster}-etcd" -o jsonpath="{.data.tls\.key}" | base64 -d > /tmp/pki/etcd/ca.key
# Get the k8s CA certificate and key.
# This is used by kubeadm to generate the API server certificates
kubectl -n "${namespace}" get secrets "${cluster}-ca" -o jsonpath="{.data.tls\.crt}" | base64 -d > /tmp/pki/ca.crt
kubectl -n "${namespace}" get secrets "${cluster}-ca" -o jsonpath="{.data.tls\.key}" | base64 -d > /tmp/pki/ca.key

# Generate certificates
kubeadm init phase certs etcd-peer --config examples/kubeadm-config.yaml
kubeadm init phase certs etcd-server --config examples/kubeadm-config.yaml
kubeadm init phase certs apiserver-etcd-client --config examples/kubeadm-config.yaml
kubeadm init phase certs apiserver --config examples/kubeadm-config.yaml

# Create secrets
kubectl -n "${namespace}" create secret tls etcd-peer --cert /tmp/pki/etcd/peer.crt --key /tmp/pki/etcd/peer.key
kubectl -n "${namespace}" create secret tls etcd-server --cert /tmp/pki/etcd/server.crt --key /tmp/pki/etcd/server.key
kubectl -n "${namespace}" create secret tls apiserver-etcd-client --cert /tmp/pki/apiserver-etcd-client.crt --key /tmp/pki/apiserver-etcd-client.key
kubectl -n "${namespace}" create secret tls apiserver --cert /tmp/pki/apiserver.crt --key /tmp/pki/apiserver.key

# Deploy etcd and API server
kubectl -n "${namespace}" apply -f examples/etcd.yaml
kubectl -n "${namespace}" apply -f examples/kube-apiserver-deployment.yaml
kubectl -n "${namespace}" wait --for=condition=Available deploy/test-kube-apiserver

# Get kubeconfig
clusterctl -n "${namespace}" get kubeconfig test > kubeconfig-test.yaml
# Edit kubeconfig to point to 127.0.0.1:6443 and set up port forward to the pod
sed -i s/test-kube-apiserver.metal3.svc.cluster.local/127.0.0.1/ kubeconfig-test.yaml
# In background
kubectl -n "${namespace}" port-forward svc/test-kube-apiserver 6443 &

# Wait for machine
while ! kubectl -n "${namespace}" get machine -o jsonpath="{.items[0].metadata.name}" &> /dev/null; do
  echo "Waiting for Machine to exist."
  sleep 1
done
# Set correct node name and apply
machine="$(kubectl -n "${namespace}" get machine -o jsonpath="{.items[0].metadata.name}")"
# Find UID of BMH by checking the annotation of the m3m that does not yet have a providerID
bmh_namespace_name="$(kubectl -n "${namespace}" get m3m -o json | jq -r '.items[] | select(.spec | has("providerID") | not) | .metadata.annotations."metal3.io/BareMetalHost"')"
bmh_name="${bmh_namespace_name#*/}"
bmh_uid="$(kubectl -n "${namespace}" get bmh "${bmh_name}" -o jsonpath="{.metadata.uid}")"
sed "s/fake-node/${machine}/g" examples/fake-node.yaml > temp-node.yaml
sed -i "s/fake-uuid/${bmh_uid}/g" temp-node.yaml
kubectl --kubeconfig=kubeconfig-test.yaml create -f temp-node.yaml
kubectl --kubeconfig=kubeconfig-test.yaml label node "${machine}" node-role.kubernetes.io/control-plane=""
# Upload kubeadm config to configmap. This will mark the KCP as initialized.
kubectl --kubeconfig=kubeconfig-test.yaml -n kube-system create cm kubeadm-config \
  --from-file=ClusterConfiguration=examples/kubeadm-config.yaml
