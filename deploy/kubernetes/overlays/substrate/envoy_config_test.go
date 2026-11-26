/*
Copyright 2026 The Kubernetes Authors.

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

// Package substrate holds tests for the GKE Agent Substrate overlay manifests.
//
// The tests guard the Envoy mTLS sidecar configuration against regressing to
// a setup where Envoy does not reload rotated certificates (b/586528501).
// Envoy honours watched_directory only when a TlsCertificate or
// CertificateValidationContext is delivered through SDS, so the serving
// certificate and the client CA bundle must stay in file-based SDS secrets
// that name the files kubelet writes into the projected volumes.
//
// The structural tests always run. Set ENVOY_BINARY to an Envoy binary to also
// run TestEnvoyValidate (bootstrap only) and TestEnvoyReloadsRotatedFiles
// (starts Envoy, loads the SDS secrets and rotates them).
package substrate

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"sigs.k8s.io/yaml"
)

const (
	envoyConfigMapFile      = "envoy_configmap.yaml"
	controllerPatchFile     = "controller_patch.yaml"
	rotationTestPatchFile   = "rotation_test_patch.yaml"
	kustomizationFile       = "kustomization.yaml"
	envoyContainerName      = "envoy"
	envoyConfigMountPath    = "/etc/envoy"
	servingCertMountPath    = "/run/servicedns"
	clientCAMountPath       = "/run/podidentity-ca"
	csiSocketPath           = "/csi/csi.sock"
	servingCertVolume       = "servicedns-certs"
	clientCAVolume          = "podidentity-ca"
	envoyConfigVolume       = "envoy-config"
	listenerName            = "csi_tls_listener"
	servingCertSecret       = "servicedns_cert"
	clientCASecret          = "podidentity_validation"
	ateAPIServerSPIFFEID    = "spiffe://cluster.local/ns/ate-system/sa/ate-api-server"
	secretTypeURL           = "type.googleapis.com/envoy.extensions.transport_sockets.tls.v3.Secret"
	minMaxExpirationSeconds = 3600
)

type obj = map[string]interface{}

func readYAML(t *testing.T, file string) obj {
	t.Helper()
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("reading %s: %v", file, err)
	}
	return parseYAML(t, file, b)
}

func parseYAML(t *testing.T, name string, b []byte) obj {
	t.Helper()
	var o obj
	if err := yaml.Unmarshal(b, &o); err != nil {
		t.Fatalf("parsing %s: %v", name, err)
	}
	return o
}

// get walks o along keys, where a string key indexes a map and an int key
// indexes a list. It fails the test if the path does not exist.
func get(t *testing.T, o interface{}, keys ...interface{}) interface{} {
	t.Helper()
	cur := o
	for i, k := range keys {
		switch k := k.(type) {
		case string:
			m, ok := cur.(obj)
			if !ok {
				t.Fatalf("path %v: element %d is %T, want map", keys, i, cur)
			}
			v, ok := m[k]
			if !ok {
				t.Fatalf("path %v: key %q not found", keys, k)
			}
			cur = v
		case int:
			l, ok := cur.([]interface{})
			if !ok || k < 0 || k >= len(l) {
				t.Fatalf("path %v: element %d is %T (len %d), want list with index %d", keys, i, cur, len(l), k)
			}
			cur = l[k]
		default:
			t.Fatalf("path %v: unsupported key type %T", keys, k)
		}
	}
	return cur
}

func getString(t *testing.T, o interface{}, keys ...interface{}) string {
	t.Helper()
	v := get(t, o, keys...)
	s, ok := v.(string)
	if !ok {
		t.Fatalf("path %v: got %T, want string", keys, v)
	}
	return s
}

func getList(t *testing.T, o interface{}, keys ...interface{}) []interface{} {
	t.Helper()
	v := get(t, o, keys...)
	l, ok := v.([]interface{})
	if !ok {
		t.Fatalf("path %v: got %T, want list", keys, v)
	}
	return l
}

func getObj(t *testing.T, o interface{}, keys ...interface{}) obj {
	t.Helper()
	v := get(t, o, keys...)
	m, ok := v.(obj)
	if !ok {
		t.Fatalf("path %v: got %T, want map", keys, v)
	}
	return m
}

func findByName(t *testing.T, list []interface{}, name, what string) obj {
	t.Helper()
	for _, e := range list {
		if m, ok := e.(obj); ok && m["name"] == name {
			return m
		}
	}
	t.Fatalf("%s %q not found", what, name)
	return nil
}

// findKeys returns the paths of every map key in o that is in keys.
func findKeys(o interface{}, keys map[string]bool, prefix string) []string {
	var found []string
	switch o := o.(type) {
	case obj:
		for k, v := range o {
			p := prefix + "." + k
			if keys[k] {
				found = append(found, p)
			}
			found = append(found, findKeys(v, keys, p)...)
		}
	case []interface{}:
		for i, v := range o {
			found = append(found, findKeys(v, keys, prefix+"["+strconv.Itoa(i)+"]")...)
		}
	}
	return found
}

// checkKeys fails the test unless m has exactly the keys in want. Fields that
// are not listed may change how Envoy verifies clients or serves certificates
// (for example trust_chain_verification: ACCEPT_UNTRUSTED), so a new field
// must be reviewed and added here on purpose.
func checkKeys(t *testing.T, what string, m obj, want ...string) {
	t.Helper()
	var got []string
	for k := range m {
		got = append(got, k)
	}
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s has fields %v, want exactly %v; review the effect of any new field on mTLS and update this test", what, got, want)
	}
}

// envoyConfig is the parsed csi-envoy-config ConfigMap.
type envoyConfig struct {
	configMap obj
	data      obj
	bootstrap obj
}

func loadEnvoyConfig(t *testing.T) envoyConfig {
	t.Helper()
	cm := readYAML(t, envoyConfigMapFile)
	data := getObj(t, cm, "data")
	return envoyConfig{
		configMap: cm,
		data:      data,
		bootstrap: parseYAML(t, "envoy.yaml", []byte(getString(t, data, "envoy.yaml"))),
	}
}

// sdsSecret checks that ref is a file-based SDS source for secret name whose
// file is a key of the ConfigMap mounted at /etc/envoy. It returns the single
// Secret in that file.
func (c envoyConfig) sdsSecret(t *testing.T, ref obj, name string) obj {
	t.Helper()
	if got := getString(t, ref, "name"); got != name {
		t.Errorf("SDS secret reference name = %q, want %q", got, name)
	}
	if v := get(t, ref, "sds_config", "resource_api_version"); v != "V3" {
		t.Errorf("SDS secret %q: resource_api_version = %v, want V3", name, v)
	}
	// path_config_source carries only path. Without watched_directory or
	// poll_interval, Envoy reloads the SDS file only when something is
	// renamed onto that exact path. Kubelet updates a ConfigMap volume by
	// renaming ..data, never the sds_*.yaml symlinks, so in practice a
	// ConfigMap edit (for example to the client SAN pin) applies only when
	// the pod restarts, the same as envoy.yaml. That keeps a wrong but valid
	// edit from taking down a running Envoy and keeps envoy.yaml and the SDS
	// files consistent. Rotated certificates are still picked up through the
	// watched_directory inside each Secret.
	pcs := getObj(t, ref, "sds_config", "path_config_source")
	checkKeys(t, fmt.Sprintf("SDS secret %q path_config_source", name), pcs, "path")
	sdsPath := getString(t, pcs, "path")
	if path.Dir(sdsPath) != envoyConfigMountPath {
		t.Errorf("SDS secret %q: path %q is not in %s, where the ConfigMap is mounted", name, sdsPath, envoyConfigMountPath)
	}
	file, ok := c.data[path.Base(sdsPath)].(string)
	if !ok {
		t.Fatalf("SDS secret %q: %s has no data key %q for %s", name, envoyConfigMapFile, path.Base(sdsPath), sdsPath)
	}
	resources := getList(t, parseYAML(t, sdsPath, []byte(file)), "resources")
	if len(resources) != 1 {
		t.Fatalf("SDS file %s: got %d resources, want 1", sdsPath, len(resources))
	}
	secret := getObj(t, resources, 0)
	if typ := getString(t, secret, "@type"); typ != secretTypeURL {
		t.Errorf("SDS file %s: @type = %q, want %q", sdsPath, typ, secretTypeURL)
	}
	if n := getString(t, secret, "name"); n != name {
		t.Errorf("SDS file %s: secret name = %q, want %q referenced by the listener", sdsPath, n, name)
	}
	return secret
}

// commonTLSContext returns the listener's DownstreamTlsContext
// common_tls_context after checking the listener's shape.
func (c envoyConfig) commonTLSContext(t *testing.T) obj {
	t.Helper()
	listeners := getList(t, c.bootstrap, "static_resources", "listeners")
	if len(listeners) != 1 {
		t.Fatalf("envoy.yaml has %d listeners, want 1", len(listeners))
	}
	listener := findByName(t, listeners, listenerName, "listener")
	chains := getList(t, listener, "filter_chains")
	if len(chains) != 1 {
		t.Fatalf("listener %q has %d filter chains, want 1", listenerName, len(chains))
	}
	// No filter_chain_match: the single chain must serve every connection.
	checkKeys(t, "filter chain", getObj(t, chains, 0), "transport_socket", "filters")
	tls := getObj(t, chains, 0, "transport_socket", "typed_config")
	checkKeys(t, "DownstreamTlsContext", tls, "@type", "require_client_certificate", "common_tls_context")
	if got := tls["require_client_certificate"]; got != true {
		t.Errorf("require_client_certificate = %v, want true", got)
	}
	ctx := getObj(t, tls, "common_tls_context")
	checkKeys(t, "common_tls_context", ctx, "alpn_protocols", "tls_certificate_sds_secret_configs", "validation_context_sds_secret_config")
	if got := getList(t, ctx, "alpn_protocols"); !reflect.DeepEqual(got, []interface{}{"h2"}) {
		t.Errorf("alpn_protocols = %v, want [h2] (CSI is gRPC)", got)
	}
	return ctx
}

// sdsTLSCertificate returns the TlsCertificate the listener serves.
func (c envoyConfig) sdsTLSCertificate(t *testing.T) obj {
	t.Helper()
	refs := getList(t, c.commonTLSContext(t), "tls_certificate_sds_secret_configs")
	if len(refs) != 1 {
		t.Fatalf("got %d tls_certificate_sds_secret_configs, want 1", len(refs))
	}
	secret := c.sdsSecret(t, getObj(t, refs, 0), servingCertSecret)
	checkKeys(t, "Secret "+servingCertSecret, secret, "@type", "name", "tls_certificate")
	cert := getObj(t, secret, "tls_certificate")
	checkKeys(t, "tls_certificate", cert, "certificate_chain", "private_key", "watched_directory")
	return cert
}

// sdsValidationContext returns the validation context for client certificates.
func (c envoyConfig) sdsValidationContext(t *testing.T) obj {
	t.Helper()
	secret := c.sdsSecret(t, getObj(t, c.commonTLSContext(t), "validation_context_sds_secret_config"), clientCASecret)
	checkKeys(t, "Secret "+clientCASecret, secret, "@type", "name", "validation_context")
	val := getObj(t, secret, "validation_context")
	// Exact keys: no trust_chain_verification, allow_expired_certificate or
	// other field that weakens client verification.
	checkKeys(t, "validation_context", val, "trusted_ca", "watched_directory", "match_typed_subject_alt_names")
	return val
}

func checkUnderWatchedDir(t *testing.T, what, file, watched, mount string) {
	t.Helper()
	if watched != mount {
		t.Errorf("%s: watched_directory = %q, want the volume mount path %q", what, watched, mount)
	}
	if path.Dir(file) != watched {
		t.Errorf("%s: file %q is not directly in watched_directory %q, so kubelet's ..data swap there would not reload it", what, file, watched)
	}
}

func TestEnvoyCertificatesUseSDSWithWatchedDirectory(t *testing.T) {
	c := loadEnvoyConfig(t)

	// Certificates, keys and CA bundles given inline in envoy.yaml are read
	// once at startup; watched_directory has no effect there. Ban every field
	// that carries such file material anywhere in the bootstrap.
	if found := findKeys(c.bootstrap, map[string]bool{
		"tls_certificates":   true,
		"validation_context": true,
		"trusted_ca":         true,
		"certificate_chain":  true,
		"private_key":        true,
		"watched_directory":  true,
	}, "envoy.yaml"); len(found) > 0 {
		t.Errorf("envoy.yaml has inline TLS file fields at %v; deliver them through file-based SDS so Envoy reloads rotated files (b/586528501)", found)
	}

	// The admin API is unauthenticated (it can drain listeners and dump
	// config), so it must stay on loopback.
	if got := getString(t, c.bootstrap, "admin", "address", "socket_address", "address"); got != "127.0.0.1" {
		t.Errorf("admin address = %q, want 127.0.0.1", got)
	}

	// Envoy refuses to start an SDS subscription without node id and cluster.
	for _, k := range []string{"id", "cluster"} {
		if getString(t, c.bootstrap, "node", k) == "" {
			t.Errorf("envoy.yaml node.%s is empty; SDS requires it", k)
		}
	}

	// For a file-based SDS secret, Envoy watches the directory of each file
	// for renames even without watched_directory, so kubelet's ..data swap is
	// picked up either way (TestEnvoyReloadsRotatedFiles passes without it on
	// v1.34.14). watched_directory is still required here: it states the
	// volume root explicitly, matches Agent Substrate's own router config,
	// and keeps working if the files move below the volume root.
	cert := c.sdsTLSCertificate(t)
	watched := getString(t, cert, "watched_directory", "path")
	checkUnderWatchedDir(t, "serving certificate chain", getString(t, cert, "certificate_chain", "filename"), watched, servingCertMountPath)
	checkUnderWatchedDir(t, "serving certificate key", getString(t, cert, "private_key", "filename"), watched, servingCertMountPath)

	val := c.sdsValidationContext(t)
	checkUnderWatchedDir(t, "client CA bundle", getString(t, val, "trusted_ca", "filename"), getString(t, val, "watched_directory", "path"), clientCAMountPath)
	sans := getList(t, val, "match_typed_subject_alt_names")
	if len(sans) != 1 || getString(t, sans[0], "san_type") != "URI" || getString(t, sans[0], "matcher", "exact") != ateAPIServerSPIFFEID {
		t.Errorf("match_typed_subject_alt_names = %v, want exactly one URI SAN pinned to %q", sans, ateAPIServerSPIFFEID)
	} else {
		checkKeys(t, "SAN matcher", getObj(t, sans[0], "matcher"), "exact")
	}
}

func envoyContainer(t *testing.T, deploy obj) obj {
	t.Helper()
	return findByName(t, getList(t, deploy, "spec", "template", "spec", "containers"), envoyContainerName, "container")
}

func TestEnvoyMountsMatchConfig(t *testing.T) {
	c := loadEnvoyConfig(t)
	deploy := readYAML(t, controllerPatchFile)
	volumes := getList(t, deploy, "spec", "template", "spec", "volumes")

	configVolume := getObj(t, findByName(t, volumes, envoyConfigVolume, "volume"), "configMap")
	if got, want := getString(t, configVolume, "name"), getString(t, c.configMap, "metadata", "name"); got != want {
		t.Errorf("volume %q uses ConfigMap %q, want %q", envoyConfigVolume, got, want)
	}
	// An items list would project only some ConfigMap keys, so an SDS file
	// could silently be missing from /etc/envoy.
	if items, ok := configVolume["items"]; ok {
		t.Errorf("volume %q projects only ConfigMap items %v; mount the whole ConfigMap so every SDS file is present", envoyConfigVolume, items)
	}

	// An optional volume source lets the pod start without the file, and
	// Envoy would then reject every handshake.
	if v, ok := configVolume["optional"]; ok && v != false {
		t.Errorf("volume %q configMap.optional = %v, want unset or false", envoyConfigVolume, v)
	}
	if v, ok := getObj(t, findByName(t, volumes, clientCAVolume, "volume"), "projected", "sources", 0, "clusterTrustBundle")["optional"]; ok && v != false {
		t.Errorf("volume %q clusterTrustBundle.optional = %v, want unset or false", clientCAVolume, v)
	}

	// The SDS secrets must name exactly the files kubelet writes into the
	// projected volumes. `envoy --mode validate` does not load SDS secrets,
	// so nothing else in CI catches a wrong file name, and Envoy would start
	// and report ready but reject every handshake.
	bundle := getString(t, findByName(t, volumes, servingCertVolume, "volume"), "projected", "sources", 0, "podCertificate", "credentialBundlePath")
	trust := getString(t, findByName(t, volumes, clientCAVolume, "volume"), "projected", "sources", 0, "clusterTrustBundle", "path")
	cert := c.sdsTLSCertificate(t)
	for _, f := range []string{"certificate_chain", "private_key"} {
		if got, want := getString(t, cert, f, "filename"), path.Join(servingCertMountPath, bundle); got != want {
			t.Errorf("serving certificate %s = %q, want %q (credentialBundlePath in %s)", f, got, want, controllerPatchFile)
		}
	}
	if got, want := getString(t, c.sdsValidationContext(t), "trusted_ca", "filename"), path.Join(clientCAMountPath, trust); got != want {
		t.Errorf("client CA bundle = %q, want %q (clusterTrustBundle path in %s)", got, want, controllerPatchFile)
	}

	envoy := envoyContainer(t, deploy)
	if got, want := get(t, envoy, "command"), []interface{}{"/usr/local/bin/envoy"}; !reflect.DeepEqual(got, want) {
		t.Errorf("envoy command = %v, want %v", got, want)
	}
	if got, want := get(t, envoy, "args"), []interface{}{"-c", envoyConfigMountPath + "/envoy.yaml"}; !reflect.DeepEqual(got, want) {
		t.Errorf("envoy args = %v, want %v", got, want)
	}
	mounts := map[string]string{}
	for _, m := range getList(t, envoy, "volumeMounts") {
		mounts[getString(t, m, "name")] = getString(t, m, "mountPath")
		// Kubelet never updates subPath mounts, so rotated certificates would
		// not reach Envoy, and watched_directory relies on kubelet swapping
		// ..data at the volume root.
		for _, k := range []string{"subPath", "subPathExpr"} {
			if _, ok := m.(obj)[k]; ok {
				t.Errorf("envoy volumeMount %v uses %s; kubelet does not update such mounts", m, k)
			}
		}
	}
	for _, m := range getList(t, envoy, "volumeMounts") {
		name := getString(t, m, "name")
		if (name == servingCertVolume || name == clientCAVolume) && m.(obj)["readOnly"] != true {
			t.Errorf("envoy volumeMount %q is not readOnly", name)
		}
	}
	for volume, want := range map[string]string{
		envoyConfigVolume: envoyConfigMountPath,
		servingCertVolume: servingCertMountPath,
		clientCAVolume:    clientCAMountPath,
	} {
		if got := mounts[volume]; got != want {
			t.Errorf("envoy container mounts volume %q at %q, want %q", volume, got, want)
		}
	}
}

func TestEnvoyImagePinned(t *testing.T) {
	image := getString(t, envoyContainer(t, readYAML(t, controllerPatchFile)), "image")
	if !regexp.MustCompile(`^envoyproxy/envoy:v\d+\.\d+\.\d+@sha256:[0-9a-f]{64}$`).MatchString(image) {
		t.Errorf("envoy image = %q, want a release tag pinned by digest (envoyproxy/envoy:vX.Y.Z@sha256:...)", image)
	}
}

// findStrings returns every string value in o that contains substr.
func findStrings(o interface{}, substr string) []string {
	var found []string
	switch o := o.(type) {
	case string:
		if strings.Contains(o, substr) {
			found = append(found, o)
		}
	case obj:
		for _, v := range o {
			found = append(found, findStrings(v, substr)...)
		}
	case []interface{}:
		for _, v := range o {
			found = append(found, findStrings(v, substr)...)
		}
	}
	return found
}

func TestKustomization(t *testing.T) {
	k := readYAML(t, kustomizationFile)
	if found := findStrings(k, rotationTestPatchFile); len(found) > 0 {
		t.Errorf("%s references %s (%v); it is test-only and would ship 1-hour certificates", kustomizationFile, rotationTestPatchFile, found)
	}
	resources := map[string]bool{}
	for _, r := range getList(t, k, "resources") {
		resources[r.(string)] = true
	}
	if !resources[envoyConfigMapFile] {
		t.Errorf("%s resources do not include %s", kustomizationFile, envoyConfigMapFile)
	}
	patches := map[string]bool{}
	for _, p := range getList(t, k, "patches") {
		if s, ok := p.(obj)["path"].(string); ok {
			patches[s] = true
		}
	}
	if !patches[controllerPatchFile] {
		t.Errorf("%s patches do not include %s", kustomizationFile, controllerPatchFile)
	}
	// An images: entry for Envoy would override the digest pin that
	// TestEnvoyImagePinned checks in controller_patch.yaml.
	if images, ok := k["images"].([]interface{}); ok {
		for _, img := range images {
			if found := findStrings(img, "envoy"); len(found) > 0 {
				t.Errorf("%s images: overrides the Envoy image (%v); pin it in %s instead", kustomizationFile, img, controllerPatchFile)
			}
		}
	}
}

func podCertificateSource(t *testing.T, file string) obj {
	t.Helper()
	volume := findByName(t, getList(t, readYAML(t, file), "spec", "template", "spec", "volumes"), servingCertVolume, "volume")
	sources := getList(t, volume, "projected", "sources")
	if len(sources) != 1 {
		t.Fatalf("%s: volume %q has %d sources, want 1", file, servingCertVolume, len(sources))
	}
	return getObj(t, sources, 0, "podCertificate")
}

func TestRotationTestPatchMatchesControllerPatch(t *testing.T) {
	prod := podCertificateSource(t, controllerPatchFile)
	test := podCertificateSource(t, rotationTestPatchFile)

	if got := test["maxExpirationSeconds"]; got != float64(minMaxExpirationSeconds) {
		t.Errorf("%s: maxExpirationSeconds = %v, want %d (the API server minimum)", rotationTestPatchFile, got, minMaxExpirationSeconds)
	}
	if _, ok := prod["maxExpirationSeconds"]; ok {
		t.Errorf("%s sets maxExpirationSeconds; production should use the API server default (86400)", controllerPatchFile)
	}
	// A strategic merge patch replaces projected.sources, so the test patch
	// must repeat every production field.
	want := obj{}
	for k, v := range prod {
		want[k] = v
	}
	want["maxExpirationSeconds"] = float64(minMaxExpirationSeconds)
	if !reflect.DeepEqual(test, want) {
		t.Errorf("%s podCertificate = %v, want the %s source plus maxExpirationSeconds: %v", rotationTestPatchFile, test, controllerPatchFile, want)
	}
}

// TestReadmeRestorePatchMatchesControllerPatch checks that the README's
// command for undoing rotation_test_patch.yaml restores exactly the production
// podCertificate source.
func TestReadmeRestorePatchMatchesControllerPatch(t *testing.T) {
	b, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`'(\{"spec":.*servicedns-certs.*\})'`).FindSubmatch(b)
	if m == nil {
		t.Fatal("README.md: restore patch for servicedns-certs not found")
	}
	sources := getList(t, findByName(t, getList(t, parseYAML(t, "README.md restore patch", m[1]), "spec", "template", "spec", "volumes"), servingCertVolume, "volume"), "projected", "sources")
	if len(sources) != 1 {
		t.Fatalf("README.md restore patch: got %d sources, want 1", len(sources))
	}
	if got, want := getObj(t, sources, 0, "podCertificate"), podCertificateSource(t, controllerPatchFile); !reflect.DeepEqual(got, want) {
		t.Errorf("README.md restore patch podCertificate = %v, want the %s source %v", got, controllerPatchFile, want)
	}
}

// envoyBinary returns $ENVOY_BINARY or skips the test.
func envoyBinary(t *testing.T) string {
	t.Helper()
	envoy := os.Getenv("ENVOY_BINARY")
	if envoy == "" {
		t.Skip("ENVOY_BINARY not set; see README.md for how to extract it from the pinned image")
	}
	return envoy
}

// writeEnvoyConfig writes the ConfigMap's files under root/etc/envoy with the
// pod paths rewritten to root, applying extra replacements to envoy.yaml, and
// returns the path of envoy.yaml.
func writeEnvoyConfig(t *testing.T, c envoyConfig, root string, extra ...string) string {
	t.Helper()
	rewrite := strings.NewReplacer(
		envoyConfigMountPath, root+envoyConfigMountPath,
		servingCertMountPath, root+servingCertMountPath,
		clientCAMountPath, root+clientCAMountPath,
		csiSocketPath, root+csiSocketPath,
	)
	if err := os.MkdirAll(root+envoyConfigMountPath, 0o755); err != nil {
		t.Fatal(err)
	}
	for k, v := range c.data {
		s, ok := v.(string)
		if !ok {
			t.Fatalf("%s: data[%s] is %T, want string", envoyConfigMapFile, k, v)
		}
		s = rewrite.Replace(s)
		if k == "envoy.yaml" {
			for i := 0; i+1 < len(extra); i += 2 {
				if !strings.Contains(s, extra[i]) {
					t.Fatalf("envoy.yaml does not contain %q to rewrite for the test", extra[i])
				}
				s = strings.ReplaceAll(s, extra[i], extra[i+1])
			}
		}
		if err := os.WriteFile(filepath.Join(root+envoyConfigMountPath, k), []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return filepath.Join(root+envoyConfigMountPath, "envoy.yaml")
}

// TestEnvoyValidate runs `envoy --mode validate` on the bootstrap. Validate
// mode checks envoy.yaml (field names, node, that the SDS files exist) but
// does not load the SDS secrets or the certificate files they name;
// TestEnvoyReloadsRotatedFiles covers those.
func TestEnvoyValidate(t *testing.T) {
	envoy := envoyBinary(t)
	cfg := writeEnvoyConfig(t, loadEnvoyConfig(t), t.TempDir())
	out, err := exec.Command(envoy, "--mode", "validate", "-c", cfg, "--log-level", "warning").CombinedOutput()
	if err != nil {
		t.Fatalf("envoy --mode validate failed: %v\n%s", err, out)
	}
	t.Logf("envoy --mode validate:\n%s", out)
}

// testCert returns a self-signed ECDSA certificate with the given serial as a
// podCertificate credential bundle (private key, then chain) and as a PEM
// certificate for a trust bundle.
func testCert(t *testing.T, serial int64) (bundle, certPEM []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(serial),
		Subject:               pkix.Name{CommonName: "envoy-config-test"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	pkcs8, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	return append(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8}), certPEM...), certPEM
}

// atomicWrite writes file into dir the way kubelet's AtomicWriter updates a
// projected volume: the content goes into a new timestamped directory, a
// ..data_tmp symlink to it is renamed over ..data, and file is a symlink to
// ..data/file.
func atomicWrite(t *testing.T, dir, file string, content []byte) {
	t.Helper()
	ts := fmt.Sprintf("..%d", time.Now().UnixNano())
	if err := os.MkdirAll(filepath.Join(dir, ts), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ts, file), content, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(ts, filepath.Join(dir, "..data_tmp")); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(dir, "..data_tmp"), filepath.Join(dir, "..data")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(dir, file)); os.IsNotExist(err) {
		if err := os.Symlink(filepath.Join("..data", file), filepath.Join(dir, file)); err != nil {
			t.Fatal(err)
		}
	}
}

// lockedBuffer collects Envoy's output while the test reads it.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

type envoyAdmin struct {
	t      *testing.T
	addr   string
	client *http.Client
}

func (a envoyAdmin) get(p string) (int, []byte) {
	a.t.Helper()
	resp, err := a.client.Get("http://" + a.addr + p)
	if err != nil {
		return 0, nil
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b
}

// stats returns the admin /stats counters and gauges by name. The listener
// counters, whose names include the bound address, are also summed under
// their name without the listener prefix (for example
// "server_ssl_socket_factory.ssl_context_update_by_sds").
func (a envoyAdmin) stats() map[string]int64 {
	a.t.Helper()
	_, b := a.get("/stats")
	stats := map[string]int64{}
	for _, line := range strings.Split(string(b), "\n") {
		name, value, ok := strings.Cut(line, ": ")
		if !ok {
			continue
		}
		n, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			continue
		}
		stats[name] = n
		if strings.HasPrefix(name, "listener.") {
			if i := strings.Index(name, ".server_ssl_socket_factory."); i >= 0 {
				stats[name[i+1:]] += n
			}
		}
	}
	return stats
}

// serials returns the serial numbers of the certificates Envoy has loaded.
func (a envoyAdmin) serials() []string {
	a.t.Helper()
	_, b := a.get("/certs")
	var certs struct {
		Certificates []struct {
			CACert []struct {
				SerialNumber string `json:"serial_number"`
			} `json:"ca_cert"`
			CertChain []struct {
				SerialNumber string `json:"serial_number"`
			} `json:"cert_chain"`
		} `json:"certificates"`
	}
	if err := json.Unmarshal(b, &certs); err != nil {
		return nil
	}
	var s []string
	for _, c := range certs.Certificates {
		for _, x := range c.CACert {
			s = append(s, "ca:"+x.SerialNumber)
		}
		for _, x := range c.CertChain {
			s = append(s, "cert:"+x.SerialNumber)
		}
	}
	sort.Strings(s)
	return s
}

// waitFor polls cond for up to 15 seconds.
func waitFor(t *testing.T, what string, cond func() bool, out *lockedBuffer) {
	t.Helper()
	for deadline := time.Now().Add(15 * time.Second); time.Now().Before(deadline); time.Sleep(100 * time.Millisecond) {
		if cond() {
			return
		}
	}
	t.Fatalf("timed out waiting for %s; Envoy output:\n%s", what, out)
}

// TestEnvoyReloadsRotatedFiles starts Envoy with the overlay's configuration
// on loopback, checks that both SDS secrets load from projected-volume style
// directories, then rotates the serving certificate and the CA bundle the way
// kubelet does and checks that Envoy reloads each one without a restart.
func TestEnvoyReloadsRotatedFiles(t *testing.T) {
	envoy := envoyBinary(t)
	root := t.TempDir()
	cfg := writeEnvoyConfig(t, loadEnvoyConfig(t), root,
		"address: 0.0.0.0, port_value: 10000", "address: 127.0.0.1, port_value: 0",
		"port_value: 15000", "port_value: 0",
	)
	// Write the files under the names the projected volumes use, so a wrong
	// file name in an SDS secret also fails here.
	volumes := getList(t, readYAML(t, controllerPatchFile), "spec", "template", "spec", "volumes")
	bundleFile := getString(t, findByName(t, volumes, servingCertVolume, "volume"), "projected", "sources", 0, "podCertificate", "credentialBundlePath")
	trustFile := getString(t, findByName(t, volumes, clientCAVolume, "volume"), "projected", "sources", 0, "clusterTrustBundle", "path")
	bundle, _ := testCert(t, 0x1001)
	_, ca := testCert(t, 0x2001)
	atomicWrite(t, root+servingCertMountPath, bundleFile, bundle)
	atomicWrite(t, root+clientCAMountPath, trustFile, ca)

	addrFile := filepath.Join(root, "admin-address")
	var out lockedBuffer
	cmd := exec.Command(envoy, "-c", cfg, "--admin-address-path", addrFile, "--disable-hot-restart",
		"--concurrency", "1", "--reject-unknown-dynamic-fields", "--log-level", "info")
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	admin := envoyAdmin{t: t, client: &http.Client{Transport: &http.Transport{Proxy: nil}, Timeout: 2 * time.Second}}
	waitFor(t, "the admin address", func() bool {
		b, err := os.ReadFile(addrFile)
		admin.addr = strings.TrimSpace(string(b))
		return err == nil && admin.addr != ""
	}, &out)
	waitFor(t, "Envoy to report ready", func() bool {
		code, _ := admin.get("/ready")
		return code == http.StatusOK
	}, &out)

	const updates = "server_ssl_socket_factory.ssl_context_update_by_sds"
	initial := admin.stats()
	for _, s := range []string{servingCertSecret, clientCASecret} {
		if got := initial["sds."+s+".update_success"]; got < 1 {
			t.Errorf("sds.%s.update_success = %d, want >= 1", s, got)
		}
		for _, bad := range []string{"update_rejected", "update_failure", "key_rotation_failed"} {
			if got := initial["sds."+s+"."+bad]; got != 0 {
				t.Errorf("sds.%s.%s = %d, want 0", s, bad, got)
			}
		}
	}
	if got := initial[updates]; got < 2 {
		t.Errorf("%s = %d at startup, want >= 2 (one per SDS secret)", updates, got)
	}
	if got, want := admin.serials(), []string{"ca:2001", "cert:1001"}; !reflect.DeepEqual(got, want) {
		t.Errorf("loaded certificates = %v, want %v", got, want)
	}

	// Rotate the serving certificate, then the CA bundle.
	for _, step := range []struct {
		dir, file string
		content   []byte
		want      []string
	}{
		{root + servingCertMountPath, bundleFile, func() []byte { b, _ := testCert(t, 0x1002); return b }(), []string{"ca:2001", "cert:1002"}},
		{root + clientCAMountPath, trustFile, func() []byte { _, c := testCert(t, 0x2002); return c }(), []string{"ca:2002", "cert:1002"}},
	} {
		before := admin.stats()[updates]
		atomicWrite(t, step.dir, step.file, step.content)
		waitFor(t, fmt.Sprintf("Envoy to load %v after rotating %s", step.want, step.file), func() bool {
			return reflect.DeepEqual(admin.serials(), step.want)
		}, &out)
		if got := admin.stats()[updates]; got <= before {
			t.Errorf("%s = %d after rotating %s, want > %d", updates, got, step.file, before)
		}
	}
	final := admin.stats()
	for _, s := range []string{servingCertSecret, clientCASecret} {
		if got := final["sds."+s+".key_rotation_failed"]; got != 0 {
			t.Errorf("sds.%s.key_rotation_failed = %d, want 0", s, got)
		}
	}
	if t.Failed() {
		t.Logf("Envoy output:\n%s", out.String())
	}
}
