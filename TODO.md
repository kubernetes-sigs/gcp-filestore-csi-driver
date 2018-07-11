* Can skaffold ignore certain file changes?
* Set image pull policy always for dev env
* 1 sanity tests are currently failing due to:
  * CreateVolume cleanup: https://github.com/kubernetes-csi/csi-test/issues/89
* Integration tests
* E2E tests
* Slim down container image. A ton of the init scripts can be removed, we
  only want the nfs and rpc ones. Evaluate if more packages can be removed
* Add nfs service health checking script
* Metrics
