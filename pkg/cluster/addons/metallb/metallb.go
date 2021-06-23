package metallb

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net"
	"os"
	"os/exec"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/kong/kubernetes-testing-framework/pkg/cluster"
	ktfcluster "github.com/kong/kubernetes-testing-framework/pkg/cluster"
	"github.com/kong/kubernetes-testing-framework/pkg/cluster/kind"
	ktfdocker "github.com/kong/kubernetes-testing-framework/pkg/util/docker"
	ktfnet "github.com/kong/kubernetes-testing-framework/pkg/util/networking"
)

// -----------------------------------------------------------------------------
// Public Methods - Metallb Addon
// -----------------------------------------------------------------------------

func New() cluster.Addon {
	return addon("metallb")
}

func (a addon) Name() string {
	return string(a)
}

func (a addon) Deploy(cluster ktfcluster.Cluster) error {
	if cluster.Type() != kind.KindClusterType {
		return fmt.Errorf("the metallb addon is currently only supported on %s clusters", kind.KindClusterType)
	}

	return deployMetallbForKindCluster(cluster.Client(), cluster.Name(), kind.DefaultKindDockerNetwork)
}

func (a addon) Delete(cluster ktfcluster.Cluster) error {
	if cluster.Type() != kind.KindClusterType {
		return fmt.Errorf("the metallb addon is currently only supported on %s clusters", kind.KindClusterType)
	}

	args := []string{
		"--namespace", metalNamespace,
		"--context", fmt.Sprintf("kind-%s", cluster.Name()),
		"delete", "configmap", metalConfig,
	}

	cmd := exec.Command("kubectl", args...) //nolint:gosec
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// -----------------------------------------------------------------------------
// Private Types, Constants & Vars
// -----------------------------------------------------------------------------

type addon string

var (
	defaultStartIP = net.ParseIP("0.0.0.240")
	defaultEndIP   = net.ParseIP("0.0.0.250")
	metalManifest  = "https://raw.githubusercontent.com/metallb/metallb/v0.9.5/manifests/metallb.yaml"
	metalNamespace = "metallb-system"
	metalConfig    = "config"
)

// -----------------------------------------------------------------------------
// Private Functions
// -----------------------------------------------------------------------------

// deployMetallbForKindCluster deploys Metallb to the given Kind cluster using the Docker network provided for LoadBalancer IPs.
func deployMetallbForKindCluster(kc *kubernetes.Clientset, kindClusterName, dockerNetwork string) error {
	// ensure the namespace for metallb is created
	ns := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: metalNamespace}}
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
			Namespace: metalNamespace,
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
	cmd := exec.Command("kubectl", deployArgs...) //nolint:gosec
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func metallbDeleteHack(clusterName string) error {
	deployArgs := []string{
		"--context", fmt.Sprintf("kind-%s", clusterName),
		"delete", "-f", metalManifest,
	}
	cmd := exec.Command("kubectl", deployArgs...) //nolint:gosec
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
