#!/bin/bash

v=$1
num=$(($v - 1))
for i in $(seq 0 $num);
do
kubectl apply -f - <<EOF
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshot
metadata:
  name: backup-$i
spec:
  volumeSnapshotClassName: csi-gcp-filestore-backup-snap-class
  source:
    persistentVolumeClaimName: www-web-$i
EOF

done

