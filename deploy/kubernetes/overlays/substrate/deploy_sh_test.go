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
	"fmt"
	"io"
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
// every invocation, validates expected flags and the StorageClass name, answers
// StorageClass queries from $STUB_DIR/existing.yaml according to $SC_STATE
// (missing, present or error), and delegates -o go-template rendering to
// TestDeployScriptKubectlHelper (real sigs.k8s.io/yaml + text/template).
const stubKubectl = `#!/usr/bin/env bash
set -euo pipefail
export LC_ALL=C

echo "kubectl $*" >> "$STUB_DIR/calls"

run_go_template() {
  local tmpl="$1"
  GO_WANT_HELPER_PROCESS=1 "$STUB_TEST_BIN" -test.run='^TestDeployScriptKubectlHelper$' -- "$tmpl"
}

case "${1:-} ${2:-}" in
  "apply -k")
    [[ $# -eq 3 && -d "$3" ]] || { echo "unexpected kubectl apply -k args: $*" >&2; exit 2; }
    exit 0
    ;;
  "apply -f")
    [[ $# -eq 3 && "$3" == "-" ]] || { echo "unexpected kubectl apply -f args: $*" >&2; exit 2; }
    cat > "$STUB_DIR/applied.yaml"
    exit 0
    ;;
  "patch --local")
    shift 2
    has_file=0
    has_type=0
    has_patch=0
    tmpl=""
    while [[ $# -gt 0 ]]; do
      case "$1" in
        -f)
          [[ $# -ge 2 && "$2" == "-" ]] || { echo "unexpected -f arg: $*" >&2; exit 2; }
          has_file=1
          shift 2
          ;;
        --type=merge)
          has_type=1
          shift
          ;;
        --type)
          [[ $# -ge 2 && "$2" == "merge" ]] || { echo "unexpected --type arg: $*" >&2; exit 2; }
          has_type=1
          shift 2
          ;;
        -p)
          [[ $# -ge 2 && "$2" == "{}" ]] || { echo "unexpected -p arg: $*" >&2; exit 2; }
          has_patch=1
          shift 2
          ;;
        -p={})
          has_patch=1
          shift
          ;;
        -o)
          [[ $# -ge 2 && "$2" == go-template=* ]] || { echo "unexpected -o arg: $*" >&2; exit 2; }
          tmpl="${2#go-template=}"
          shift 2
          ;;
        -o=go-template=*)
          tmpl="${1#-o=go-template=}"
          shift
          ;;
        *)
          echo "unexpected kubectl patch arg: $1" >&2
          exit 2
          ;;
      esac
    done
    [[ $has_file -eq 1 && $has_type -eq 1 && $has_patch -eq 1 && -n "$tmpl" ]] || {
      echo "missing required kubectl patch flags" >&2
      exit 2
    }
    if [[ "${PATCH_ERROR:-}" == "1" ]]; then
      echo "error: patch --local failed" >&2
      exit 1
    fi
    run_go_template "$tmpl"
    exit 0
    ;;
  "get storageclass")
    shift 2
    ignore_not_found=0
    saw_dashdash=0
    tmpl=""
    sc_name=""
    while [[ $# -gt 0 ]]; do
      case "$1" in
        --ignore-not-found)
          ignore_not_found=1
          shift
          ;;
        -o)
          [[ $# -ge 2 && "$2" == go-template=* ]] || { echo "unexpected -o arg: $*" >&2; exit 2; }
          tmpl="${2#go-template=}"
          shift 2
          ;;
        -o=go-template=*)
          tmpl="${1#-o=go-template=}"
          shift
          ;;
        --)
          saw_dashdash=1
          shift
          [[ $# -eq 1 ]] || { echo "expected single storageclass name after --, got: $*" >&2; exit 2; }
          sc_name="$1"
          shift
          ;;
        *)
          echo "unexpected kubectl get arg: $1" >&2
          exit 2
          ;;
      esac
    done
    [[ $saw_dashdash -eq 1 && -n "$sc_name" && -n "$tmpl" ]] || {
      echo "missing --, storageclass name, or go-template in kubectl get" >&2
      exit 2
    }
    if [[ -n "${WANT_SC_NAME:-}" && "$sc_name" != "$WANT_SC_NAME" ]]; then
      echo "kubectl get storageclass got name '$sc_name', want '$WANT_SC_NAME'" >&2
      exit 2
    fi
    case "${SC_STATE:-}" in
      error)
        echo "error: You must be logged in to the server (Unauthorized)" >&2
        exit 1
        ;;
      missing)
        if [[ $ignore_not_found -ne 1 ]]; then
          echo "Error from server (NotFound): storageclasses.storage.k8s.io \"$sc_name\" not found" >&2
          exit 1
        fi
        exit 0
        ;;
      present)
        run_go_template "$tmpl" < "$STUB_DIR/existing.yaml"
        exit 0
        ;;
      *)
        echo "unexpected SC_STATE: ${SC_STATE:-}" >&2
        exit 2
        ;;
    esac
    ;;
esac
echo "unexpected kubectl call: $*" >&2
exit 2
`

