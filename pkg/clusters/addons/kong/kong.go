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

	// KongLicenseSecretName is the kong license data secret name
	KongLicenseSecretName = "kong-enterprise-license"

	// EnterpriseLicense is the kong enterprise test license
	EnterpriseLicense = "KONG_ENTERPRISE_LICENSE"

	// EnterpriseSuperUseerPwd is the super user password
	EnterpriseSuperUseerPwd = "kong-enterprise-superuser-password"

	// EnterprisePWD is password
	EnterprisePWD = "password"
)

// Addon is a Kong Proxy addon which can be deployed on a clusters.Cluster.
type Addon struct {
	namespace  string
	deployArgs []string
	dbmode     DBMode
	proxyOnly  bool
	enterprise bool
	repo       string
	tag        string
	license    string
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

	if err := clusters.CreateNamespace(ctx, cluster, a.namespace); err != nil {
		return err
	}

	if a.enterprise {
		imageRepo := fmt.Sprintf("image.repository=%s", a.repo)
		imageTag := fmt.Sprintf("image.tag=%s", a.tag)
		a.deployArgs = append(a.deployArgs,
			"--set", imageRepo,
			"--set", imageTag,
		)

		if err := deployKongEnterpriseLicenseSecret(ctx, cluster, a.namespace, KongLicenseSecretName); err != nil {
			return err
		}

		if err := prepareSecrets(ctx, a.namespace); err != nil {
			return err
		}

		enterpriseLicenseSecret := fmt.Sprintf("enterprise.license_secret=%s", a.license)
		postgresspwd := fmt.Sprintf("env.password.valueFrom.secretKeyRef.name=%s", EnterpriseSuperUseerPwd)
		a.deployArgs = append(a.deployArgs,
			"--set", enterpriseLicenseSecret,
			"--set", "enterprise.rbac.enabled=true",
			"--set", "kong_password=password",

			"--set", "admin.type=LoadBalancer",
			"--set", "admin.enabled=true",
			"--set", "admin.http.enabled=true",
			"--set", "admin.tls.enabled=false",
			"--set", "tls.enabled=false",
			"--set", "env.prefix=/kong_prefix/",
			"--set", "admin.tls.enabled=false",
			"--set", "enterprise.enabled=true",
			"--set", enterpriseLicenseSecret,
			"--set", "enterprise.vitals.enabled=true",
			"--set", "enterprise.portal.enabled=true",
			"--set", "enterprise.rbac.enabled=true",
			"--set", "manager.enabled=true",
			"--set", "manager.type=LoadBalancer",
			"--set", "manager.http.enabled=true",
			"--set", "manager.tls.enabled=false",
			"--set", "manager.ingress.enabled=true",
		)

		if a.dbmode == PostgreSQL {
			a.deployArgs = append(a.deployArgs,
				"--set", "env.database=postgres",
				"--set", postgresspwd,
				"--set", "env.password.valueFrom.secretKeyRef.key=password",
			)

		}

	}

	if a.enterprise && a.dbmode == PostgreSQL {
		// do the deployment and installation
		args := []string{"-n", a.namespace, "apply", "-f", "https://raw.githubusercontent.com/Kong/kubernetes-testing-framework/2486cfecf1cac9be7f285f6b077765a75cdc649e/test/integration/enterprise-postgress.yaml"}
		stderr = new(bytes.Buffer)
		cmd = exec.CommandContext(ctx, "kubectl", args...)
		cmd.Stdout = io.Discard
		cmd.Stderr = stderr
		if err := cmd.Run(); err != nil {
			if !strings.Contains(stderr.String(), "cannot re-use") {
				return fmt.Errorf("%s: %w", stderr.String(), err)
			}
		}

		return runUDPServiceHack(ctx, cluster, DefaultNamespace)
	}

	// do the deployment and install the chart
	args := []string{"--kubeconfig", kubeconfig.Name(), "install", DefaultDeploymentName, "kong/kong"}
	args = append(args, "--namespace", a.namespace)
	args = append(args, a.deployArgs...)
	stderr = new(bytes.Buffer)
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
			return url.Parse(fmt.Sprintf("http://%s:%d", service.Status.LoadBalancer.Ingress[0].IP, service.Spec.Ports[0].Port))
		}
	default:
		if service.Spec.ClusterIP != "" {
			return url.Parse(fmt.Sprintf("http://%s:%d", service.Spec.ClusterIP, port))
		}
	}

	return nil, fmt.Errorf("service %s has not yet been provisoned", service.Name)
}

func deployKongEnterpriseLicenseSecret(ctx context.Context, cluster clusters.Cluster, namespace, name string) error {
	license := os.Getenv("KONG_ENTERPRISE_LICENSE")
	if license == "" {
		return fmt.Errorf("failed kong enterprise license from environment setting")
	}

	newSecret := &corev1.Secret{
		Type: corev1.SecretTypeOpaque,
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"license": []byte(license),
		},
	}

	_, err := cluster.Client().CoreV1().Secrets(namespace).Create(ctx, newSecret, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed creating kong-enterprise-license secret, err %v", err)
	}
	fmt.Printf("successfully deploy kong-enterprise-license secret into the cluster.")
	return nil
}

func prepareSecrets(ctx context.Context, namespace string) error {
	stderr := new(bytes.Buffer)
	pwd := fmt.Sprintf("--from-literal=password=%s", EnterprisePWD)
	cmd := exec.CommandContext(ctx, "kubectl", "create", "secret", "generic", EnterpriseSuperUseerPwd, "-n", namespace, pwd)
	cmd.Stdout = io.Discard
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed creating super-admin secret %s: %w", stderr.String(), err)
	}
	fmt.Printf("successfully created super-admin secret.")

	pwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed getting current dir, err %v", err)
	}

	gui_f := pwd + "/admin_gui_session_conf"

	fi, err := os.Create(gui_f)
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := fi.Close(); err != nil {
			panic(err)
		}
	}()

	portal_f := pwd + "/portal_session_conf"
	fp, err := os.Create(portal_f)
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := fp.Close(); err != nil {
			panic(err)
		}
	}()

	err = ioutil.WriteFile(gui_f, []byte(`{"cookie_name":"admin_session","cookie_samesite":"off","secret":"admin-secret-CHANGEME","cookie_secure":true,"storage":"kong"}`), 0644)
	if err != nil {
		return fmt.Errorf("failed writing file admin_gui_session_conf, err %v", err)
	}

	err = ioutil.WriteFile(portal_f, []byte(`{"cookie_name":"portal_session","cookie_samesite":"off","secret":"portal-secret-CHANGEME","cookie_secure":true,"storage":"kong"}`), 0644)
	if err != nil {
		return fmt.Errorf("failed writing file portal_session_conf, err %v", err)
	}

	gui_file := fmt.Sprintf("--from-file=%s", gui_f)
	port_file := fmt.Sprintf("--from-file=%s", portal_f)
	cmd = exec.CommandContext(ctx, "kubectl", "-n", namespace, "create", "secret", "generic", "kong-session-config", gui_file, port_file)
	cmd.Stdout = io.Discard
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed creating kong-session-config secret %s: %w", stderr.String(), err)
	}
	fmt.Printf("successfully created kong-session-config secret.")
	return nil
}
