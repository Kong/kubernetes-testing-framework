package kong

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"strings"

	pwgen "github.com/sethvargo/go-password/password"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	"github.com/kong/kubernetes-testing-framework/internal/utils"
	"github.com/kong/kubernetes-testing-framework/pkg/clusters"
	"github.com/kong/kubernetes-testing-framework/pkg/clusters/addons/metallb"
	"github.com/kong/kubernetes-testing-framework/pkg/clusters/types/kind"
)

// -----------------------------------------------------------------------------
// Kong Addon
// -----------------------------------------------------------------------------

const (
	// AddonName is the unique name of the Kong cluster.Addon
	AddonName clusters.AddonName = "kong"

	// DefaultEnterpriseImageRepo default kong enterprise image
	DefaultEnterpriseImageRepo = "kong/kong-gateway"

	// DefaultEnterpriseImageTag latest kong enterprise image tag
	DefaultEnterpriseImageTag = "2.7.0.0-alpine"

	// DefaultEnterpriseLicenseSecretName is the name that will be used by default for the
	// Kubernetes secret containing the Kong enterprise license that will be
	// deployed when enterprise mode is enabled.
	DefaultEnterpriseLicenseSecretName = "kong-enterprise-license"

	// DefaultEnterpriseAdminPasswordSecretName is the secret name that will be used
	// by default for the Kubernetes secret that will be deployed containing the
	// superuser admin password in enterprise mode.
	DefaultEnterpriseAdminPasswordSecretName = "kong-enterprise-superuser-password"

	// DefaultAdminGUISessionConfSecretName is the secret name that will be used by
	// default for the Kbuernetes secret that will be deployed containing the
	// session configuration for the Kong Admin GUI in enterprise mode.
	DefaultAdminGUISessionConfSecretName = "kong-session-config"
)

