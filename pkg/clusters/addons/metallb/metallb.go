package metallb

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"github.com/kong/kubernetes-testing-framework/pkg/clusters"
	"github.com/kong/kubernetes-testing-framework/pkg/clusters/types/kind"
	"github.com/kong/kubernetes-testing-framework/pkg/utils/docker"
	"github.com/kong/kubernetes-testing-framework/pkg/utils/networking"
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

func (a *addon) Dependencies(_ context.Context, _ clusters.Cluster) []clusters.AddonName {
	return nil
}

func (a *addon) Deploy(ctx context.Context, cluster clusters.Cluster) error {
	if cluster.Type() != kind.KindClusterType {
		return fmt.Errorf("the metallb addon is currently only supported on %s clusters", kind.KindClusterType)
	}

	return deployMetallbForKindCluster(ctx, cluster, kind.DefaultKindDockerNetwork)
}

func (a *addon) Delete(ctx context.Context, cluster clusters.Cluster) error {
	if cluster.Type() != kind.KindClusterType {
		return fmt.Errorf("the metallb addon is currently only supported on %s clusters", kind.KindClusterType)
	}

	// generate a temporary kubeconfig since we're going to be using kubectl
	kubeconfig, err := clusters.TempKubeconfig(cluster)
	if err != nil {
		return err
	}
	defer os.Remove(kubeconfig.Name())

	args := []string{
		"--kubeconfig", kubeconfig.Name(),
		"--namespace", DefaultNamespace,
		"delete", "configmap", metalConfig,
	}

	stderr := new(bytes.Buffer)
	cmd := exec.CommandContext(ctx, "kubectl", args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", stderr.String(), err)
	}

	return metallbDeleteHack(kubeconfig)
}

func (a *addon) Ready(ctx context.Context, cluster clusters.Cluster) ([]runtime.Object, bool, error) {
	deployment, err := cluster.Client().AppsV1().Deployments(DefaultNamespace).
		Get(context.TODO(), "controller", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, false, nil
		}
		return nil, false, err
	}

	if deployment.Status.AvailableReplicas != *deployment.Spec.Replicas {
		return []runtime.Object{deployment}, false, nil
	}

	return nil, true, nil
}

func (a *addon) DumpDiagnostics(ctx context.Context, cluster clusters.Cluster) (map[string][]byte, error) {
	diagnostics := make(map[string][]byte)
	return diagnostics, nil
}

// -----------------------------------------------------------------------------
// Private Types, Constants & Vars
// -----------------------------------------------------------------------------

var (
	defaultStartIP = net.ParseIP("0.0.0.240")
	defaultEndIP   = net.ParseIP("0.0.0.250")
	metalManifest  = "https://raw.githubusercontent.com/metallb/metallb/v0.13.5/config/manifests/metallb-native.yaml"
	metalConfig    = "config"
	secretKeyLen   = 128
)

// -----------------------------------------------------------------------------
// Private Functions
// -----------------------------------------------------------------------------

// deployMetallbForKindCluster deploys Metallb to the given Kind cluster using the Docker network provided for LoadBalancer IPs.
func deployMetallbForKindCluster(ctx context.Context, cluster clusters.Cluster, dockerNetwork string) error {
	// ensure the namespace for metallb is created
	ns := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: DefaultNamespace}}
	if _, err := cluster.Client().CoreV1().Namespaces().Create(ctx, &ns, metav1.CreateOptions{}); err != nil {
		if !errors.IsAlreadyExists(err) {
			return err
		}
	}

	// create the metallb deployment and related resources (do this first so that
	// we can create the IPAddressPool below with its CRD already in place).
	if err := metallbDeployHack(cluster); err != nil {
		return fmt.Errorf("failed to deploy metallb: %w", err)
	}

	// create an ip address pool
	if err := createIPAddressPool(ctx, cluster, dockerNetwork); err != nil {
		return err
	}

	// generate and deploy a metallb memberlist secret
	secretKey := make([]byte, secretKeyLen)
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
	if _, err := cluster.Client().CoreV1().Secrets(ns.Name).Create(ctx, secret, metav1.CreateOptions{}); err != nil {
		if !errors.IsAlreadyExists(err) {
			return err
		}
	}

	return nil
}

