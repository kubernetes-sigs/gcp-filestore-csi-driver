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

package substrate

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// stubKubectl stands in for kubectl when deploy.sh runs in tests. It records
// applied manifests and answers the StorageClass queries from
// $STUB_DIR/existing.yaml according to $SC_STATE (missing, present or error).
// For "patch --local" and "get -o go-template" it prints the fields that the
// go-template asks for (provisioner, reclaimPolicy, volumeBindingMode,
// parameters), read from the manifest with awk, so no real kubectl is needed.
const stubKubectl = `#!/bin/bash
fields() {
  local tmpl="$1"
  awk -v want_prov="$([[ "$tmpl" == *'{{.provisioner}}'* ]] && echo 1)" \
      -v want_rp="$([[ "$tmpl" == *'{{.reclaimPolicy}}'* ]] && echo 1)" \
      -v want_vbm="$([[ "$tmpl" == *'{{.volumeBindingMode}}'* ]] && echo 1)" \
      -v want_params="$([[ "$tmpl" == *'.parameters'* ]] && echo 1)" '
    /^[[:space:]]*#/ { next }
    /^provisioner:/ { sub(/^provisioner:[[:space:]]*/, ""); prov = $0; inparams = 0; next }
    /^reclaimPolicy:/ { sub(/^reclaimPolicy:[[:space:]]*/, ""); rp = $0; inparams = 0; next }
    /^volumeBindingMode:/ { sub(/^volumeBindingMode:[[:space:]]*/, ""); vbm = $0; inparams = 0; next }
    /^parameters:/ { inparams = 1; next }
    /^[^[:space:]]/ { inparams = 0 }
    inparams && /^[[:space:]]+[^[:space:]]+:/ {
      line = $0; sub(/^[[:space:]]+/, "", line)
      key = line; sub(/:.*/, "", key)
      val = line; sub(/^[^:]*:[[:space:]]*/, "", val); gsub(/"/, "", val)
      params[key] = val
    }
    END {
      if (want_prov) print "provisioner=" prov
      if (want_rp) print "reclaimPolicy=" rp
      if (want_vbm) print "volumeBindingMode=" vbm
      if (!want_params) exit
      n = 0; for (k in params) keys[++n] = k
      for (i = 1; i <= n; i++) for (j = i + 1; j <= n; j++) if (keys[j] < keys[i]) { t = keys[i]; keys[i] = keys[j]; keys[j] = t }
      for (i = 1; i <= n; i++) print "parameters." keys[i] "=" params[keys[i]]
    }'
}
echo "kubectl $*" >> "$STUB_DIR/calls"
tmpl=""
for arg in "$@"; do [[ "$arg" == -o=go-template=* || "$arg" == go-template=* ]] && tmpl="${arg#*go-template=}"; done
case "$1 $2" in
  "apply -k") exit 0 ;;
  "apply -f") cat > "$STUB_DIR/applied.yaml"; exit 0 ;;
  "patch --local") fields "$tmpl"; exit 0 ;;
  "get storageclass")
    case "$SC_STATE" in
      error) echo "error: You must be logged in to the server (Unauthorized)" >&2; exit 1 ;;
      missing) exit 0 ;;
    esac
    if [[ "$*" == *--ignore-not-found* ]]; then echo "storageclass.storage.k8s.io/$3"; exit 0; fi
    fields "$tmpl" < "$STUB_DIR/existing.yaml"; exit 0 ;;
esac
echo "unexpected kubectl call: $*" >&2
exit 2
`

// stubGcloud makes every gcloud call succeed and reports the IAM binding that
// deploy.sh checks for.
const stubGcloud = `#!/bin/bash
case "$*" in
  *get-iam-policy*) echo "roles/file.editor" ;;
esac
exit 0
`

