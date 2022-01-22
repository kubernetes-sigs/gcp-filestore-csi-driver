FROM golang:1.17.2 as builder
WORKDIR /go/src/sigs.k8s.io/gcp-filestore-csi-driver
ADD . .
ARG TAG=latest
RUN make webhook BINDIR=/bin GCP_FS_CSI_STAGING_VERSION=${TAG}

FROM launcher.gcr.io/google/debian10 as deps
ARG WEBHOOKBINARY=gcp-filestore-csi-driver-webhook
COPY --from=builder /bin/${WEBHOOKBINARY} /${WEBHOOKBINARY}
ENTRYPOINT ["/gcp-filestore-csi-driver-webhook"]