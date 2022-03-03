
> :warning: **WARNING**: The webhook is not yet ready for use, and is under development.


Steps to deploy the validation and mutation webhook:

1. `./deploy/kubernetes/webhook-example/create-cert.sh`

2. build the image by running `GCP_FS_CSI_WEBHOOK_STAGING_IMAGE=YOUR_IMAGE_REGISTRY GCP_FS_CSI_STAGING_VERSION=VERSION make webhook-image`

3. Modify `./deploy/kubernetes/webhook-example/deployment.yaml` to the correct image registry and image version

4. `cat ./deploy/kubernetes/webhook-example/mutation-configuration-template | ./deploy/kubernetes/webhook-example/patch-ca-bundle.sh > ./deploy/kubernetes/webhook-example/mutation-configuration.yaml`

6. `kubectl apply -f ./deploy/kubernetes/webhook-example/`