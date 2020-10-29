package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	apimachineryversion "k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
)

func gkeLocationArgs(gceZone, gceRegion string) (locationArg, locationVal string, err error) {
	switch {
	case len(gceZone) > 0:
		locationArg = "--zone"
		locationVal = gceZone
	case len(gceRegion) > 0:
		locationArg = "--region"
		locationVal = gceRegion
	default:
		return "", "", fmt.Errorf("zone and region unspecified")
	}
	return
}

func buildKubernetes(k8sDir, command string) error {
	cmd := exec.Command("make", "-C", k8sDir, command)
	err := runCommand("Building Kubernetes", cmd)
	if err != nil {
		return fmt.Errorf("failed to build Kubernetes: %v", err)
	}
	return nil
}

func clusterUpGCE(k8sDir, gceZone string, numNodes int, imageType string) error {
	kshPath := filepath.Join(k8sDir, "cluster", "kubectl.sh")
	_, err := os.Stat(kshPath)
	if err == nil {
		// Set kubectl to the one bundled in the k8s tar for versioning
		err = os.Setenv("GCE_PD_KUBECTL", kshPath)
		if err != nil {
			return fmt.Errorf("failed to set cluster specific kubectl: %v", err)
		}
	} else {
		klog.Errorf("could not find cluster kubectl at %s, falling back to default kubectl", kshPath)
	}

	if len(*kubeFeatureGates) != 0 {
		err = os.Setenv("KUBE_FEATURE_GATES", *kubeFeatureGates)
		if err != nil {
			return fmt.Errorf("failed to set kubernetes feature gates: %v", err)
		}
		klog.V(4).Infof("Set Kubernetes feature gates: %v", *kubeFeatureGates)
	}

	err = setImageTypeEnvs(imageType)
	if err != nil {
		return fmt.Errorf("failed to set image type environment variables: %v", err)
	}

	err = os.Setenv("NUM_NODES", strconv.Itoa(numNodes))
	if err != nil {
		return err
	}

	err = os.Setenv("KUBE_GCE_ZONE", gceZone)
	if err != nil {
		return err
	}

	if *deployOverlayName == "dev" && *devOverlaySA != "" {
		nodeScope := "https://www.googleapis.com/auth/cloud-platform"
		klog.Infof("For dev overlay setting KUBE_GCE_NODE_SERVICE_ACCOUNT=%s, NODE_SCOPES=%s", *devOverlaySA, nodeScope)
		if err = os.Setenv("KUBE_GCE_NODE_SERVICE_ACCOUNT", *devOverlaySA); err != nil {
			return err
		}

		if err = os.Setenv("NODE_SCOPES", nodeScope); err != nil {
			return err
		}
	}

	if err = os.Setenv("KUBE_GCE_NETWORK", gceInstanceNetwork); err != nil {
		return err
	}

	cmd := exec.Command(filepath.Join(k8sDir, "hack", "e2e-internal", "e2e-up.sh"))
	err = runCommand("Starting E2E Cluster on GCE", cmd)
	if err != nil {
		return fmt.Errorf("failed to bring up kubernetes e2e cluster on gce: %v", err)
	}

	return nil
}

func clusterDownGCE(k8sDir string) error {
	cmd := exec.Command(filepath.Join(k8sDir, "hack", "e2e-internal", "e2e-down.sh"))
	err := runCommand("Bringing Down E2E Cluster on GCE", cmd)
	if err != nil {
		return fmt.Errorf("failed to bring down kubernetes e2e cluster on gce: %v", err)
	}
	return nil
}

func setImageTypeEnvs(imageType string) error {
	switch strings.ToLower(imageType) {
	case "cos":
	case "gci": // GCI/COS is default type and does not need env vars set
	case "ubuntu":
		return errors.New("setting environment vars for bringing up *ubuntu* cluster on GCE is unimplemented")
	default:
		return fmt.Errorf("could not set env for image type %s, only gci, cos, ubuntu supported", imageType)
	}
	return nil
}

func downloadKubernetesSource(pkgDir, k8sIoDir, kubeVersion string) error {
	k8sDir := filepath.Join(k8sIoDir, "kubernetes")
	klog.V(4).Infof("Staging Kubernetes folder not found, downloading now")
	err := os.MkdirAll(k8sIoDir, 0777)
	if err != nil {
		return err
	}

	kubeTarDir := filepath.Join(k8sIoDir, fmt.Sprintf("kubernetes-%s.tar.gz", kubeVersion))

	var vKubeVersion string
	if kubeVersion == "master" {
		vKubeVersion = kubeVersion
		// A hack to be able to build Kubernetes in this nested place
		// KUBE_GIT_VERSION_FILE set to file to load kube version from
		err = os.Setenv("KUBE_GIT_VERSION_FILE", filepath.Join(pkgDir, "test", "k8s-integration", ".dockerized-kube-version-defs"))
		if err != nil {
			return err
		}
	} else {
		vKubeVersion = "v" + kubeVersion
	}
	out, err := exec.Command("curl", "-L", fmt.Sprintf("https://github.com/kubernetes/kubernetes/archive/%s.tar.gz", vKubeVersion), "-o", kubeTarDir).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to curl kubernetes version %s: %s, err: %v", kubeVersion, out, err)
	}

	out, err = exec.Command("tar", "-C", k8sIoDir, "-xvf", kubeTarDir).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to untar %s: %s, err: %v", kubeTarDir, out, err)
	}

	err = os.RemoveAll(k8sDir)
	if err != nil {
		return err
	}

	err = os.Rename(filepath.Join(k8sIoDir, fmt.Sprintf("kubernetes-%s", kubeVersion)), k8sDir)
	if err != nil {
		return err
	}

	klog.V(4).Infof("Successfully downloaded Kubernetes v%s to %s", kubeVersion, k8sDir)

	return nil
}

func getKubeClusterVersion() (string, error) {
	out, err := exec.Command("kubectl", "version", "-o=json").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to obtain cluster version, error: %v", err)
	}
	type version struct {
		ClientVersion *apimachineryversion.Info `json:"clientVersion,omitempty" yaml:"clientVersion,omitempty"`
		ServerVersion *apimachineryversion.Info `json:"serverVersion,omitempty" yaml:"serverVersion,omitempty"`
	}

	var v version
	err = json.Unmarshal(out, &v)
	if err != nil {
		return "", fmt.Errorf("Failed to parse kubectl version output, error: %v", err)
	}

	return v.ServerVersion.GitVersion, nil
}

func mustGetKubeClusterVersion() string {
	ver, err := getKubeClusterVersion()
	if err != nil {
		klog.Fatalf("Error: %v", err)
	}
	return ver
}

// getKubeConfig returns the full path to the
// kubeconfig file set in $KUBECONFIG env.
// If unset, then it defaults to $HOME/.kube/config
func getKubeConfig() (string, error) {
	config, ok := os.LookupEnv("KUBECONFIG")
	if ok {
		return config, nil
	}
	homeDir, ok := os.LookupEnv("HOME")
	if !ok {
		return "", fmt.Errorf("HOME env not set")
	}
	return filepath.Join(homeDir, ".kube/config"), nil
}

// getKubeClient returns a Kubernetes client interface
// for the test cluster
func getKubeClient() (kubernetes.Interface, error) {
	kubeConfig, err := getKubeConfig()
	if err != nil {
		return nil, err
	}
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create config: %v", err)
	}
	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %v", err)
	}
	return kubeClient, nil
}
