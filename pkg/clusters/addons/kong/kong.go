package kong

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"os/exec"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	"github.com/kong/kubernetes-testing-framework/pkg/clusters"
)

// -----------------------------------------------------------------------------
// Kong Addon
// -----------------------------------------------------------------------------

const (
	// AddonName is the unique name of the Kong cluster.Addon
	AddonName clusters.AddonName = "kong"
)

// Addon is a Kong Proxy addon which can be deployed on a clusters.Cluster.
type Addon struct {
	namespace  string
	name       string
	deployArgs []string
	dbmode     DBMode
	proxyOnly  bool
}

// New produces a new clusters.Addon for Kong but uses a very opionated set of
// default configurations (see the defaults() function for more details).
// If you need to customize your Kong deployment, use the kong.Builder instead.
func New() *Addon {
	return NewBuilder().Build()
}

// Namespace indicates the operational namespace of Kong addon components,
// e.g. where the controller and proxy pods live.
func (a *Addon) Namespace() string {
	return a.namespace
}

// ProxyURL provides a routable *url.URL for accessing the Kong proxy.
func (a *Addon) ProxyURL(ctx context.Context, cluster clusters.Cluster) (*url.URL, error) {
	waitForObjects, ready, err := a.Ready(ctx, cluster)
	if err != nil {
		return nil, err
	}

	if !ready {
		return nil, fmt.Errorf("the addon is not ready on cluster %s: non-empty unresolved objects list: %+v", cluster.Name(), waitForObjects)
	}

	return urlForService(ctx, cluster, types.NamespacedName{Namespace: a.namespace, Name: DefaultProxyServiceName}, 80)
}

// ProxyAdminURL provides a routable *url.URL for accessing the Kong Admin API.
func (a *Addon) ProxyAdminURL(ctx context.Context, cluster clusters.Cluster) (*url.URL, error) {
	waitForObjects, ready, err := a.Ready(ctx, cluster)
	if err != nil {
		return nil, err
	}

	if !ready {
		return nil, fmt.Errorf("the addon is not ready on cluster %s, see: %+v", cluster.Name(), waitForObjects)
	}

	return urlForService(ctx, cluster, types.NamespacedName{Namespace: a.namespace, Name: DefaultAdminServiceName}, DefaultAdminServicePort)
}

// ProxyUDPURL provides a routable *url.URL for accessing the default UDP service for the Kong Proxy.
func (a *Addon) ProxyUDPURL(ctx context.Context, cluster clusters.Cluster) (*url.URL, error) {
	waitForObjects, ready, err := a.Ready(ctx, cluster)
	if err != nil {
		return nil, err
	}

	if !ready {
		return nil, fmt.Errorf("the addon is not ready on cluster %s, see: %+v", cluster.Name(), waitForObjects)
	}

	return urlForService(ctx, cluster, types.NamespacedName{Namespace: a.namespace, Name: DefaultUDPServiceName}, DefaultUDPServicePort)
}

// -----------------------------------------------------------------------------
// Kong Addon - Addon Implementation
// -----------------------------------------------------------------------------

func (a *Addon) Name() clusters.AddonName {
	return AddonName
}

func (a *Addon) Deploy(ctx context.Context, cluster clusters.Cluster) error {
	// TODO: derive kubeconfig from cluster object

	stderr := new(bytes.Buffer)
	cmd := exec.CommandContext(ctx, "helm", "repo", "add", "kong", KongHelmRepoURL)
	cmd.Stdout = io.Discard
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", stderr.String(), err)
	}

	stderr = new(bytes.Buffer)
	cmd = exec.CommandContext(ctx, "helm", "repo", "update")
	cmd.Stdout = io.Discard
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", stderr.String(), err)
	}

	if a.dbmode == PostgreSQL {
		a.deployArgs = append(a.deployArgs,
			"--set", "env.database=postgres",
			"--set", "postgresql.enabled=true",
			"--set", "postgresql.postgresqlUsername=kong",
			"--set", "postgresql.postgresqlDatabase=kong",
			"--set", "postgresql.service.port=5432",
		)
	}

	if a.proxyOnly {
		a.deployArgs = append(a.deployArgs,
			"--set", "ingressController.enabled=false",
			"--set", "ingressController.installCRDs=false",
			"--skip-crds",
		)
	}

	args := []string{"install", DefaultDeploymentName, "kong/kong"}
	args = append(args, "--create-namespace", "--namespace", a.namespace)
	args = append(args, a.deployArgs...)
	stderr = new(bytes.Buffer)
	cmd = exec.CommandContext(ctx, "helm", args...) //nolint:gosec
	cmd.Stdout = io.Discard
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		if !strings.Contains(stderr.String(), "cannot re-use") { // ignore if addon is already deployed
			return fmt.Errorf("%s: %w", stderr.String(), err)
		}
	}

	return runUDPServiceHack(ctx, cluster, DefaultNamespace, DefaultDeploymentName)
}

