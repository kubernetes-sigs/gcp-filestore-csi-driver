#!/bin/sh

set -o errexit

trap "{ exit 0 }" TERM

service rpcbind start

# If statd is already running, for example becuase of an existing nfs mount, we will fail
# to service nfs-common start. If we successfully query the statd service (rpc program
# number 100024), that means it's running, and we don't need to start it. This command
# is put in /etc/default/nfs-common as the NEED_STATD variable must be set there.

if rpcinfo -T udp 127.0.0.1 100024; then
  echo statd already running
  echo NEED_STATD=no >> /etc/default/nfs-common
else
  echo no statd found
fi

service nfs-common start  

sleep infinity
