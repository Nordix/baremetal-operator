# Scaling experiments

The `produce-hosts.sh` script can be used to generate lots of BareMetalHosts (and matching secrets):

```bash
./produce-hosts.sh 100 > test-hosts.yaml
```

It takes one argument, which is the number of hosts to produce.
These can then be applied in the cluster:

```bash
kubectl -n metal3 apply -f test-hosts.yaml
```

## Plan for scaling experiment with single node clusters

**Goal:** To run 1000 single node target clusters managed by 1 management cluster with 3 control plane nodes.
It should be possible to create and destroy the clusters.
All target clusters should be in separate namespaces.

- Create 3-node kind cluster ✅
- Run BMO in test-mode (in cluster) ✅
- Script setup of management cluster ✅
- Script creation of target clusters ✅
- Larger cloud-init to check how etcd handles it!

Questions:
- Do we need a separate API server and ETCD for each target cluster?
  - Answer: Not necessarily. The status of the KCPs becomes a bit "weird" but it "works"
- Creating clusters takes too much time. This could be because of the unhealthy KCP. What can we do about this?
  - Try using external ETCD configuration. Then CAPI should not try to health check the etcd pod.
  - Create "real" nodes for all clusters (like KinD or CAPD). This will require a lot of resources...
  - Try to configure CAPI differently?

## Notes

```bash
# ------------------------------------------------------------------
# Create management cluster and first target cluster with API server
# ------------------------------------------------------------------
./examples/setup-scaling-experiment.sh

# ------------------------------
# Create more target clusters...
# ------------------------------
./examples/create-clusters.sh 100
```

### Temp

Certificate expiry and static pods

```bash
cert_expiry_annotation="machine.cluster.x-k8s.io/certificates-expiry"
expiry="2025-01-02T15:04:05Z07:00"
num="100"
for (( i = 1; i <= num; ++i )); do
  namespace="test-$i"
  machine="$(kubectl -n "${namespace}" get machine -o jsonpath="{.items[0].metadata.name}")"
  # Add certificate expiry annotations to make kubeadm control plane manager happy
  # kubectl -n "${namespace}" annotate machine "${machine}" "${cert_expiry_annotation}=${expiry}"
  kubectl -n "${namespace}" annotate kubeadmconfig --all "${cert_expiry_annotation}=${expiry}"
  # Add static pods to make kubeadm control plane manager happy
  sed "s/node-name/${machine}/g" examples/kube-apiserver-pod.yaml > "/tmp/kube-apiserver-${machine}-pod.yaml"
  kubectl --kubeconfig=kubeconfig-test.yaml create -f "/tmp/kube-apiserver-${machine}-pod.yaml"
  sed "s/node-name/${machine}/g" examples/kube-controller-manager-pod.yaml > "/tmp/kube-controller-manager-${machine}-pod.yaml"
  kubectl --kubeconfig=kubeconfig-test.yaml create -f "/tmp/kube-controller-manager-${machine}-pod.yaml"
  sed "s/node-name/${machine}/g" examples/kube-scheduler-pod.yaml > "/tmp/kube-scheduler-${machine}-pod.yaml"
  kubectl --kubeconfig=kubeconfig-test.yaml create -f "/tmp/kube-scheduler-${machine}-pod.yaml"
  # Set status on the pods (it is not added when using create/apply).
  kubectl --kubeconfig=kubeconfig-test.yaml -n kube-system patch pod "kube-apiserver-${machine}" \
    --subresource=status --patch-file=examples/kube-apiserver-pod-status.yaml
  kubectl --kubeconfig=kubeconfig-test.yaml -n kube-system patch pod "kube-controller-manager-${machine}" \
    --subresource=status --patch-file=examples/kube-controller-manager-pod-status.yaml
  kubectl --kubeconfig=kubeconfig-test.yaml -n kube-system patch pod "kube-scheduler-${machine}" \
    --subresource=status --patch-file=examples/kube-scheduler-pod-status.yaml

  rm "/tmp/kube-scheduler-${machine}-pod.yaml"
  rm "/tmp/kube-controller-manager-${machine}-pod.yaml"
  rm "/tmp/kube-apiserver-${machine}-pod.yaml"
done
```

