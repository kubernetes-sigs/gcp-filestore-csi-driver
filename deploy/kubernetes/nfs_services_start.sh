#!/bin/sh

set -o errexit
trap "exit 0" TERM

# 1. Recreate paths shadowed by the Kubernetes emptyDir mount over /run
mkdir -p /run/sendsigs.omit.d
mkdir -p /run/rpc_pipefs

# 2. Start rpcbind safely (don't fail errexit if port 111 is already bound)
if ! rpcinfo -p 127.0.0.1 >/dev/null 2>&1; then
  service rpcbind start || echo "rpcbind start returned non-zero, continuing..."
fi

# 2. Check if statd is already registered (existing node mount)
if rpcinfo -T udp 127.0.0.1 100024 >/dev/null 2>&1; then
  echo "statd already running on host"
  sed -i '/NEED_STATD/d' /etc/default/nfs-common 2>/dev/null || true
  echo "NEED_STATD=no" >> /etc/default/nfs-common
else
  echo "no statd found, proceeding with start"
  sed -i '/NEED_STATD/d' /etc/default/nfs-common 2>/dev/null || true
  echo "NEED_STATD=yes" >> /etc/default/nfs-common
fi

# 3. Attempt to start the service normally
service nfs-common start

# 4. Verification Loop
MAX_RETRIES=5
COUNT=0
until rpcinfo -T udp 127.0.0.1 100024 >/dev/null 2>&1; do
  COUNT=$((COUNT + 1))
  if [ "$COUNT" -ge "$MAX_RETRIES" ]; then
    echo "--- DIAGNOSTIC FAILURE ---" >&2
    echo "statd failed to register after $MAX_RETRIES attempts." >&2
    echo "Attempting to launch statd in foreground for debug logs..." >&2
    
    # Kill any zombie background process that might be hanging
    pkill rpc.statd 2>/dev/null || true
    
    # Launch in foreground: -F (foreground), -d (debug/stderr)
    # This will block and stream logs until it crashes or is killed by K8s
    /sbin/rpc.statd -F -d
    
    # If the foreground process somehow exits, exit the container to trigger a restart
    exit 1
  fi
  echo "Waiting for statd registration (attempt $COUNT/$MAX_RETRIES)..."
  sleep 2
done

echo "statd is healthy and registered."
sleep infinity