func createIPAddressPool(ctx context.Context, cluster clusters.Cluster, dockerNetwork string) error {
	// get an IP range for the docker container network to use for MetalLB
	network, err := docker.GetDockerContainerIPNetwork(docker.GetKindContainerID(cluster.Name()), dockerNetwork)
	if err != nil {
		return err
	}
	ipStart, ipEnd := getIPRangeForMetallb(*network)

	dynamicClient, err := dynamic.NewForConfig(cluster.Config())
	if err != nil {
		return fmt.Errorf("failed to create dynamic client: %w", err)
	}

	res := dynamicClient.Resource(schema.GroupVersionResource{
		Group:    "metallb.io",
		Version:  "v1beta1",
		Resource: "ipaddresspools",
	}).Namespace("metallb-system")

	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	for {
		_, err = res.Create(ctx, &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "metallb.io/v1beta1",
				"kind":       "IPAddressPool",
				"metadata": map[string]string{
					"name": "metallb-addresspool",
				},
				"spec": map[string]interface{}{
					"addresses": []string{
						networking.GetIPRangeStr(ipStart, ipEnd),
					},
				},
			},
		}, metav1.CreateOptions{})

		if err != nil {
			select {
			case <-time.After(time.Second):
				continue
			case <-ctx.Done():
				return fmt.Errorf("failed to create metallb.io/v1beta1 IPAddressPool: %w", ctx.Err())
			}
		}

		break
	}
	return nil
}

// getIPRangeForMetallb provides a range of IP addresses to use for MetalLB given an IPv4 Network
//
// TODO: Just choosing specific default IPs for now, need to check range validity and dynamically assign IPs.
//
// See: https://github.com/Kong/kubernetes-testing-framework/issues/24
func getIPRangeForMetallb(network net.IPNet) (startIP, endIP net.IP) {
	startIP = networking.ConvertUint32ToIPv4(networking.ConvertIPv4ToUint32(network.IP) | networking.ConvertIPv4ToUint32(defaultStartIP))
	endIP = networking.ConvertUint32ToIPv4(networking.ConvertIPv4ToUint32(network.IP) | networking.ConvertIPv4ToUint32(defaultEndIP))
	return
}

// TODO: needs to be replaced with non-kubectl, just used this originally for speed.
//
// See: https://github.com/Kong/kubernetes-testing-framework/issues/25
func metallbDeployHack(cluster clusters.Cluster) error {
	// generate a temporary kubeconfig since we're going to be using kubectl
	kubeconfig, err := clusters.TempKubeconfig(cluster)
	if err != nil {
		return err
	}
	defer os.Remove(kubeconfig.Name())

	deployArgs := []string{
		"--kubeconfig", kubeconfig.Name(),
		"apply", "-f", metalManifest,
	}

	stderr := new(bytes.Buffer)
	cmd := exec.Command("kubectl", deployArgs...)
	cmd.Stdout = io.Discard
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", stderr.String(), err)
	}

	return nil
}

func metallbDeleteHack(kubeconfig *os.File) error {
	deployArgs := []string{
		"--kubeconfig", kubeconfig.Name(),
		"delete", "-f", metalManifest,
	}

	stderr := new(bytes.Buffer)
	cmd := exec.Command("kubectl", deployArgs...)
	cmd.Stdout = io.Discard
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", stderr.String(), err)
	}

	stderr = new(bytes.Buffer)
	cmd = exec.Command("kubectl", "--kubeconfig", kubeconfig.Name(), "delete", "namespace", DefaultNamespace) //nolint:gosec
	cmd.Stdout = io.Discard
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", stderr.String(), err)
	}

	return nil
}