### CAPD scaling for comparison

```bash
cat > kind-cluster-with-extramounts.yaml <<EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  extraMounts:
    - hostPath: /var/run/docker.sock
      containerPath: /var/run/docker.sock
- role: control-plane
  extraMounts:
      - hostPath: /var/run/docker.sock
        containerPath: /var/run/docker.sock
- role: control-plane
  extraMounts:
      - hostPath: /var/run/docker.sock
        containerPath: /var/run/docker.sock
EOF
kind create cluster --config kind-cluster-with-extramounts.yaml
kubectl taint node kind-control-plane node-role.kubernetes.io/control-plane-
kubectl taint node kind-control-plane2 node-role.kubernetes.io/control-plane-
kubectl taint node kind-control-plane3 node-role.kubernetes.io/control-plane-

export CLUSTER_TOPOLOGY=true
clusterctl init --infrastructure=docker
cluster_template="/tmp/cluster-template.yaml"
cluster_class="/tmp/cluster-class.yaml"
wget -O "${cluster_template}" "https://github.com/kubernetes-sigs/cluster-api/releases/download/v1.3.2/cluster-template-development.yaml"
wget -O "${cluster_class}" "https://github.com/kubernetes-sigs/cluster-api/releases/download/v1.3.2/clusterclass-quick-start.yaml"

cluster="test-1"
namespace="${cluster}"
kubectl create namespace "${namespace}"
kubectl -n "${namespace}" create -f "${cluster_class}"
clusterctl generate cluster "${cluster}" --from "${cluster_template}" --kubernetes-version v1.26.0 \
  --control-plane-machine-count=1 --worker-machine-count=0 --target-namespace "${namespace}" | kubectl apply -f -

num="100"
# Add more clusters in steps of step.
step="10"
for (( i = 1; i <= num; ++i )); do
  cluster="test-$i"
  namespace="${cluster}"
  kubectl create namespace "${namespace}"
  kubectl -n "${namespace}" create -f "${cluster_class}"
  clusterctl generate cluster "${cluster}" --from "${cluster_template}" --kubernetes-version v1.26.0 \
    --control-plane-machine-count=1 --worker-machine-count=0 --target-namespace "${namespace}" | kubectl apply -f -
  if (( $i % $step == 0 )); then
    echo "Waiting for $i clusters to be created in the background."
    # Wait for machine
    while [[ "$(kubectl -n "${namespace}" get machine -o jsonpath="{.items[0].status.phase}")" != "Running" ]] &> /dev/null; do
      # echo "Waiting for Machine to exist."
      sleep 5
    done
  fi
done
```

### Issues and TODO

- KCP not fully healthy due to missing control plane pods in the target cluster
- "Weird" replica count for KCP when reusing the same API server and ETCD for all target clusters

### Performance and metrics

- Install metrics-server:
  ```bash
  kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml
  kubectl patch -n kube-system deployment metrics-server --type=json -p '[{"op":"add","path":"/spec/template/spec/containers/0/args/-","value":"--kubelet-insecure-tls"}]'
  ```
- Management cluster KinD, 3 control plane nodes, adding 10 clusters at a time.
  - External ETCD: Scale to 100 clusters in ~15 minutes.
  - Internal ETCD: Scale to 100 clusters in ~30 minutes.
  - External ETCD: Scale to 300 clusters in ~135 minutes.
    - Adding just 1 cluster more at this scale takes more than 8 minutes.
- Single node management cluster, adding 10 clusters at a time.
  - External ETCD: Scale to 100 clusters in ~15 minutes.
