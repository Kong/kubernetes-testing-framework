package kongargo

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/kong/kubernetes-testing-framework/pkg/clusters"
	"github.com/kong/kubernetes-testing-framework/pkg/clusters/addons/argocd"
)

const (
	// AddonName indicates the unique name of this addon.
	AddonName clusters.AddonName = "kong-argo"

	// DefaultNamespace indicates the default namespace this addon will be deployed to.
	DefaultNamespace = "kong-argo"

	defaultProject = "ktf-kong"

	defaultVersion = "2.25.0"

	defaultRelease = "ktf-argo"

	defaultAppName = "kong"
)

type Addon struct {
	name      string
	namespace string
	version   string
	project   string
	release   string
	appName   string
}

func New() clusters.Addon {
	return &Addon{}
}

func (a *Addon) Name() clusters.AddonName {
	return AddonName
}

func (a *Addon) Dependencies(_ context.Context, _ clusters.Cluster) []clusters.AddonName {
	return []clusters.AddonName{
		argocd.AddonName,
	}
}

func getArgo(cluster clusters.Cluster) (*argocd.Addon, error) {
	addon, err := cluster.GetAddon(argocd.AddonName)
	if err != nil {
		return nil, err
	}
	argo, ok := addon.(*argocd.Addon)
	if !ok {
		return nil, fmt.Errorf("argocd addon is not actually an argocd addon")
	}
	return argo, nil
}

func (a *Addon) Deploy(ctx context.Context, cluster clusters.Cluster) error {
	// wait for dependency addons to be ready first
	if err := clusters.WaitForAddonDependencies(ctx, cluster, a); err != nil {
		return fmt.Errorf("failure waiting for addon dependencies: %w", err)
	}

	ns := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: a.namespace}}
	if _, err := cluster.Client().CoreV1().Namespaces().Create(ctx, &ns, metav1.CreateOptions{}); err != nil {
		if !errors.IsAlreadyExists(err) {
			return fmt.Errorf("could not create Kong namespace: %w", err)
		}
	}

	argo, err := getArgo(cluster)
	if err != nil {
		return fmt.Errorf("could not get ArgoCD instance: %w", err)
	}
	err = argo.CreateApplication(ctx, a.getUnstructuredApplication())
	if err != nil {
		return fmt.Errorf("could not create Application: %w", err)
	}
	err = argo.CreateAppProject(ctx, a.getUnstructuredAppProject())
	if err != nil {
		return fmt.Errorf("could not create AppProject: %w", err)
	}

	return nil
}

func (a *Addon) Ready(ctx context.Context, cluster clusters.Cluster) ([]runtime.Object, bool, error) {
	deployment, err := cluster.Client().AppsV1().Deployments(a.namespace).
		Get(ctx, a.release+"-kong", metav1.GetOptions{})
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

func (a *Addon) Delete(ctx context.Context, cluster clusters.Cluster) error {
	argo, err := getArgo(cluster)
	if err != nil {
		return fmt.Errorf("could not get ArgoCD instance: %w", err)
	}
	err = argo.DeleteApplication(ctx, a.appName)
	if err != nil {
		return fmt.Errorf("could not delete Application: %w", err)
	}
	err = argo.DeleteAppProject(ctx, a.project)
	if err != nil {
		return fmt.Errorf("could not delete AppProject: %w", err)
	}
	return nil
}

func (a *Addon) DumpDiagnostics(context.Context, clusters.Cluster) (map[string][]byte, error) {
	diagnostics := make(map[string][]byte)
	return diagnostics, nil
}

func (a *Addon) getUnstructuredApplication() *unstructured.Unstructured {
	app := &unstructured.Unstructured{}
	app.SetUnstructuredContent(map[string]interface{}{
		"apiVersion": "argoproj.io/v1alpha1",
		"kind":       "Application",
		"metadata": map[string]interface{}{
			"name":       a.appName,
			"finalizers": []string{"resources-finalizer.argocd.argoproj.io"},
		},
		"spec": map[string]interface{}{
			"project": a.project,
			"syncPolicy": map[string]interface{}{
				"automated": map[string]interface{}{},
			},
			"source": map[string]interface{}{
				"chart":          "kong",
				"repoURL":        "https://charts.konghq.com",
				"targetRevision": a.version,
				"helm": map[string]interface{}{
					"releaseName": a.release,
				},
			},
			"destination": map[string]interface{}{
				"server":    argocd.DefaultServer,
				"namespace": a.namespace,
			},
		},
	})
	return app
}

func (a *Addon) getUnstructuredAppProject() *unstructured.Unstructured {
	proj := &unstructured.Unstructured{}
	proj.SetUnstructuredContent(map[string]interface{}{
		"apiVersion": "argoproj.io/v1alpha1",
		"kind":       "AppProject",
		"metadata": map[string]interface{}{
			"name":       a.project,
			"finalizers": []string{"resources-finalizer.argocd.argoproj.io"},
		},
		"spec": map[string]interface{}{
			"description": "KTF Argo Kong Project",
			"sourceRepos": []string{"*"},
			"destinations": []map[string]interface{}{
				{
					"namespace": a.namespace,
					"server":    argocd.DefaultServer,
				},
			},
			"clusterResourceWhitelist": []map[string]interface{}{
				{
					"group": "",
					"kind":  "Namespace",
				},
				{
					"group": "apiextensions.k8s.io",
					"kind":  "CustomResourceDefinition",
				},
				{
					"group": "networking.k8s.io",
					"kind":  "IngressClass",
				},
				{
					"group": "admissionregistration.k8s.io",
					"kind":  "ValidatingWebhookConfiguration",
				},
				{
					"group": "rbac.authorization.k8s.io",
					"kind":  "ClusterRoleBinding",
				},
				{
					"group": "rbac.authorization.k8s.io",
					"kind":  "ClusterRole",
				},
			},
			"namespaceResourceBlacklist": []map[string]interface{}{
				{
					"group": "",
					"kind":  "ResourceQuota",
				},
				{
					"group": "",
					"kind":  "LimitRange",
				},
				{
					"group": "",
					"kind":  "NetworkPolicy",
				},
			},
		},
	})
	return proj
}
