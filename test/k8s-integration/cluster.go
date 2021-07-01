package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/oauth2"
	container "google.golang.org/api/container/v1beta1"
	"google.golang.org/api/option"
	"k8s.io/apimachinery/pkg/util/wait"
	apimachineryversion "k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
)

const KubeSystemNamespace = "kube-system"
const FilestoreNodeGkeDaemonset = "filestore-node"

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

func isRegionalGKECluster(gceZone, gceRegion string) bool {
	return len(gceRegion) > 0
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

	if *deploymentStrat != "gke" {
		if err = os.Setenv("KUBE_GCE_NETWORK", gceInstanceNetwork); err != nil {
			return err
		}
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
	if err := os.RemoveAll(k8sDir); err != nil {
		return err
	}

	if kubeVersion == "master" {
		// Clone of master. We cannot download the master version from the archive, because the k8s
		// version is not set, which affects which APIs are removed in the running cluster. We cannot
		// use a shallow clone, because in order to find the revision git searches through the tags,
		// and tags are not fetched in a shallow clone. Not using a shallow clone adds about 700M to the
		// ~5G archive directory, after make quick-release, so this is not disastrous.
		out, err := exec.Command("git", "clone", "https://github.com/kubernetes/kubernetes", k8sDir).CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to clone kubernetes master: %s, err: %v", out, err)
		}
		klog.V(4).Infof("Successfully cloned Kubernetes master to %s", k8sDir)
	} else {
		// Download from the release archives rather than cloning the repo.
		vKubeVersion := "v" + kubeVersion
		kubeTarDir := filepath.Join(k8sIoDir, fmt.Sprintf("kubernetes-%s.tar.gz", kubeVersion))
		out, err := exec.Command("curl", "-L", fmt.Sprintf("https://github.com/kubernetes/kubernetes/archive/%s.tar.gz", vKubeVersion), "-o", kubeTarDir).CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to curl kubernetes version %s: %s, err: %v", kubeVersion, out, err)
		}

		out, err = exec.Command("tar", "-C", k8sIoDir, "-xvf", kubeTarDir).CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to untar %s: %s, err: %v", kubeTarDir, out, err)
		}

		err = os.Rename(filepath.Join(k8sIoDir, fmt.Sprintf("kubernetes-%s", kubeVersion)), k8sDir)
		if err != nil {
			return err
		}

		klog.V(4).Infof("Successfully downloaded Kubernetes v%s to %s", kubeVersion, k8sDir)
	}
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

func clusterDownGKE(gceZone, gceRegion string) error {
	locationArg, locationVal, err := gkeLocationArgs(gceZone, gceRegion)
	if err != nil {
		return err
	}

	fmt.Printf("Bringing down GKE cluster %v, location arg %v, location val %v", *gkeTestClusterName, locationArg, locationVal)
	out, err := exec.Command("gcloud", "container", "clusters", "delete", *gkeTestClusterName,
		locationArg, locationVal, "--quiet").CombinedOutput()
	fmt.Printf("cluster delete output:\n%v", string(out))
	if err != nil && !isNotFoundError(string(out)) {
		return fmt.Errorf("failed to bring down kubernetes e2e cluster on gke: %v", err)
	}
	return nil
}

