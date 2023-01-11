#!/bin/bash

cluster_template=/tmp/cluster-template.yaml
export CLUSTER_APIENDPOINT_HOST="test-kube-apiserver.metal3.svc.cluster.local"
export CLUSTER_APIENDPOINT_PORT="6443"
# For external ETCD
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
        external:
          endpoints:
            - https://etcd-server.metal3.cluster.svc.local:2379
          caFile: /etc/kubernetes/pki/etcd/ca.crt
          certFile: /etc/kubernetes/pki/apiserver-etcd-client.crt
          keyFile: /etc/kubernetes/pki/apiserver-etcd-client.key"
# For internal ETCD
# export CTLPLANE_KUBEADM_EXTRA_CONFIG="
#     clusterConfiguration:
#       controlPlaneEndpoint: test-kube-apiserver.metal3.svc.cluster.local:6443
#       apiServer:
#         certSANs:
#         - localhost
#         - 127.0.0.1
#         - 0.0.0.0
#         - test-kube-apiserver.metal3.svc.cluster.local
#       etcd:
#         local:
#           serverCertSANs:
#             - etcd-server.metal3.cluster.svc.local
#           peerCertSANs:
#             - etcd-0.etcd.metal3.cluster.svc.local"
export IMAGE_CHECKSUM="97830b21ed272a3d854615beb54cf004"
export IMAGE_CHECKSUM_TYPE="md5"
export IMAGE_FORMAT="raw"
export IMAGE_URL="http://172.22.0.1/images/rhcos-ootpa-latest.qcow2"
export KUBERNETES_VERSION="v1.25.3"
export WORKERS_KUBEADM_EXTRA_CONFIG=""



function create_cluster() {
  cluster="${1:-test-1}"
  namespace="${2:-${cluster}}"

  echo "Creating cluster ${cluster} in namespace ${namespace}"

  # Create namespace and BMH
  kubectl create namespace "${namespace}"
  kubectl -n "${namespace}" apply -f test-hosts.yaml

  # Upload same CA certs as used in the first cluster
  kubectl -n "${namespace}" create secret tls "${cluster}-etcd" --cert /tmp/pki/etcd/peer.crt --key /tmp/pki/etcd/peer.key
  kubectl -n "${namespace}" create secret tls "${cluster}-ca" --cert /tmp/pki/ca.crt --key /tmp/pki/ca.key
  # For external ETCD
  kubectl -n "${namespace}" create secret tls "${cluster}-apiserver-etcd-client" --cert /tmp/pki/apiserver-etcd-client.crt --key /tmp/pki/apiserver-etcd-client.key

  # Create cluster!
  clusterctl generate cluster "${cluster}" \
    --from "${cluster_template}" \
    --target-namespace "${namespace}" | kubectl apply -f -

  # Wait for BMH to be available (or provisioned if rerunning the script)
  bmh_state="$(kubectl -n "${namespace}" get bmh -o jsonpath="{.items[0].status.provisioning.state}")"
  while [[ "${bmh_state}" != "available" ]] && [[ "${bmh_state}" != "provisioned" ]]; do
    # echo "Waiting for BMH to become available. bmh_state: ${bmh_state}"
    sleep 3
    bmh_state="$(kubectl -n "${namespace}" get bmh -o jsonpath="{.items[0].status.provisioning.state}")"
  done

  # Wait for machine
  while ! kubectl -n "${namespace}" get machine -o jsonpath="{.items[0].metadata.name}" &> /dev/null; do
    # echo "Waiting for Machine to exist."
    sleep 5
  done

  # Set correct node name and apply
  machine="$(kubectl -n "${namespace}" get machine -o jsonpath="{.items[0].metadata.name}")"
  # Find UID of BMH by checking the annotation of the m3m that does not yet have a providerID
  # bmh_namespace_name="$(kubectl -n "${namespace}" get m3m -o json | jq -r '.items[] | select(.spec | has("providerID") | not) | .metadata.annotations."metal3.io/BareMetalHost"')"
  # bmh_name="${bmh_namespace_name#*/}"
  # bmh_uid="$(kubectl -n "${namespace}" get bmh "${bmh_name}" -o jsonpath="{.metadata.uid}")"
  # Simplified when working with single node clusters in separate namespaces
  bmh_uid="$(kubectl -n "${namespace}" get bmh -o jsonpath="{.items[0].metadata.uid}")"
  sed "s/fake-node/${machine}/g" examples/fake-node.yaml > "/tmp/${machine}-node.yaml"
  sed -i "s/fake-uuid/${bmh_uid}/g" "/tmp/${machine}-node.yaml"
  kubectl --kubeconfig=kubeconfig-test.yaml create -f "/tmp/${machine}-node.yaml"
  kubectl --kubeconfig=kubeconfig-test.yaml label node "${machine}" node-role.kubernetes.io/control-plane=""
}

num=${1:-10}
# Add more clusters in steps of step.
step="10"
for (( i = 1; i <= num; ++i )); do
  create_cluster "test-$i" &
  if (( $i % $step == 0 )); then
    echo "Waiting for $i clusters to be created in the background."
    wait
  fi
done

wait
echo "Created $num clusters"
