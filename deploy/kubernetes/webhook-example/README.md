
> :warning: **WARNING**: The webhook is not yet ready for use, and is under development.


Steps to deploy the validation and mutation webhook:

1. ./deploy/kubernetes/webhook-example/create-cert.sh

2. cat ./deploy/kubernetes/webhook-example/admission-configuration-template | ./deploy/kubernetes/webhook-example/patch-ca-bundle.sh > ./deploy/kubernetes/webhook-example/admission-configuration.yaml

3. cat ./deploy/kubernetes/webhook-example/mutation-configuration-template | ./deploy/kubernetes/webhook-example/patch-ca-bundle.sh > ./deploy/kubernetes/webhook-example/mutation-configuration.yaml

4. kubectl apply -f ./deploy/kubernetes/webhook-example/