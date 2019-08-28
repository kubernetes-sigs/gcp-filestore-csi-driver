#!/bin/bash

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

kubectl delete -f $DIR/powershell-nettest-pod.yaml
kubectl delete -f $DIR/nettest-pod.yaml
kubectl delete -f $DIR/pvc.yaml
kubectl delete -f $DIR/pv.yaml
kubectl delete -f $DIR/secrets.yaml
