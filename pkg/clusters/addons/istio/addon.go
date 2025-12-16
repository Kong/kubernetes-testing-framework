package istio

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/blang/semver/v4"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/kong/kubernetes-testing-framework/internal/utils"
	"github.com/kong/kubernetes-testing-framework/pkg/clusters"
	"github.com/kong/kubernetes-testing-framework/pkg/utils/github"
	"github.com/kong/kubernetes-testing-framework/pkg/utils/kubernetes/generators"
)

// -----------------------------------------------------------------------------
// Istio Addon
// -----------------------------------------------------------------------------

const (
	// AddonName is the unique name of the Kong cluster.Addon
	AddonName clusters.AddonName = "istio"

	// Namespace is the namespace that the Addon compontents
	// will be deployed under when deployment finishes
	Namespace = "istio-system"
)

// Addon is a Kong Proxy addon which can be deployed on a clusters.Cluster.
type Addon struct {
	name string

	istioVersion      semver.Version
	istioDeployScript *corev1.ConfigMap
	istioDeployJob    *batchv1.Job

	prometheusEnabled bool
	grafanaEnabled    bool
	jaegerEnabled     bool
	kialiEnabled      bool
}

// New produces a new clusters.Addon for Kong but uses a very opionated set of
// default configurations (see the defaults() function for more details).
// If you need to customize your Kong deployment, use the kong.Builder instead.
func New() *Addon {
	return NewBuilder().Build()
}

// Namespace indicates the namespace where the Istio addon components are to be
// deployed and managed.
func (a *Addon) Namespace() string {
	return Namespace
}

// Version indicates the Istio version for this addon.
func (a *Addon) Version() semver.Version {
	return a.istioVersion
}

// -----------------------------------------------------------------------------
// Istio Addon - Public Methods
// -----------------------------------------------------------------------------

// EnableMeshForNamespace will add the "istio-injection=enabled" label to the provided namespace
// by name which will indicate to Istio to inject sidecard pods to add it to the mesh network.
func (a *Addon) EnableMeshForNamespace(ctx context.Context, cluster clusters.Cluster, name string) error {
	const (
		namspaceWaitTime = time.Second
	)

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context completed while trying to enable mesh for namespace %s: %w", name, ctx.Err())
		default:
			namespace, err := cluster.Client().CoreV1().Namespaces().Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("could not enable mesh for namespace %s: %w", name, err)
			}
			namespace.Labels["istio-injection"] = "enabled"
			_, err = cluster.Client().CoreV1().Namespaces().Update(ctx, namespace, metav1.UpdateOptions{})
			if err != nil {
				if errors.IsConflict(err) {
					// if there's a conflict then an update happened since we pulled the namespace,
					// simply pull and try again.
					select {
					case <-time.After(namspaceWaitTime):
						continue // try again if its not there yet
					case <-ctx.Done():
						continue // this will return an error in the next iteration
					}
				}
				return fmt.Errorf("could not enable mesh for namespace %s: %w", name, err)
			}
			return nil
		}
	}
}

// -----------------------------------------------------------------------------
// Istio Addon - Addon Implementation
// -----------------------------------------------------------------------------

func (a *Addon) Name() clusters.AddonName {
	return AddonName
}

func (a *Addon) Dependencies(_ context.Context, _ clusters.Cluster) []clusters.AddonName {
	return nil
}

