/*
Copyright 2022 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package webhook

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/spf13/cobra"

	v1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/certwatcher"
)

var (
	certFile string
	keyFile  string
	port     int
)

// CmdWebhook is used by Cobra.
var CmdWebhook = &cobra.Command{
	Use:   "validation-webhook",
	Short: "Starts a HTTP server, uses MutatingAdmissionWebhook and ValidatingAdmissionWebhook on StorageClass",
	Long:  `Starts a HTTP server, uses MutatingAdmissionWebhook and ValidatingAdmissionWebhook on StorageClass. After deploying it to Kubernetes cluster, the Administrator needs to create a MutatingAdmissionWebhook and ValidatingWebhookConfiguration in the Kubernetes cluster to register remote webhook admission controllers.`,
	Args:  cobra.MaximumNArgs(0),
	Run:   main,
}

func init() {
	CmdWebhook.Flags().StringVar(&certFile, "tls-cert-file", "",
		"File containing the x509 Certificate for HTTPS. (CA cert, if any, concatenated after server cert). Required.")
	CmdWebhook.Flags().StringVar(&keyFile, "tls-private-key-file", "",
		"File containing the x509 private key matching --tls-cert-file. Required.")
	CmdWebhook.Flags().IntVar(&port, "port", 443,
		"Secure port that the webhook listens on")
	CmdWebhook.MarkFlagRequired("tls-cert-file")
	CmdWebhook.MarkFlagRequired("tls-private-key-file")
}

type admitv1Func func(v1.AdmissionReview) *v1.AdmissionResponse

// admitHandler is a handler, for both validators and mutators, that supports multiple admission review versions
type admitHandler struct {
	v1 admitv1Func
}

func newDelegateToV1AdmitHandler(f admitv1Func) admitHandler {
	return admitHandler{
		v1: f,
	}
}

// serve handles the http portion of a request prior to handing to an admit
// function
func serve(w http.ResponseWriter, r *http.Request, admit admitHandler) {
	var body []byte
	if r.Body == nil {
		msg := "Expected request body to be non-empty"
		klog.Error(msg)
		http.Error(w, msg, http.StatusBadRequest)
	}

	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		msg := fmt.Sprintf("Request could not be decoded: %v", err)
		klog.Error(msg)
		http.Error(w, msg, http.StatusBadRequest)
	}
	body = data

	// verify the content type is accurate
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		msg := fmt.Sprintf("contentType=%s, expect application/json", contentType)
		klog.Errorf(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	klog.V(2).Info(fmt.Sprintf("handling request: %s", body))

	deserializer := codecs.UniversalDeserializer()
	obj, gvk, err := deserializer.Decode(body, nil, nil)
	if err != nil {
		msg := fmt.Sprintf("Request could not be decoded: %v", err)
		klog.Error(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	var responseObj runtime.Object
	switch *gvk {
	case v1.SchemeGroupVersion.WithKind("AdmissionReview"):
		requestedAdmissionReview, ok := obj.(*v1.AdmissionReview)
		if !ok {
			msg := fmt.Sprintf("Expected v1.AdmissionReview but got: %T", obj)
			klog.Errorf(msg)
			http.Error(w, msg, http.StatusBadRequest)
			return
		}
		responseAdmissionReview := &v1.AdmissionReview{}
		responseAdmissionReview.SetGroupVersionKind(*gvk)
		responseAdmissionReview.Response = admit.v1(*requestedAdmissionReview)
		responseAdmissionReview.Response.UID = requestedAdmissionReview.Request.UID
		responseObj = responseAdmissionReview
	default:
		msg := fmt.Sprintf("Unsupported group version kind: %v", gvk)
		klog.Error(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	klog.V(2).Info(fmt.Sprintf("sending response: %v", responseObj))
	respBytes, err := json.Marshal(responseObj)
	if err != nil {
		klog.Error(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(respBytes); err != nil {
		klog.Error(err)
	}
}

func serveStorageClassMutate(w http.ResponseWriter, r *http.Request) {
	serve(w, r, newDelegateToV1AdmitHandler(mutateStorageClass))
}

func startServer(ctx context.Context, tlsConfig *tls.Config, cw *certwatcher.CertWatcher) error {
	go func() {
		if err := cw.Start(ctx); err != nil {
			klog.Errorf("certificate watcher error: %v", err)
		}

	}()

	fmt.Println("Starting webhook server")
	mux := http.NewServeMux()
	mux.HandleFunc("/storageclasses", serveStorageClassMutate)
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, req *http.Request) { w.Write([]byte("ok")) })
	srv := &http.Server{
		Handler:   mux,
		TLSConfig: tlsConfig,
	}

	// listener is always closed by srv.Serve
	listener, err := tls.Listen("tcp", fmt.Sprintf(":%d", port), tlsConfig)
	if err != nil {
		return err
	}

	return srv.Serve(listener)
}

func main(cmd *cobra.Command, args []string) {
	// Create new cert watcher
	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel() // stops certwatcher
	cw, err := certwatcher.New(certFile, keyFile)
	if err != nil {
		klog.Fatalf("failed to initialize new cert watcher: %v", err.Error())
	}
	tlsConfig := &tls.Config{
		GetCertificate: cw.GetCertificate,
	}

	if err := startServer(ctx, tlsConfig, cw); err != nil {
		klog.Fatalf("server stopped: %v", err.Error())
	}
}