- CAPD: 20 clusters in ~5 minutes.
  - 50 clusters is already hitting limits of 32 GB machine. This way of scaling will require a lot of resources.
Resource usage in the management cluster while creating target clusters (~180 created):

```console
vscode ➜ /workspaces/baremetal-operator (lentzi90/scaling-experiments ✗) $ k top pods -A
NAMESPACE                           NAME                                                             CPU(cores)   MEMORY(bytes)
baremetal-operator-system           baremetal-operator-controller-manager-5b9bb4747-pj68b            3m           34Mi
capi-kubeadm-bootstrap-system       capi-kubeadm-bootstrap-controller-manager-78c76cd689-7ng2z       3m           44Mi
capi-kubeadm-control-plane-system   capi-kubeadm-control-plane-controller-manager-785c94c5d4-64f45   548m         730Mi
capi-system                         capi-controller-manager-7b6df78867-4nj7k                         207m         761Mi
capm3-system                        capm3-controller-manager-669989d4-9lp4m                          289m         80Mi
capm3-system                        ipam-controller-manager-65fc446776-qv99w                         2m           15Mi
cert-manager                        cert-manager-74d949c895-b2vlw                                    1m           48Mi
cert-manager                        cert-manager-cainjector-d9bc5979d-8dst8                          1m           51Mi
cert-manager                        cert-manager-webhook-84b7ddd796-qt6vn                            1m           12Mi
kube-system                         coredns-565d847f94-2b9cx                                         5m           18Mi
kube-system                         coredns-565d847f94-7jqnv                                         5m           19Mi
kube-system                         etcd-kind-control-plane                                          194m         146Mi
kube-system                         etcd-kind-control-plane2                                         194m         132Mi
kube-system                         etcd-kind-control-plane3                                         127m         134Mi
kube-system                         kindnet-9vpgt                                                    1m           12Mi
kube-system                         kindnet-jzthr                                                    1m           12Mi
kube-system                         kindnet-nkkdf                                                    1m           10Mi
kube-system                         kube-apiserver-kind-control-plane                                544m         1313Mi
kube-system                         kube-apiserver-kind-control-plane2                               381m         1161Mi
kube-system                         kube-apiserver-kind-control-plane3                               177m         1076Mi
kube-system                         kube-controller-manager-kind-control-plane                       28m          102Mi
kube-system                         kube-controller-manager-kind-control-plane2                      2m           22Mi
kube-system                         kube-controller-manager-kind-control-plane3                      2m           23Mi
kube-system                         kube-proxy-bqmjk                                                 1m           19Mi
kube-system                         kube-proxy-crgzb                                                 1m           12Mi
kube-system                         kube-proxy-zwvwh                                                 1m           13Mi
kube-system                         kube-scheduler-kind-control-plane                                3m           19Mi
kube-system                         kube-scheduler-kind-control-plane2                               3m           23Mi
kube-system                         kube-scheduler-kind-control-plane3                               2m           21Mi
kube-system                         metrics-server-55dd79d7bf-fqsxl                                  4m           17Mi
local-path-storage                  local-path-provisioner-684f458cdd-2qmkv                          1m           7Mi
metal3                              etcd-0                                                           14m          35Mi
metal3                              test-kube-apiserver-69dd6dd947-b7zkn                             111m         375Mi
```

Resource usage "idle" at 200 target clusters:

