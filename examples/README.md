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
./examples/produce-available-hosts.sh 3 > test-hosts.yaml
kubectl create namespace metal3
kubectl -n metal3 apply -f test-hosts.yaml

kubectl -n metal3 apply -f examples/cluster.yaml
```

### Certificate management for the control plane

```bash
# Get the etcd CA certificate and key.
# This is used by kubeadm to generate etcd peer, server and client certificates
kubectl -n metal3 get secrets test-etcd -o jsonpath="{.data.tls\.crt}" | base64 -d > etcd-ca-tls.crt
kubectl -n metal3 get secrets test-etcd -o jsonpath="{.data.tls\.key}" | base64 -d > etcd-ca-tls.key
# Move it to where kubeadm expects it to be
sudo mkdir -p /etc/kubernetes/pki/etcd
sudo cp etcd-ca-tls.key /etc/kubernetes/pki/etcd/ca.key
sudo cp etcd-ca-tls.crt /etc/kubernetes/pki/etcd/ca.crt

# Generate peer certificate and upload to a secret
sudo kubeadm init phase certs etcd-peer --config examples/kubeadm-config.yaml
sudo cp /etc/kubernetes/pki/etcd/peer.key peer.key
sudo cp /etc/kubernetes/pki/etcd/peer.crt peer.crt
sudo chown "${USER}":"${USER}" peer.key
sudo chown "${USER}":"${USER}" peer.crt
kubectl -n metal3 create secret tls etcd-peer --cert peer.crt --key peer.key

# Generate server certificate and upload to a secret
sudo kubeadm init phase certs etcd-server --config examples/kubeadm-config.yaml
sudo cp /etc/kubernetes/pki/etcd/server.key server.key
sudo cp /etc/kubernetes/pki/etcd/server.crt server.crt
sudo chown "${USER}":"${USER}" server.key
sudo chown "${USER}":"${USER}" server.crt
kubectl -n metal3 create secret tls etcd-server --cert server.crt --key server.key

# Generate client certificate and upload to a secret
sudo kubeadm init phase certs apiserver-etcd-client --config examples/kubeadm-config.yaml
sudo cp /etc/kubernetes/pki/apiserver-etcd-client.key apiserver-etcd-client.key
sudo cp /etc/kubernetes/pki/apiserver-etcd-client.crt apiserver-etcd-client.crt
sudo chown "${USER}":"${USER}" apiserver-etcd-client.key
sudo chown "${USER}":"${USER}" apiserver-etcd-client.crt
kubectl -n metal3 create secret tls apiserver-etcd-client --cert apiserver-etcd-client.crt --key apiserver-etcd-client.key

# Get the k8s CA certificate and key.
# This is used by kubeadm to generate the API server certificates
kubectl -n metal3 get secrets test-ca -o jsonpath="{.data.tls\.crt}" | base64 -d > ca-tls.crt
kubectl -n metal3 get secrets test-ca -o jsonpath="{.data.tls\.key}" | base64 -d > ca-tls.key
# Move it to where kubeadm expects it to be
sudo mkdir -p /etc/kubernetes/pki
sudo cp ca-tls.key /etc/kubernetes/pki/ca.key
sudo cp ca-tls.crt /etc/kubernetes/pki/ca.crt

# Generate API server certificate and upload to a secret
sudo kubeadm init phase certs apiserver --config examples/kubeadm-config.yaml
sudo cp /etc/kubernetes/pki/apiserver.key apiserver.key
sudo cp /etc/kubernetes/pki/apiserver.crt apiserver.crt
sudo chown "${USER}":"${USER}" apiserver.key
sudo chown "${USER}":"${USER}" apiserver.crt
kubectl -n metal3 create secret tls apiserver --cert apiserver.crt --key apiserver.key

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
# In separate terminal!
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
# Upload kubeadm config to configmap. This will mark the KCP as initialized.
kubectl --kubeconfig=kubeconfig-test.yaml -n kube-system create cm kubeadm-config \
  --from-file=ClusterConfiguration=examples/kubeadm-config.yaml

# Check result
clusterctl -n metal3 describe cluster test
```

### Scaling

```bash
kubectl -n metal3 scale md test --replicas=x

# Find provisioning machine
provisioning_machine="$(kubectl -n metal3 get machine -o json | jq -r '.items[] | select(.status.phase == "Provisioning") | .metadata.name')"
# Find UID of BMH by checking the annotation of the m3m that does not yet have a providerID
bmh_namespace_name="$(kubectl -n metal3 get m3m -o json | jq -r '.items[] | select(.spec | has("providerID") | not) | .metadata.annotations."metal3.io/BareMetalHost"')"
bmh_name="${bmh_namespace_name#*/}"
bmh_uid="$(kubectl -n metal3 get bmh "${bmh_name}" -o jsonpath="{.metadata.uid}")"
sed "s/fake-node/${provisioning_machine}/g" examples/fake-node.yaml > temp-node.yaml
sed -i "s/fake-uuid/${bmh_uid}/g" temp-node.yaml
kubectl --kubeconfig=kubeconfig-test.yaml create -f temp-node.yaml
```
