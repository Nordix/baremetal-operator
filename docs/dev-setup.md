# Setup Development Environment

## Toolchain prerequisites

BMO ships its provisioners as Go plugins (`.so` files), so building the
manager and the bundled plugins locally requires:

- A C compiler (`gcc` on Debian/Ubuntu, `cc` on Fedora/Arch). `make manager`
  builds the ironic and demo plugins via `go build -buildmode=plugin`, which
  needs `CGO_ENABLED=1` and a working C toolchain.
- The Go version printed by `make go-version`. Plugins must be built with
  the same Go toolchain as the host binary, so version mismatches surface
  as "plugin was built with a different version of package …" errors at
  load time.

`make run` and `make demo` build the relevant plugin into `bin/` and run the
manager with `PROVISIONER_PLUGIN_DIR=$PWD/bin`. `make run-test-mode` runs the
manager with `-provisioner=fixture`, which skips plugin loading entirely
because the fixture provisioner is compiled into the manager binary — this
is the easiest way to iterate without installing a C toolchain.

## With minikube

1. Install and launch minikube

   <https://kubernetes.io/docs/setup/minikube/>

1. Create a namespace to host the operator

    ```bash
    kubectl create namespace baremetal-operator-system
    ```

1. Install cert-manager

    ```bash
    kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.19.2/cert-manager.yaml
    ```

1. Verify that it is deployed correctly. Note: don't move to the
    next step before you see all three pods in Running state.

    ```bash
    kubectl get pods --namespace cert-manager

    NAME                                       READY   STATUS    RESTARTS   AGE
    cert-manager-5c6866597-zw7kh               1/1     Running   0          2m
    cert-manager-cainjector-577f6d9fd7-tr77l   1/1     Running   0          2m
    cert-manager-webhook-787858fcdb-nlzsq      1/1     Running   0          2m
    ```

1. Install operator in the cluster

    ```bash
    eval $(go env)
    mkdir -p $GOPATH/src/github.com/metal3-io
    cd $GOPATH/src/github.com/metal3-io
    git clone https://github.com/metal3-io/baremetal-operator.git
    cd baremetal-operator
    kustomize build config/default | kubectl apply -f -
    ```

1. OR Launch the operator locally

    ```bash
    export OPERATOR_NAME=baremetal-operator
    export DEPLOY_KERNEL_URL=http://172.22.0.1/images/ironic-python-agent.kernel
    export DEPLOY_RAMDISK_URL=http://172.22.0.1/images/ironic-python-agent.initramfs
    export IRONIC_ENDPOINT=http://localhost:6385/v1/
    make run
    ```

1. Create the CR

    ```bash
    kubectl create namespace metal3
    kubectl apply -f examples/example-host.yaml -n metal3
    ```

## Running without Ironic

In environments where Ironic is not available, and the only real need
is to be able to have some test data, use the test fixture provisioner
instead of the real Ironic provisioner by passing `-provisioner=fixture`
to the operator when launching it.

```bash
make run-test-mode
```

## Running a local instance of Ironic

There is a script available that will run a set of containers locally using
`podman` to stand up Ironic for development and testing.

See `tools/run_local_ironic.sh`.

> **⚠️ Development use only.** This script runs Ironic without TLS or
> basic-auth by default. It is not suitable for production or shared
> environments. If you need authentication, export `IRONIC_USERNAME` and
> `IRONIC_PASSWORD` before running the script (see below).

Note that this script may need customizations to some of the `podman run`
commands, to include environment variables that configure the containers for
your environment. All ironic related environment variables are set by default
if they are not passed through the environment.

The following environment variables can be passed to configure the ironic:

- HTTP_PORT - port used by httpd server (default 6180)
- PROVISIONING_IP - provisioning interface IP address to use for ironic,
  dnsmasq(dhcpd) and httpd (default 172.22.0.1)
- CLUSTER_PROVISIONING_IP - cluster provisioning interface IP address (default 172.22.0.2)
- PROVISIONING_INTERFACE - interface to use for ironic, dnsmasq(dhcpd) and
  httpd (default ironicendpoint)
- CLUSTER_DHCP_RANGE - dhcp range to use for provisioning (default 172.22.0.10-172.22.0.100)
- DEPLOY_KERNEL_URL - the URL of the kernel to deploy ironic-python-agent
- DEPLOY_RAMDISK_URL - the URL of the ramdisk to deploy ironic-python-agent
- IRONIC_ENDPOINT - the endpoint of the ironic
- CACHEURL - the URL of the cached images. Defaults to
  `http://${PROVISIONING_IP}/images` (typically `http://172.22.0.1/images`).
  This points to a local HTTP server on the host itself. In production
  deployments where the cache server is on a different host or network
  segment, consider using HTTPS to protect image integrity in transit.