```console
vscode ➜ /workspaces/baremetal-operator (lentzi90/scaling-experiments ✗) $ k top pods -A
NAMESPACE                           NAME                                                             CPU(cores)   MEMORY(bytes)
baremetal-operator-system           baremetal-operator-controller-manager-5b9bb4747-pj68b            4m           33Mi
capi-kubeadm-bootstrap-system       capi-kubeadm-bootstrap-controller-manager-78c76cd689-7ng2z       2m           41Mi
capi-kubeadm-control-plane-system   capi-kubeadm-control-plane-controller-manager-785c94c5d4-64f45   481m         861Mi
capi-system                         capi-controller-manager-7b6df78867-4nj7k                         189m         887Mi
capm3-system                        capm3-controller-manager-669989d4-9lp4m                          227m         86Mi
capm3-system                        ipam-controller-manager-65fc446776-qv99w                         1m           15Mi
cert-manager                        cert-manager-74d949c895-b2vlw                                    1m           49Mi
cert-manager                        cert-manager-cainjector-d9bc5979d-8dst8                          1m           53Mi
cert-manager                        cert-manager-webhook-84b7ddd796-qt6vn                            1m           13Mi
kube-system                         coredns-565d847f94-2b9cx                                         5m           19Mi
kube-system                         coredns-565d847f94-7jqnv                                         6m           19Mi
kube-system                         etcd-kind-control-plane                                          161m         168Mi
kube-system                         etcd-kind-control-plane2                                         164m         154Mi
kube-system                         etcd-kind-control-plane3                                         100m         157Mi
kube-system                         kindnet-9vpgt                                                    1m           12Mi
kube-system                         kindnet-jzthr                                                    1m           12Mi
kube-system                         kindnet-nkkdf                                                    1m           10Mi
kube-system                         kube-apiserver-kind-control-plane                                476m         1319Mi
kube-system                         kube-apiserver-kind-control-plane2                               367m         1220Mi
kube-system                         kube-apiserver-kind-control-plane3                               172m         1203Mi
kube-system                         kube-controller-manager-kind-control-plane                       26m          107Mi
kube-system                         kube-controller-manager-kind-control-plane2                      2m           22Mi
kube-system                         kube-controller-manager-kind-control-plane3                      1m           23Mi
kube-system                         kube-proxy-bqmjk                                                 1m           19Mi
kube-system                         kube-proxy-crgzb                                                 1m           12Mi
kube-system                         kube-proxy-zwvwh                                                 1m           13Mi
kube-system                         kube-scheduler-kind-control-plane                                3m           19Mi
kube-system                         kube-scheduler-kind-control-plane2                               2m           23Mi
kube-system                         kube-scheduler-kind-control-plane3                               2m           21Mi
kube-system                         metrics-server-55dd79d7bf-fqsxl                                  5m           18Mi
local-path-storage                  local-path-provisioner-684f458cdd-2qmkv                          1m           7Mi
metal3                              etcd-0                                                           13m          36Mi
metal3                              test-kube-apiserver-69dd6dd947-b7zkn                             121m         384Mi
```

Idle at 300 clusters:

