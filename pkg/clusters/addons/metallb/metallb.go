package metallb

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/netip"
	"os"
	"time"

	"go4.org/netipx"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/kustomize/api/types"
	kustomize "sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/resid"

	"github.com/kong/kubernetes-testing-framework/internal/retry"
	"github.com/kong/kubernetes-testing-framework/pkg/clusters"
	"github.com/kong/kubernetes-testing-framework/pkg/clusters/types/kind"
	"github.com/kong/kubernetes-testing-framework/pkg/utils/docker"
	"github.com/kong/kubernetes-testing-framework/pkg/utils/kubernetes/kubectl"
)

// -----------------------------------------------------------------------------
// Metallb Addon
// -----------------------------------------------------------------------------

const (
	// AddonName indicates the unique name of this addon.
	AddonName clusters.AddonName = "metallb"

	// DefaultNamespace indicates the default namespace this addon will be deployed to.
	DefaultNamespace = "metallb-system"

	addressPoolName     = "ktf-pool"
	l2AdvertisementName = "ktf-empty"
)

var (
	ipapResource = schema.GroupVersionResource{
		Group:    "metallb.io",
		Version:  "v1beta1",
		Resource: "ipaddresspools",
	}
	l2aResource = schema.GroupVersionResource{
		Group:    "metallb.io",
		Version:  "v1beta1",
		Resource: "l2advertisements",
	}
)

type Addon struct {
	disablePoolCreation bool
}

func New() clusters.Addon {
	return &Addon{}
}

// -----------------------------------------------------------------------------
// Metallb Addon - Addon Implementation
// -----------------------------------------------------------------------------

func (a *Addon) Name() clusters.AddonName {
	return AddonName
}

func (a *Addon) Dependencies(_ context.Context, _ clusters.Cluster) []clusters.AddonName {
	return nil
}

func (a *Addon) Deploy(ctx context.Context, cluster clusters.Cluster) error {
	if cluster.Type() != kind.KindClusterType {
		return fmt.Errorf("the metallb addon is currently only supported on %s clusters", kind.KindClusterType)
	}

	return a.deployMetallbForKindCluster(ctx, cluster, kind.DefaultKindDockerNetwork)
}

func (a *Addon) Delete(ctx context.Context, cluster clusters.Cluster) error {
	if cluster.Type() != kind.KindClusterType {
		return fmt.Errorf("the metallb addon is currently only supported on %s clusters", kind.KindClusterType)
	}

	dynamicClient, err := dynamic.NewForConfig(cluster.Config())
	if err != nil {
		return fmt.Errorf("failed to create dynamic client: %w", err)
	}

	res := dynamicClient.Resource(l2aResource).Namespace(DefaultNamespace)
	err = res.Delete(ctx, l2AdvertisementName, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	res = dynamicClient.Resource(ipapResource).Namespace(DefaultNamespace)
	err = res.Delete(ctx, addressPoolName, metav1.DeleteOptions{})
	if err != nil {
		return err
	}

	// generate a temporary kubeconfig since we're going to be using kubectl
	kubeconfig, err := clusters.TempKubeconfig(cluster)
	if err != nil {
		return err
	}
	defer os.Remove(kubeconfig.Name())

	return metallbDeleteHack(ctx, kubeconfig)
}

func (a *Addon) Ready(ctx context.Context, cluster clusters.Cluster) ([]runtime.Object, bool, error) {
	deployment, err := cluster.Client().AppsV1().Deployments(DefaultNamespace).
		Get(ctx, "controller", metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, false, nil
		}
		return nil, false, err
	}

	if deployment.Status.AvailableReplicas != *deployment.Spec.Replicas {
		return []runtime.Object{deployment}, false, nil
	}

	return nil, true, nil
}

func (a *Addon) DumpDiagnostics(context.Context, clusters.Cluster) (map[string][]byte, error) {
	diagnostics := make(map[string][]byte)
	return diagnostics, nil
}

// -----------------------------------------------------------------------------
// Private Types, Constants & Vars
// -----------------------------------------------------------------------------

