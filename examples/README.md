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
./examples/produce-available-hosts.sh 1000 > test-hosts.yaml
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
kubectl --kubeconfig=kubeconfig-test.yaml label node "${machine}" node-role.kubernetes.io/control-plane=""
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

```console
$ k -n metal3 get bmh | grep provisioning
worker-465    deprovisioning   test-workers-6llbf        false            21h
worker-604    provisioning     test-workers-new-km5cp    true             21h
$ k -n metal3 describe bmh worker-604
Name:         worker-604
Namespace:    metal3
Labels:       cluster.x-k8s.io/cluster-name=test
Annotations:  <none>
API Version:  metal3.io/v1alpha1
Kind:         BareMetalHost
Metadata:
  Creation Timestamp:  2022-12-21T13:33:10Z
  Finalizers:
    baremetalhost.metal3.io
  Generation:  7
  Managed Fields:
    API Version:  metal3.io/v1alpha1
    Fields Type:  FieldsV1
    fieldsV1: ...
    Manager:      main
    Operation:    Update
    Time:         2022-12-21T13:33:39Z
    API Version:  metal3.io/v1alpha1
    Fields Type:  FieldsV1
    fieldsV1: ...
    Manager:      cluster-api-provider-metal3-manager
    Operation:    Update
    Time:         2022-12-22T11:04:47Z
    API Version:  metal3.io/v1alpha1
    Fields Type:  FieldsV1
    fieldsV1: ...
    Manager:      main
    Operation:    Update
    Subresource:  status
    Time:         2022-12-22T11:04:47Z
  Owner References:
    API Version:     infrastructure.cluster.x-k8s.io/v1beta1
    Controller:      true
    Kind:            Metal3Machine
    Name:            test-workers-new-km5cp
    UID:             5b2bd790-1b26-4622-a136-403cd774eed3
  Resource Version:  166219
  UID:               f671c527-f274-4ea1-a831-1508a75f34dd
Spec:
  Automated Cleaning Mode:  metadata
  Bmc:
    Address:           libvirt://192.168.122.604:6233/
    Credentials Name:  worker-604-bmc-secret
  Consumer Ref:
    API Version:  infrastructure.cluster.x-k8s.io/v1beta1
    Kind:         Metal3Machine
    Name:         test-workers-new-km5cp
    Namespace:    metal3
  Image:
    Checksum:       97830b21ed272a3d854615beb54cf005
    Checksum Type:  md5
    Format:         raw
    URL:            http://172.22.0.1/images/rhcos-ootpa-latest-new.qcow2
  Meta Data:
    Name:       test-workers-new-km5cp-metadata
    Namespace:  metal3
  Online:       true
  User Data:
    Name:       test-workers-w52jh
    Namespace:  metal3
Status:
  Error Count:    0
  Error Message:
  Good Credentials:
    Credentials:
      Name:               worker-604-bmc-secret
      Namespace:          metal3
    Credentials Version:  166195
  Hardware: ...
  Hardware Profile:  libvirt
  Last Updated:      2022-12-22T11:04:47Z
  Operation History:
    Deprovision:
      End:    2022-12-22T10:42:32Z
      Start:  2022-12-22T10:41:13Z
    Inspect:
      End:    2022-12-21T13:39:06Z
      Start:  2022-12-21T13:37:26Z
    Provision:
      End:    <nil>
      Start:  2022-12-22T11:04:47Z
    Register:
      End:             2022-12-22T11:04:47Z
      Start:           2022-12-22T11:04:47Z
  Operational Status:  OK
  Powered On:          false
  Provisioning:
    ID:         temporary-fake-id
    Boot Mode:  UEFI
    Image:
      URL:
    Raid:
      Hardware RAID Volumes:  <nil>
      Software RAID Volumes:
    Root Device Hints:
      Device Name:  /dev/vda
    State:          provisioning
  Tried Credentials:
    Credentials:
      Name:               worker-604-bmc-secret
      Namespace:          metal3
    Credentials Version:  166195
Events:
  Type    Reason                Age    From                         Message
  ----    ------                ----   ----                         -------
  Normal  BMCAccessValidated    28m    metal3-baremetal-controller  Verified access to BMC
  Normal  DeprovisionComplete   26m    metal3-baremetal-controller  Image deprovisioning completed
  Normal  BMCAccessValidated    4m39s  metal3-baremetal-controller  Verified access to BMC
  Normal  ProvisioningComplete  4m39s  metal3-baremetal-controller  Image provisioning completed
  Normal  ProvisioningComplete  4m29s  metal3-baremetal-controller  Image provisioning completed
  Normal  ProvisioningComplete  4m19s  metal3-baremetal-controller  Image provisioning completed
  Normal  ProvisioningComplete  4m9s   metal3-baremetal-controller  Image provisioning completed
  Normal  ProvisioningComplete  3m59s  metal3-baremetal-controller  Image provisioning completed
  Normal  ProvisioningComplete  3m49s  metal3-baremetal-controller  Image provisioning completed
  Normal  ProvisioningComplete  3m39s  metal3-baremetal-controller  Image provisioning completed
  Normal  ProvisioningComplete  3m29s  metal3-baremetal-controller  Image provisioning completed
  Normal  ProvisioningComplete  3m19s  metal3-baremetal-controller  Image provisioning completed
  Normal  ProvisioningComplete  3m9s   metal3-baremetal-controller  Image provisioning completed
  Normal  ProvisioningComplete  2m59s  metal3-baremetal-controller  Image provisioning completed
  Normal  ProvisioningComplete  2m49s  metal3-baremetal-controller  Image provisioning completed
  Normal  ProvisioningComplete  2m39s  metal3-baremetal-controller  Image provisioning completed
  Normal  ProvisioningComplete  2m29s  metal3-baremetal-controller  Image provisioning completed
  Normal  ProvisioningComplete  2m19s  metal3-baremetal-controller  Image provisioning completed
  Normal  ProvisioningComplete  2m9s   metal3-baremetal-controller  Image provisioning completed
  Normal  ProvisioningComplete  119s   metal3-baremetal-controller  Image provisioning completed
  Normal  ProvisioningComplete  109s   metal3-baremetal-controller  Image provisioning completed
  Normal  ProvisioningComplete  99s    metal3-baremetal-controller  Image provisioning completed
  Normal  ProvisioningComplete  89s    metal3-baremetal-controller  Image provisioning completed
  Normal  ProvisioningComplete  79s    metal3-baremetal-controller  Image provisioning completed
  Normal  ProvisioningComplete  69s    metal3-baremetal-controller  Image provisioning completed
  Normal  ProvisioningComplete  59s    metal3-baremetal-controller  Image provisioning completed
  Normal  ProvisioningComplete  49s    metal3-baremetal-controller  Image provisioning completed
  Normal  ProvisioningComplete  39s    metal3-baremetal-controller  Image provisioning completed
  Normal  ProvisioningComplete  29s    metal3-baremetal-controller  Image provisioning completed
  Normal  ProvisioningComplete  19s    metal3-baremetal-controller  Image provisioning completed
  Normal  ProvisioningComplete  9s     metal3-baremetal-controller  Image provisioning completed
$ k -n metal3 describe bmh worker-465
Name:         worker-465
Namespace:    metal3
Labels:       cluster.x-k8s.io/cluster-name=test
Annotations:  <none>
API Version:  metal3.io/v1alpha1
Kind:         BareMetalHost
Metadata:
  Creation Timestamp:  2022-12-21T13:33:08Z
  Finalizers:
    baremetalhost.metal3.io
  Generation:  8
  Managed Fields:
    API Version:  metal3.io/v1alpha1
    Fields Type:  FieldsV1
    fieldsV1: ...
    Manager:      kubectl-client-side-apply
    Operation:    Update
    Time:         2022-12-21T13:33:08Z
    API Version:  metal3.io/v1alpha1
    Fields Type:  FieldsV1
    fieldsV1: ...
    Manager:      cluster-api-provider-metal3-manager
    Operation:    Update
    Time:         2022-12-22T11:01:39Z
    API Version:  metal3.io/v1alpha1
    Fields Type:  FieldsV1
    fieldsV1: ...
    Manager:      main
    Operation:    Update
    Subresource:  status
    Time:         2022-12-22T11:01:42Z
  Owner References:
    API Version:     infrastructure.cluster.x-k8s.io/v1beta1
    Controller:      true
    Kind:            Metal3Machine
    Name:            test-workers-6llbf
    UID:             f1449182-b56f-4d90-8f4c-116543338c63
  Resource Version:  164924
  UID:               c04fd3bb-3cca-4860-8ad2-2daa4440f116
Spec:
  Automated Cleaning Mode:  metadata
  Bmc:
    Address:           libvirt://192.168.122.465:6233/
    Credentials Name:  worker-465-bmc-secret
  Consumer Ref:
    API Version:  infrastructure.cluster.x-k8s.io/v1beta1
    Kind:         Metal3Machine
    Name:         test-workers-6llbf
    Namespace:    metal3
  Online:         false
Status:
  Error Count:    0
  Error Message:
  Good Credentials:
    Credentials:
      Name:               worker-465-bmc-secret
      Namespace:          metal3
    Credentials Version:  164740
  Hardware: ...
  Hardware Profile:  libvirt
  Last Updated:      2022-12-22T11:01:42Z
  Operation History:
    Deprovision:
      End:    <nil>
      Start:  2022-12-22T11:01:42Z
    Inspect:
      End:    2022-12-21T13:38:52Z
      Start:  2022-12-21T13:37:12Z
    Provision:
      End:    2022-12-22T11:00:20Z
      Start:  2022-12-22T11:00:20Z
    Register:
      End:             2022-12-22T11:01:39Z
      Start:           2022-12-22T11:01:39Z
  Operational Status:  OK
  Powered On:          false
  Provisioning:
    ID:         temporary-fake-id
    Boot Mode:  UEFI
    Image:
      Checksum:       97830b21ed272a3d854615beb54cf004
      Checksum Type:  md5
      Format:         raw
      URL:            http://172.22.0.1/images/rhcos-ootpa-latest.qcow2
    Raid:
      Hardware RAID Volumes:  <nil>
      Software RAID Volumes:
    Root Device Hints:
      Device Name:  /dev/vda
    State:          deprovisioning
  Tried Credentials:
    Credentials:
      Name:               worker-465-bmc-secret
      Namespace:          metal3
    Credentials Version:  164740
Events:
  Type    Reason               Age    From                         Message
  ----    ------               ----   ----                         -------
  Normal  PowerOn              26m    metal3-baremetal-controller  Host powered on
  Normal  BMCAccessValidated   26m    metal3-baremetal-controller  Verified access to BMC
  Normal  DeprovisionComplete  22m    metal3-baremetal-controller  Image deprovisioning completed
  Normal  BMCAccessValidated   8m50s  metal3-baremetal-controller  Verified access to BMC
  Normal  PowerOn              8m45s  metal3-baremetal-controller  Host powered on
  Normal  PowerOn              8m45s  metal3-baremetal-controller  Host powered on
  Normal  PowerOn              8m44s  metal3-baremetal-controller  Host powered on
  Normal  PowerOn              8m44s  metal3-baremetal-controller  Host powered on
  Normal  PowerOn              8m43s  metal3-baremetal-controller  Host powered on
  Normal  PowerOn              8m42s  metal3-baremetal-controller  Host powered on
  Normal  PowerOn              8m40s  metal3-baremetal-controller  Host powered on
  Normal  PowerOn              8m4s   metal3-baremetal-controller  Host powered on
  Normal  BMCAccessValidated   7m26s  metal3-baremetal-controller  Verified access to BMC
  Normal  DeprovisionStarted   4m39s  metal3-baremetal-controller  Image deprovisioning started
  Normal  DeprovisionStarted   4m29s  metal3-baremetal-controller  Image deprovisioning started
  Normal  DeprovisionStarted   4m19s  metal3-baremetal-controller  Image deprovisioning started
  Normal  DeprovisionStarted   4m9s   metal3-baremetal-controller  Image deprovisioning started
  Normal  DeprovisionStarted   3m59s  metal3-baremetal-controller  Image deprovisioning started
  Normal  DeprovisionStarted   3m49s  metal3-baremetal-controller  Image deprovisioning started
  Normal  DeprovisionStarted   3m39s  metal3-baremetal-controller  Image deprovisioning started
  Normal  DeprovisionStarted   3m29s  metal3-baremetal-controller  Image deprovisioning started
  Normal  DeprovisionStarted   3m19s  metal3-baremetal-controller  Image deprovisioning started
  Normal  DeprovisionStarted   3m9s   metal3-baremetal-controller  Image deprovisioning started
  Normal  DeprovisionStarted   2m59s  metal3-baremetal-controller  Image deprovisioning started
  Normal  DeprovisionStarted   2m49s  metal3-baremetal-controller  Image deprovisioning started
  Normal  DeprovisionStarted   2m39s  metal3-baremetal-controller  Image deprovisioning started
  Normal  DeprovisionStarted   2m29s  metal3-baremetal-controller  Image deprovisioning started
  Normal  DeprovisionStarted   2m19s  metal3-baremetal-controller  Image deprovisioning started
  Normal  DeprovisionStarted   2m9s   metal3-baremetal-controller  Image deprovisioning started
  Normal  DeprovisionStarted   119s   metal3-baremetal-controller  Image deprovisioning started
  Normal  DeprovisionStarted   109s   metal3-baremetal-controller  Image deprovisioning started
  Normal  DeprovisionStarted   99s    metal3-baremetal-controller  Image deprovisioning started
  Normal  DeprovisionStarted   89s    metal3-baremetal-controller  Image deprovisioning started
  Normal  DeprovisionStarted   79s    metal3-baremetal-controller  Image deprovisioning started
  Normal  DeprovisionStarted   69s    metal3-baremetal-controller  Image deprovisioning started
  Normal  DeprovisionStarted   59s    metal3-baremetal-controller  Image deprovisioning started
  Normal  DeprovisionStarted   49s    metal3-baremetal-controller  Image deprovisioning started
  Normal  DeprovisionStarted   39s    metal3-baremetal-controller  Image deprovisioning started
  Normal  DeprovisionStarted   29s    metal3-baremetal-controller  Image deprovisioning started
  Normal  DeprovisionStarted   19s    metal3-baremetal-controller  Image deprovisioning started
  Normal  DeprovisionStarted   9s     metal3-baremetal-controller  Image deprovisioning started
```
