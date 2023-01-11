#!/bin/bash

cluster="${1:-test-1}"
namespace="${2:-${cluster}}"

machine_name="$(kubectl -n "${namespace}" get machine -o jsonpath="{.items[0].metadata.name}")"
kubectl -n "${namespace}" delete cluster "${cluster}"
kubectl --kubeconfig=kubeconfig-test.yaml delete node "${machine_name}"
kubectl delete namespace "${namespace}"