```console
vscode ➜ /workspaces/baremetal-operator (lentzi90/scaling-experiments ✗) $ k top pods -A
NAMESPACE                           NAME                                                             CPU(cores)   MEMORY(bytes)
baremetal-operator-system           baremetal-operator-controller-manager-5b9bb4747-kwzpb            6m           36Mi
capi-kubeadm-bootstrap-system       capi-kubeadm-bootstrap-controller-manager-78c76cd689-8jz54       2m           40Mi
capi-kubeadm-control-plane-system   capi-kubeadm-control-plane-controller-manager-77c89fc5bc-tl7w4   968m         1736Mi
capi-system                         capi-controller-manager-7dc886bcd8-kwhmf                         127m         1720Mi
capm3-system                        capm3-controller-manager-669989d4-vn7dp                          61m          110Mi
capm3-system                        ipam-controller-manager-65fc446776-tvqrf                         2m           13Mi
cert-manager                        cert-manager-74d949c895-rdd86                                    1m           57Mi
cert-manager                        cert-manager-cainjector-d9bc5979d-drqxc                          2m           66Mi
cert-manager                        cert-manager-webhook-84b7ddd796-nq46l                            1m           10Mi
kube-system                         coredns-565d847f94-dvw8f                                         2m           15Mi
kube-system                         coredns-565d847f94-gw2hm                                         2m           15Mi
kube-system                         etcd-kind-control-plane                                          88m          159Mi
kube-system                         etcd-kind-control-plane2                                         87m          146Mi
kube-system                         etcd-kind-control-plane3                                         124m         148Mi
kube-system                         kindnet-jw7dd                                                    1m           8Mi
kube-system                         kindnet-l9hhh                                                    1m           8Mi
kube-system                         kindnet-lhn5z                                                    1m           8Mi
kube-system                         kube-apiserver-kind-control-plane                                125m         1379Mi
kube-system                         kube-apiserver-kind-control-plane2                               213m         1322Mi
kube-system                         kube-apiserver-kind-control-plane3                               172m         1310Mi
kube-system                         kube-controller-manager-kind-control-plane                       20m          122Mi
kube-system                         kube-controller-manager-kind-control-plane2                      1m           16Mi
kube-system                         kube-controller-manager-kind-control-plane3                      1m           16Mi
kube-system                         kube-proxy-kzj59                                                 1m           11Mi
kube-system                         kube-proxy-sm8c4                                                 1m           11Mi
kube-system                         kube-proxy-xvztx                                                 1m           11Mi
kube-system                         kube-scheduler-kind-control-plane                                3m           19Mi
kube-system                         kube-scheduler-kind-control-plane2                               2m           17Mi
kube-system                         kube-scheduler-kind-control-plane3                               2m           16Mi
kube-system                         metrics-server-55dd79d7bf-7xg5t                                  3m           21Mi
local-path-storage                  local-path-provisioner-684f458cdd-pwcfg                          1m           7Mi
metal3                              etcd-0                                                           11m          37Mi
metal3                              test-kube-apiserver-69dd6dd947-d6vfx                             135m         429Mi
```


## Example run with only BMO

Setup cluster with CRDs and cert-manager.

```bash
kind create cluster
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.10.1/cert-manager.yaml
kubectl apply -k config/crd
```

Run BMO in test mode (in a separate terminal):

```bash
# If you use docker instead of podman, you need to set CONTAINER_RUNTIME=docker
CONTAINER_RUNTIME=docker make run-test-mode
```

Generate BareMetalHosts and secrets:

```bash
./produce-hosts.sh 3 > test-hosts.yaml
```

Apply and check result.

```bash
kubectl create namespace metal3
kubectl -n metal3 apply -f test-hosts.yaml
```

After a few seconds you can already see them going into `provisioned` state:

```console
$ kubectl -n metal3 get bmh
NAME       STATE          CONSUMER   ONLINE   ERROR   AGE
worker-1   provisioned               true             10s
worker-2   provisioning              true             10s
worker-3   provisioned               true             10s
```

## Experiment with CAPI/CAPM3, API server and etcd for target cluster

```bash
# Setup kind cluster and init metal3
kind create cluster
kubectl apply -k config/crd
clusterctl init --infrastructure=metal3

# In separate terminal
CONTAINER_RUNTIME=docker make run-test-mode

# Create BMHs
./examples/produce-available-hosts.sh 1000 > test-hosts.yaml
kubectl create namespace metal3
kubectl -n metal3 apply -f test-hosts.yaml
# Create Cluster and related resources
kubectl -n metal3 apply -f examples/cluster.yaml
```

### Certificate management for the control plane

