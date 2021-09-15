package kong

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"os/exec"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	"github.com/kong/kubernetes-testing-framework/internal/utils"
	"github.com/kong/kubernetes-testing-framework/pkg/clusters"
)

// -----------------------------------------------------------------------------
// Kong Addon
// -----------------------------------------------------------------------------

const (
	// AddonName is the unique name of the Kong cluster.Addon
	AddonName clusters.AddonName = "kong"

	httpPort = 80

	// DefaultEnterpriseImageRepo default kong enterprise image
	DefaultEnterpriseImageRepo = "kong/kong-gateway"

	// DefaultEnterpriseImageTag latest kong enterprise image tag
	DefaultEnterpriseImageTag = "2.5.0.0-alpine"

	// KongEnterpriseLicense is the kong license data secret name
	KongEnterpriseLicense = "kong-enterprise-license"

	// EnterpriseAdminPasswordSecretName is the kong admin seed password
	EnterpriseAdminPasswordSecretName = "kong-enterprise-superuser-password"
)

// Addon is a Kong Proxy addon which can be deployed on a clusters.Cluster.
type Addon struct {
	namespace                    string
	deployArgs                   []string
	dbmode                       DBMode
	proxyOnly                    bool
	enterprise                   bool
	proxyImage                   string
	proxyImageTag                string
	enterpriseLicenseJSONString  string
	kongAdminPassword            string
	adminServiceTypeLoadBalancer bool
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

	return urlForService(ctx, cluster, types.NamespacedName{Namespace: a.namespace, Name: DefaultProxyServiceName}, httpPort)
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
	// generate a temporary kubeconfig since we're going to be using the helm CLI
	kubeconfig, err := utils.TempKubeconfig(cluster)
	if err != nil {
		return err
	}
	defer os.Remove(kubeconfig.Name())

	// ensure the repo exists
	stderr := new(bytes.Buffer)
	cmd := exec.CommandContext(ctx, "helm", "--kubeconfig", kubeconfig.Name(), "repo", "add", "--force-update", "kong", KongHelmRepoURL) //nolint:gosec
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

	// configure for dbmode options
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

	if a.adminServiceTypeLoadBalancer {
		a.deployArgs = append(a.deployArgs, "--set", "admin.type=LoadBalancer")
	}

	if err := clusters.CreateNamespace(ctx, cluster, a.namespace); err != nil {
		return err
	}

	// do the deployment and install the chart
	args := []string{"--kubeconfig", kubeconfig.Name(), "install", DefaultDeploymentName, "kong/kong"}
	args = append(args, "--namespace", a.namespace)
	args = append(args, a.deployArgs...)
	if a.enterprise {

		if a.enterpriseLicenseJSONString == "" {
			fmt.Printf("apply license json from environment variable")
			a.enterpriseLicenseJSONString = os.Getenv("KONG_ENTERPRISE_LICENSE")
			if a.enterpriseLicenseJSONString == "" {
				return fmt.Errorf("license json should not be empty")
			}
		}

		if err := deployKongEnterpriseLicenseSecret(ctx, cluster, a.namespace, KongEnterpriseLicense, a.enterpriseLicenseJSONString); err != nil {
			return err
		}

		if a.kongAdminPassword == "" {
			return fmt.Errorf("kong admin password should not be empty")
		}
		if err := prepareSecrets(ctx, a.namespace, a.kongAdminPassword); err != nil {
			return err
		}

		args = append(args, "--version", "2.3.0", "-f", "https://raw.githubusercontent.com/Kong/charts/main/charts/kong/example-values/minimal-k4k8s-with-kong-enterprise.yaml")
		license := fmt.Sprintf("enterprise.license_secret=%s", KongEnterpriseLicense)
		password := fmt.Sprintf("env.kong_password=%s", a.kongAdminPassword)
		args = append(args,
			"--set", "admin.annotations.konghq.com/protocol=http",
			"--set", "enterprise.rbac.enabled=true",
			"--set", "env.enforce_rbac=on",
			"--set", password,
			"--set", license,
			// expose new ports
			"--set", "proxy.stream[0].containerPort=8888",
			"--set", "proxy.stream[0].servicePort=8888",
			"--set", "proxy.stream[1].containerPort=9999",
			"--set", "proxy.stream[1].servicePort=9999",
			"--set", "proxy.stream[1].parameters[0]=udp",
			"--set", "proxy.stream[1].parameters[1]=reuseport")
	}

	stderr = new(bytes.Buffer)
	fmt.Printf("deployment args %s", args)
	cmd = exec.CommandContext(ctx, "helm", args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		if !strings.Contains(stderr.String(), "cannot re-use") { // ignore if addon is already deployed
			return fmt.Errorf("%s: %w", stderr.String(), err)
		}
	}

	// run any other cleanup jobs or ancillary tasks
	return runUDPServiceHack(ctx, cluster, DefaultNamespace)
}

func (a *Addon) Delete(ctx context.Context, cluster clusters.Cluster) error {
	// generate a temporary kubeconfig since we're going to be using the helm CLI
	kubeconfig, err := utils.TempKubeconfig(cluster)
	if err != nil {
		return err
	}
	defer os.Remove(kubeconfig.Name())

	// delete the chart release from the cluster
	stderr := new(bytes.Buffer)
	cmd := exec.Command("helm", "--kubeconfig", kubeconfig.Name(), "uninstall", DefaultDeploymentName, "--namespace", a.namespace) //nolint:gosec
	cmd.Stdout = io.Discard
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", stderr.String(), err)
	}

	if a.enterpriseLicenseJSONString != "" {
		stderr := new(bytes.Buffer)
		cmd = exec.Command("kubectl", "delete", "secret", KongEnterpriseLicense, "--namespace", a.namespace) //nolint:gosec
		cmd.Stdout = io.Discard
		cmd.Stderr = stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("delete secret err msg %s: %w", stderr.String(), err)
		}
	}

	return nil
}

