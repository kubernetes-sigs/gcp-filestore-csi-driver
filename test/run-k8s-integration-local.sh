readonly PKGDIR=${GOPATH}/src/sigs.k8s.io/gcp-filestore-csi-driver

# Some commonly run subset of tests focus strings.
all_external_tests_focus="External.*Storage"
subpath_test_focus="External.*Storage.*default.*fs.*subPath"
snapshot_test_focus="External.*Storage.*snapshot"
multivolume_fs_test_focus="External.*Storage.*filesystem.*multiVolume"
expansion_test_focus="External.*Storage.*allowExpansion"

# This version of the command builds and deploys the GCE PS CSI driver for dev overlay.
# Points to a local K8s repository to get the e2e test binary, does not bring up
# or tear down the kubernetes cluster. In addition, it runs a subset of tests based on the test focus ginkgo string.
# For 'dev' overlay, GCE_FS_DEV_OVERLAY_SA_NAME is expected to be set. If deploy/project_setup.sh is used to create the SA, it would be 'gcp-filestore-csi-driver-sa@<your-gcp-project>.iam.gserviceaccount.com'

# E.g. run command: GCE_FS_CSI_STAGING_IMAGE=gcr.io/<your-gcp-project>/gcp-filestore-csi-driver KTOP=$GOPATH/src/k8s.io/kubernetes/ GCE_FS_DEV_OVERLAY_SA_NAME=gcp-filestore-csi-driver-sa@<your-gcp-project>.iam.gserviceaccount.com test/run-k8s-integration-local.sh | tee log

${PKGDIR}/bin/k8s-integration-test --run-in-prow=false \
--staging-image=${GCE_FS_CSI_STAGING_IMAGE} \
--deploy-overlay-name=dev --dev-overlay-sa=${GCE_FS_DEV_OVERLAY_SA_NAME:-} --bringup-cluster=false --teardown-cluster=false --teardown-driver=false --test-focus=${subpath_test_focus} --local-k8s-dir=$KTOP \
--do-driver-build=true --gce-zone="us-central1-b" --num-nodes=${NUM_NODES:-3}