- IRONIC_FAST_TRACK - whether to enable fast_track provisioning or not
  (default true)
- IRONIC_KERNEL_PARAMS - Kernel parameters to pass to IPA (default console=ttyS0)
- IRONIC_INSPECTOR_VLAN_INTERFACES - VLAN interfaces included in introspection,
       `all` - all VLANs on all interfaces, using LLDP information (default),
       interface - all VLANs on an interface, using LLDP information,
       interface.vlan - a particular VLAN interface, not using LLDP
- IRONIC_BOOT_ISO_SOURCE - where the boot iso image will be served from, possible
  values are: local (default), to download the image, prepare it and serve it from
  the conductor; http, to serve it directly from its HTTP URL.
- IPA_DOWNLOAD_ENABLED - Enables the use of the Ironic Python Agent Downloader
  container to download IPA archive (default true)
- USE_LOCAL_IPA - Enables the use of locally supplied IPA archive. This
  condition is handled by BMO and this has effect only when
  IPA_DOWNLOAD_ENABLED is "false", otherwise IPA_DOWNLOAD_ENABLED takes
  precedence. (default false)
- LOCAL_IPA_PATH - This has effect only when USE_LOCAL_IPA is set to "true",
  points to the directory where
  the IPA archive is located. This variable is handled by BMO. The variable
  should contain an arbitrary path pointing to the
  directory that contains the `ironic-python-agent.tar`.
  (default `/opt/metal3-dev-env/dib`). The path must not be world-writable.
  If an `ironic-python-agent.tar.sha256sum` file exists alongside the archive,
  its integrity will be verified before extraction.

**NOTES**
In case both `IPA_DOWNLOAD_ENABLED` and `USE_LOCAL_IPA` are set to true then
the IPA archive will be downloaded and the use of local IPA
archive will be skipped.

In case both `IPA_DOWNLOAD_ENABLED` and `USE_LOCAL_IPA` are set to false then
the IPA download/copy process won't start at all. Skipping the IPA
download/copy process could help in avoiding unwanted overwrites of the
"ironic-python-agent.tar" in case somebody
is testing some functionality where the unwanted overwrite of the archive
would cause issues.

In case you want to run the local ironic containers with TLS and basic
authentication enabled, you also need to export the following variables:

### TLS variables

- IRONIC_CACERT_FILE
- IRONIC_CERT_FILE
- IRONIC_KEY_FILE

### Basic authentication variables

- IRONIC_USERNAME
- IRONIC_PASSWORD

The names of these variables are self explanatory. TLS variables expect the
path of the corresponding certificate/key file as their value. Basic
authentication variables expect the corresponding value as string.  Note that,
these variables **do not** have any default value. So if they are not set, the
ironic container will run with TLS and basic authentication disabled.

## Using Tilt for development

It is easy to use Tilt for BMO deployment. Once you have a local instance
of Ironic running, just run

```sh
  make tilt-up
```

and clean it with

```sh
  make kind-reset
```

