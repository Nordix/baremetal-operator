# BMO chainsaw demo (fixture provisioner)

This is a working demo of using [chainsaw](https://github.com/kyverno/chainsaw)
to exercise the BareMetalHost (BMH) lifecycle against BMO's in-memory
["fixture" provisioner](../../pkg/provisioner/fixture/fixture.go), instead of
the full Ginkgo/CAPI-test-framework e2e suite in [`test/e2e`](../e2e).

## Prerequisites

- `docker`
- `kind`
- `kubectl`
- `chainsaw`

## 1. Stand up a demo cluster

```bash
# 1. kind cluster (same script used for local dev / the fixture e2e job)
./hack/kind_with_registry.sh   # creates a "bmo" kind cluster

# 2. Build the operator image with the tag config/overlays/fixture expects
IMG=quay.io/metal3-io/baremetal-operator IMG_TAG=e2e make docker
kind load docker-image quay.io/metal3-io/baremetal-operator:e2e --name bmo

# 3. cert-manager (required by config/base's webhook wiring)
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.19.2/cert-manager.yaml
kubectl wait --for=condition=Available --timeout=120s -n cert-manager deploy --all

# 4. Deploy BMO with the fixture provisioner
tools/bin/kustomize build config/overlays/fixture | kubectl apply -f -
kubectl -n baremetal-operator-system rollout status deploy/baremetal-operator-controller-manager
```

## 2. Run the demo

```bash
chainsaw test test/chainsaw/fixture-lifecycle --config test/chainsaw/.chainsaw.yaml
```

Sample (trimmed) output:

```text
| create BMC credentials secret     | APPLY  | OK   | v1/Secret @ .../demo-bmc-credentials
| create the BareMetalHost          | APPLY  | OK   | .../BareMetalHost @ .../demo-host
| create the BareMetalHost          | ASSERT | DONE | state: registering
| inspection starts                 | ASSERT | DONE | state: inspecting
| host becomes available ...        | ASSERT | DONE | state: available
| host becomes available ...        | ASSERT | DONE | hardware.cpu/nics match fixture data
| provision an image                | ASSERT | DONE | state: provisioning
| provisioning completes            | ASSERT | DONE | state: provisioned
| deprovision the host              | ASSERT | DONE | state: deprovisioning
| host returns to available         | ASSERT | DONE | state: available
--- PASS: chainsaw/fixture-lifecycle (26.66s)
```

Useful flags while iterating:

- `--skip-delete` — leave the namespace/BMH around after a failure for
  `kubectl describe`/`kubectl get -o yaml` inspection.
- `--fail-fast` — stop at the first failed step.
- Validate the test file's schema without touching a cluster:

  ```bash
  chainsaw lint test -f test/chainsaw/fixture-lifecycle/chainsaw-test.yaml
  ```
