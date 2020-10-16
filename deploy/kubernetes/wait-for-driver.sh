#!/bin/bash

# This script waits for the deployments from ./cluster_setup.sh to be ready.

controller_deployment=gcp-filestore-csi-controller
node_daemonset=gcp-filestore-csi-node
driver_namespace=gcp-filestore-csi-driver

echo "wait for controller to start"
kubectl wait -n ${driver_namespace} deployment ${controller_deployment} --for condition=available

retries=15
while [[ $retries -ge 0 ]];do
    ready=$(kubectl -n "${driver_namespace}" get daemonset "${node_daemonset}" -o jsonpath="{.status.numberReady}")
    required=$(kubectl -n "${driver_namespace}" get daemonset "${node_daemonset}" -o jsonpath="{.status.desiredNumberScheduled}")
    if [[ $ready -eq $required ]];then
        echo "Daemonset $node_daemonset found"
        exit 0
    fi
    ((retries--))
    sleep 10s
done
echo "Timeout waiting for node daemonset $node_daemonset"
exit -1

