# Copyright 2022 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

ARG BUILDPLATFORM

# Build driver go binary
FROM --platform=$BUILDPLATFORM golang:1.17.8 as builder

ARG STAGINGVERSION
ARG TARGETPLATFORM

WORKDIR /go/src/sigs.k8s.io/gcp-filestore-csi-driver
ADD . .
RUN GOARCH=$(echo $TARGETPLATFORM | cut -f2 -d '/') make driver BINDIR=/bin GCP_FS_CSI_STAGING_VERSION=${STAGINGVERSION}

# Install nfs packages
FROM launcher.gcr.io/google/debian11 as deps
ENV DEBIAN_FRONTEND noninteractive
RUN apt-get update && apt-get dist-upgrade -y && apt-get install -y --no-install-recommends \
    mount \
    nfs-common

# This is needed for rpcbind
RUN mkdir /run/sendsigs.omit.d

FROM gcr.io/distroless/base-debian11
# nfs-common: https://packages.debian.org/bullseye/amd64/nfs-common/filelist
COPY --from=deps /etc/default/nfs-common /etc/default/nfs-common
COPY --from=deps /etc/init.d/nfs-common /etc/init.d/nfs-common
COPY --from=deps /etc/request-key.d/id_resolver.conf /etc/request-key.d/id_resolver.conf
COPY --from=deps /lib/systemd/system/auth-rpcgss-module.service /lib/systemd/system/auth-rpcgss-module.service
# nfs files in systemd/system cannot be copied over being copied over because they don't exist 
# COPY --from=deps /lib/systemd/system/nfs-* /lib/systemd/system/
COPY --from=deps /lib/systemd/system/proc-fs-nfsd.mount /lib/systemd/system/proc-fs-nfsd.mount
COPY --from=deps /lib/systemd/system/rpc-* /lib/systemd/system/
COPY --from=deps /lib/systemd/system/run-rpc_pipefs.mount /lib/systemd/system/run-rpc_pipefs.mount
COPY --from=deps /sbin/*mount*  /sbin/
##### No error, but not being copied over:
COPY --from=deps /sbin/sm-notify  /sbin/sm-notify
COPY --from=deps /sbin/osd_login  /sbin/osd_login
#####
COPY --from=deps /sbin/rpc.statd  /sbin/rpc.statd
COPY --from=deps /usr/lib/systemd/scripts/nfs-utils_env.sh  /usr/lib/systemd/scripts/nfs-utils_env.sh
COPY --from=deps /usr/sbin/blkmapd /usr/sbin/blkmapd
COPY --from=deps /usr/sbin/mountstats /usr/sbin/mountstats
COPY --from=deps /usr/sbin/nfs* /usr/sbin/
COPY --from=deps /usr/sbin/rpc* /usr/sbin/
COPY --from=deps /usr/sbin/start-statd /usr/sbin/start-statd
COPY --from=deps /var/lib/nfs/state /var/lib/nfs/state 
# adduser dependency
COPY --from=deps /usr/sbin/add* /usr/sbin/
COPY --from=deps /sbin/del* /sbin/
COPY --from=deps /etc/deluser.conf /etc/deluser.conf  
COPY --from=deps /lib/x86_64-linux-gnu/libcom_err.so.2 /lib/x86_64-linux-gnu/libcom_err.so.2
COPY --from=deps /lib/x86_64-linux-gnu/libc.so.6 /lib/x86_64-linux-gnu/libc.so.6
COPY --from=deps /lib/x86_64-linux-gnu/libkeyutils.so.1 /lib/x86_64-linux-gnu/libkeyutils.so.1
COPY --from=deps /lib/x86_64-linux-gnu/libnfsidmap.so.0 /lib/x86_64-linux-gnu/libnfsidmap.so.0
COPY --from=deps /lib/x86_64-linux-gnu/libnfsidmap/* /lib/x86_64-linux-gnu/libnfsidmap/
COPY --from=deps /lib/x86_64-linux-gnu/libdevmapper.so.1.02.1 /lib/x86_64-linux-gnu/libdevmapper.so.1.02.1
COPY --from=deps /lib/x86_64-linux-gnu/libevent-2.1.so.7 /lib/x86_64-linux-gnu/libevent-2.1.so.7
COPY --from=deps /lib/x86_64-linux-gnu/libgssapi_krb5.so.2 /lib/x86_64-linux-gnu/libgssapi_krb5.so.2
# ucf dependency: https://packages.debian.org/bullseye/all/ucf/filelist 
# passwd dependency: https://packages.debian.org/bullseye/amd64/passwd/filelist
COPY --from=deps /usr/sbin/*passwd /usr/sbin/
COPY --from=deps /usr/bin/*passwd /usr/bin/

# mount: https://packages.debian.org/bullseye/amd64/mount/filelist
COPY --from=deps /bin/*mount* /bin/
COPY --from=deps /sbin/losetup /sbin/losetup
COPY --from=deps /sbin/swapo* /sbin/
COPY --from=deps /lib/x86_64-linux-gnu/libblkid.so.1 /lib/x86_64-linux-gnu/libblkid.so.1
COPY --from=deps /lib/x86_64-linux-gnu/libmount.so.1 /lib/x86_64-linux-gnu/libmount.so.1
COPY --from=deps /lib/x86_64-linux-gnu/libsmartcols.so.1 /lib/x86_64-linux-gnu/libsmartcols.so.1
# mount has util-linux dependency: https://packages.debian.org/bullseye/amd64/util-linux/filelist
COPY --from=deps /lib/x86_64-linux-gnu/libaudit.so.1 /lib/x86_64-linux-gnu/libaudit.so.1
COPY --from=deps /lib/x86_64-linux-gnu/libcap.so.2 /lib/x86_64-linux-gnu/libcap.so.2
COPY --from=deps /lib/x86_64-linux-gnu/libcrypt.so.1 /lib/x86_64-linux-gnu/libcrypt.so.1
COPY --from=deps /lib/x86_64-linux-gnu/libpam.so.0 /lib/x86_64-linux-gnu/libpam.so.0
COPY --from=deps /lib/x86_64-linux-gnu/libsystemd.so.0 /lib/x86_64-linux-gnu/libsystemd.so.0
COPY --from=deps /lib/x86_64-linux-gnu/libselinux.so.1 /lib/x86_64-linux-gnu/libselinux.so.1
COPY --from=deps /lib/x86_64-linux-gnu/libtic.so.6 /lib/x86_64-linux-gnu/libtic.so.6
COPY --from=deps /lib/x86_64-linux-gnu/libtinfo.so.6 /lib/x86_64-linux-gnu/libtinfo.so.6
COPY --from=deps /lib/x86_64-linux-gnu/libudev.so.1 /lib/x86_64-linux-gnu/libudev.so.1
COPY --from=deps /lib/x86_64-linux-gnu/libuuid.so.1 /lib/x86_64-linux-gnu/libuuid.so.1
COPY --from=deps /lib/x86_64-linux-gnu/libz.so.1 /lib/x86_64-linux-gnu/libz.so.1
COPY --from=deps /bin/mountpoint /bin/mountpoint
COPY --from=deps /bin/lsblk /bin/lsblk
COPY --from=deps /bin/findmnt /bin/findmnt
COPY --from=deps /sbin/blkid /sbin/blkid
COPY --from=deps /sbin/blockdev /sbin/blockdev
COPY --from=deps /bin/ch* /bin/
COPY --from=deps /sbin/fs* /sbin/
COPY --from=deps /sbin/mkfs* /sbin/
# util-linux has so many more, not sure what is needed. 

# rpcbind: https://packages.debian.org/bullseye/amd64/rpcbind/filelist
COPY --from=deps /etc/default/rpcbind /etc/default/rpcbind
COPY --from=deps /etc/init.d/rpcbind /etc/init.d/rpcbind
COPY --from=deps /etc/insserv.conf.d/rpcbind /etc/insserv.conf.d/rpcbind
COPY --from=deps /lib/systemd/system/portmap.service /lib/systemd/system/portmap.service
COPY --from=deps /lib/systemd/system/rpcbind* /lib/systemd/system/
COPY --from=deps /sbin/rpcbind  /sbin/rpcbind
COPY --from=deps /usr/bin/rpcinfo /usr/bin/rpcinfo
COPY --from=deps /usr/lib/tmpfiles.d/rpcbind.conf /usr/lib/tmpfiles.d/rpcbind.conf
COPY --from=deps /usr/sbin/rpcinfo /usr/sbin/rpcinfo
COPY --from=deps /lib/lsb/init-functions /lib/lsb/init-functions
COPY --from=deps /lib/lsb/init-functions.d/00-verbose /lib/lsb/init-functions.d/00-verbose
COPY --from=deps /lib/x86_64-linux-gnu/libtirpc.so.3 /lib/x86_64-linux-gnu/libtirpc.so.3
COPY --from=deps /lib/x86_64-linux-gnu/libwrap.so.0 /lib/x86_64-linux-gnu/libwrap.so.0

# others
COPY --from=deps /sbin/start-stop-daemon /sbin/start-stop-daemon
COPY --from=deps /bin/mkdir /bin/mkdir
COPY --from=deps /bin/sh /bin/sh
# mkdir dependencies
COPY --from=deps /lib/x86_64-linux-gnu/libpcre* /lib/x86_64-linux-gnu/
COPY --from=deps /lib/x86_64-linux-gnu/libdl.so.2 /lib/x86_64-linux-gnu/libdl.so.2
COPY --from=deps /lib/x86_64-linux-gnu/libpthread.so.0 /lib/x86_64-linux-gnu/libpthread.so.0

# COPY --from=deps /sbin/fstab-decode /sbin/fstab-decode
# COPY --from=deps /sbin/service /sbin/service
# COPY --from=deps /etc/services /etc/services
# COPY --from=deps /etc/pam.d/* /etc/pam.d/
# COPY --from=deps /etc/passwd* /etc/
# COPY --from=deps /etc/protocols /etc/protocols
# COPY --from=deps /etc/*.conf /etc/
# COPY --from=deps /etc/b* /etc/
# COPY --from=deps /etc/e* /etc/
# COPY --from=deps /etc/f* /etc/
# COPY --from=deps /etc/group* /etc/
# COPY --from=deps /etc/rpc /etc/rpc 
# COPY --from=deps /lib/x86_64-linux-gnu/lib* /lib/x86_64-linux-gnu/
# COPY --from=deps /bin/ucf* /bin/

# This is needed for rpcbind
RUN mkdir /run/sendsigs.omit.d


# Copy driver into image
ARG DRIVERBINARY=gcp-filestore-csi-driver
COPY --from=builder /bin/${DRIVERBINARY} /${DRIVERBINARY}
RUN true
COPY deploy/kubernetes/nfs_services_start.sh /nfs_services_start.sh


ENTRYPOINT ["/gcp-filestore-csi-driver"]

# Current error: 
#   Warning  FailedMount  4s    kubelet            MountVolume.MountDevice failed for volume "pvc-343c4c7c-3584-4495-901a-54fef719088b" : rpc error: code = Internal desc = mount "/var/lib/kubelet/plugins/kubernetes.io/csi/pv/pvc-343c4c7c-3584-4495-901a-54fef719088b/globalmount" failed: mount failed: exit status 32
# Mounting command: mount
# Mounting arguments: -t nfs 10.252.208.114:/vol1 /var/lib/kubelet/plugins/kubernetes.io/csi/pv/pvc-343c4c7c-3584-4495-901a-54fef719088b/globalmount
# Output: mount.nfs: Protocol not supported
# 2022-04-05 13:47:09.498 PDT
# gcp-filestore-driver
# "Mount "/var/lib/kubelet/plugins/kubernetes.io/csi/pv/pvc-d9457577-eb41-42bb-837d-95554a4455b1/globalmount" failed, cleaning up"
# "Warning: "/var/lib/kubelet/plugins/kubernetes.io/csi/pv/pvc-d9457577-eb41-42bb-837d-95554a4455b1/globalmount" is not a mountpoint, deleting"