var (
	metalManifest = "https://github.com/metallb/metallb/config/native?ref=v0.13.11&timeout=2m"
	secretKeyLen  = 128
)

// -----------------------------------------------------------------------------
// Private Functions
// -----------------------------------------------------------------------------

// deployMetallbForKindCluster deploys Metallb to the given Kind cluster using the Docker network provided for LoadBalancer IPs.
func (a *Addon) deployMetallbForKindCluster(ctx context.Context, cluster clusters.Cluster, dockerNetwork string) error {
	// ensure the namespace for metallb is created
	ns := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: DefaultNamespace}}
	if _, err := cluster.Client().CoreV1().Namespaces().Create(ctx, &ns, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
	}

	// create the metallb deployment and related resources (do this first so that
	// we can create the IPAddressPool below with its CRD already in place).
	if err := metallbDeployHack(ctx, cluster); err != nil {
		return fmt.Errorf("failed to deploy metallb: %w", err)
	}

	// create an ip address pool
	if !a.disablePoolCreation {
		if err := createIPAddressPool(ctx, cluster, dockerNetwork); err != nil {
			return err
		}
	}

	if err := createL2Advertisement(ctx, cluster); err != nil {
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
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
	}

	return nil
}

func createIPAddressPool(ctx context.Context, cluster clusters.Cluster, dockerNetwork string) error {
	// get an IP range for the docker container network to use for MetalLB
	// this returns addresses based on the _Docker network_ the cluster runs on, not the cluster itself. this may,
	// for example, return IPv4 addresses even for an IPv6-only cluster. although unsupported addresses will be listed
	// in the IPAddressPool, speaker will not actually assign them if they are not compatible with the cluster network.
	network, network6, err := docker.GetDockerContainerIPNetwork(docker.GetKindContainerID(cluster.Name()), dockerNetwork)
	if err != nil {
		return err
	}
	ipStart, ipEnd := getIPRangeForMetallb(*network)
	ip6Start, ip6End := getIPRangeForMetallb(*network6)

	dynamicClient, err := dynamic.NewForConfig(cluster.Config())
	if err != nil {
		return fmt.Errorf("failed to create dynamic client: %w", err)
	}

	res := dynamicClient.Resource(ipapResource).Namespace(DefaultNamespace)

	ctx, cancel := context.WithTimeout(ctx, time.Minute*3) //nolint:gomnd
	defer cancel()

	var lastErr error
	for {
		_, err = res.Create(ctx, &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "metallb.io/v1beta1",
				"kind":       "IPAddressPool",
				"metadata": map[string]string{
					"name": addressPoolName,
				},
				"spec": map[string]interface{}{
					"addresses": []string{
						fmt.Sprintf("%s-%s", ipStart, ipEnd),
						fmt.Sprintf("%s-%s", ip6Start, ip6End),
					},
				},
			},
		}, metav1.CreateOptions{})

		if err != nil {
			if apierrors.IsAlreadyExists(err) {
				// delete the existing resource and recreate it in another round of loop.
				err = res.Delete(ctx, addressPoolName, metav1.DeleteOptions{})
			}

			select {
			case <-time.After(time.Second):
				lastErr = err
				continue
			case <-ctx.Done():
				return fmt.Errorf("failed to create metallb.io/v1beta1 IPAddressPool: %w, last error on create: %v", ctx.Err(), lastErr)
			}
		}

		break
	}
	return nil
}

func createL2Advertisement(ctx context.Context, cluster clusters.Cluster) error {
	dynamicClient, err := dynamic.NewForConfig(cluster.Config())
	if err != nil {
		return fmt.Errorf("failed to create dynamic client: %w", err)
	}

	res := dynamicClient.Resource(l2aResource).Namespace(DefaultNamespace)

	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	var lastErr error
	for {
		_, err = res.Create(ctx, &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "metallb.io/v1beta1",
				"kind":       "L2Advertisement",
				"metadata": map[string]string{
					"name": l2AdvertisementName,
				},
			},
		}, metav1.CreateOptions{})

		if err != nil {
			if apierrors.IsAlreadyExists(err) {
				// delete the existing resource and recreate it in another round of loop.
				err = res.Delete(ctx, l2AdvertisementName, metav1.DeleteOptions{})
			}

			lastErr = err
			select {
			case <-time.After(time.Second):
				continue
			case <-ctx.Done():
				return fmt.Errorf("failed to create metallb.io/v1beta1 L2Advertisement: %w, last error %v", ctx.Err(), lastErr)
			}
		}

		break
	}
	return nil
}

