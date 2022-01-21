package knative

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/kong/kubernetes-testing-framework/pkg/clusters"
	"github.com/kong/kubernetes-testing-framework/pkg/utils/github"
)

// -----------------------------------------------------------------------------
// Knative Addon
// -----------------------------------------------------------------------------

const (
	// AddonName indicates the unique name of this addon.
	AddonName clusters.AddonName = "knative"

	// DefaultNamespace indicates the default namespace this addon will be deployed to.
	DefaultNamespace = "knative-serving"

	// DefaultVersion is the Knative version deployed when the user requests no specific version
	DefaultVersion = "0.0.0"
)

type addon struct {
	version string
}

func New() clusters.Addon {
	return &addon{version: DefaultVersion}
}

// -----------------------------------------------------------------------------
// Knative Addon - Addon Implementation
// -----------------------------------------------------------------------------

func (a *addon) Name() clusters.AddonName {
	return AddonName
}

func (a *addon) Dependencies(_ context.Context, _ clusters.Cluster) []clusters.AddonName {
	return nil
}

func (a *addon) Deploy(ctx context.Context, cluster clusters.Cluster) error {
	if a.version == "0.0.0" {
		if err := a.useLatestKnativeVersion(); err != nil {
			return err
		}
	}
	return deployKnative(ctx, cluster, a.version)
}

func (a *addon) Delete(ctx context.Context, cluster clusters.Cluster) error {
	return deleteKnative(ctx, cluster, a.version)
}

func (a *addon) Ready(ctx context.Context, cluster clusters.Cluster) ([]runtime.Object, bool, error) {
	deploymentList, err := cluster.Client().AppsV1().Deployments(DefaultNamespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, false, err
	}

	var waitingForObjects []runtime.Object
	for i := 0; i < len(deploymentList.Items); i++ {
		deployment := &(deploymentList.Items[i])
		if deployment.Status.AvailableReplicas < *deployment.Spec.Replicas {
			waitingForObjects = append(waitingForObjects, deployment)
		}
	}

	if len(waitingForObjects) > 0 {
		return waitingForObjects, false, nil
	}

	return nil, true, nil
}

// -----------------------------------------------------------------------------
// Private Functions & Vars
// -----------------------------------------------------------------------------

const (
	knativeCRDs = "https://github.com/knative/serving/releases/download/%s/serving-crds.yaml"
	knativeCore = "https://github.com/knative/serving/releases/download/%s/serving-core.yaml"
)

func deployKnative(ctx context.Context, cluster clusters.Cluster, version string) error {
	// generate a temporary kubeconfig since we use kubectl to deploy this addon
	kubeconfig, err := clusters.TempKubeconfig(cluster)
	if err != nil {
		return err
	}
	defer os.Remove(kubeconfig.Name())

	// apply the CRDs: we wait here as this avoids any subsecond timing issues
	cmd := exec.CommandContext(ctx, "kubectl", "--kubeconfig", kubeconfig.Name(), "apply", "--wait", "-f", fmt.Sprintf(knativeCRDs, version)) //nolint:gosec
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("knative CRD deployment failed STDOUT=(%s) STDERR=(%s): %w", stdout.String(), stderr.String(), err)
	}

	// apply the core deployments, but don't wait because we're going to patch them
	cmd = exec.CommandContext(ctx, "kubectl", "--kubeconfig", kubeconfig.Name(), "apply", "-f", fmt.Sprintf(knativeCore, version)) //nolint:gosec
	stdout, stderr = new(bytes.Buffer), new(bytes.Buffer)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("knative core deployment failed STDOUT=(%s) STDERR=(%s): %w", stdout.String(), stderr.String(), err)
	}

	// the deployment manifests for knative include some CPU and Memory limits which
	// are good for production, but mostly just problematic when running simple tests
	// where these components are going to be brought up and torn down quickly.
	// we tear out these requirements ad as long as the pods start we will likely have
	// all the CPU and memory we need to complete tests.
	for {
		select {
		case <-ctx.Done():
			if err := ctx.Err(); err != nil {
				return fmt.Errorf("context completed with error while waiting to patch knative deployments: %w", err)
			}
			return fmt.Errorf("context completed while trying to deploy knative components")
		default:
			// list all the deployments in the namespace to make patches to them
			deploymentList, err := cluster.Client().AppsV1().Deployments(DefaultNamespace).List(ctx, metav1.ListOptions{})
			if err != nil {
				return fmt.Errorf("could not change knative deployment resource quotas: %w", err)
			}

			// iterate through each deployment in the list, attempting updates as needed
			retryNeededDueToConflict := false
			for i := 0; i < len(deploymentList.Items); i++ {
				// check whether the containers have any resource requirements associated with them
				deploymentNeedsUpdate := false
				deployment := deploymentList.Items[i]
				for j := 0; j < len(deployment.Spec.Template.Spec.Containers); j++ {
					if len(deployment.Spec.Template.Spec.Containers[j].Resources.Limits) > 0 ||
						len(deployment.Spec.Template.Spec.Containers[j].Resources.Requests) > 0 {
						// there are resources requirements on this container, patch the deployment
						deployment.Spec.Template.Spec.Containers[j].Resources = corev1.ResourceRequirements{}
						deploymentNeedsUpdate = true
					}
				}

				// don't need to bother updating deployments that don't have resource requirements on their containers
				if !deploymentNeedsUpdate {
					continue
				}

				// run the update to remove any resource requirements for deployments in the namespace
				_, err := cluster.Client().AppsV1().Deployments(DefaultNamespace).Update(ctx, &deployment, metav1.UpdateOptions{})
				if err != nil {
					if errors.IsConflict(err) { // retry on conflict, need a fresh copy
						retryNeededDueToConflict = true
						continue
					}
					return fmt.Errorf("couldn't update resource quotas for knative deployment %s: %w", deployment.Name, err)
				}
			}

			// if any deployments didn't get a full update because the object was out of date, retry
			if retryNeededDueToConflict {
				continue
			}

			return nil // patches complete, all set
		}
	}
}

