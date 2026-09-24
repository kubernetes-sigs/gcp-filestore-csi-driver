#!/bin/sh

set -o errexit
trap 'service nfs-common stop 2>/dev/null || true; service rpcbind stop 2>/dev/null || true; exit 0' TERM INT

# 1. Ensure required runtime directories exist on /run
mkdir -p /run/sendsigs.omit.d
mkdir -p /run/rpc_pipefs

# 2. Start rpcbind safely (don't fail errexit if port 111 is already bound)
if ! rpcinfo -p 127.0.0.1 >/dev/null 2>&1; then
  service rpcbind start || echo "rpcbind start returned non-zero, continuing..."
fi

# 3. Check if statd is already registered (existing node mount)
if rpcinfo -T udp 127.0.0.1 100024 >/dev/null 2>&1; then
  echo "statd already running on host"
  sed -i '/NEED_STATD/d' /etc/default/nfs-common 2>/dev/null || true
  echo "NEED_STATD=no" >> /etc/default/nfs-common
else
  echo "no statd found, proceeding with start"
  sed -i '/NEED_STATD/d' /etc/default/nfs-common 2>/dev/null || true
  echo "NEED_STATD=yes" >> /etc/default/nfs-common
fi

# 4. Attempt to start the service normally
service nfs-common start

# 5. Verification Loop
MAX_RETRIES=5
COUNT=0
until rpcinfo -T udp 127.0.0.1 100024 >/dev/null 2>&1; do
  COUNT=$((COUNT + 1))
  if [ "$COUNT" -ge "$MAX_RETRIES" ]; then
    echo "--- DIAGNOSTIC FAILURE ---" >&2
    echo "statd failed to register after $MAX_RETRIES attempts." >&2
    echo "Current RPC services registered with portmapper:" >&2
    rpcinfo -p 127.0.0.1 >&2 || true
    service nfs-common status >&2 || true
    exit 1
  fi
  echo "Waiting for statd registration (attempt $COUNT/$MAX_RETRIES)..."
  sleep 2
done

echo "statd is healthy and registered."
sleep infinity &
wait $!