// TODO use netip throughout. this converts because old public APIs used net/ip instead of net/netip

// getIPRangeForMetallb provides a range of IP addresses to use for MetalLB given an IPv4 Network
//
// TODO: this just chooses the upper half of the Docker network (minus the network and broadcast addresses for the
// chosen subnet), although those IPs may be in use. Speaker will happily assign those, but they won't work.
// In practice this doesn't appear to cause many problems, since the IPs are normally not in use by KIND components
// (it appears to assign starting from the bottom of the Docker net)
//
// See: https://github.com/Kong/kubernetes-testing-framework/issues/24
func getIPRangeForMetallb(network net.IPNet) (startIP, endIP netip.Addr) {
	// we trust that this is a valid prefix here because we already checked it in docker.GetDockerContainerIPNetwork
	prefix := netip.MustParsePrefix(network.String())
	half := prefix.Bits() + 1
	wholeRange := netipx.RangeOfPrefix(prefix)
	upperHalfPrefix := netip.PrefixFrom(wholeRange.To(), half).Masked()
	halfRange := netipx.RangeOfPrefix(upperHalfPrefix)
	return halfRange.From().Next(), halfRange.To().Prev()
}

// TODO: needs to be replaced with non-kubectl, just used this originally for speed.
//
// See: https://github.com/Kong/kubernetes-testing-framework/issues/25

const admissionPatch = `
- op: replace
  # ipaddresspoolvalidationwebhook.metallb.io
  path: /webhooks/5/failurePolicy
  value: "Ignore"
- op: replace
  # l2advertisementvalidationwebhook.metallb.io
  path: /webhooks/6/failurePolicy
  value: "Ignore"
`

func getManifest() (io.Reader, error) {
	return kubectl.GetKustomizedManifest(kustomize.Kustomization{
		Resources: []string{metalManifest},
		Patches: []kustomize.Patch{
			{
				Patch: admissionPatch,
				Target: &types.Selector{
					ResId: resid.ResId{
						Gvk: resid.Gvk{
							Group:   "admissionregistration.k8s.io",
							Version: "v1",
							Kind:    "ValidatingWebhookConfiguration",
						},
						Name:      "metallb-webhook-configuration",
						Namespace: "metallb-system",
					},
				},
			},
		},
	})
}

func metallbDeployHack(ctx context.Context, cluster clusters.Cluster) error {
	// generate a temporary kubeconfig since we're going to be using kubectl
	kubeconfig, err := clusters.TempKubeconfig(cluster)
	if err != nil {
		return err
	}

	defer os.Remove(kubeconfig.Name())

	deployArgs := []string{
		"--kubeconfig", kubeconfig.Name(),
		"apply", "-f", "-",
	}

	manifest, err := getManifest()
	if err != nil {
		return fmt.Errorf("could not deploy metallb: %w", err)
	}

	// ensure the repo exists
	return retry.Command("kubectl", deployArgs...).
		WithStdin(manifest).
		WithStdout(io.Discard).
		Do(ctx)
}

func metallbDeleteHack(ctx context.Context, kubeconfig *os.File) error {
	deployArgs := []string{
		"--kubeconfig", kubeconfig.Name(),
		"delete", "-f", "-",
	}

	manifest, err := getManifest()
	if err != nil {
		return fmt.Errorf("could not delete metallb: %w", err)
	}

	return retry.Command("kubectl", deployArgs...).
		WithStdin(manifest).
		WithStdout(io.Discard).
		Do(ctx)
}