It is also possible to develop Baremetal Operator using Tilt with CAPM3. Please
refer to
[the development setup guide of CAPM3](https://github.com/metal3-io/cluster-api-provider-metal3/blob/main/docs/dev-setup.md#tilt-for-dev-in-capm3)
and specially the [Baremetal Operator Integration](https://github.com/metal3-io/cluster-api-provider-metal3/blob/main/docs/dev-setup.md#including-baremetal-operator-and-ip-address-manager)

### Alternative Tilt Setup Method

In case the standard Tilt setup (`make tilt-up`) does not work as expected,
you can reproduce the same steps manually. There are two flavors of this,
depending on how much of the real stack you want to exercise:

- **Lightweight setup with the fixture provisioner**: no libvirt, no
  BMC emulation, no real Ironic. BareMetalHosts go from `registering` to
  `available` in a few seconds. Good for iterating on controller logic.
- **Full setup with a real Ironic**: deploys
  [ironic-standalone-operator](https://github.com/metal3-io/ironic-standalone-operator)
  (IrSO) and uses real libvirt VMs, so BareMetalHosts go through the full
  state machine for real (`registering` -> `inspecting` -> `available`),
  which takes a few minutes. This is closer to what
  [the E2E tests](../test/e2e/) do.

In both cases, the cluster must be named `bmo`, since that is the only
[`allowed_contexts`](../Tiltfile) the `Tiltfile` will talk to.

#### Lightweight setup with the fixture provisioner

This runs the manager with `-provisioner=fixture` (see
[Running without Ironic](#running-without-ironic)), so no BMC, libvirt VM, or
Ironic of any kind is required.

**Create a Kind Cluster**:

```sh
kind create cluster --name bmo
```

**Install Cert-Manager**:

```sh
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/latest/download/cert-manager.yaml
```

**Configure Tilt to use the fixture provisioner**, by creating
`tilt-settings.json` at the repository root:

```json
{
    "extra_args": {
        "metal3-bmo": ["--provisioner=fixture"]
    }
}
```

**Launch Tilt**:

```sh
tilt up
```

Once Tilt has built and deployed the manager, create a BareMetalHost:

```sh
kubectl apply -f examples/fixture-host.yaml
```

Since the fixture provisioner never actually talks to a BMC, the host should
go from `registering` to `inspecting` to `available` within a few seconds:

```sh
kubectl get baremetalhost fixture-baremetalhost -w
```

To make additional hosts, copy `examples/fixture-host.yaml` and change the
`name`s (they must be unique).

To tear down, delete the BareMetalHost(s) (this completes instantly, since
there is no real backend to wait for) and the cluster:

```sh
kubectl delete -f examples/fixture-host.yaml
kind delete cluster --name bmo   # or: make kind-reset
```

#### Full setup with a real Ironic (ironic-standalone-operator)

This deploys a real Ironic with
[ironic-standalone-operator](https://github.com/metal3-io/ironic-standalone-operator)
(IrSO) and uses real libvirt VMs as BareMetalHosts, connected through
[sushy-tools](https://opendev.org/openstack/sushy-tools) (Redfish virtual
media). libvirt-clients and libvirt-daemon-system are required. VM creation,
BMC emulation, and network setup are all handled by
[`vbmctl`](../test/vbmctl/README.md), the same tool used by the E2E tests.
Build it with:

```sh
make build-vbmctl
```

This requires a C compiler and the `libvirt-dev`/`libvirt-devel` headers, since
`vbmctl` links against libvirt (see
[Toolchain prerequisites](#toolchain-prerequisites)).

**Set up the network and BMC emulator**. This must run *before* creating the
kind cluster:

```sh
tools/bmh_test/run_local_bmh_test_setup.sh
```

This creates a libvirt network named `baremetal-e2e` (bridge `metal3`,
`192.168.222.1/24`, matching [`hack/e2e/net.xml`](../hack/e2e/net.xml)),
starts a sushy-tools container, and, like the E2E tests, creates a Docker
network named `kind` bridged to the libvirt network via a veth pair (see
[`tools/bmh_test/vbmctl.yaml`](../tools/bmh_test/vbmctl.yaml)). Creating this
Docker network ahead of time is what allows `kind create cluster` below to
join it automatically instead of creating its own isolated network, so that
the kind node ends up on the same network as the VMs and the BMC emulator.

**Create a Kind Cluster**:

```sh
kind create cluster --name bmo
```

**Install Cert-Manager**:

```sh
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/latest/download/cert-manager.yaml
```

**Install ironic-standalone-operator**, and wait for it to be ready:

```sh
kubectl apply -f https://github.com/metal3-io/ironic-standalone-operator/releases/download/v0.11.0/install.yaml
kubectl wait --for=condition=Available deployment/ironic-standalone-operator-controller-manager \
  -n ironic-standalone-operator-system --timeout=120s
```

**Deploy Ironic**, using the minimal development `Ironic` custom resource
provided in [`tools/bmh_test/ironic.yaml`](../tools/bmh_test/ironic.yaml) (no
TLS, no manual credentials -- IrSO generates its own API credentials secret
automatically):

```sh
kubectl create namespace baremetal-operator-system
kubectl apply -f tools/bmh_test/ironic.yaml
kubectl wait --for=condition=Ready ironic/ironic -n baremetal-operator-system --timeout=300s
```

**Configure Tilt to deploy BMO against this Ironic**, by creating
`tilt-settings.json` at the repository root:

```json
{
    "kustomize_config_path": "config/use-irso"
}
```

**Launch Tilt**:

```sh
tilt up
```

Once Tilt has built and deployed the manager, you
should see it running with:

```sh
kubectl get pods -n baremetal-operator-system
```

When Tilt is up, it is possible to make BareMetalHosts by pressing a
button in the Tilt localhost interface. This is currently only supported
for Unix based systems. This button runs the content of file

```sh
tools/bmh_test/create_bmh.sh <NAME>
```

and adds the values given to the button as arguments. `create_bmh.sh`
creates the VM with `vbmctl` and registers it as a BareMetalHost using its
libvirt domain UUID as the Redfish system ID, since that is how sushy-tools
identifies systems. Controlplane host can be created with

```sh
tools/bmh_test/create_bmh.sh <NAME> <CONSUMER> <CONSUMER_NAMESPACE>
```

Since this goes through a real Ironic and boots a real (virtual) machine,
expect the host to take a few minutes to go from `registering` through
`inspecting` to `available`:

```sh
kubectl get baremetalhost bmh-test-<NAME> -w
```

The network, BMC emulator, and virtual machines can be cleaned with

```sh
tools/bmh_test/clean_local_bmh_test_setup.sh
```

To tear down everything, delete the cluster and clean up the VM/network
resources (order does not matter):

```sh
kind delete cluster --name bmo   # or: make kind-reset
tools/bmh_test/clean_local_bmh_test_setup.sh
```

## Using libvirt VMs with Ironic

In order to use VMs as hosts, they need to be connected to
[vbmc](https://docs.openstack.org/virtualbmc/latest/index.html)
and the `bootMACAddress` field needs to be set to the MAC address of the
network interface that will PXE boot.

For example:

```yaml
apiVersion: metal3.io/v1alpha1
kind: BareMetalHost
metadata:
  name: worker-0
spec:
  online: true
  bmc:
    address: libvirt://192.168.122.1:6233/
    credentialsName: worker-0-bmc-secret
  bootMACAddress: 00:73:49:3a:76:8e
```

The `make-virt-host` utility can be used to generate a YAML file for
registering a host. It takes as input the name of the `virsh` domain
and produces as output the basic YAML to register that host properly,
with the boot MAC address and BMC address filled in.

> **⚠️ Security note:** This tool generates manifests with hard-coded HTTP
> image URLs and MD5 checksum references (co-located on the same HTTP
> server). This is acceptable for local development with virtual machines
> but provides **no integrity guarantee** against network-level attackers.
> Do not copy this pattern into production deployments. For production,
> use HTTPS image URLs with inline SHA-256/SHA-512 checksums or OCI image
> references pinned by digest.

```bash
$ go run cmd/make-virt-host/main.go openshift_worker_1
---
apiVersion: v1
kind: Secret
metadata:
  name: openshift-worker-1-bmc-secret
type: Opaque
data:
  username: YWRtaW4=
  password: cGFzc3dvcmQ=

---
apiVersion: metal3.io/v1alpha1
kind: BareMetalHost
metadata:
  name: openshift-worker-1
spec:
  online: true
  bmc:
    address: libvirt://192.168.122.1:6234/
    credentialsName: openshift-worker-1-bmc-secret
  bootMACAddress: 00:1a:74:74:e5:cf
```

The output can be passed directly to `kubectl apply` like this:

```bash
go run cmd/make-virt-host/main.go openshift_worker_1 | kubectl apply -f -
```

When the host is a *controlplane host*, include the `-consumer` and
`-consumer-namespace` options to associate the host with the existing
`Machine` object.

```bash
$ go run cmd/make-virt-host/main.go -consumer ostest-controlplane-1 \
  -consumer-namespace openshift-machine-api  openshift_controlplane_1
---
apiVersion: v1
kind: Secret
metadata:
  name: openshift-controlplane-1-bmc-secret
type: Opaque
data:
  username: YWRtaW4=
  password: cGFzc3dvcmQ=

---
apiVersion: metal3.io/v1alpha1
kind: BareMetalHost
metadata:
  name: openshift-controlplane-1
spec:
  online: true
  bmc:
    address: libvirt://192.168.122.1:6231/
    credentialsName: openshift-controlplane-1-bmc-secret
  bootMACAddress: 00:c9:a0:f2:e0:59
  consumerRef:
    name: ostest-controlplane-1
    namespace: openshift-machine-api
```

## Using Bare Metal Hosts

The `make-bm-worker` tool may be a more convenient way of creating
YAML definitions for workers than editing the files directly. If
deploying baremetal hosts you might want to consider setting
IRONIC_KERNEL_PARAMS="", when deploying ironic as the default directs
the console to ttyS0 and is lost in Bare Metal Hosts.

```bash
$ go run cmd/make-bm-worker/main.go -address 1.2.3.4 \
  -password password -user admin worker-99
---
apiVersion: v1
kind: Secret
metadata:
  name: worker-99-bmc-secret
type: Opaque
data:
  username: YWRtaW4=
  password: cGFzc3dvcmQ=

---
apiVersion: metal3.io/v1alpha1
kind: BareMetalHost
metadata:
  name: worker-99
spec:
  online: true
  bmc:
    address: 1.2.3.4
    credentialsName: worker-99-bmc-secret
    disableCertificateVerification: true
```
