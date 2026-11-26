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
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"text/template"

	"sigs.k8s.io/yaml"
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

	const rerunHint = "./deploy/kubernetes/overlays/substrate/deploy.sh --project-id proj --volumepool-location us-central1 --volumepool-name pool --storageclass-name sc1"

	tests := []struct {
		name        string
		state       string // stub SC_STATE
		existing    string
		args        []string
		env         []string
		wantLine    string // a whole output line that must be present
		wantRC      int    // deploy.sh exit code
		wantApplied bool
		wantOut     []string
		notWantOut  []string
	}{
		{name: "missing class is created", state: "missing", wantApplied: true, notWantOut: []string{warning}},
		{name: "same class is applied", state: "present", existing: rendered, wantApplied: true, notWantOut: []string{warning}},
		{
			name: "class with old parameters is left unchanged with a warning", state: "present", existing: oldClass,
			wantOut:    []string{warning, "kubectl delete storageclass sc1"},
			wantLine:   rerunHint,
			notWantOut: []string{"--service-account"},
		},
		{
			name: "custom service account is kept in the re-run hint", state: "present", existing: oldClass,
			args:     []string{"--service-account", customGSA},
			wantOut:  []string{warning},
			wantLine: rerunHint + " --service-account " + customGSA,
		},
		{
			name: "custom service account from the environment is kept in the re-run hint", state: "present", existing: oldClass,
			env:      []string{"GCP_SERVICE_ACCOUNT=" + customGSA},
			wantOut:  []string{warning},
			wantLine: rerunHint + " --service-account " + customGSA,
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
			name: "class of another driver is left unchanged, without a delete hint, and fails", state: "present", existing: otherDriver,
			wantRC:     1,
			wantOut:    []string{provWarning, "example.com/other"},
			notWantOut: []string{"kubectl delete storageclass", warning},
		},
		{name: "other kubectl errors stop the script", state: "error", wantRC: 1},
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
			// Later entries win, so a case can set any of these.
			cmd.Env = append(cmd.Env, tc.env...)
			cmd.Stdin = nil
			outBytes, err := cmd.CombinedOutput()
			out := string(outBytes)
			var exitErr *exec.ExitError
			if err != nil && !errors.As(err, &exitErr) {
				t.Fatalf("deploy.sh didn't run: %v", err)
			}
			if got := cmd.ProcessState.ExitCode(); got != tc.wantRC {
				t.Fatalf("deploy.sh exit code = %d, want %d; output:\n%s", got, tc.wantRC, out)
			}
			// The driver is deployed before the StorageClass is checked.
			if calls, err := os.ReadFile(filepath.Join(dir, "calls")); err != nil || !strings.Contains(string(calls), "kubectl apply -k ") {
				t.Errorf("deploy.sh didn't apply the overlay (kubectl apply -k); calls:\n%s", calls)
			}
			applied, readErr := os.ReadFile(filepath.Join(dir, "applied.yaml"))
			if gotApplied := readErr == nil; gotApplied != tc.wantApplied {
				t.Errorf("StorageClass applied = %t, want %t; output:\n%s", gotApplied, tc.wantApplied, out)
			}
			if tc.wantApplied && strings.TrimSpace(string(applied)) != strings.TrimSpace(rendered) {
				t.Errorf("applied manifest differs from the rendered template:\n%s", applied)
			}
			if tc.wantLine != "" {
				found := false
				for _, line := range strings.Split(out, "\n") {
					if strings.TrimSpace(line) == tc.wantLine {
						found = true
					}
				}
				if !found {
					t.Errorf("output has no line %q; output:\n%s", tc.wantLine, out)
				}
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

// TestDeployScriptFieldsTemplate executes deploy.sh's SC_FIELDS_TEMPLATE, the
// go-template it passes to kubectl, with text/template (which kubectl's
// go-template output uses) on the rendered StorageClass. It checks that the
// template is valid and prints the fields deploy.sh compares, and that the
// kubectl stub used by TestDeployScriptStorageClass prints the same lines, so
// a change to the template can't pass that test unnoticed.
func TestDeployScriptFieldsTemplate(t *testing.T) {
	repoRoot := filepath.Join("..", "..", "..", "..")
	script, err := os.ReadFile(filepath.Join(repoRoot, "deploy/kubernetes/overlays/substrate/deploy.sh"))
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?m)^\s*SC_FIELDS_TEMPLATE='([^']*)'$`).FindSubmatch(script)
	if m == nil {
		t.Fatal("SC_FIELDS_TEMPLATE not found in deploy.sh")
	}
	tmplText := string(m[1])
	tmpl, err := template.New("fields").Parse(tmplText)
	if err != nil {
		t.Fatalf("SC_FIELDS_TEMPLATE doesn't parse: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(repoRoot, "examples/kubernetes/substrate/storageclass.yaml.tmpl"))
	if err != nil {
		t.Fatal(err)
	}
	rendered := strings.NewReplacer("<PROJECT_ID>", "proj", "<LOCATION>", "us-central1", "<VOLUMEPOOL_NAME>", "pool", "<STORAGECLASS_NAME>", "sc1").Replace(string(raw))
	var obj map[string]interface{}
	if err := yaml.Unmarshal([]byte(rendered), &obj); err != nil {
		t.Fatal(err)
	}
	var got bytes.Buffer
	if err := tmpl.Option("missingkey=error").Execute(&got, obj); err != nil {
		t.Fatalf("SC_FIELDS_TEMPLATE doesn't execute on the StorageClass: %v", err)
	}
	want := "provisioner=filestore.csi.storage.gke.io\n" +
		"reclaimPolicy=Delete\n" +
		"volumeBindingMode=Immediate\n" +
		"parameters.mount-options=vers=3,proto=tcp,mountproto=tcp,port=2049,mountport=2050,nolock,nconnect=8,rsize=524288,wsize=524288,actimeo=60\n" +
		"parameters.volume-pool=projects/proj/locations/us-central1/volumePools/pool\n"
	if got.String() != want {
		t.Errorf("SC_FIELDS_TEMPLATE output:\n%s\nwant:\n%s", got.String(), want)
	}

	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not found")
	}
	dir := t.TempDir()
	stub := filepath.Join(dir, "kubectl")
	if err := os.WriteFile(stub, []byte(stubKubectl), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bash, stub, "patch", "--local", "-f", "-", "--type=merge", "-p", "{}", "-o", "go-template="+tmplText)
	cmd.Env = append(os.Environ(), "STUB_DIR="+dir)
	cmd.Stdin = strings.NewReader(rendered)
	stubOut, err := cmd.Output()
	if err != nil {
		t.Fatalf("stub kubectl failed: %v", err)
	}
	if string(stubOut) != got.String() {
		t.Errorf("stub kubectl prints:\n%s\nbut the template prints:\n%s", stubOut, got.String())
	}
}