```bash
## Generate certificates

# Kubeadm is configured to use /tmp/pki as certificate directory
rm --recursive /tmp/pki
mkdir -p /tmp/pki/etcd

# Get the etcd CA certificate and key.
# This is used by kubeadm to generate etcd peer, server and client certificates
kubectl -n metal3 get secrets test-etcd -o jsonpath="{.data.tls\.crt}" | base64 -d > /tmp/pki/etcd/ca.crt
kubectl -n metal3 get secrets test-etcd -o jsonpath="{.data.tls\.key}" | base64 -d > /tmp/pki/etcd/ca.key
# Get the k8s CA certificate and key.
# This is used by kubeadm to generate the API server certificates
kubectl -n metal3 get secrets test-ca -o jsonpath="{.data.tls\.crt}" | base64 -d > /tmp/pki/ca.crt
kubectl -n metal3 get secrets test-ca -o jsonpath="{.data.tls\.key}" | base64 -d > /tmp/pki/ca.key

# Generate certificates
kubeadm init phase certs etcd-peer --config examples/kubeadm-config.yaml
kubeadm init phase certs etcd-server --config examples/kubeadm-config.yaml
kubeadm init phase certs apiserver-etcd-client --config examples/kubeadm-config.yaml
kubeadm init phase certs apiserver --config examples/kubeadm-config.yaml

# Create secrets
kubectl -n metal3 create secret tls etcd-peer --cert /tmp/pki/etcd/peer.crt --key /tmp/pki/etcd/peer.key
kubectl -n metal3 create secret tls etcd-server --cert /tmp/pki/etcd/server.crt --key /tmp/pki/etcd/server.key
kubectl -n metal3 create secret tls apiserver-etcd-client --cert /tmp/pki/apiserver-etcd-client.crt --key /tmp/pki/apiserver-etcd-client.key
kubectl -n metal3 create secret tls apiserver --cert /tmp/pki/apiserver.crt --key /tmp/pki/apiserver.key

# Deploy etcd and API server
kubectl -n metal3 apply -f examples/etcd.yaml
kubectl -n metal3 apply -f examples/kube-apiserver-deployment.yaml
```

### Add initial fake node and kubeadm config

```bash
# Get kubeconfig
clusterctl -n metal3 get kubeconfig test > kubeconfig-test.yaml
# Edit kubeconfig to point to 127.0.0.1:6443 and set up port forward to the pod
sed -i s/test-kube-apiserver.metal3.svc.cluster.local/127.0.0.1/ kubeconfig-test.yaml
# Port forward in separate terminal!
kubectl -n metal3 port-forward svc/test-kube-apiserver 6443

# Set correct node name and apply
machine="$(kubectl -n metal3 get machine -o jsonpath="{.items[0].metadata.name}")"
# Find UID of BMH by checking the annotation of the m3m that does not yet have a providerID
bmh_namespace_name="$(kubectl -n metal3 get m3m -o json | jq -r '.items[] | select(.spec | has("providerID") | not) | .metadata.annotations."metal3.io/BareMetalHost"')"
bmh_name="${bmh_namespace_name#*/}"
bmh_uid="$(kubectl -n metal3 get bmh "${bmh_name}" -o jsonpath="{.metadata.uid}")"
sed "s/fake-node/${machine}/g" examples/fake-node.yaml > temp-node.yaml
sed -i "s/fake-uuid/${bmh_uid}/g" temp-node.yaml
kubectl --kubeconfig=kubeconfig-test.yaml create -f temp-node.yaml
kubectl --kubeconfig=kubeconfig-test.yaml label node "${machine}" node-role.kubernetes.io/control-plane=""
# Upload kubeadm config to configmap. This will mark the KCP as initialized.
kubectl --kubeconfig=kubeconfig-test.yaml -n kube-system create cm kubeadm-config \
  --from-file=ClusterConfiguration=examples/kubeadm-config.yaml

# Check result
clusterctl -n metal3 describe cluster test
# watch -n 10 clusterctl -n metal3 describe cluster test
```

### Scaling with fake-controller

```bash
# Start the fake-controller in a separate terminal
./examples/fake-controller.sh

# Scale the MD and watch the Machines successfully provision!
kubectl -n metal3 scale md test --replicas=x
```

## Caveats

- The KCP will have some issues since it is not "real", including unknown health for etcd and such.
  This also means that scaling the KCP does not work.

There is some kind of issue when doing rolling upgrades.
Probably something to do with workload cluster Nodes not deleted or there is some race condition in BMO when doing rolling upgrades.
