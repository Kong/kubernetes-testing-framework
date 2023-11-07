package kong

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/kong/deck/dump"
	"github.com/kong/deck/file"
	"github.com/kong/deck/state"
	deckutils "github.com/kong/deck/utils"
	"github.com/kong/go-kong/kong"
	pwgen "github.com/sethvargo/go-password/password"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/kubectl/pkg/cmd/create"
	"k8s.io/kubectl/pkg/scheme"

	"github.com/kong/kubernetes-testing-framework/internal/retry"
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
	DefaultEnterpriseImageTag = "3.4-ubuntu"

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

	// ProxyPullSecretName is the name of the Secret used by the WithProxyImagePullSecret() builder
	ProxyPullSecretName = "proxy-pull"
)

// Addon is a Kong Proxy addon which can be deployed on a clusters.Cluster.
type Addon struct {
	logger *logrus.Logger

	name string

	// kubernetes and helm chart related configuration options
	namespace       string
	helmReleaseName string
	deployArgs      []string
	chartVersion    string

	// ingress controller configuration options
	ingressControllerDisabled bool
	ingressControllerImage    string
	ingressControllerImageTag string

	// proxy server general configuration options
	proxyAdminServiceTypeLoadBalancer bool
	proxyDBMode                       DBMode
	proxyImage                        string
	proxyImageTag                     string
	proxyPullSecret                   pullSecret
	proxyLogLevel                     string
	proxyServiceType                  corev1.ServiceType
	proxyEnvVars                      map[string]string
	proxyReadinessProbePath           string

	// Node ports
	httpNodePort  int
	adminNodePort int

	// proxy server enterprise mode configuration options
	proxyEnterpriseEnabled            bool
	proxyEnterpriseSuperAdminPassword string
	proxyEnterpriseLicenseJSON        string
	// additionalValues stores values that are set during installing by helm.
	// for each key-value pair, an argument `--set <key>=<value>` is added.
	additionalValues map[string]string
}

