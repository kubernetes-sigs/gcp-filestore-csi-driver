#!/bin/bash

id=$1
backupnum=$2
num=$(($backupnum - 1))
for i in $(seq 0 $num);
do
kubectl apply -f - <<EOF
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshot
metadata:
  name: backup-$id-$i
spec:
  volumeSnapshotClassName: csi-gcp-filestore-backup-snap-class
  source:
    persistentVolumeClaimName: www-web-$id
EOF

done

