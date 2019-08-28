#!/bin/bash

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

kubectl apply -f $DIR/secrets.yaml
kubectl apply -f $DIR/pv.yaml
kubectl apply -f $DIR/pvc.yaml
kubectl apply -f $DIR/powershell-nettest-pod.yaml
kubectl apply -f $DIR/nettest-pod.yaml