// stubGcloud makes every gcloud call succeed and reports the IAM binding that
// deploy.sh checks for.
const stubGcloud = `#!/usr/bin/env bash
case "$*" in
  *get-iam-policy*) echo "roles/file.editor" ;;
esac
exit 0
`

// TestDeployScriptKubectlHelper runs as a subprocess of stubKubectl (when
// GO_WANT_HELPER_PROCESS=1) to execute -o go-template against YAML on stdin
// with Kubernetes's YAML unmarshaling and missingkey=default semantics.
func TestDeployScriptKubectlHelper(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	idx := -1
	for i, arg := range os.Args {
		if arg == "--" {
			idx = i
			break
		}
	}
	if idx == -1 || idx+1 >= len(os.Args) {
		fmt.Fprintln(os.Stderr, "kubectl helper: expected template argument after --")
		os.Exit(2)
	}
	tmplText := os.Args[idx+1]
	in, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "kubectl helper: read stdin: %v\n", err)
		os.Exit(1)
	}
	var obj map[string]interface{}
	if err := yaml.Unmarshal(in, &obj); err != nil {
		fmt.Fprintf(os.Stderr, "kubectl helper: parse yaml: %v\n", err)
		os.Exit(1)
	}
	tmpl, err := template.New("kubectl").Option("missingkey=default").Parse(tmplText)
	if err != nil {
		fmt.Fprintf(os.Stderr, "kubectl helper: parse template: %v\n", err)
		os.Exit(1)
	}
	if err := tmpl.Execute(os.Stdout, obj); err != nil {
		fmt.Fprintf(os.Stderr, "kubectl helper: execute template: %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

func testBinaryPath(t *testing.T) string {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable(): %v", err)
	}
	return exe
}

func renderSubstrateSCTemplate(rawTmpl, project, location, pool, scName string) string {
	return strings.NewReplacer(
		"<PROJECT_ID>", project,
		"<LOCATION>", location,
		"<VOLUMEPOOL_NAME>", pool,
		"<STORAGECLASS_NAME>", scName,
	).Replace(rawTmpl)
}

// TestDeployScriptStorageClass runs deploy.sh with stub gcloud and kubectl and
// checks how it handles StorageClass creation, comparison, errors and hints.
func TestDeployScriptStorageClass(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not found")
	}
	testBin := testBinaryPath(t)
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
		scriptRel      = "deploy/kubernetes/overlays/substrate/deploy.sh"
		saTmplRel      = "deploy/kubernetes/overlays/substrate/serviceaccount_patch.yaml.tmpl"
		scTmplRel      = "examples/kubernetes/substrate/storageclass.yaml.tmpl"
		customGSA      = "my-gsa@proj.iam.gserviceaccount.com"
		warning        = "already exists with different settings"
		provWarning    = "may belong to another driver"
		completeBanner = "✅ Deployment complete!"
		defaultSCName  = "substrate-volumepool-sc"
		redundantNote  = "Use the same deploy.sh arguments"
	)
	scTmplRaw := read(scTmplRel)
	rendered := renderSubstrateSCTemplate(scTmplRaw, "proj", "us-central1", "pool", "sc1")

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
	otherPool := strings.Replace(rendered, "/volumePools/pool", "/volumePools/other-pool", 1)
	if otherPool == rendered {
		t.Fatal("the template has no volumePools/pool to replace")
	}
	otherMountOpts := strings.Replace(rendered, "nolock,nconnect=8", "nolock,nconnect=4", 1)
	if otherMountOpts == rendered {
		t.Fatal("the template has no nolock,nconnect=8 to replace")
	}
	// Insert extra-key under parameters so it belongs to parameters.
	extraParam := strings.Replace(rendered, "  volume-pool:", "  extra-key: \"extra-val\"\n  volume-pool:", 1)
	if extraParam == rendered {
		t.Fatal("the template has no volume-pool line to insert extra parameter")
	}
	otherPolicy := strings.Replace(rendered, "reclaimPolicy: Delete", "reclaimPolicy: Retain", 1)
	if otherPolicy == rendered {
		t.Fatal("the template has no reclaimPolicy line to replace")
	}
	otherBinding := strings.Replace(rendered, "volumeBindingMode: Immediate", "volumeBindingMode: WaitForFirstConsumer", 1)
	if otherBinding == rendered {
		t.Fatal("the template has no volumeBindingMode line to replace")
	}
	otherExpansion := strings.Replace(rendered, "allowVolumeExpansion: false", "allowVolumeExpansion: true", 1)
	if otherExpansion == rendered {
		t.Fatal("the template has no allowVolumeExpansion line to replace")
	}
	withMountOptions := rendered + "\nmountOptions:\n  - vers=3\n  - nolock\n"
	withAllowedTopologies := rendered + "\nallowedTopologies:\n  - matchLabelExpressions:\n      - key: topology.gke.io/zone\n        values:\n          - us-central1-a\n"
	sameProvNoParams := `apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: sc1
provisioner: filestore.csi.storage.gke.io
allowVolumeExpansion: false
reclaimPolicy: Delete
volumeBindingMode: Immediate
`
	otherDriverNoParams := `apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: sc1
provisioner: kubernetes.io/no-provisioner
volumeBindingMode: WaitForFirstConsumer
`
	multilineSpoofParam := strings.Replace(
		rendered,
		"  volume-pool:",
		"  note: |\n    line1\n    provisioner=example.com/other\n  volume-pool:",
		1,
	)

	tests := []struct {
		name              string
		state             string // stub SC_STATE
		existing          string
		args              []string
		noDefaultFlags    bool
		runFromOverlayDir bool
		repoSubdir        string // defaults to "repo"
		wantSCName        string // defaults to "sc1"
		wantRendered      string // defaults to rendered
		env               []string
		wantLine          string // trimmed output line that must be present (SCRIPT_PATH placeholder expanded)
		wantRC            int    // deploy.sh exit code
		wantApplied       bool
		wantReplaceFile   bool
		wantOut           []string
		notWantOut        []string
		wantCalls         []string
		notWantCalls      []string
	}{
		{
			name:        "missing class is created",
			state:       "missing",
			wantApplied: true,
			notWantOut:  []string{warning},
		},
		{
			name:           "default storageclass name is used when omitted in non-TTY",
			state:          "missing",
			noDefaultFlags: true,
			args:           []string{"--project-id", "proj", "--volumepool-location", "us-central1", "--volumepool-name", "pool"},
			wantSCName:     defaultSCName,
			wantRendered:   renderSubstrateSCTemplate(scTmplRaw, "proj", "us-central1", "pool", defaultSCName),
			wantApplied:    true,
			notWantOut:     []string{warning},
		},
		{
			name:           "env-var inputs configure project, location, volumepool and storageclass",
			state:          "missing",
			noDefaultFlags: true,
			env: []string{
				"PROJECT_ID=env-proj",
				"VOLUMEPOOL_LOCATION=europe-west1",
				"VOLUMEPOOL_NAME=env-pool",
				"STORAGECLASS_NAME=env-sc",
				"GCP_SERVICE_ACCOUNT=env-sa@env-proj.iam.gserviceaccount.com",
			},
			wantSCName:   "env-sc",
			wantRendered: renderSubstrateSCTemplate(scTmplRaw, "env-proj", "europe-west1", "env-pool", "env-sc"),
			wantApplied:  true,
			notWantOut:   []string{warning},
		},
		{
			name:           "step 5 is skipped when volumepool location and name are omitted",
			noDefaultFlags: true,
			args:           []string{"--project-id", "proj"},
			wantApplied:    false,
			notWantCalls:   []string{"kubectl get storageclass", "kubectl apply -f"},
		},
		{
			name:           "step 5 is skipped when only volumepool location is provided",
			noDefaultFlags: true,
			args:           []string{"--project-id", "proj", "--volumepool-location", "us-central1"},
			wantApplied:    false,
			notWantCalls:   []string{"kubectl get storageclass", "kubectl apply -f"},
		},
		{
			name:        "same class is applied",
			state:       "present",
			existing:    rendered,
			wantApplied: true,
			notWantOut:  []string{warning},
		},
		{
			name:            "class with old parameters is left unchanged with a warning",
			state:           "present",
			existing:        oldClass,
			wantReplaceFile: true,
			wantOut:         []string{warning, "kubectl replace --force -f ", "kubectl delete storageclass sc1", "--storageclass-name", "when they're next staged on a node"},
			wantLine:        "<SCRIPT_PATH> --project-id proj --volumepool-location us-central1 --volumepool-name pool --storageclass-name sc1",
			notWantOut:      []string{"--service-account", redundantNote},
		},
		{
			name:              "re-run hint uses invoked ./deploy.sh path from overlay dir and quotes spaces",
			state:             "present",
			existing:          renderSubstrateSCTemplate(oldClass, "proj", "us-central1", "my pool", "sc1"),
			noDefaultFlags:    true,
			runFromOverlayDir: true,
			args:              []string{"--project-id", "proj", "--volumepool-location", "us-central1", "--volumepool-name", "my pool", "--storageclass-name", "sc1"},
			wantRendered:      renderSubstrateSCTemplate(scTmplRaw, "proj", "us-central1", "my pool", "sc1"),
			wantReplaceFile:   true,
			wantOut:           []string{warning, "kubectl replace --force -f ", "kubectl delete storageclass sc1"},
			wantLine:          "./deploy.sh --project-id proj --volumepool-location us-central1 --volumepool-name my\\ pool --storageclass-name sc1",
		},
		{
			name:            "re-run hint shell-quotes script path with spaces",
			state:           "present",
			existing:        oldClass,
			repoSubdir:      "repo with spaces",
			wantReplaceFile: true,
			wantOut:         []string{warning, "kubectl replace --force -f "},
			wantLine:        "<SCRIPT_PATH_QUOTED> --project-id proj --volumepool-location us-central1 --volumepool-name pool --storageclass-name sc1",
		},
		{
			name:            "custom service account is kept in the re-run hint",
			state:           "present",
			existing:        oldClass,
			args:            []string{"--service-account", customGSA},
			wantReplaceFile: true,
			wantOut:         []string{warning},
			wantLine:        "<SCRIPT_PATH> --project-id proj --volumepool-location us-central1 --volumepool-name pool --storageclass-name sc1 --service-account " + customGSA,
		},
		{
			name:            "custom service account from the environment is kept in the re-run hint",
			state:           "present",
			existing:        oldClass,
			env:             []string{"GCP_SERVICE_ACCOUNT=" + customGSA},
			wantReplaceFile: true,
			wantOut:         []string{warning},
			wantLine:        "<SCRIPT_PATH> --project-id proj --volumepool-location us-central1 --volumepool-name pool --storageclass-name sc1 --service-account " + customGSA,
		},
		{
			name:     "class with different volume-pool value is left unchanged with a warning",
			state:    "present",
			existing: otherPool,
			wantOut:  []string{warning, "parameters.volume-pool=projects/proj/locations/us-central1/volumePools/other-pool"},
		},
		{
			name:     "class with different mount-options value is left unchanged with a warning",
			state:    "present",
			existing: otherMountOpts,
			wantOut:  []string{warning, "nconnect=4"},
		},
		{
			name:     "class with extra parameter key is left unchanged with a warning",
			state:    "present",
			existing: extraParam,
			wantOut:  []string{warning, "parameters.extra-key=extra-val"},
		},
		{
			name:     "class with another reclaimPolicy is left unchanged with a warning",
			state:    "present",
			existing: otherPolicy,
			wantOut:  []string{warning, "reclaimPolicy=Retain"},
		},
		{
			name:     "class with another volumeBindingMode is left unchanged with a warning",
			state:    "present",
			existing: otherBinding,
			wantOut:  []string{warning, "volumeBindingMode=WaitForFirstConsumer"},
		},
		{
			name:     "class with allowVolumeExpansion true is left unchanged with a warning",
			state:    "present",
			existing: otherExpansion,
			wantOut:  []string{warning, "allowVolumeExpansion=true"},
		},
		{
			name:     "class with mountOptions is left unchanged with a warning",
			state:    "present",
			existing: withMountOptions,
			wantOut:  []string{warning, "mountOptions=vers=3", "mountOptions=nolock"},
		},
		{
			name:     "class with allowedTopologies is left unchanged with a warning",
			state:    "present",
			existing: withAllowedTopologies,
			wantOut:  []string{warning, "allowedTopologies="},
		},
		{
			name:       "class with no parameters is left unchanged with a warning",
			state:      "present",
			existing:   sameProvNoParams,
			wantOut:    []string{warning},
			notWantOut: []string{"<no value>"},
		},
		{
			name:       "class with no parameters belonging to another driver fails",
			state:      "present",
			existing:   otherDriverNoParams,
			wantRC:     1,
			wantOut:    []string{provWarning, "kubernetes.io/no-provisioner"},
			notWantOut: []string{"kubectl delete storageclass", "kubectl replace", warning},
		},
		{
			name:       "class of another driver is left unchanged, without a delete hint, and fails",
			state:      "present",
			existing:   otherDriver,
			wantRC:     1,
			wantOut:    []string{provWarning, "example.com/other"},
			notWantOut: []string{"kubectl delete storageclass", "kubectl replace", warning},
		},
		{
			name:       "multiline parameter containing provisioner= does not spoof provisioner",
			state:      "present",
			existing:   multilineSpoofParam,
			wantOut:    []string{warning, "parameters.note="},
			notWantOut: []string{provWarning},
		},
		{
			name:   "other kubectl get errors stop the script",
			state:  "error",
			wantRC: 1,
		},
		{
			name:     "kubectl patch --local failure stops the script",
			state:    "present",
			existing: rendered,
			env:      []string{"PATCH_ERROR=1"},
			wantRC:   1,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			repoSubdir := tc.repoSubdir
			if repoSubdir == "" {
				repoSubdir = "repo"
			}
			for rel, content := range map[string]string{scriptRel: read(scriptRel), saTmplRel: read(saTmplRel), scTmplRel: scTmplRaw} {
				path := filepath.Join(dir, repoSubdir, rel)
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

			scriptPath := filepath.Join(dir, repoSubdir, scriptRel)
			invokePath := scriptPath
			if tc.runFromOverlayDir {
				invokePath = "./deploy.sh"
			}
			var args []string
			if tc.noDefaultFlags {
				args = append([]string{invokePath}, tc.args...)
			} else {
				args = append([]string{invokePath,
					"--project-id", "proj", "--volumepool-location", "us-central1", "--volumepool-name", "pool", "--storageclass-name", "sc1"}, tc.args...)
			}
			wantSCName := tc.wantSCName
			if wantSCName == "" {
				wantSCName = "sc1"
			}
			wantRendered := tc.wantRendered
			if wantRendered == "" {
				wantRendered = rendered
			}

			cmd := exec.Command(bash, args...)
			if tc.runFromOverlayDir {
				cmd.Dir = filepath.Dir(scriptPath)
			}
			cmd.Env = append(os.Environ(),
				"PATH="+bin+":"+os.Getenv("PATH"),
				"TMPDIR="+dir,
				"STUB_DIR="+dir,
				"STUB_TEST_BIN="+testBin,
				"SC_STATE="+tc.state,
				"WANT_SC_NAME="+wantSCName,
				"PROJECT_ID=", "GCP_SERVICE_ACCOUNT=", "VOLUMEPOOL_LOCATION=", "VOLUMEPOOL_NAME=", "STORAGECLASS_NAME=",
			)
			// Later entries win, so a case can set any of these.
			cmd.Env = append(cmd.Env, tc.env...)
			cmd.Stdin = nil
			var stdoutBuf, stderrBuf bytes.Buffer
			cmd.Stdout = &stdoutBuf
			cmd.Stderr = &stderrBuf
			err := cmd.Run()
			stdout := stdoutBuf.String()
			stderr := stderrBuf.String()
			out := stdout + stderr
			var exitErr *exec.ExitError
			if err != nil && !errors.As(err, &exitErr) {
				t.Fatalf("deploy.sh didn't run: %v", err)
			}
			if got := cmd.ProcessState.ExitCode(); got != tc.wantRC {
				t.Fatalf("deploy.sh exit code = %d, want %d; stdout:\n%s\nstderr:\n%s", got, tc.wantRC, stdout, stderr)
			}
			if tc.wantRC == 0 {
				if !strings.Contains(stdout, completeBanner) {
					t.Errorf("exit 0 run missing completion banner on stdout; stdout:\n%s", stdout)
				}
			} else {
				if strings.Contains(stdout, completeBanner) {
					t.Errorf("failed run printed completion banner on stdout; stdout:\n%s", stdout)
				}
			}
			// The driver is deployed before the StorageClass is checked.
			callsBytes, callsErr := os.ReadFile(filepath.Join(dir, "calls"))
			calls := string(callsBytes)
			if callsErr != nil || !strings.Contains(calls, "kubectl apply -k ") {
				t.Errorf("deploy.sh didn't apply the overlay (kubectl apply -k); calls:\n%s", calls)
			}
			for _, wantCall := range tc.wantCalls {
				if !strings.Contains(calls, wantCall) {
					t.Errorf("calls don't contain %q; calls:\n%s", wantCall, calls)
				}
			}
			for _, notWantCall := range tc.notWantCalls {
				if strings.Contains(calls, notWantCall) {
					t.Errorf("calls unexpectedly contain %q; calls:\n%s", notWantCall, calls)
				}
			}
			applied, readErr := os.ReadFile(filepath.Join(dir, "applied.yaml"))
			if gotApplied := readErr == nil; gotApplied != tc.wantApplied {
				t.Errorf("StorageClass applied = %t, want %t; output:\n%s", gotApplied, tc.wantApplied, out)
			}
			if tc.wantApplied && strings.TrimSpace(string(applied)) != strings.TrimSpace(wantRendered) {
				t.Errorf("applied manifest differs from the rendered template:\n%s", applied)
			}
			if tc.wantReplaceFile {
				matches, globErr := filepath.Glob(filepath.Join(dir, "substrate-storageclass.*"))
				if globErr != nil || len(matches) != 1 {
					t.Fatalf("expected 1 rendered temp file in %s, got %v (err=%v); stderr:\n%s", dir, matches, globErr, stderr)
				}
				tempContent, tempErr := os.ReadFile(matches[0])
				if tempErr != nil {
					t.Fatalf("reading temp manifest %s: %v", matches[0], tempErr)
				}
				if strings.TrimSpace(string(tempContent)) != strings.TrimSpace(wantRendered) {
					t.Errorf("temp manifest differs from rendered template:\n%s", tempContent)
				}
				quotedPath := strings.ReplaceAll(matches[0], " ", `\ `)
				if !strings.Contains(stderr, "kubectl replace --force -f "+quotedPath) {
					t.Errorf("stderr missing kubectl replace --force -f %s; stderr:\n%s", quotedPath, stderr)
				}
			}
			if tc.wantLine != "" {
				wantLine := strings.NewReplacer(
					"<SCRIPT_PATH>", invokePath,
					"<SCRIPT_PATH_QUOTED>", strings.ReplaceAll(invokePath, " ", `\ `),
				).Replace(tc.wantLine)
				found := false
				for _, line := range strings.Split(out, "\n") {
					if strings.TrimSpace(line) == wantLine {
						found = true
					}
				}
				if !found {
					t.Errorf("output has no line %q; output:\n%s", wantLine, out)
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
// go-template output uses with missingkey=default) across the rendered
// StorageClass, a class without parameters, and a class with optional fields.
// It also checks that stubKubectl produces the same output for every fixture.
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
	tmpl, err := template.New("fields").Option("missingkey=default").Parse(tmplText)
	if err != nil {
		t.Fatalf("SC_FIELDS_TEMPLATE doesn't parse: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(repoRoot, "examples/kubernetes/substrate/storageclass.yaml.tmpl"))
	if err != nil {
		t.Fatal(err)
	}
	rendered := renderSubstrateSCTemplate(string(raw), "proj", "us-central1", "pool", "sc1")

	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not found")
	}
	testBin := testBinaryPath(t)

	fixtures := []struct {
		name string
		yaml string
		want string
	}{
		{
			name: "rendered template",
			yaml: rendered,
			want: "provisioner=filestore.csi.storage.gke.io\n" +
				"reclaimPolicy=Delete\n" +
				"volumeBindingMode=Immediate\n" +
				"allowVolumeExpansion=false\n" +
				"parameters.mount-options=vers=3,proto=tcp,mountproto=tcp,port=2049,mountport=2050,nolock,nconnect=8,rsize=524288,wsize=524288,actimeo=60\n" +
				"parameters.volume-pool=projects/proj/locations/us-central1/volumePools/pool\n",
		},
		{
			name: "other driver class without parameters",
			yaml: `apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: sc1
provisioner: kubernetes.io/no-provisioner
volumeBindingMode: WaitForFirstConsumer
`,
			want: "provisioner=kubernetes.io/no-provisioner\n" +
				"reclaimPolicy=<no value>\n" +
				"volumeBindingMode=WaitForFirstConsumer\n" +
				"allowVolumeExpansion=<no value>\n",
		},
		{
			name: "class with allowVolumeExpansion, mountOptions and allowedTopologies",
			yaml: rendered + "\nmountOptions:\n  - vers=3\n  - nolock\nallowedTopologies:\n  - matchLabelExpressions:\n      - key: topology.gke.io/zone\n        values:\n          - us-central1-a\n",
			want: "provisioner=filestore.csi.storage.gke.io\n" +
				"reclaimPolicy=Delete\n" +
				"volumeBindingMode=Immediate\n" +
				"allowVolumeExpansion=false\n" +
				"mountOptions=vers=3\n" +
				"mountOptions=nolock\n" +
				"allowedTopologies=map[matchLabelExpressions:[map[key:topology.gke.io/zone values:[us-central1-a]]]]\n" +
				"parameters.mount-options=vers=3,proto=tcp,mountproto=tcp,port=2049,mountport=2050,nolock,nconnect=8,rsize=524288,wsize=524288,actimeo=60\n" +
				"parameters.volume-pool=projects/proj/locations/us-central1/volumePools/pool\n",
		},
	}

	for _, fx := range fixtures {
		t.Run(fx.name, func(t *testing.T) {
			var obj map[string]interface{}
			if err := yaml.Unmarshal([]byte(fx.yaml), &obj); err != nil {
				t.Fatal(err)
			}
			var got bytes.Buffer
			if err := tmpl.Execute(&got, obj); err != nil {
				t.Fatalf("SC_FIELDS_TEMPLATE doesn't execute: %v", err)
			}
			if fx.want != "" && got.String() != fx.want {
				t.Errorf("SC_FIELDS_TEMPLATE output:\n%s\nwant:\n%s", got.String(), fx.want)
			}

			dir := t.TempDir()
			stub := filepath.Join(dir, "kubectl")
			if err := os.WriteFile(stub, []byte(stubKubectl), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "existing.yaml"), []byte(fx.yaml), 0o644); err != nil {
				t.Fatal(err)
			}

			patchCmd := exec.Command(bash, stub, "patch", "--local", "-f", "-", "--type=merge", "-p", "{}", "-o", "go-template="+tmplText)
			patchCmd.Env = append(os.Environ(), "STUB_DIR="+dir, "STUB_TEST_BIN="+testBin)
			patchCmd.Stdin = strings.NewReader(fx.yaml)
			patchOut, err := patchCmd.Output()
			if err != nil {
				t.Fatalf("stub kubectl patch failed: %v", err)
			}
			if string(patchOut) != got.String() {
				t.Errorf("stub kubectl patch prints:\n%s\nbut template prints:\n%s", patchOut, got.String())
			}

			getCmd := exec.Command(bash, stub, "get", "storageclass", "--ignore-not-found", "-o", "go-template="+tmplText, "--", "sc1")
			getCmd.Env = append(os.Environ(), "STUB_DIR="+dir, "STUB_TEST_BIN="+testBin, "SC_STATE=present", "WANT_SC_NAME=sc1")
			getOut, err := getCmd.Output()
			if err != nil {
				t.Fatalf("stub kubectl get failed: %v", err)
			}
			if string(getOut) != got.String() {
				t.Errorf("stub kubectl get prints:\n%s\nbut template prints:\n%s", getOut, got.String())
			}
		})
	}
}