// TestDeployScriptStorageClass runs deploy.sh with stub gcloud and kubectl and
// checks how it handles an existing StorageClass.
func TestDeployScriptStorageClass(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not found")
	}
	repoRoot := filepath.Join("..", "..", "..", "..")
	read := func(rel string) string {
		t.Helper()
		b, err := os.ReadFile(filepath.Join(repoRoot, rel))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	const (
		scriptRel   = "deploy/kubernetes/overlays/substrate/deploy.sh"
		saTmplRel   = "deploy/kubernetes/overlays/substrate/serviceaccount_patch.yaml.tmpl"
		scTmplRel   = "examples/kubernetes/substrate/storageclass.yaml.tmpl"
		customGSA   = "my-gsa@proj.iam.gserviceaccount.com"
		warning     = "already exists with different settings"
		provWarning = "may belong to another driver"
	)
	rendered := strings.NewReplacer(
		"<PROJECT_ID>", "proj",
		"<LOCATION>", "us-central1",
		"<VOLUMEPOOL_NAME>", "pool",
		"<STORAGECLASS_NAME>", "sc1",
	).Replace(read(scTmplRel))
	var oldLines []string
	for _, l := range strings.Split(rendered, "\n") {
		if !strings.Contains(l, "mount-options:") {
			oldLines = append(oldLines, l)
		}
	}
	// A StorageClass created from the earlier template, without mount-options.
	oldClass := strings.Join(oldLines, "\n")
	if oldClass == rendered {
		t.Fatal("the template has no mount-options line to remove")
	}
	otherDriver := strings.Replace(rendered, "provisioner: filestore.csi.storage.gke.io", "provisioner: example.com/other", 1)
	if otherDriver == rendered {
		t.Fatal("the template has no filestore provisioner line to replace")
	}
	otherPolicy := strings.Replace(rendered, "reclaimPolicy: Delete", "reclaimPolicy: Retain", 1)
	if otherPolicy == rendered {
		t.Fatal("the template has no reclaimPolicy line to replace")
	}
	otherBinding := strings.Replace(rendered, "volumeBindingMode: Immediate", "volumeBindingMode: WaitForFirstConsumer", 1)
	if otherBinding == rendered {
		t.Fatal("the template has no volumeBindingMode line to replace")
	}

	tests := []struct {
		name        string
		state       string // stub SC_STATE
		existing    string
		args        []string
		wantErr     bool
		wantApplied bool
		wantOut     []string
		notWantOut  []string
	}{
		{name: "missing class is created", state: "missing", wantApplied: true, notWantOut: []string{warning}},
		{name: "same class is applied", state: "present", existing: rendered, wantApplied: true, notWantOut: []string{warning}},
		{
			name: "class with old parameters is left unchanged with a warning", state: "present", existing: oldClass,
			wantOut:    []string{warning, "kubectl delete storageclass sc1", "deploy.sh --project-id proj"},
			notWantOut: []string{"--service-account"},
		},
		{
			name: "custom service account is kept in the re-run hint", state: "present", existing: oldClass,
			args:    []string{"--service-account", customGSA},
			wantOut: []string{warning, "--service-account " + customGSA},
		},
		{
			name: "class with another reclaimPolicy is left unchanged with a warning", state: "present", existing: otherPolicy,
			wantOut: []string{warning, "reclaimPolicy=Retain"},
		},
		{
			name: "class with another volumeBindingMode is left unchanged with a warning", state: "present", existing: otherBinding,
			wantOut: []string{warning, "volumeBindingMode=WaitForFirstConsumer"},
		},
		{
			name: "class of another driver is left unchanged without a delete hint", state: "present", existing: otherDriver,
			wantOut:    []string{provWarning, "example.com/other"},
			notWantOut: []string{"kubectl delete storageclass", warning},
		},
		{name: "other kubectl errors stop the script", state: "error", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for rel, content := range map[string]string{scriptRel: read(scriptRel), saTmplRel: read(saTmplRel), scTmplRel: read(scTmplRel)} {
				path := filepath.Join(dir, "repo", rel)
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			bin := filepath.Join(dir, "bin")
			if err := os.MkdirAll(bin, 0o755); err != nil {
				t.Fatal(err)
			}
			for name, content := range map[string]string{"kubectl": stubKubectl, "gcloud": stubGcloud} {
				if err := os.WriteFile(filepath.Join(bin, name), []byte(content), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(filepath.Join(dir, "existing.yaml"), []byte(tc.existing), 0o644); err != nil {
				t.Fatal(err)
			}

			args := append([]string{filepath.Join(dir, "repo", scriptRel),
				"--project-id", "proj", "--volumepool-location", "us-central1", "--volumepool-name", "pool", "--storageclass-name", "sc1"}, tc.args...)
			cmd := exec.Command(bash, args...)
			cmd.Env = append(os.Environ(), "PATH="+bin+":"+os.Getenv("PATH"), "STUB_DIR="+dir, "SC_STATE="+tc.state,
				"PROJECT_ID=", "GCP_SERVICE_ACCOUNT=", "VOLUMEPOOL_LOCATION=", "VOLUMEPOOL_NAME=", "STORAGECLASS_NAME=")
			cmd.Stdin = nil
			outBytes, err := cmd.CombinedOutput()
			out := string(outBytes)
			if (err != nil) != tc.wantErr {
				t.Fatalf("deploy.sh error = %v, want error %t; output:\n%s", err, tc.wantErr, out)
			}
			applied, readErr := os.ReadFile(filepath.Join(dir, "applied.yaml"))
			if gotApplied := readErr == nil; gotApplied != tc.wantApplied {
				t.Errorf("StorageClass applied = %t, want %t; output:\n%s", gotApplied, tc.wantApplied, out)
			}
			if tc.wantApplied && strings.TrimSpace(string(applied)) != strings.TrimSpace(rendered) {
				t.Errorf("applied manifest differs from the rendered template:\n%s", applied)
			}
			for _, want := range tc.wantOut {
				if !strings.Contains(out, want) {
					t.Errorf("output doesn't contain %q; output:\n%s", want, out)
				}
			}
			for _, notWant := range tc.notWantOut {
				if strings.Contains(out, notWant) {
					t.Errorf("output contains %q; output:\n%s", notWant, out)
				}
			}
		})
	}
}