// Addon is a Kong Proxy addon which can be deployed on a clusters.Cluster.
type Addon struct {
	logger *logrus.Logger

	// kubernetes and helm chart related configuration options
	namespace  string
	name       string
	deployArgs []string

	// ingress controller configuration options
	ingressControllerDisabled bool

	// proxy server general configuration options
	proxyAdminServiceTypeLoadBalancer bool
	proxyDBMode                       DBMode
	proxyImage                        string
	proxyImageTag                     string

	// proxy server enterprise mode configuration options
	proxyEnterpriseEnabled            bool
	proxyEnterpriseSuperAdminPassword string
	proxyEnterpriseLicenseJSON        string
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

// -----------------------------------------------------------------------------
// Kong Addon - Proxy Endpoint Methods
// -----------------------------------------------------------------------------

const defaultHTTPPort = 80

// ProxyURL provides a routable *url.URL for accessing the Kong proxy.
func (a *Addon) ProxyURL(ctx context.Context, cluster clusters.Cluster) (*url.URL, error) {
	waitForObjects, ready, err := a.Ready(ctx, cluster)
	if err != nil {
		return nil, err
	}

	if !ready {
		return nil, fmt.Errorf("the addon is not ready on cluster %s: non-empty unresolved objects list: %+v", cluster.Name(), waitForObjects)
	}

	return urlForService(ctx, cluster, types.NamespacedName{Namespace: a.namespace, Name: DefaultProxyServiceName}, defaultHTTPPort)
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

func (a *Addon) Dependencies(ctx context.Context, cluster clusters.Cluster) []clusters.AddonName {
	if _, ok := cluster.(*kind.Cluster); ok {
		if a.proxyAdminServiceTypeLoadBalancer {
			return []clusters.AddonName{
				metallb.AddonName,
			}
		}
	}
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

	// if the dbmode is postgres, set several related values
	args := []string{"--kubeconfig", kubeconfig.Name(), "install", DefaultDeploymentName, "kong/kong"}
	if a.proxyDBMode == PostgreSQL {
		a.deployArgs = append(a.deployArgs, []string{
			"--set", "env.database=postgres",
			"--set", "postgresql.enabled=true",
			"--set", "postgresql.postgresqlUsername=kong",
			"--set", "postgresql.postgresqlDatabase=kong",
			"--set", "postgresql.service.port=5432",
		}...)
	}

	// if the ingress controller is disabled flag it in the chart and don't install any CRDs
	if a.ingressControllerDisabled {
		a.deployArgs = append(a.deployArgs, []string{
			"--set", "ingressController.enabled=false",
			"--set", "ingressController.installCRDs=false",
			"--skip-crds",
		}...)
	}

	// set the container image values if provided by the caller
	if a.proxyImage != "" {
		a.deployArgs = append(a.deployArgs, []string{"--set", fmt.Sprintf("image.repository=%s", a.proxyImage)}...)
	}
	if a.proxyImageTag != "" {
		a.deployArgs = append(a.deployArgs, []string{"--set", fmt.Sprintf("image.tag=%s", a.proxyImageTag)}...)
	}

	// set the service type of the proxy admin's Kubernetes service
	if a.proxyAdminServiceTypeLoadBalancer {
		a.deployArgs = append(a.deployArgs, []string{"--set", "admin.type=LoadBalancer"}...)
	} else {
		a.deployArgs = append(a.deployArgs, []string{"--set", "admin.type=ClusterIP"}...)
	}

	// create a namespace ahead of deployment so things like license secrets and other configurations
	// can be preloaded.
	if err := clusters.CreateNamespace(ctx, cluster, a.namespace); err != nil {
		return err
	}

	// deploy licenses and other configurations for enterprise mode
	if a.proxyEnterpriseEnabled {
		// deploy the license as a Kubernetes secret to enable enterprise features for the proxy
		if err := deployKongEnterpriseLicenseSecret(ctx, cluster, a.namespace, DefaultEnterpriseLicenseSecretName, a.proxyEnterpriseLicenseJSON); err != nil {
			return err
		}

		// deploy the superadmin password as a Kubernetes secret adjacent to the proxy pod and configure
		// the chart to use that secret to configure controller auth to the admin API.
		a.proxyEnterpriseSuperAdminPassword, err = deployEnterpriseSuperAdminPasswordSecret(ctx, cluster, a.namespace, a.proxyEnterpriseSuperAdminPassword)
		if err != nil {
			return err
		}
		if !a.ingressControllerDisabled {
			a.deployArgs = append(a.deployArgs, []string{"--set", fmt.Sprintf("ingressController.env.kong_admin_token.valueFrom.secretKeyRef.name=%s", DefaultEnterpriseAdminPasswordSecretName)}...)
			a.deployArgs = append(a.deployArgs, []string{"--set", "ingressController.env.kong_admin_token.valueFrom.secretKeyRef.key=password"}...)
		}
		a.deployArgs = append(a.deployArgs, []string{"--set", fmt.Sprintf("env.password.valueFrom.secretKeyRef.name=%s", DefaultEnterpriseAdminPasswordSecretName)}...)
		a.deployArgs = append(a.deployArgs, []string{"--set", "env.password.valueFrom.secretKeyRef.key=password"}...)

		// deploy the admin session configuration needed for enterprise enabled mode
		if err := deployKongEnterpriseAdminGUISessionConf(ctx, cluster, a.namespace); err != nil {
			return err
		}

		// set the enterprise defaults helm installation values
		a.deployArgs = append(a.deployArgs, enterpriseDefaults()...)
	}

	// compile the helm installation values
	args = append(args, "--namespace", a.namespace)
	args = append(args, a.deployArgs...)
	args = append(args, defaults()...)
	args = append(args, exposePortsDefault()...)
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

	// run any other cleanup jobs or ancillary tasks
	return runUDPServiceHack(ctx, cluster, DefaultNamespace)
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
	cmd := exec.Command("helm", "--kubeconfig", kubeconfig.Name(), "uninstall", DefaultDeploymentName, "--namespace", a.namespace) //nolint:gosec
	cmd.Stdout = io.Discard
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", stderr.String(), err)
	}

	if a.proxyEnterpriseLicenseJSON != "" {
		stderr := new(bytes.Buffer)
		cmd = exec.Command("kubectl", "delete", "secret", DefaultEnterpriseLicenseSecretName, "--namespace", a.namespace, "--kubeconfig", kubeconfig.Name()) //nolint:gosec
		cmd.Stdout = io.Discard
		cmd.Stderr = stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("delete secret err msg %s: %w", stderr.String(), err)
		}
	}

	return nil
}

func (a *Addon) Ready(ctx context.Context, cluster clusters.Cluster) (waitForObjects []runtime.Object, ready bool, err error) {
	return utils.IsNamespaceAvailable(ctx, cluster, a.namespace)
}

// -----------------------------------------------------------------------------
// Kong Addon - Private Secret Generation Config Options
// -----------------------------------------------------------------------------

// all of these configuration options govern the length contents and complexity
// of secrets/passwords that are generated by this addon for things such as
// created kong admin passwords, e.t.c.
//
// See: https://pkg.go.dev/github.com/sethvargo/go-password/password
const (
	secretMinLength   = 32
	secretMinNumeric  = 8
	secretMinSymbols  = 0
	secretNoUpper     = false
	secretAllowRepeat = false
)

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
		"--set", "tls.enabled=false",
		// the proxy expects a LoadBalancer Service provisioner (such as MetalLB).
		"--set", "proxy.type=LoadBalancer",
	}
}

