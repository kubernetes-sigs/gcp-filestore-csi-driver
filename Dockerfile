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

# Build driver
FROM golang:1.10.1-alpine3.7 as builder
WORKDIR /go/src/sigs.k8s.io/gcp-filestore-csi-driver
ADD . .
ARG TAG
RUN CGO_ENABLED=0 go build -a -ldflags '-X main.version='"${TAG:-latest}"' -extldflags "-static"' -o bin/gcp-filestore-csi-driver ./cmd

# Install nfs packages
FROM launcher.gcr.io/google/debian9 as deps
ENV DEBIAN_FRONTEND noninteractive
RUN apt-get update && apt-get dist-upgrade -y && apt-get install -y --no-install-recommends \
    mount \
    nfs-common

# This is needed for rpcbind
RUN mkdir /run/sendsigs.omit.d

# Hold required packages to avoid breaking the installation of packages
RUN apt-mark hold apt gnupg adduser passwd libsemanage1 libcap2 mount nfs-common init

# Clean up unnecessary packages
# This list is copied from
# https://github.com/kubernetes/kubernetes/blob/master/build/debian-base/Dockerfile.build
# and modified to keep nfs dependencies
RUN echo "Yes, do as I say!" | apt-get purge \
    # bash \
    e2fslibs \
    e2fsprogs \
    # init \
    # initscripts \
    # libkmod2 \
    # libmount1 \
    # libsmartcols1 \
    # libudev1 \
    # libblkid1 \
    libncursesw5 \
    libss2 \
    ncurses-base \
    ncurses-bin \
    # systemd \
    # systemd-sysv \
    tzdata

# Cleanup cached and unnecessary files.
RUN apt-get autoremove -y && \
    apt-get clean -y && \
    tar -czf /usr/share/copyrights.tar.gz /usr/share/common-licenses /usr/share/doc/*/copyright && \
    rm -rf \
        /usr/share/doc \
        /usr/share/man \
        /usr/share/info \
        /usr/share/locale \
        /var/lib/apt/lists/* \
        /var/log/* \
        /var/cache/debconf/* \
        /usr/share/common-licenses* \
        /usr/share/bash-completion \
        ~/.bashrc \
        ~/.profile \
        # /etc/systemd \
        # /lib/lsb \
        /lib/udev \
        /usr/lib/x86_64-linux-gnu/gconv/IBM* \
        /usr/lib/x86_64-linux-gnu/gconv/EBC* && \
    mkdir -p /usr/share/man/man1 /usr/share/man/man2 \
        /usr/share/man/man3 /usr/share/man/man4 \
        /usr/share/man/man5 /usr/share/man/man6 \
        /usr/share/man/man7 /usr/share/man/man8

# Copy driver into image
FROM deps
COPY --from=builder /go/src/sigs.k8s.io/gcp-filestore-csi-driver/bin/gcp-filestore-csi-driver /gcp-filestore-csi-driver
COPY deploy/kubernetes/nfs_services_start.sh /nfs_services_start.sh

ENTRYPOINT ["/gcp-filestore-csi-driver"]
