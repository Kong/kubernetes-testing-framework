package kuma

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/blang/semver/v4"
	"github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/kong/kubernetes-testing-framework/internal/utils"
	"github.com/kong/kubernetes-testing-framework/pkg/clusters"
	"github.com/kong/kubernetes-testing-framework/pkg/utils/github"
)

// -----------------------------------------------------------------------------
// Kuma Addon
// -----------------------------------------------------------------------------

const (
	// AddonName is the unique name of the Kong cluster.Addon
	AddonName clusters.AddonName = "kuma"

	// Namespace is the namespace that the Addon compontents
	// will be deployed under when deployment finishes
	Namespace = "kuma-system"

	// KumaHelmRepo is the Kuma Helm repo URL
	KumaHelmRepo = "https://kumahq.github.io/charts"

	// DefaultReleaseName is the default Helm release name
	DefaultReleaseName = "ktfkuma"
)

// Addon is a Kuma addon which can be deployed on a clusters.Cluster.
type Addon struct {
	name   string
	logger *logrus.Logger

	version          semver.Version
	kumaDeployScript *corev1.ConfigMap
	kumaDeployJob    *batchv1.Job

	mtlsEnabled bool
}

// New produces a new clusters.Addon for Kuma with MTLS enabled
func New() *Addon {
	return NewBuilder().WithMTLS().Build()
}

// Namespace indicates the namespace where the Kuma addon components are to be
// deployed and managed.
func (a *Addon) Namespace() string {
	return Namespace
}

// Version indicates the Kuma version for this addon.
func (a *Addon) Version() semver.Version {
	return a.version
}

// -----------------------------------------------------------------------------
// Kuma Addon - Public Methods
// -----------------------------------------------------------------------------

// EnableMeshForNamespace will add the "kuma.io/sidecar-injection: enabled" label to the provided namespace,
// enabling sidecar injections fo all Pods in the namespace
func EnableMeshForNamespace(ctx context.Context, cluster clusters.Cluster, name string) error {
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context completed while trying to enable mesh for namespace %s: %w", name, ctx.Err())
		default:
			namespace, err := cluster.Client().CoreV1().Namespaces().Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("could not enable mesh for namespace %s: %w", name, err)
			}
			namespace.ObjectMeta.Labels["kuma.io/sidecar-injection"] = "enabled"
			_, err = cluster.Client().CoreV1().Namespaces().Update(ctx, namespace, metav1.UpdateOptions{})
			if err != nil {
				if errors.IsConflict(err) {
					// if there's a conflict then an update happened since we pulled the namespace,
					// simply pull and try again.
					time.Sleep(time.Second)
					continue
				}
				return fmt.Errorf("could not enable mesh for namespace %s: %w", name, err)
			}
			return nil
		}
	}
}

// -----------------------------------------------------------------------------
// Kuma Addon - Addon Implementation
// -----------------------------------------------------------------------------

func (a *Addon) Name() clusters.AddonName {
	return AddonName
}

func (a *Addon) Dependencies(_ context.Context, _ clusters.Cluster) []clusters.AddonName {
	return nil
}

func (a *Addon) Deploy(ctx context.Context, cluster clusters.Cluster) error {
	// wait for dependency addons to be ready first
	if err := clusters.WaitForAddonDependencies(ctx, cluster, a); err != nil {
		return fmt.Errorf("failure waiting for addon dependencies: %w", err)
	}

	// generate a temporary kubeconfig since we're going to be using the helm CLI
	kubeconfig, err := clusters.TempKubeconfig(cluster)
	if err != nil {
		return err
	}
	defer os.Remove(kubeconfig.Name())

	// ensure the repo exists
	stderr := new(bytes.Buffer)
	cmd := exec.CommandContext(ctx, "helm", "--kubeconfig", kubeconfig.Name(), "repo", "add", "--force-update", "kuma", KumaHelmRepo) //nolint:gosec
	cmd.Stdout = io.Discard
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", stderr.String(), err)
	}

	// ensure all repos are up to date
	stderr = new(bytes.Buffer)
	cmd = exec.CommandContext(ctx, "helm", "--kubeconfig", kubeconfig.Name(), "repo", "update") //nolint:gosec
	cmd.Stdout = io.Discard
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", stderr.String(), err)
	}

	// if the dbmode is postgres, set several related values
	args := []string{"--kubeconfig", kubeconfig.Name(), "install", DefaultReleaseName, "kuma/kuma"}

	// compile the helm installation values
	args = append(args, "--create-namespace", "--namespace", Namespace)
	a.logger.Debugf("helm install arguments: %+v", args)

	// run the helm install command
	stderr = new(bytes.Buffer)
	cmd = exec.CommandContext(ctx, "helm", args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		if !strings.Contains(stderr.String(), "cannot re-use") { // ignore if addon is already deployed
			return fmt.Errorf("%s: %w", stderr.String(), err)
		}
	}

	if a.mtlsEnabled {
		if err := a.enableMTLS(ctx, cluster); err != nil {
			return fmt.Errorf("unable to deploy MTLS Mesh configuration: %w", err)
		}
	}

	return nil
}

func (a *Addon) Delete(ctx context.Context, cluster clusters.Cluster) error {
	// generate a temporary kubeconfig since we're going to be using the helm CLI
	kubeconfig, err := clusters.TempKubeconfig(cluster)
	if err != nil {
		return err
	}
	defer os.Remove(kubeconfig.Name())

	// delete the chart release from the cluster
	stderr := new(bytes.Buffer)
	cmd := exec.Command("helm", "--kubeconfig", kubeconfig.Name(), "uninstall", DefaultReleaseName, "--namespace", Namespace) //nolint:gosec
	cmd.Stdout = io.Discard
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", stderr.String(), err)
	}

	return nil
}

func (a *Addon) Ready(ctx context.Context, cluster clusters.Cluster) (waitForObjects []runtime.Object, ready bool, err error) {
	return utils.IsNamespaceAvailable(ctx, cluster, Namespace)
}

// -----------------------------------------------------------------------------
// Kuma Addon - Private Methods
// -----------------------------------------------------------------------------

// useLatestKumaVersion locates and sets the kuma version to deploy to the latest
// non-prelease tag found.
func (a *Addon) useLatestKumaVersion() error {
	latestVersion, err := github.FindLatestReleaseForRepo("kumahq", "kuma")
	if err != nil {
		return err
	}
	a.version = *latestVersion
	return nil
}

// TODO this actually just clobbers the default mesh, which ideally we don't want to do
// however, Kuma apparently doesn't have a clientset, so vov. could do JSON patches, but eh

const (
	mtlsEnabledDefaultMesh = `apiVersion: kuma.io/v1alpha1
kind: Mesh
metadata:
  name: default
spec:
  mtls:
    backends:
    - conf:
        caCert:
          RSAbits: 2048
          expiration: 10y
      dpCert:
        rotation:
          expiration: 1d
      name: ca-1
      type: builtin
    enabledBackend: ca-1`
)

// enableMTLS attempts to apply a Mesh resource with a basic retry mechanism to deal with delays in the Kuma webhook
// startup
func (a *Addon) enableMTLS(ctx context.Context, cluster clusters.Cluster) (err error) {
	for i := 0; i < 5; i++ {
		err = clusters.ApplyYAML(ctx, cluster, mtlsEnabledDefaultMesh)
		if err != nil {
			time.Sleep(time.Second * 5) //nolint:gomnd
		} else {
			break
		}
	}
	return err
}