type pullSecret struct {
	Server   string
	Username string
	Password string
	Email    string
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

// ProxyURL provides a routable *url.URL for accessing the Kong proxy.
func (a *Addon) ProxyURL(ctx context.Context, cluster clusters.Cluster) (*url.URL, error) {
	waitForObjects, ready, err := a.Ready(ctx, cluster)
	if err != nil {
		return nil, err
	}

	if !ready {
		return nil, fmt.Errorf("the addon is not ready on cluster %s: non-empty unresolved objects list: %+v", cluster.Name(), waitForObjects)
	}

	return urlForService(ctx, cluster, types.NamespacedName{Namespace: a.namespace, Name: DefaultProxyServiceName}, DefaultProxyHTTPPort)
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
	return clusters.AddonName(a.name)
}

func (a *Addon) Dependencies(_ context.Context, cluster clusters.Cluster) []clusters.AddonName {
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
	err = retry.Command("helm", "--kubeconfig", kubeconfig.Name(), "repo", "add", "--force-update", "kong", KongHelmRepoURL).Do(ctx)
	if err != nil {
		return err
	}

	// ensure all repos are up to date
	err = retry.Command("helm", "--kubeconfig", kubeconfig.Name(), "repo", "update").Do(ctx)
	if err != nil {
		return err
	}

	// create a namespace ahead of deployment so things like license secrets and other configurations
	// can be preloaded.
	if err := clusters.CreateNamespace(ctx, cluster, a.namespace); err != nil {
		return err
	}

	// Pin the chart version if specified.
	if a.chartVersion != "" {
		a.deployArgs = append(a.deployArgs, "--version", a.chartVersion)
	}

	if a.proxyPullSecret != (pullSecret{}) {
		// create the pull Secret
		opts := create.CreateSecretDockerRegistryOptions{
			Name:       ProxyPullSecretName,
			Namespace:  a.namespace,
			Client:     cluster.Client().CoreV1(),
			PrintFlags: genericclioptions.NewPrintFlags("created").WithTypeSetter(scheme.Scheme),
			Username:   a.proxyPullSecret.Username,
			Email:      a.proxyPullSecret.Email,
			Password:   a.proxyPullSecret.Password,
			Server:     a.proxyPullSecret.Server,
		}
		if opts.Server == "" {
			opts.Server = "https://index.docker.io/v1/"
		}
		opts.PrintObj = func(obj runtime.Object) error {
			return nil
		}

		if err := opts.Run(); err != nil {
			return err
		}

		// use the pull Secret
		a.deployArgs = append(a.deployArgs,
			"--set", "image.pullSecrets={proxy-pull}",
		)
	}

	// if the dbmode is postgres, set several related values
	args := []string{"--kubeconfig", kubeconfig.Name(), "upgrade", "--install", a.helmReleaseName, "kong/kong"}
	if a.proxyDBMode == PostgreSQL {
		a.deployArgs = append(a.deployArgs,
			"--set", "env.database=postgres",
			"--set", "postgresql.enabled=true",
			"--set", "postgresql.auth.username=kong",
			"--set", "postgresql.auth.database=kong",
			"--set", "postgresql.service.port=5432",
		)
	}

	if cluster.IPFamily() == clusters.IPv6 {
		a.deployArgs = append(a.deployArgs,
			"--set", "proxy.address=[::]",
			"--set", "admin.address=[::1]",
			"--set", "status.address=[::]",
			"--set", "cluster.address=[::]",
			"--set", "ingressController.admissionWebhook.address=[::]",
		)
	}

	// if the ingress controller is disabled flag it in the chart and don't install any CRDs
	if a.ingressControllerDisabled {
		a.deployArgs = append(a.deployArgs,
			"--set", "ingressController.enabled=false",
			"--set", "ingressController.installCRDs=false",
			"--skip-crds",
		)
	}

	// set the ingress controller container image values if provided by the caller
	if a.ingressControllerImage != "" {
		a.deployArgs = append(a.deployArgs, "--set", fmt.Sprintf("ingressController.image.repository=%s", a.ingressControllerImage))
	}
	if a.ingressControllerImageTag != "" {
		a.deployArgs = append(a.deployArgs, "--set", fmt.Sprintf("ingressController.image.tag=%s", a.ingressControllerImageTag))
	}

	// set the container image values if provided by the caller
	if a.proxyImage != "" {
		a.deployArgs = append(a.deployArgs, "--set", fmt.Sprintf("image.repository=%s", a.proxyImage))
	}
	if a.proxyImageTag != "" {
		a.deployArgs = append(a.deployArgs, "--set", fmt.Sprintf("image.tag=%s", a.proxyImageTag))
	}

	// set the service type of the proxy admin's Kubernetes service
	if a.proxyAdminServiceTypeLoadBalancer {
		a.deployArgs = append(a.deployArgs, "--set", "admin.type=LoadBalancer")
	} else {
		a.deployArgs = append(a.deployArgs, "--set", "admin.type=ClusterIP")
	}

	// set the service type of the proxy's Kubernetes service
	if a.proxyServiceType == corev1.ServiceTypeExternalName {
		return fmt.Errorf("Service type ExternalName is not currently supported")
	}
	a.deployArgs = append(a.deployArgs, "--set", fmt.Sprintf("proxy.type=%s", a.proxyServiceType))
	a.deployArgs = append(a.deployArgs, "--set", fmt.Sprintf("udpProxy.type=%s", a.proxyServiceType))

	// set the proxy log level
	if len(a.proxyLogLevel) > 0 {
		a.deployArgs = append(a.deployArgs, "--set", fmt.Sprintf("env.log_level=%s", a.proxyLogLevel))
	}

	// Set the proxy readiness probe path.
	if len(a.proxyReadinessProbePath) > 0 {
		a.deployArgs = append(a.deployArgs, "--set", fmt.Sprintf("readinessProbe.httpGet.path=%s", a.proxyReadinessProbePath))
	}

	// Deploy licenses and other configurations for enterprise mode.
	if a.proxyEnterpriseEnabled {
		// Set the enterprise defaults helm installation values.
		a.deployArgs = append(a.deployArgs, enterpriseDefaults()...)
		// Deploy the license as a Kubernetes secret to enable enterprise features for the proxy.
		if err := deployKongEnterpriseLicenseSecret(ctx, cluster, a.namespace, DefaultEnterpriseLicenseSecretName, a.proxyEnterpriseLicenseJSON); err != nil {
			return err
		}
		// For DB-less mode, admin password can't be configured because there is nowhere for it to be stored.
		if a.proxyDBMode != DBLESS {
			// Deploy the superadmin password as a Kubernetes secret adjacent to the proxy pod and configure
			// the chart to use that secret to configure controller auth to the admin API.
			a.proxyEnterpriseSuperAdminPassword, err = deployEnterpriseSuperAdminPasswordSecret(ctx, cluster, a.namespace, a.proxyEnterpriseSuperAdminPassword)
			if err != nil {
				return err
			}
			a.deployArgs = append(a.deployArgs, "--set", fmt.Sprintf("env.password.valueFrom.secretKeyRef.name=%s", DefaultEnterpriseAdminPasswordSecretName))
			a.deployArgs = append(a.deployArgs, "--set", "env.password.valueFrom.secretKeyRef.key=password")
			a.deployArgs = append(a.deployArgs, "--set", "enterprise.rbac.enabled=true", "--set", "env.enforce_rbac=on")
			if !a.ingressControllerDisabled {
				a.deployArgs = append(
					a.deployArgs,
					"--set", fmt.Sprintf("ingressController.env.kong_admin_token.valueFrom.secretKeyRef.name=%s", DefaultEnterpriseAdminPasswordSecretName),
				)
				a.deployArgs = append(a.deployArgs, "--set", "ingressController.env.kong_admin_token.valueFrom.secretKeyRef.key=password")
			}
		}

		// Deploy the admin session configuration needed for enterprise enabled mode.
		if err := deployKongEnterpriseAdminGUISessionConf(ctx, cluster, a.namespace); err != nil {
			return err
		}

	}

	for name, value := range a.proxyEnvVars {
		a.deployArgs = append(a.deployArgs, "--set", fmt.Sprintf("env.%s=%s", name, value))
	}

	for name, value := range a.additionalValues {
		a.deployArgs = append(a.deployArgs, "--set", fmt.Sprintf("%s=%s", name, value))
	}

	// compile the helm installation values
	args = append(args, "--namespace", a.namespace)
	args = append(args, a.deployArgs...)
	args = append(args, defaults()...)

	if a.httpNodePort > 0 {
		args = append(args, "--set", fmt.Sprintf("proxy.http.nodePort=%d", a.httpNodePort))
	}
	if a.adminNodePort > 0 {
		args = append(args, "--set", fmt.Sprintf("admin.http.nodePort=%d", a.adminNodePort))
	}

	args = append(args, exposePortsDefault()...)
	a.logger.Debugf("helm install arguments: %+v", args)

	// Sometimes running helm install fails. Just in case this happens, retry.
	return retry.
		Command("helm", args...).
		DoWithErrorHandling(ctx, func(err error, _, stderr *bytes.Buffer) error {
			// ignore if addon is already deployed
			if strings.Contains(stderr.String(), "cannot re-use") {
				return nil
			}
			return fmt.Errorf("%s: %w", stderr, err)
		})
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
	cmd := exec.CommandContext(ctx, "helm", "--kubeconfig", kubeconfig.Name(), "uninstall", a.helmReleaseName, "--namespace", a.namespace) //nolint:gosec
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

func (a *Addon) DumpDiagnostics(ctx context.Context, cluster clusters.Cluster) (map[string][]byte, error) {
	diagnostics := make(map[string][]byte)
	admin, err := a.ProxyAdminURL(ctx, cluster)
	if err != nil {
		return diagnostics, fmt.Errorf("could not build diagnostic Kong client: %w", err)
	}
	resp, err := http.Get(admin.String() + "/")
	if err != nil {
		return diagnostics, fmt.Errorf("could not retrieve Kong root: %w", err)
	}
	defer resp.Body.Close()
	b := new(bytes.Buffer)
	_, err = b.ReadFrom(resp.Body)
	if err != nil {
		return diagnostics, err
	}
	diagnostics["root_endpoint.json"] = b.Bytes()

	// Extract the version from the root endpoint.
	var kongVersion struct {
		Version string `json:"version"`
	}
	err = json.Unmarshal(b.Bytes(), &kongVersion)
	if err != nil {
		return diagnostics, fmt.Errorf("could not unmarshal Kong version: %w", err)
	}

	switch a.proxyDBMode {
	case PostgreSQL:
		out, err := os.CreateTemp(os.TempDir(), "ktf-kong-")
		if err != nil {
			return diagnostics, fmt.Errorf("could not create temp file: %w", err)
		}
		defer os.Remove(out.Name())
		dumpConfig := dump.Config{}
		addr, err := a.ProxyAdminURL(ctx, cluster)
		if err != nil {
			return diagnostics, fmt.Errorf("could not build Kong client: %w", err)
		}
		opts := deckutils.KongClientConfig{
			Address: addr.String(),
			HTTPClient: &http.Client{
				Timeout: time.Second * 90, //nolint:gomnd
			},
			TLSSkipVerify: true,
		}
		if a.proxyEnterpriseSuperAdminPassword != "" {
			opts.Headers = append(opts.Headers, "kong-admin-token:"+a.proxyEnterpriseSuperAdminPassword)
		}
		client, err := deckutils.GetKongClient(opts)
		if err != nil {
			return diagnostics, fmt.Errorf("could not build Kong client: %w", err)
		}
		workspaces, err := client.Workspaces.ListAll(ctx)
		var kongAPIError *kong.APIError
		if errors.As(err, &kongAPIError) && kongAPIError.Code() == http.StatusNotFound {
			defaultws := kong.Workspace{Name: kong.String("default")}
			workspaces = []*kong.Workspace{&defaultws}
		} else if err != nil {
			return diagnostics, fmt.Errorf("could get workspaces: %w", err)
		}

		for _, workspace := range workspaces {
			wsOpts := opts
			wsOpts.Workspace = *workspace.Name
			var wsClient *kong.Client
			if *workspace.Name == "default" {
				// arguably a workspaced client for default should work on OSS, but it doesn't!
				wsClient = client
			} else {
				wsClient, err = deckutils.GetKongClient(wsOpts)
				if err != nil {
					return diagnostics, fmt.Errorf("could not build Kong client: %w", err)
				}
			}
			// deck will forcibly append the extension if you omit it
			out, err := os.CreateTemp(os.TempDir(), "ktf-kong-config-*.yaml")
			if err != nil {
				return diagnostics, fmt.Errorf("could not create temp file: %w", err)
			}
			defer os.Remove(out.Name())
			rawState, err := dump.Get(ctx, wsClient, dumpConfig)
			if err != nil {
				return diagnostics, fmt.Errorf("could not retrieve config from Kong: %w", err)
			}
			currentState, err := state.Get(rawState)
			if err != nil {
				return diagnostics, fmt.Errorf("could not build Kong state: %w", err)
			}
			err = file.KongStateToFile(currentState, file.WriteConfig{
				Filename:    out.Name(),
				FileFormat:  file.YAML,
				KongVersion: kongVersion.Version,
			})
			if err != nil {
				return diagnostics, fmt.Errorf("could not write Kong config: %w", err)
			}
			config, err := os.ReadFile(out.Name())
			if err != nil {
				return diagnostics, fmt.Errorf("could not read Kong config: %w", err)
			}
			diagnostics[*workspace.Name+"_pg_config.yaml"] = config
		}
	case DBLESS:
		resp, err := http.Get(admin.String() + "/config")
		if err != nil {
			return diagnostics, fmt.Errorf("could not retrieve Kong /config: %w", err)
		}
		defer resp.Body.Close()
		var kongConfig struct {
			Config string `json:"config,omitempty" yaml:"config,omitempty"`
		}
		err = json.NewDecoder(resp.Body).Decode(&kongConfig)
		if err != nil {
			return diagnostics, fmt.Errorf("could not parse config: %w", err)
		}
		yaml := strings.ReplaceAll(kongConfig.Config, "\\\\n", "\n")

		diagnostics["dbless_config.yaml"] = []byte(yaml)
	}
	return diagnostics, nil
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
		"--set", "admin.enabled=true",
		"--set", "admin.http.enabled=true",
		"--set", "admin.tls.enabled=false",
		"--set", "tls.enabled=false",
		"--set", "udpProxy.enabled=true",
	}
}

// enterpriseDefaults provides the default values needed to enable enterprise mode for the Kong proxy server.
func enterpriseDefaults() []string {
	return []string{
		"--set", "enterprise.enabled=true",
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
		"--set", fmt.Sprintf("proxy.stream[1].containerPort=%d", DefaultTLSServicePort),
		"--set", fmt.Sprintf("proxy.stream[1].servicePort=%d", DefaultTLSServicePort),
		"--set", "proxy.stream[1].parameters[0]=ssl",
		"--set", "proxy.stream[1].parameters[1]=reuseport",
		"--set", fmt.Sprintf("udpProxy.stream[0].containerPort=%d", DefaultUDPServicePort),
		"--set", fmt.Sprintf("udpProxy.stream[0].servicePort=%d", DefaultUDPServicePort),
		"--set", "udpProxy.stream[0].protocol=UDP",
		"--set", "udpProxy.stream[0].parameters[0]=udp",
		"--set", "udpProxy.stream[0].parameters[1]=reuseport",
	}
}

func urlForService(ctx context.Context, cluster clusters.Cluster, nsn types.NamespacedName, port int) (*url.URL, error) {
	service, err := cluster.Client().CoreV1().Services(nsn.Namespace).Get(ctx, nsn.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

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
