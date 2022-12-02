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

## Experiment with BMO + CAPM3 + CAPI

- BMO running in test mode
- CAPM3 running normally
- CAPI running normally

Setup:

1. Create kind cluster
2. Initialize CAPI and Metal3 (clusterctl init --provider=metal3)
3. Add BMO CRDs and start BMO in test mode
4. Generate BMHs
5. Generate CAPI/CAPM3 resources

```bash
kind create cluster
kubectl apply -k config/crd
clusterctl init --infrastructure=metal3

# In separate terminal
CONTAINER_RUNTIME=docker make run-test-mode

./examples/produce-available-hosts.sh 3 > test-hosts.yaml
kubectl create namespace metal3
kubectl -n metal3 apply -f test-hosts.yaml

# Generate cluster
export CLUSTER_APIENDPOINT_HOST=127.0.0.1
export CLUSTER_APIENDPOINT_PORT=6443
export CTLPLANE_KUBEADM_EXTRA_CONFIG=""
export IMAGE_CHECKSUM=97830b21ed272a3d854615beb54cf004
export IMAGE_CHECKSUM_TYPE=md5
export IMAGE_FORMAT=raw
export IMAGE_URL="http://172.22.0.1/images/rhcos-ootpa-latest.qcow2"
export WORKERS_KUBEADM_EXTRA_CONFIG=""

clusterctl -n metal3 generate cluster test --kubernetes-version v1.25.3 > cluster.yaml
# Apply
kubectl -n metal3 apply -f cluster.yaml
```

Status at this point: 3 available BMHs, 1 provisioned cluster, 1 machine, 1 metal3machine, 1 BMH with consumer (but still available).

Now for some cheating/helping to move things along.

```bash
# Find the BMH with consumerRef and add provisioning data to it
consumed_bmh="$(kubectl -n metal3 get bmh -o json | jq -r '.items[] | select(.spec | has("consumerRef")) | .metadata.name')"
kubectl -n metal3 patch bmh "${consumed_bmh}" --type='merge' -p '{"spec":{"image":{"url": "http://172.22.0.1/images/rhcos-ootpa-latest.qcow2", "checksum": "97830b21ed272a3d854615beb54cf004"}}}'

# Get relevant machine and metal3machine
metal3machine="$(kubectl -n metal3 get m3m -o jsonpath="{.items[0].metadata.name}")"
machine="$(kubectl -n metal3 get machine -o jsonpath="{.items[0].metadata.name}")"

# Calculate providerID
provider_id="metal3://metal3/${consumed_bmh}/${metal3machine}"
# Add providerID to machine
patch="{\"spec\":{\"providerID\":\"${provider_id}\"}}"
kubectl -n metal3 patch machine "${machine}" --type='merge' -p "${patch}"
# Add providerID to metal3machine
kubectl -n metal3 patch m3m "${metal3machine}" --type='merge' -p "${patch}"

# Set nodeRef
patch="{\"status\":{\"nodeRef\":{\"apiVersion\": \"v1\", \"name\": \"${machine}\", \"uid\": \"4026e005-2358-411b-a06f-3046af577901\"}}}"
kubectl -n metal3 patch machine "${machine}" --subresource='status' --type='merge' -p "${patch}"
# Mark m3m as ready
kubectl -n metal3 patch m3m "${metal3machine}" --subresource='status' --type='merge' -p '{"status":{"ready": true}}'
```

Cluster, m3cluster, machine, m3m, bmh all fine.

```console
vscode ➜ /workspaces/baremetal-operator (lentzi90/scaling-experiments ✗) $ k -n metal3 get cluster
NAME   PHASE         AGE     VERSION
test   Provisioned   5h46m
vscode ➜ /workspaces/baremetal-operator (lentzi90/scaling-experiments ✗) $ k -n metal3 get m3cluster
NAME   AGE     READY   ERROR   CLUSTER   ENDPOINT
test   5h46m   true            test      {"host":"127.0.0.1","port":6443}
vscode ➜ /workspaces/baremetal-operator (lentzi90/scaling-experiments ✗) $ k -n metal3 get machine
NAME         CLUSTER   NODENAME     PROVIDERID                                         PHASE     AGE     VERSION
test-8kmk2   test      test-8kmk2   metal3://metal3/worker-3/test-controlplane-gsphk   Running   5h46m   v1.25.3
vscode ➜ /workspaces/baremetal-operator (lentzi90/scaling-experiments ✗) $ k -n metal3 get m3m
NAME                      AGE     PROVIDERID                                         READY   CLUSTER   PHASE
test-controlplane-gsphk   5h46m   metal3://metal3/worker-3/test-controlplane-gsphk   true    test
vscode ➜ /workspaces/baremetal-operator (lentzi90/scaling-experiments ✗) $ k -n metal3 get bmh
NAME       STATE         CONSUMER                  ONLINE   ERROR   AGE
worker-1   available                               true             6h28m
worker-2   available                               true             6h28m
worker-3   provisioned   test-controlplane-gsphk   true             6h28m
```

However, the KCP cannot go to ready until there is a real API server to talk to.

TODO:

- How to make the KCP happy? For ultimate scaling, maybe envtest?