func clusterUpGKE(gceZone, gceRegion string, numNodes int, imageType string, useStagingDriver bool) error {
	locationArg, locationVal, err := gkeLocationArgs(gceZone, gceRegion)
	if err != nil {
		return err
	}

	out, err := exec.Command("gcloud", "container", "clusters", "list",
		locationArg, locationVal, "--verbosity", "none",
		fmt.Sprintf("--filter=name=%s", *gkeTestClusterName)).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to check for previous test cluster: %v %s", err, out)
	}
	fmt.Printf("cluster list output:\n%v", string(out))
	if len(out) > 0 {
		klog.Infof("Detected previous cluster %s. Deleting so a new one can be created...", *gkeTestClusterName)
		err = clusterDownGKE(gceZone, gceRegion)
		if err != nil {
			return err
		}
	}

	if useStagingDriver {
		accessToken, err := getAccessToken()
		if err != nil {
			return err
		}

		token := &oauth2.Token{AccessToken: string(accessToken)}
		oauthClient := oauth2.NewClient(context.Background(), oauth2.StaticTokenSource(token))
		service, err := container.NewService(context.Background(), option.WithHTTPClient(oauthClient))
		if err != nil {
			return fmt.Errorf("failed to create a new service: %v", err)
		}

		// TODO: change to read CLOUDSDK_API_ENDPOINT_OVERRIDES_CONTAINER env var
		service.BasePath = "https://test-container.sandbox.googleapis.com/" // e.g test, staging env URL

		// TODO: honor the gkeNodeVersion flag
		request := &container.CreateClusterRequest{
			Cluster: &container.Cluster{
				Name:             *gkeTestClusterName,
				InitialNodeCount: int64(numNodes),
				AddonsConfig: &container.AddonsConfig{
					GcpFilestoreCsiDriverConfig: &container.GcpFilestoreCsiDriverConfig{Enabled: true},
				},
				ReleaseChannel: &container.ReleaseChannel{Channel: "RAPID"},
				NodeConfig: &container.NodeConfig{
					MachineType: "n1-standard-2",
					ImageType:   imageType,
					OauthScopes: []string{
						"https://www.googleapis.com/auth/devstorage.read_only",
					},
				},
			},
		}

		klog.Infof("Creating kubernetes e2e cluster on gke")
		project, err := getCurrProject()
		if err != nil {
			return err
		}
		parent := fmt.Sprintf("projects/%s/locations/%s", project, locationVal)
		klog.Infof("Creating cluster under parent path %s", parent)
		op, err := service.Projects.Locations.Clusters.Create(parent, request).Do()
		if err != nil {
			return fmt.Errorf("failed to Create kubernetes e2e cluster on gke: %v", err)
		}
		err = waitForOp(service.Projects.Locations.Operations, parent, op)
		if err != nil {
			return fmt.Errorf("WaitFor Cluster Create operation failed: %v", err)
		}

		// fetch context because otherwise kubectl won't be able to talk to cluster
		cmd := exec.Command("gcloud", "container", "clusters", "get-credentials", *gkeTestClusterName, "--project", project, locationArg, locationVal)
		err = runCommand(fmt.Sprintf("fetching credentials from cluster %s", *gkeTestClusterName), cmd)
		if err != nil {
			return fmt.Errorf("failed to fetch credential from cluster: %v", err)
		}

		// wait for driver to be ready
		err = waitForNodeDaemonset(KubeSystemNamespace, FilestoreNodeGkeDaemonset)
		if err != nil {
			return fmt.Errorf("issue while waiting for node daemonset: %v", err)
		}

	} else {

		var cmd *exec.Cmd
		cmdParams := []string{"container", "clusters", "create", *gkeTestClusterName,
			locationArg, locationVal, "--num-nodes", strconv.Itoa(numNodes),
			"--quiet", "--machine-type", "n1-standard-2", "--image-type", imageType}
		if isVariableSet(gkeClusterVer) {
			cmdParams = append(cmdParams, "--cluster-version", *gkeClusterVer)
		} else {
			cmdParams = append(cmdParams, "--release-channel", *gkeReleaseChannel)
			// release channel based GKE clusters require autorepair to be enabled.
			cmdParams = append(cmdParams, "--enable-autorepair")
		}

		if isVariableSet(gkeNodeVersion) {
			cmdParams = append(cmdParams, "--node-version", *gkeNodeVersion)
		}

		cmd = exec.Command("gcloud", cmdParams...)
		err = runCommand("Starting E2E Cluster on GKE", cmd)
		if err != nil {
			return fmt.Errorf("failed to bring up kubernetes e2e cluster on gke: %v", err)
		}
	}

	return nil
}