func (a *Addon) Deploy(ctx context.Context, cluster clusters.Cluster) error {
	// if an specific version was not provided we'll fetch and use the latest release tag
	if a.istioVersion.String() == "0.0.0" {
		if err := a.useLatestIstioVersion(ctx); err != nil {
			return err
		}
	}

	// generate a configMap deploy script and a job to run it to deploy Istio
	a.istioDeployScript, a.istioDeployJob = generators.GenerateBashJob(
		istioCTLImage,
		a.istioVersion.String(),
		"istioctl x precheck",
		"istioctl install -y",
	)

	// create the configmap script in the admin namespace
	var err error
	a.istioDeployScript, err = cluster.Client().CoreV1().ConfigMaps(utils.AdminNamespace).Create(ctx, a.istioDeployScript, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	// deploy the job which will mount and run the install script. We do this in the admin namespace
	// in order to have cluster-admin access to the cluster for the job's pods.
	a.istioDeployJob, err = cluster.Client().BatchV1().Jobs(utils.AdminNamespace).Create(ctx, a.istioDeployJob, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	const jobWaitTime = time.Second

	// wait for the job to complete
	var deployJobCompleted bool
	for !deployJobCompleted {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context completed while waiting for istio deployment job to finish: %w", ctx.Err())
		default:
			a.istioDeployJob, err = cluster.Client().BatchV1().Jobs(utils.AdminNamespace).Get(ctx, a.istioDeployJob.Name, metav1.GetOptions{})
			if err != nil {
				return err
			}
			if a.istioDeployJob.Status.Succeeded > 0 {
				deployJobCompleted = true
			} else {
				select {
				case <-time.After(jobWaitTime):
					continue
				case <-ctx.Done():
					continue // this will return an error in the next iteration
				}
			}
		}
	}

	// deploy any additional addons or extra components if the caller configured for them
	return a.deployExtras(ctx, cluster)
}

func (a *Addon) Delete(ctx context.Context, cluster clusters.Cluster) error {
	// cleanup the istio install script and job we used to deploy it in the first place.
	if err := cluster.Client().BatchV1().Jobs(utils.AdminNamespace).Delete(ctx, a.istioDeployJob.Name, metav1.DeleteOptions{}); err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
	}
	if err := cluster.Client().CoreV1().ConfigMaps(utils.AdminNamespace).Delete(ctx, a.istioDeployScript.Name, metav1.DeleteOptions{}); err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
	}

	// istio deploys everything it needs to the istio-system namespace, so deletion
	// is simplified by simply deleting that namespace.
	watcher, err := cluster.Client().CoreV1().Namespaces().Watch(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	if err := cluster.Client().CoreV1().Namespaces().Delete(ctx, Namespace, metav1.DeleteOptions{}); err != nil {
		return err
	}

	// wait for the deletion to complete, within the extent of the provided context
	var istioDeletionComplete bool
	for !istioDeletionComplete {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context completed before istio deployment could complete: %w", ctx.Err())
		case event := <-watcher.ResultChan():
			if event.Type == watch.Deleted {
				istioDeletionComplete = true
			}
		}
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
// Istio Addon - Private Vars & Consts
// -----------------------------------------------------------------------------

const (
	// istioCTLImage is the container image name for the istioctl
	// tool which will be used to run istio installation commands.
	//
	// See: https://hub.docker.com/r/istio/istioctl
	istioCTLImage = "istio/istioctl"
)

// -----------------------------------------------------------------------------
// Istio Addon - Private Methods
// -----------------------------------------------------------------------------

// useLatestIstioVersion locates and sets the istio version to deploy to the latest
// non-prelease tag found.
func (a *Addon) useLatestIstioVersion(ctx context.Context) error {
	latestVersion, err := github.FindLatestReleaseForRepo(ctx, "istio", "istio")
	if err != nil {
		return err
	}
	a.istioVersion = *latestVersion
	return nil
}

const (
	// istioAddonTemplate provides a URL template to the manifests for Istio extension components.
	istioAddonTemplate = "https://raw.githubusercontent.com/istio/istio/release-%d.%d/samples/addons/%s.yaml"
)

// deployExtras deploys any additional Prometheus addons or extra components
// requested: for instance Prometheus or other supportive functionality that
// isn't part of the critical Istio deployment path.
func (a *Addon) deployExtras(ctx context.Context, cluster clusters.Cluster) error {
	// generate a temporary kubeconfig since we're going to be using kubectl
	kubeconfig, err := clusters.TempKubeconfig(cluster)
	if err != nil {
		return err
	}
	defer os.Remove(kubeconfig.Name())

	if a.kialiEnabled {
		// kiali needs at least prometheus, grafana and jaeger are optional
		a.prometheusEnabled = true

		// render the URL for Istio Kiali manifests for the current Istio version
		manifestsURL := fmt.Sprintf(istioAddonTemplate, a.istioVersion.Major, a.istioVersion.Minor, "kiali")

		// deploy the Kiali manifests (these will deploy to the istio-system namespace)
		if err := retryKubectlApply(ctx, "--kubeconfig", kubeconfig.Name(), "apply", "-f", manifestsURL); err != nil {
			return err
		}
	}

	if a.jaegerEnabled {
		// render the URL for Istio Jaeger manifests for the current Istio version
		manifestsURL := fmt.Sprintf(istioAddonTemplate, a.istioVersion.Major, a.istioVersion.Minor, "jaeger")

		// deploy the Jaeger manifests (these will deploy to the istio-system namespace)
		if err := retryKubectlApply(ctx, "--kubeconfig", kubeconfig.Name(), "apply", "-f", manifestsURL); err != nil {
			return err
		}
	}

	if a.prometheusEnabled {
		// render the URL for Istio Prometheus manifests for the current Istio version
		manifestsURL := fmt.Sprintf(istioAddonTemplate, a.istioVersion.Major, a.istioVersion.Minor, "prometheus")

		// deploy the Prometheus manifests (these will deploy to the istio-system namespace)
		if err := retryKubectlApply(ctx, "--kubeconfig", kubeconfig.Name(), "apply", "-f", manifestsURL); err != nil {
			return err
		}
	}

	if a.grafanaEnabled {
		// render the URL for Istio Grafana manifests for the current Istio version
		manifestsURL := fmt.Sprintf(istioAddonTemplate, a.istioVersion.Major, a.istioVersion.Minor, "grafana")

		// deploy the Grafana manifests (these will deploy to the istio-system namespace)
		if err := retryKubectlApply(ctx, "--kubeconfig", kubeconfig.Name(), "apply", "-f", manifestsURL); err != nil {
			return err
		}
	}

	return nil
}

const (
	defaultRetries       = 3
	defaultRetryWaitTime = 3 * time.Second
)

// retryKubectlApply retries a command multiple times with a limit, and is particularly
// useful for kubectl commands with older Istio releases where manifests include
// CRDs that have small timing issues that can crop up.
func retryKubectlApply(ctx context.Context, args ...string) (err error) {
	count := defaultRetries
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	for count > 0 {
		cmd := exec.CommandContext(ctx, "kubectl", args...)
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		if err = cmd.Run(); err == nil {
			break
		}
		select {
		case <-ctx.Done():
			continue
		case <-time.After(defaultRetryWaitTime):
			count--
		}
	}

	if err != nil {
		err = fmt.Errorf("kubectl failed: ARGS=(kubectl %v) STDOUT=(%s) STDERR=(%s): %w", args, stdout.String(), stderr.String(), err)
	}

	return
}