func deleteKnative(ctx context.Context, cluster clusters.Cluster, version string) error {
	// generate a temporary kubeconfig since we use kubectl to cleanup this addon
	kubeconfig, err := clusters.TempKubeconfig(cluster)
	if err != nil {
		return err
	}
	defer os.Remove(kubeconfig.Name())

	// cleanup the core deployments, waiting for all components to tear down
	cmd := exec.CommandContext(ctx, "kubectl", "--kubeconfig", kubeconfig.Name(), "delete", "--wait", "-f", fmt.Sprintf(knativeCore, version)) //nolint:gosec
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		if !strings.Contains(stderr.String(), "NotFound") { // tolerate previous cleanup to make cleanup idempotent
			return fmt.Errorf("knative core cleanup failed STDOUT=(%s) STDERR=(%s): %w", stdout.String(), stderr.String(), err)
		}
	}

	// cleanup the CRDs, wait for them to be removed
	cmd = exec.CommandContext(ctx, "kubectl", "--kubeconfig", kubeconfig.Name(), "delete", "--wait", "-f", fmt.Sprintf(knativeCRDs, version)) //nolint:gosec
	stdout, stderr = new(bytes.Buffer), new(bytes.Buffer)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		if !strings.Contains(stderr.String(), "NotFound") { // tolerate previous cleanup to make cleanup idempotent
			return fmt.Errorf("knative CRD deployment failed STDOUT=(%s) STDERR=(%s): %w", stdout.String(), stderr.String(), err)
		}
	}

	// wait for the namespace to tear down
	for {
		select {
		case <-ctx.Done():
			if err := ctx.Err(); err != nil {
				return fmt.Errorf("context completed with error while waiting for knative namespace %s to cleanup: %w", DefaultNamespace, err)
			}
			return fmt.Errorf("context completed while waiting for knative namespace %s to cleanup", DefaultNamespace)
		default:
			if err := cluster.Client().CoreV1().Namespaces().Delete(ctx, DefaultNamespace, metav1.DeleteOptions{}); err != nil {
				if errors.IsNotFound(err) {
					return nil
				}
				return err
			}
			time.Sleep(time.Second)
		}
	}
}

// useLatestKnativeVersion locates and sets the knative version to deploy to the latest
// non-prelease tag found.
func (a *addon) useLatestKnativeVersion() error {
	latestVersion, err := github.FindRawLatestReleaseForRepo("knative", "serving")
	if err != nil {
		return err
	}
	a.version = latestVersion
	return nil
}