// enterpriseDefaults provides the default values needed to enable enterprise mode for the Kong proxy server.
func enterpriseDefaults() []string {
	return []string{
		"--set", "enterprise.enabled=true",
		"--set", "enterprise.rbac.enabled=true",
		"--set", "env.enforce_rbac=on",
		"--set", "admin.annotations.konghq.com/protocol=http",
		"--set", fmt.Sprintf("enterprise.license_secret=%s", DefaultEnterpriseLicenseSecretName),
	}
}

// we set up a few default ports for TCP and UDP proxy stream, it's up to
// test cases to use these how they see fit AND clean up after themselves.
func exposePortsDefault() []string {
	return []string{
		"--set", fmt.Sprintf("proxy.stream[0].containerPort=%d", DefaultTCPServicePort),
		"--set", fmt.Sprintf("proxy.stream[0].servicePort=%d", DefaultTCPServicePort),
		"--set", fmt.Sprintf("proxy.stream[1].containerPort=%d", DefaultUDPServicePort),
		"--set", fmt.Sprintf("proxy.stream[1].servicePort=%d", DefaultUDPServicePort),
		"--set", "proxy.stream[1].parameters[0]=udp",
		"--set", "proxy.stream[1].parameters[1]=reuseport",
		"--set", "proxy.stream[2].containerPort=8899",
		"--set", "proxy.stream[2].servicePort=8899",
		"--set", "proxy.stream[2].parameters[0]=ssl",
		"--set", "proxy.stream[2].parameters[1]=reuseport",
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

	switch service.Spec.Type { //nolint:exhaustive
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

// deployKongEnterpriseLicenseSecret deploys a Kubernetes secret containing the enterprise license data
// which the Kong proxy will use to validate an enable enterprise featuresets.
func deployKongEnterpriseLicenseSecret(ctx context.Context, cluster clusters.Cluster, namespace, name, licenseJSON string) error {
	if licenseJSON == "" {
		return fmt.Errorf("enterprise mode was enabled but no license was provided")
	}

	kongLicenseSecret := &corev1.Secret{
		Type: corev1.SecretTypeOpaque,
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"license": []byte(licenseJSON),
		},
	}

	_, err := cluster.Client().CoreV1().Secrets(namespace).Create(ctx, kongLicenseSecret, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create the enterprise license secret: %w", err)
	}

	return nil
}

// deployEnterpriseSuperAdminPasswordSecret will deploy a secret for the provided kong admin superuser password that is provided.
// If no specific password is provided, one will be automatically generated.
func deployEnterpriseSuperAdminPasswordSecret(ctx context.Context, cluster clusters.Cluster, namespace, password string) (string, error) {
	var err error
	if password == "" {
		password, err = pwgen.Generate(secretMinLength, secretMinNumeric, secretMinSymbols, secretNoUpper, secretAllowRepeat)
		if err != nil {
			return "", fmt.Errorf("no admin password was provided so an attempt was made to generate one, but it failed: %w", err)
		}
	}

	kongSuperAdminPasswordSecret := &corev1.Secret{
		Type: corev1.SecretTypeOpaque,
		ObjectMeta: metav1.ObjectMeta{
			Name:      DefaultEnterpriseAdminPasswordSecretName,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"password": []byte(password),
		},
	}

	_, err = cluster.Client().CoreV1().Secrets(namespace).Create(ctx, kongSuperAdminPasswordSecret, metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to create the superuser admin password secret: %w", err)
	}

	return password, nil
}

// deployKongEnterpriseAdminGUISessionConf generates a session configuration for enterprise mode admin GUI and deploys that
// as a secret to the cluster to be picked up by the enterprise enabled proxy.
func deployKongEnterpriseAdminGUISessionConf(ctx context.Context, cluster clusters.Cluster, namespace string) error {
	sessionSecret, err := pwgen.Generate(secretMinLength, secretMinNumeric, secretMinSymbols, secretNoUpper, secretAllowRepeat)
	if err != nil {
		return fmt.Errorf("failed to generate a secure secret for the admin gui session config: %w", err)
	}
	sessionConfJSON := fmt.Sprintf(`{"cookie_secure":false,"storage":"kong","cookie_name":"admin_session","cookie_lifetime":31557600,"cookie_samesite":"off","secret":"%s"}`, sessionSecret)

	newSecret := &corev1.Secret{
		Type: corev1.SecretTypeOpaque,
		ObjectMeta: metav1.ObjectMeta{
			Name:      DefaultAdminGUISessionConfSecretName,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"admin_gui_session_conf": []byte(sessionConfJSON),
			"portal_session_conf":    []byte(sessionConfJSON),
		},
	}

	_, err = cluster.Client().CoreV1().Secrets(namespace).Create(ctx, newSecret, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create secret for admin gui session config: %w", err)
	}

	return nil
}
