# Copyright 2018 The Kubernetes Authors.
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
# Note that the newer debian bullseye image does not work with nfs-common; I
# believe that libcap needs extra configuration.
FROM k8s.gcr.io/build-image/debian-base:buster-v1.9.0 as deps
ENV DEBIAN_FRONTEND noninteractive

# The netbase package is needed to get rpcbind to work correctly,
# there is a version 2 portmapper service that is not started if only
# nfs-common is installed. The older launcher.gcr.io image used here
# did not need the netbase package.
#
# If nfs is not working, the rpcinfo command is useful for
# debugging. rpcinfo -p queries using legacy version 2, and will show
# "No remote programs registered." Without netbase, rpcinfo without
# the -p options shows some services with no name, but not the key
# portmapper service.
#
# If future problems come up, looking for different files in /etc
# between older and newer distros (in this case it was /etc/rpc
# existing only in the old launcher.gcr.io image) and using dpgk -S
# <file> to determine which package supplies it, can be helpful.
RUN apt-get update && apt-get dist-upgrade -y && apt-get install -y --no-install-recommends \
    mount \
    netbase \
    ca-certificates \
    nfs-common \
    bash

# This is needed for rpcbind
RUN mkdir /run/sendsigs.omit.d

RUN cd /tmp && \
    apt-get update && apt-get download \
    #     ca-certificates && \
    # apt-get download \ 
        ca-certificates \
        mount \
        netbase \
        rpcbind \
        adduser \
        passwd \
        init \
        gnupg \
        keyutils \
        nfs-common \
        bash \
        libevent-2.1-6 \
        libsemanage1 \
        libgssapi-krb5-2 \
        libk5crypto3 && \
    # We need status for vulnerability scanners: https://github.com/GoogleContainerTools/distroless/issues/863#issuecomment-984389747 
    mkdir -p /dpkg/var/lib/dpkg/status.d/ && \
    for deb in *.deb; do \
            package_name=$(dpkg-deb -I ${deb} | awk '/^ Package: .*$/ {print $2}'); \ 
            echo "Process: ${package_name}"; \
            dpkg --ctrl-tarfile $deb | tar -Oxvf - ./control > /dpkg/var/lib/dpkg/status.d/${package_name}; \
            dpkg --extract $deb /dpkg || exit 10; \
    done 

# Hold required packages to avoid breaking the installation of packages
RUN apt-mark hold apt gnupg adduser passwd libsemanage1 libcap2 mount nfs-common init

