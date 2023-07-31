package argocd

import (
	"context"
	"fmt"
	"io"
	"os"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"github.com/kong/kubernetes-testing-framework/internal/retry"
	"github.com/kong/kubernetes-testing-framework/pkg/clusters"
)

const (
	// AddonName indicates the unique name of this addon.
	AddonName clusters.AddonName = "argocd"

	// DefaultNamespace indicates the default namespace this addon will be deployed to.
	DefaultNamespace = "argocd"

	canaryDeployment = "argocd-repo-server"

	manifestURL = "https://raw.githubusercontent.com/argoproj/argo-cd/%s/manifests/core-install.yaml"

	// DefaultServer is the default server value for Applications and AppProjects.
	DefaultServer = "https://kubernetes.default.svc"
)

type Addon struct {
	name      string
	namespace string
	version   string
	client    *dynamic.DynamicClient
}

func New() clusters.Addon {
	return &Addon{}
}

// importing the argocd package requires replacing a bunch of Kubernetes libs for reasons:
// https://argo-cd.readthedocs.io/en/stable/user-guide/import/
// unfortunately, the library versions are ancient and not compatible with what KTF wants, so we need to use
// unstructured types

func applicationGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "argoproj.io",
		Version:  "v1alpha1",
		Resource: "applications",
	}
}

// CreateApplication takes an (unstructured) Application and creates it.
func (a *Addon) CreateApplication(ctx context.Context, app *unstructured.Unstructured) error {
	applicationClient := a.client.Resource(applicationGVR()).Namespace(a.namespace)

	_, err := applicationClient.Create(ctx, app, metav1.CreateOptions{})
	return err
}

// DeleteApplication takes an Application name and deletes it.
func (a *Addon) DeleteApplication(ctx context.Context, proj string) error {
	client := a.client.Resource(applicationGVR()).Namespace(a.namespace)
	return client.Delete(ctx, proj, metav1.DeleteOptions{})
}

func appProjectGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "argoproj.io",
		Version:  "v1alpha1",
		Resource: "appprojects",
	}
}

// CreateAppProject takes an (unstructured) AppProject and creates it.
func (a *Addon) CreateAppProject(ctx context.Context, proj *unstructured.Unstructured) error {
	projectClient := a.client.Resource(appProjectGVR()).Namespace(a.namespace)
	_, err := projectClient.Create(ctx, proj, metav1.CreateOptions{})
	return err
}

// DeleteAppProject takes an AppProject name and deletes it.
func (a *Addon) DeleteAppProject(ctx context.Context, proj string) error {
	projectClient := a.client.Resource(appProjectGVR()).Namespace(a.namespace)
	return projectClient.Delete(ctx, proj, metav1.DeleteOptions{})
}

// -----------------------------------------------------------------------------
// ArgoCD Addon - Addon Implementation
// -----------------------------------------------------------------------------

func (a *Addon) Name() clusters.AddonName {
	return AddonName
}

func (a *Addon) Dependencies(_ context.Context, _ clusters.Cluster) []clusters.AddonName {
	return nil
}

func (a *Addon) Deploy(ctx context.Context, cluster clusters.Cluster) error {
	ns := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: a.namespace}}
	if _, err := cluster.Client().CoreV1().Namespaces().Create(ctx, &ns, metav1.CreateOptions{}); err != nil {
		if !errors.IsAlreadyExists(err) {
			return fmt.Errorf("could not create ArgoCD namespace: %w", err)
		}
	}

	kubeconfig, err := clusters.TempKubeconfig(cluster)
	if err != nil {
		return fmt.Errorf("could not get cluster config: %w", err)
	}

	defer os.Remove(kubeconfig.Name())

	deployArgs := []string{
		"--kubeconfig", kubeconfig.Name(),
		"apply", "-n", a.namespace, "-f", fmt.Sprintf(manifestURL, a.version),
	}

	if err := retry.Command("kubectl", deployArgs...).WithStdout(io.Discard).Do(ctx); err != nil {
		return fmt.Errorf("could not deploy ArgoCD: %w", err)
	}

	dynamicClient, err := dynamic.NewForConfig(cluster.Config())
	if err != nil {
		return fmt.Errorf("could not create client: %w", err)
	}
	a.client = dynamicClient

	return nil
}

func (a *Addon) Delete(ctx context.Context, cluster clusters.Cluster) error {
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context completed before addon could be deleted: %w", ctx.Err())
		default:
			if err := cluster.Client().CoreV1().Namespaces().
				Delete(ctx, a.namespace, metav1.DeleteOptions{}); err != nil {
				if errors.IsNotFound(err) {
					return nil
				}
				return fmt.Errorf("could not delete namespace: %w", err)
			}
		}
	}
}

func (a *Addon) Ready(ctx context.Context, cluster clusters.Cluster) ([]runtime.Object, bool, error) {
	deployment, err := cluster.Client().AppsV1().Deployments(a.namespace).
		Get(ctx, canaryDeployment, metav1.GetOptions{})
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

func (a *Addon) DumpDiagnostics(context.Context, clusters.Cluster) (map[string][]byte, error) {
	diagnostics := make(map[string][]byte)
	return diagnostics, nil
}
