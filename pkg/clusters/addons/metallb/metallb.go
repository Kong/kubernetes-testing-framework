package metallb

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"os/exec"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"

	"github.com/kong/kubernetes-testing-framework/pkg/clusters"
	"github.com/kong/kubernetes-testing-framework/pkg/clusters/types/kind"
	ktfdocker "github.com/kong/kubernetes-testing-framework/pkg/utils/docker"
	ktfnet "github.com/kong/kubernetes-testing-framework/pkg/utils/networking"
)

// -----------------------------------------------------------------------------
// Metallb Addon
// -----------------------------------------------------------------------------

const (
	// AddonName indicates the unique name of this addon.
	AddonName clusters.AddonName = "metallb"

	// DefaultNamespace indicates the default namespace this addon will be deployed to.
	DefaultNamespace = "metallb-system"
)

type addon struct{}

func New() clusters.Addon {
	return &addon{}
}

// -----------------------------------------------------------------------------
// Metallb Addon - Addon Implementation
// -----------------------------------------------------------------------------

func (a *addon) Name() clusters.AddonName {
	return AddonName
}

func (a *addon) Deploy(ctx context.Context, cluster clusters.Cluster) error {
	// TODO: derive kubeconfig from cluster object
	if cluster.Type() != kind.KindClusterType {
		return fmt.Errorf("the metallb addon is currently only supported on %s clusters", kind.KindClusterType)
	}

	return deployMetallbForKindCluster(cluster.Client(), cluster.Name(), kind.DefaultKindDockerNetwork)
}

func (a *addon) Delete(ctx context.Context, cluster clusters.Cluster) error {
	// TODO: derive kubeconfig from cluster object
	if cluster.Type() != kind.KindClusterType {
		return fmt.Errorf("the metallb addon is currently only supported on %s clusters", kind.KindClusterType)
	}

	args := []string{
		"--namespace", DefaultNamespace,
		"--context", fmt.Sprintf("kind-%s", cluster.Name()),
		"delete", "configmap", metalConfig,
	}

	stderr := new(bytes.Buffer)
	cmd := exec.CommandContext(ctx, "kubectl", args...) //nolint:gosec
	cmd.Stdout = io.Discard
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", stderr.String(), err)
	}

	return metallbDeleteHack(cluster.Name())
}

func (a *addon) Ready(ctx context.Context, cluster clusters.Cluster) ([]runtime.Object, bool, error) {
	deployment, err := cluster.Client().AppsV1().Deployments(DefaultNamespace).
		Get(context.TODO(), "controller", metav1.GetOptions{})
	if err != nil {
		return nil, false, err
	}

	if deployment.Status.ReadyReplicas != *deployment.Spec.Replicas {
		return []runtime.Object{deployment}, false, nil
	}

	return nil, true, nil
}

// -----------------------------------------------------------------------------
// Private Types, Constants & Vars
// -----------------------------------------------------------------------------

var (
	defaultStartIP = net.ParseIP("0.0.0.240")
	defaultEndIP   = net.ParseIP("0.0.0.250")
	metalManifest  = "https://raw.githubusercontent.com/metallb/metallb/v0.9.5/manifests/metallb.yaml"
	metalConfig    = "config"
)

// -----------------------------------------------------------------------------
// Private Functions
// -----------------------------------------------------------------------------

// deployMetallbForKindCluster deploys Metallb to the given Kind cluster using the Docker network provided for LoadBalancer IPs.
func deployMetallbForKindCluster(kc *kubernetes.Clientset, kindClusterName, dockerNetwork string) error {
	// ensure the namespace for metallb is created
	ns := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: DefaultNamespace}}
	if _, err := kc.CoreV1().Namespaces().Create(context.Background(), &ns, metav1.CreateOptions{}); err != nil {
		if !errors.IsAlreadyExists(err) {
			return err
		}
	}

	// get an IP range for the docker container network to use for MetalLB
	network, err := ktfdocker.GetDockerContainerIPNetwork(ktfdocker.GetKindContainerID(kindClusterName), dockerNetwork)
	if err != nil {
		return err
	}
	ipStart, ipEnd := getIPRangeForMetallb(*network)

	// deploy the metallb configuration
	cfgMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      metalConfig,
			Namespace: DefaultNamespace,
		},
		Data: map[string]string{
			"config": getMetallbYAMLCfg(ipStart, ipEnd),
		},
	}
	if _, err := kc.CoreV1().ConfigMaps(ns.Name).Create(context.Background(), cfgMap, metav1.CreateOptions{}); err != nil {
		if !errors.IsAlreadyExists(err) {
			return err
		}
	}

	// generate and deploy a metallb memberlist secret
	secretKey := make([]byte, 128)
	if _, err := rand.Read(secretKey); err != nil {
		return err
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "memberlist",
			Namespace: ns.Name,
		},
		StringData: map[string]string{
			"secretkey": base64.StdEncoding.EncodeToString(secretKey),
		},
	}
	if _, err := kc.CoreV1().Secrets(ns.Name).Create(context.Background(), secret, metav1.CreateOptions{}); err != nil {
		if !errors.IsAlreadyExists(err) {
			return err
		}
	}

	// create the metallb deployment and related resources
	return metallbDeployHack(kindClusterName)
}

// getIPRangeForMetallb provides a range of IP addresses to use for MetalLB given an IPv4 Network
//
// TODO: Just choosing specific default IPs for now, need to check range validity and dynamically assign IPs.
//       See: https://github.com/Kong/kubernetes-testing-framework/issues/24
func getIPRangeForMetallb(network net.IPNet) (startIP, endIP net.IP) {
	startIP = ktfnet.ConvertUint32ToIPv4(ktfnet.ConvertIPv4ToUint32(network.IP) | ktfnet.ConvertIPv4ToUint32(defaultStartIP))
	endIP = ktfnet.ConvertUint32ToIPv4(ktfnet.ConvertIPv4ToUint32(network.IP) | ktfnet.ConvertIPv4ToUint32(defaultEndIP))
	return
}

func getMetallbYAMLCfg(ipStart, ipEnd net.IP) string {
	return fmt.Sprintf(`
address-pools:
- name: default
  protocol: layer2
  addresses:
  - %s
`, ktfnet.GetIPRangeStr(ipStart, ipEnd))
}

// TODO: needs to be replaced with non-kubectl, just used this originally for speed.
//       See: https://github.com/Kong/kubernetes-testing-framework/issues/25
func metallbDeployHack(clusterName string) error {
	deployArgs := []string{
		"--context", fmt.Sprintf("kind-%s", clusterName),
		"apply", "-f", metalManifest,
	}

	stderr := new(bytes.Buffer)
	cmd := exec.Command("kubectl", deployArgs...) //nolint:gosec
	cmd.Stdout = io.Discard
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", stderr.String(), err)
	}

	return nil
}

func metallbDeleteHack(clusterName string) error {
	deployArgs := []string{
		"--context", fmt.Sprintf("kind-%s", clusterName),
		"delete", "-f", metalManifest,
	}

	stderr := new(bytes.Buffer)
	cmd := exec.Command("kubectl", deployArgs...) //nolint:gosec
	cmd.Stdout = io.Discard
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", stderr.String(), err)
	}

	stderr = new(bytes.Buffer)
	cmd = exec.Command("kubectl", "delete", "namespace", DefaultNamespace)
	cmd.Stdout = io.Discard
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", stderr.String(), err)
	}

	return nil
}