func (a *Addon) Delete(ctx context.Context, cluster clusters.Cluster) error {
	// TODO: derive kubeconfig from cluster object

	stderr := new(bytes.Buffer)
	cmd := exec.Command("helm", "uninstall", DefaultDeploymentName, "--namespace", a.namespace)
	cmd.Stdout = io.Discard
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", stderr.String(), err)
	}

	return nil
}

func (a *Addon) Ready(ctx context.Context, cluster clusters.Cluster) (waitForObjects []runtime.Object, ready bool, err error) {
	var deployments *appsv1.DeploymentList
	deployments, err = cluster.Client().AppsV1().Deployments(a.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return
	}

	for _, deployment := range deployments.Items {
		if deployment.Status.ReadyReplicas != *deployment.Spec.Replicas {
			waitForObjects = append(waitForObjects, &deployment)
		}
	}

	var services *corev1.ServiceList
	services, err = cluster.Client().CoreV1().Services(a.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return
	}

	for _, service := range services.Items {
		if service.Spec.Type == corev1.ServiceTypeLoadBalancer && len(service.Status.LoadBalancer.Ingress) < 1 {
			waitForObjects = append(waitForObjects, &service)
		}
	}

	ready = len(waitForObjects) == 0
	return
}

// -----------------------------------------------------------------------------
// Kong Addon - Private Functions
// -----------------------------------------------------------------------------

// defaults provides a list of opinionated default deployment options for the Kong
// proxy intended to cover the "general use case" and intentionally omitting the
// Kong Kubernetes Ingress Controller (KIC) component with the expectation that the
// caller will be providing their own ingress controller separately.
func defaults() []string {
	return []string{
		// exposing the admin API and enabling raw HTTP for using it is convenient,
		// but again keep in mind this is meant ONLY for testing scenarios and isn't secure.
		"--set", "proxy.http.nodePort=30080",
		"--set", "admin.enabled=true",
		"--set", "admin.http.enabled=true",
		"--set", "admin.http.nodePort=32080",
		"--set", "admin.tls.enabled=false",
		"--set", "tls.enabled=false",
		// this deployment expects a LoadBalancer Service provisioner (such as MetalLB).
		"--set", "proxy.type=LoadBalancer",
		"--set", "admin.type=LoadBalancer",
		// we set up a few default ports for TCP and UDP proxy stream, it's up to
		// test cases to use these how they see fit AND clean up after themselves.
		"--set", "proxy.stream[0].containerPort=8888",
		"--set", "proxy.stream[0].servicePort=8888",
		"--set", "proxy.stream[1].containerPort=9999",
		"--set", "proxy.stream[1].servicePort=9999",
		"--set", "proxy.stream[1].parameters[0]=udp",
		"--set", "proxy.stream[1].parameters[1]=reuseport",
	}
}

// TODO: this is a hack in place to workaround problems in the Kong helm chart when UDP ports are in use:
//       See: https://github.com/Kong/charts/issues/329
func runUDPServiceHack(ctx context.Context, cluster clusters.Cluster, namespace, name string) error {
	udpServicePorts := []corev1.ServicePort{{
		Name:     DefaultUDPServiceName,
		Port:     DefaultUDPServicePort,
		Protocol: corev1.ProtocolUDP,
	}}
	udpService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: DefaultUDPServiceName,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeLoadBalancer,
			Selector: map[string]string{
				"app.kubernetes.io/component": "app",
				"app.kubernetes.io/instance":  DefaultDeploymentName,
				"app.kubernetes.io/name":      "kong",
			},
			Ports: udpServicePorts,
		},
	}
	_, err := cluster.Client().CoreV1().Services(namespace).Create(ctx, udpService, metav1.CreateOptions{})
	if err != nil && strings.Contains(err.Error(), "already exists") { // don't fail if the svc already exists
		err = nil
	}
	return err
}

func urlForService(ctx context.Context, cluster clusters.Cluster, nsn types.NamespacedName, port int) (*url.URL, error) {
	service, err := cluster.Client().CoreV1().Services(nsn.Namespace).Get(ctx, nsn.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	if len(service.Status.LoadBalancer.Ingress) == 1 {
		return url.Parse(fmt.Sprintf("http://%s:%d", service.Status.LoadBalancer.Ingress[0].IP, port))
	}

	return nil, fmt.Errorf("service %s has not yet been provisoned", service.Name)
}
