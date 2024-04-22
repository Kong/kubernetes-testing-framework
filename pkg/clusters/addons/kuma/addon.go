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
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/kong/kubernetes-testing-framework/internal/retry"
	"github.com/kong/kubernetes-testing-framework/internal/utils"
	"github.com/kong/kubernetes-testing-framework/pkg/clusters"
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

	version *semver.Version

	mtlsEnabled      bool
	additionalValues map[string]string
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

// Version returns the version of the Kuma Helm chart deployed by the addon.
// If the version is not set, the second return value will be false and the latest local
// chart version will be used.
func (a *Addon) Version() (v semver.Version, ok bool) {
	if a.version == nil {
		return semver.Version{}, false
	}
	return *a.version, true
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

	if a.version != nil {
		args = append(args, "--version", a.version.String())
	}

	// compile the helm installation values
	args = append(args, "--create-namespace", "--namespace", Namespace)

	for name, value := range a.additionalValues {
		args = append(args, "--set", fmt.Sprintf("%s=%s", name, value))
	}

	a.logger.Debugf("helm install arguments: %+v", args)

	// Sometimes running helm install fails. Just in case this happens, retry.
	err = retry.
		Command("helm", args...).
		DoWithErrorHandling(ctx, func(err error, _, stderr *bytes.Buffer) error {
			// ignore if addon is already deployed
			if strings.Contains(stderr.String(), "cannot re-use") {
				return nil
			}
			return fmt.Errorf("%s: %w", stderr, err)
		})
	if err != nil {
		return err
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
	cmd := exec.CommandContext(ctx, "helm", "--kubeconfig", kubeconfig.Name(), "uninstall", DefaultReleaseName, "--namespace", Namespace) //nolint:gosec
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

func (a *Addon) DumpDiagnostics(context.Context, clusters.Cluster) (map[string][]byte, error) {
	diagnostics := make(map[string][]byte)
	return diagnostics, nil
}

// -----------------------------------------------------------------------------
// Kuma Addon - Private Methods
// -----------------------------------------------------------------------------

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

	allowAllTrafficPermission = `apiVersion: kuma.io/v1alpha1
kind: MeshTrafficPermission
metadata:
  name: allow-all
  namespace: kuma-system
  labels:
    kuma.io/mesh: default
spec:
  targetRef:
    kind: Mesh
  from:
  - targetRef:
      kind: Mesh
    default:
      action: Allow`
)

var (
	// From Kuma 2.6.0, the default mesh traffic permission is no longer created by default
	// and must be created manually if mTLS is enabled.
	// https://github.com/kumahq/kuma/blob/2.6.0/UPGRADE.md#default-trafficroute-and-trafficpermission-resources-are-not-created-when-creating-a-new-mesh
	installDefaultMeshTrafficPermissionCutoffVersion = semver.MustParse("2.6.0")
)

// enableMTLS attempts to apply a Mesh resource with a basic retry mechanism to deal with delays in the Kuma webhook
// startup
func (a *Addon) enableMTLS(ctx context.Context, cluster clusters.Cluster) (err error) {
	ticker := time.NewTicker(5 * time.Second) //nolint:mnd
	defer ticker.Stop()
	timeoutTimer := time.NewTimer(time.Minute)

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context completed while retrying to apply Mesh")
		case <-ticker.C:
			yamlToApply := mtlsEnabledDefaultMesh
			if v, ok := a.Version(); ok && v.GTE(installDefaultMeshTrafficPermissionCutoffVersion) {
				a.logger.Infof("Kuma version is %s or later, creating default mesh traffic permission", installDefaultMeshTrafficPermissionCutoffVersion)
				yamlToApply = strings.Join([]string{mtlsEnabledDefaultMesh, allowAllTrafficPermission}, "\n---\n")
			}
			err = clusters.ApplyManifestByYAML(ctx, cluster, yamlToApply)
			if err == nil {
				return nil
			}
		case <-timeoutTimer.C:
			// return the error of last retry.
			return err
		}
	}
}
