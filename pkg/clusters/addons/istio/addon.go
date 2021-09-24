package istio

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
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
}

// New produces a new clusters.Addon for Kong but uses a very opionated set of
// default configurations (see the defaults() function for more details).
// If you need to customize your Kong deployment, use the kong.Builder instead.
func New() *Addon {
	return NewBuilder().Build()
}

// -----------------------------------------------------------------------------
// Istio Addon - Public Methods
// -----------------------------------------------------------------------------

// EnableMeshForNamespace will add the "istio-injection=enabled" label to the provided namespace
// by name which will indicate to Istio to inject sidecard pods to add it to the mesh network.
func (a *Addon) EnableMeshForNamespace(ctx context.Context, cluster clusters.Cluster, name string) error {
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context completed while trying to enable mesh for namespace %s: %w", name, ctx.Err())
		default:
			namespace, err := cluster.Client().CoreV1().Namespaces().Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("could not enable mesh for namespace %s: %w", name, err)
			}
			namespace.ObjectMeta.Labels["istio-injection"] = "enabled"
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
// Istio Addon - Addon Implementation
// -----------------------------------------------------------------------------

func (a *Addon) Name() clusters.AddonName {
	return AddonName
}

func (a *Addon) Deploy(ctx context.Context, cluster clusters.Cluster) error {
	// if an specific version was not provided we'll fetch and use the latest release tag
	if a.istioVersion.String() == "0.0.0" {
		if err := a.useLatestIstioVersion(); err != nil {
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
				time.Sleep(time.Second)
			}
		}
	}

	return nil
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
	return utils.IsNamespaceReady(ctx, cluster, Namespace)
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

	// istioReleaseURL is the URL at which the lates release will be
	// searched for if a specific version is not supplied for deployment.
	istioReleaseURL = "https://api.github.com/repos/istio/istio/releases/latest"
)

// -----------------------------------------------------------------------------
// Istio Addon - Private Methods
// -----------------------------------------------------------------------------

// useLatestIstioVersion locates and sets the istio version to deploy to the latest
// non-prelease tag found.
func (a *Addon) useLatestIstioVersion() error {
	resp, err := http.Get(istioReleaseURL)
	if err != nil {
		return fmt.Errorf("couldn't determine latest istio release: %w", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	type latestReleaseData struct {
		TagName string `json:"tag_name"`
	}

	data := latestReleaseData{}
	if err := json.Unmarshal(body, &data); err != nil {
		return fmt.Errorf("bad data from api when fetching latest istio release tag: %w", err)
	}

	latestVersion, err := semver.Parse(data.TagName)
	if err != nil {
		return fmt.Errorf("bad release tag returned from api when fetching latest istio release tag: %w", err)
	}
	a.istioVersion = latestVersion

	return nil
}