func (a *Addon) Ready(ctx context.Context, cluster clusters.Cluster) (waitForObjects []runtime.Object, ready bool, err error) {
	var deployments *appsv1.DeploymentList
	deployments, err = cluster.Client().AppsV1().Deployments(a.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return
	}

	for i := 0; i < len(deployments.Items); i++ {
		deployment := &(deployments.Items[i])
		if deployment.Status.ReadyReplicas != *deployment.Spec.Replicas {
			waitForObjects = append(waitForObjects, deployment)
		}
	}

	var services *corev1.ServiceList
	services, err = cluster.Client().CoreV1().Services(a.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return
	}

	for i := 0; i < len(services.Items); i++ {
		service := &(services.Items[i])
		if service.Spec.Type == corev1.ServiceTypeLoadBalancer && len(service.Status.LoadBalancer.Ingress) < 1 {
			waitForObjects = append(waitForObjects, service)
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
		// we configure a cluster network exposed admin API over HTTP with no auth for testing convenience,
		// but again keep in mind this is meant ONLY for transient testing scenarios and isn't secure.
		"--set", "proxy.http.nodePort=30080",
		"--set", "admin.enabled=true",
		"--set", "admin.http.enabled=true",
		"--set", "admin.http.nodePort=32080",
		"--set", "admin.tls.enabled=false",
		"--set", "admin.type=ClusterIP",
		"--set", "tls.enabled=false",
		// we set up a few default ports for TCP and UDP proxy stream, it's up to
		// test cases to use these how they see fit AND clean up after themselves.
		"--set", "proxy.stream[0].containerPort=8888",
		"--set", "proxy.stream[0].servicePort=8888",
		"--set", "proxy.stream[1].containerPort=9999",
		"--set", "proxy.stream[1].servicePort=9999",
		"--set", "proxy.stream[1].parameters[0]=udp",
		"--set", "proxy.stream[1].parameters[1]=reuseport",
		// the proxy expects a LoadBalancer Service provisioner (such as MetalLB).
		"--set", "proxy.type=LoadBalancer",
	}
}

// TODO: this is a hack in place to workaround problems in the Kong helm chart when UDP ports are in use:
//       See: https://github.com/Kong/charts/issues/329
func runUDPServiceHack(ctx context.Context, cluster clusters.Cluster, namespace string) error {
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

	//nolint:exhaustive
	switch service.Spec.Type {
	case corev1.ServiceTypeLoadBalancer:
		if len(service.Status.LoadBalancer.Ingress) == 1 {
			return url.Parse(fmt.Sprintf("http://%s:%d", service.Status.LoadBalancer.Ingress[0].IP, port))
		}
	default:
		if service.Spec.ClusterIP != "" {
			return url.Parse(fmt.Sprintf("http://%s:%d", service.Spec.ClusterIP, port))
		}
	}

	return nil, fmt.Errorf("service %s has not yet been provisoned", service.Name)
}

// deployKongEnterpriseLicenseSecret deploy secret using license json data
func deployKongEnterpriseLicenseSecret(ctx context.Context, cluster clusters.Cluster, namespace, name, licenseJSON string) error {
	newSecret := &corev1.Secret{
		Type: corev1.SecretTypeOpaque,
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"license": []byte(licenseJSON),
		},
	}

	_, err := cluster.Client().CoreV1().Secrets(namespace).Create(ctx, newSecret, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed creating kong-enterprise-license secret, err %w", err)
	}

	fmt.Printf("successfully deployed kong-enterprise-license into the cluster .")
	return nil
}

func prepareSecrets(ctx context.Context, namespace, password string) error {
	stderr := new(bytes.Buffer)
	pwd := fmt.Sprintf("--from-literal=password=%s", password)
	cmd := exec.CommandContext(ctx, "kubectl", "create", "secret", "generic", EnterpriseAdminPasswordSecretName, "-n", namespace, pwd)
	cmd.Stdout = io.Discard
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed creating super-admin secret %s: %w", stderr.String(), err)
	}
	fmt.Printf("successfully created kong admin secret.")

	pwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed getting current dir, err %w", err)
	}

	guiF := pwd + "/secret_conf"

	fi, err := os.Create(guiF)
	if err != nil {
		return err
	}
	defer func() {
		if err := fi.Close(); err != nil {
			fmt.Println("failed closing file %w", err)
			return
		}
	}()

	err = ioutil.WriteFile(guiF, []byte(`{"cookie_name":"04tm34l","secret":"change-this-secret","cookie_secure":false,"storage":"kong"}`), 0600)
	if err != nil {
		return fmt.Errorf("failed writing file admin_gui_session_conf, err %v", err)
	}

	guiFile := fmt.Sprintf("--from-file=admin_gui_session_conf=%s", guiF)
	portFile := fmt.Sprintf("--from-file=portal_session_conf=%s", guiF)
	cmd = exec.CommandContext(ctx, "kubectl", "-n", namespace, "create", "secret", "generic", "kong-session-config", guiFile, portFile)
	cmd.Stdout = io.Discard
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed creating kong-session-config secret %s: %w", stderr.String(), err)
	}
	return nil
}