# Cleanup cached and unnecessary files.
RUN apt-get autoremove -y && \
    apt-get clean -y && \
    rm -rf \
        /dpkg/usr/share/doc \
        /dpkg/usr/share/man \
        /dpkg/usr/share/info \
        /dpkg/usr/share/locale \
        /dpkg/var/lib/apt/lists/* \
        /dpkg/var/log/* \
        /dpkg/var/cache/debconf/* \
        /dpkg/usr/share/common-licenses* \
        /dpkg/usr/share/bash-completion \
        ~/.bashrc \
        ~/.profile \
        # /etc/systemd \
        # /lib/lsb \
        /dpkg/lib/udev \
        /dpkg/usr/lib/x86_64-linux-gnu/gconv/IBM* \
        /dpkg/usr/lib/x86_64-linux-gnu/gconv/EBC* && \
    mkdir -p /dpkg/usr/share/man/man1 /dpkg/usr/share/man/man2 \
        /dpkg/usr/share/man/man3 /dpkg/usr/share/man/man4 \
        /dpkg/usr/share/man/man5 /dpkg/usr/share/man/man6 \
        /dpkg/usr/share/man/man7 /dpkg/usr/share/man/man8

# Since we're leveraging apt to pull in dependencies, we use `gcr.io/distroless/base` because it includes glibc.
FROM gcr.io/distroless/base-debian10 as distroless-base
# The distroless amd64 image has a target triplet of x86_64
FROM distroless-base AS distroless-amd64
ENV LIB_DIR_PREFIX x86_64

# The distroless arm64 image has a target triplet of aarch64
FROM distroless-base AS distroless-arm64
ENV LIB_DIR_PREFIX aarch64

FROM distroless-$TARGETARCH as output-image

ARG DRIVERBINARY=gcp-filestore-csi-driver
COPY --from=builder /bin/${DRIVERBINARY} /${DRIVERBINARY}


# Copy the libraries from the extractor stage into root
COPY --from=deps /dpkg /

# Copy necessary dependencies into distroless base.
COPY --from=deps /sbin/start-stop-daemon /sbin/start-stop-daemon
COPY --from=deps /etc/ca-certificates.conf /etc/ca-certificates.conf
COPY --from=deps /bin/bash /bin/bash
COPY --from=deps /bin/mkdir /bin/mkdir
COPY --from=deps /bin/grep /bin/grep
COPY --from=deps /sbin/blkid /sbin/blkid
COPY --from=deps /sbin/blockdev /sbin/blockdev
COPY --from=deps /sbin/fsck* /sbin/
COPY --from=debian /sbin/mkfs* /sbin/

# Old buster distro has /bin and /sbin packages duplicated to /usr/bin and /usr/sbin.
COPY --from=deps /bin/* /usr/bin/
COPY --from=deps /sbin/* /usr/sbin/

COPY --from=deps /lib/${LIB_DIR_PREFIX}-linux-gnu/libblkid.so.1 \
                 /lib/${LIB_DIR_PREFIX}-linux-gnu/libkeyutils.so.1 \
                 /lib/${LIB_DIR_PREFIX}-linux-gnu/libc.so.6 \
                 /lib/${LIB_DIR_PREFIX}-linux-gnu/libcom_err.so.2 \
                 /lib/${LIB_DIR_PREFIX}-linux-gnu/libudev.so.1 \
                 /lib/${LIB_DIR_PREFIX}-linux-gnu/libmount.so.1 \
                 /lib/${LIB_DIR_PREFIX}-linux-gnu/libsmartcols.so.1 \
                 /lib/${LIB_DIR_PREFIX}-linux-gnu/libaudit.so.1 \
                 /lib/${LIB_DIR_PREFIX}-linux-gnu/libudev.so.1 \
                 /lib/${LIB_DIR_PREFIX}-linux-gnu/libpam.so.0 \
                 /lib/${LIB_DIR_PREFIX}-linux-gnu/libsystemd.so.0 \
                 /lib/${LIB_DIR_PREFIX}-linux-gnu/libtinfo.so.6 \
                 /lib/${LIB_DIR_PREFIX}-linux-gnu/libuuid.so.1 \
                 /lib/${LIB_DIR_PREFIX}-linux-gnu/libtirpc.so.3 \
                 /lib/${LIB_DIR_PREFIX}-linux-gnu/libz.so.1  \
                 /lib/${LIB_DIR_PREFIX}-linux-gnu/libdl.so.2  \
                 /lib/${LIB_DIR_PREFIX}-linux-gnu/libpthread.so.0 \
                 /lib/${LIB_DIR_PREFIX}-linux-gnu/libpcre.so.3 \
                 /lib/${LIB_DIR_PREFIX}-linux-gnu/libnfsidmap.so.0  \
                 /lib/${LIB_DIR_PREFIX}-linux-gnu/libwrap.so.0  \
                 /lib/${LIB_DIR_PREFIX}-linux-gnu/libcap.so.2  \
                 /lib/${LIB_DIR_PREFIX}-linux-gnu/libdevmapper.so.1.02.1  \
                 /lib/${LIB_DIR_PREFIX}-linux-gnu/libcrypt.so.1 \
                 /lib/${LIB_DIR_PREFIX}-linux-gnu/libselinux.so.1  \
                 /usr/lib/${LIB_DIR_PREFIX}-linux-gnu/libkrb5.so.3 \
                 /usr/lib/${LIB_DIR_PREFIX}-linux-gnu/libkrb5support.so.0 \
                 /usr/lib/${LIB_DIR_PREFIX}-linux-gnu/libacl.so.1 \
                 # /usr/sbin/useradd 
                 /lib/${LIB_DIR_PREFIX}-linux-gnu/liblzma.so.5 \
                 /usr/lib/${LIB_DIR_PREFIX}-linux-gnu/liblz4.so.1 \
                 /lib/${LIB_DIR_PREFIX}-linux-gnu/libgcrypt.so.20 \
                 # sbin/rpcbind
                 /lib/${LIB_DIR_PREFIX}-linux-gnu/libpam_misc.so.0 \
                 /lib/${LIB_DIR_PREFIX}-linux-gnu/libcap-ng.so.0 \
                 /lib/${LIB_DIR_PREFIX}-linux-gnu/libgpg-error.so.0 \
                 # /usr/bin/gpasswd 
                 /lib/${LIB_DIR_PREFIX}-linux-gnu/libsepol.so.1 \
                 /lib/${LIB_DIR_PREFIX}-linux-gnu/libbz2.so.1.0  \
                 /usr/lib/${LIB_DIR_PREFIX}-linux-gnu/libattr.so.1 /usr/lib/${LIB_DIR_PREFIX}-linux-gnu/

# Build stage used for validation of the output-image
# See validate-container-linux-* targets in Makefile
FROM output-image as validation-image

COPY --from=debian /usr/bin/ldd /usr/bin/find /usr/bin/xargs /usr/bin/
COPY --from=builder /go/src/sigs.k8s.io/gcp-filestore-csi-driver/hack/print-missing-deps.sh /print-missing-deps.sh
SHELL ["/bin/bash", "-c"]
RUN mkdir -p /run/sendsigs.omit.d
RUN /print-missing-deps.sh
RUN true
COPY deploy/kubernetes/nfs_services_start.sh /nfs_services_start.sh

# Final build stage, create the real Docker image with ENTRYPOINT
FROM output-image

ENTRYPOINT ["/gcp-filestore-csi-driver"]
