module sigs.k8s.io/gcp-filestore-csi-driver

go 1.14

require (
	cloud.google.com/go v0.56.0
	github.com/container-storage-interface/spec v1.3.0
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/golang/protobuf v1.4.2 // indirect
	github.com/kubernetes-csi/csi-lib-utils v0.7.0
	github.com/kubernetes-csi/csi-test/v3 v3.1.1
	github.com/onsi/ginkgo v1.11.0
	github.com/onsi/gomega v1.8.1
	github.com/spf13/pflag v1.0.5
	golang.org/x/net v0.0.0-20200707034311-ab3426394381
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d
	google.golang.org/api v0.28.0
	google.golang.org/appengine v1.6.6 // indirect
	google.golang.org/genproto v0.0.0-20200626011028-ee7919e894b5 // indirect
	google.golang.org/grpc v1.30.0
	google.golang.org/protobuf v1.25.0 // indirect
	k8s.io/apimachinery v0.18.5
	k8s.io/utils v0.0.0-20200619165400-6e3d28b6ed19
	sigs.k8s.io/boskos v0.0.0-20200617235605-f289ba6555ba
)