func getAccessToken() ([]byte, error) {
	accessToken, err := exec.Command("gcloud", "auth", "print-access-token").Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get authentication token: %v", err)
	}
	return accessToken[:len(accessToken)-1], nil
}

func waitForNodeDaemonset(driverNamespace string, nodeDaemonset string) error {
	retries := 15
	for ; retries > 0; retries-- {
		ready, err := exec.Command("kubectl", "-n", driverNamespace, "get", "daemonset", nodeDaemonset, "-o", "jsonpath=\"{.status.numberReady}\"").CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to query the readyness of daemonset %s", nodeDaemonset)
		}
		required, err := exec.Command("kubectl", "-n", driverNamespace, "get", "daemonset", nodeDaemonset, "-o", "jsonpath=\"{.status.desiredNumberScheduled}\"").CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to query the desired state of daemonset %s", nodeDaemonset)
		}
		if string(ready) == string(required) {
			klog.Infof("Daemonset %s is ready with %s pods", nodeDaemonset, string(ready))
			break
		}

		klog.Infof("required: %s, ready: %s", string(required), string(ready))

		time.Sleep(10 * time.Second)
	}

	if retries == 0 {
		return fmt.Errorf("timeout waiting for daemonset %s to become ready", nodeDaemonset)
	}

	return nil
}

func waitForOp(operationsService *container.ProjectsLocationsOperationsService, parent string, op *container.Operation) error {
	klog.Infof("Waiting for the %s call to finish", op.Name)
	opName := fmt.Sprintf("%s/operations/%s", parent, op.Name)
	return wait.Poll(5*time.Second, 5*time.Minute, func() (bool, error) {
		pollOp, err := operationsService.Get(opName).Do()
		if err != nil {
			return false, err
		}
		return isOpDone(pollOp)
	})
}

func isOpDone(op *container.Operation) (bool, error) {
	if op == nil {
		return false, nil
	}
	if op.Error != nil {
		return true, fmt.Errorf("operation %v failed (%v): %v", op.Name, op.Error.Code, op.Error.Message)
	}
	return op.Status == "DONE", nil
}

func getGKEKubeTestArgs(gceZone, gceRegion, imageType string) ([]string, error) {
	var locationArg, locationVal string
	switch {
	case len(gceZone) > 0:
		locationArg = "--gcp-zone"
		locationVal = gceZone
	case len(gceRegion) > 0:
		locationArg = "--gcp-region"
		locationVal = gceRegion
	}

	var gkeEnv string
	switch gkeURL := os.Getenv("CLOUDSDK_API_ENDPOINT_OVERRIDES_CONTAINER"); gkeURL {
	case "https://staging-container.sandbox.googleapis.com/":
		gkeEnv = "staging"
	case "https://test-container.sandbox.googleapis.com/":
		gkeEnv = "test"
	case "":
		gkeEnv = "prod"
	default:
		// if the URL does not match to an option, assume it is a custom GKE backend
		// URL and pass that to kubetest
		gkeEnv = gkeURL
	}

	project, err := getCurrProject()
	if err != nil {
		return nil, err
	}

	args := []string{
		"--up=false",
		"--down=false",
		"--provider=gke",
		"--gcp-network=default",
		"--check-version-skew=false",
		"--deployment=gke",
		fmt.Sprintf("--gcp-node-image=%s", imageType),
		fmt.Sprintf("--cluster=%s", *gkeTestClusterName),
		fmt.Sprintf("--gke-environment=%s", gkeEnv),
		fmt.Sprintf("%s=%s", locationArg, locationVal),
		fmt.Sprintf("--gcp-project=%s", project),
	}

	return args, nil
}

func isNotFoundError(errstr string) bool {
	return strings.Contains(strings.ToLower(errstr), "code=404")
}

func getCurrProject() (string, error) {
	cmd := exec.Command("gcloud", "config", "get-value", "project")
	project, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get current project: %v", err)
	}
	return string(project[:len(project)-1]), nil
}
