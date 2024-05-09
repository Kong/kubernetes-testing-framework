package ktf

import (
	"context"
	"fmt"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/spf13/cobra"

	"github.com/kong/kubernetes-testing-framework/pkg/clusters/addons/argocd"
	"github.com/kong/kubernetes-testing-framework/pkg/clusters/addons/certmanager"
	"github.com/kong/kubernetes-testing-framework/pkg/clusters/addons/httpbin"
	"github.com/kong/kubernetes-testing-framework/pkg/clusters/addons/istio"
	"github.com/kong/kubernetes-testing-framework/pkg/clusters/addons/kong"
	"github.com/kong/kubernetes-testing-framework/pkg/clusters/addons/kongargo"
	"github.com/kong/kubernetes-testing-framework/pkg/clusters/addons/kuma"
	"github.com/kong/kubernetes-testing-framework/pkg/clusters/addons/metallb"
	"github.com/kong/kubernetes-testing-framework/pkg/clusters/addons/registry"
	"github.com/kong/kubernetes-testing-framework/pkg/clusters/types/kind"
	"github.com/kong/kubernetes-testing-framework/pkg/environments"
)

// -----------------------------------------------------------------------------
// Environments - Base Command
// -----------------------------------------------------------------------------

func init() { //nolint:gochecknoinits
	rootCmd.AddCommand(environmentsCmd)
}

var environmentsCmd = &cobra.Command{
	Use:     "environments",
	Aliases: []string{"environs", "envs", "env"},
	Short:   "create and manage testing environments",
}

// -----------------------------------------------------------------------------
// Environments - Create Subcommand
// -----------------------------------------------------------------------------

func init() { //nolint:gochecknoinits
	environmentsCmd.AddCommand(environmentsCreateCmd)

	// environment naming
	environmentsCreateCmd.PersistentFlags().String("name", DefaultEnvironmentName, "name to give the new testing environment")
	environmentsCreateCmd.PersistentFlags().Bool("generate-name", false, "indicate whether or not to use a generated name for the environment")

	// cluster configurations
	environmentsCreateCmd.PersistentFlags().String("kubernetes-version", "", "which kubernetes version to use (default: latest for driver)")
	environmentsCreateCmd.PersistentFlags().Bool("cni-calico", false, "use Calico for cluster CNI instead of the default CNI")
	environmentsCreateCmd.PersistentFlags().Bool("ipv6-only", false, "only use IPv6")

	// addon configurations
	environmentsCreateCmd.PersistentFlags().StringArray("addon", nil, "name of an addon to deploy to the testing environment's cluster")
	environmentsCreateCmd.PersistentFlags().Bool("kong-disable-controller", false, "indicate whether the kong addon should have the controller disabled (proxy only)")
	environmentsCreateCmd.PersistentFlags().Bool("kong-admin-service-loadbalancer", false, "indicate whether the kong addon should deploy the proxy admin service as a LoadBalancer type")
	environmentsCreateCmd.PersistentFlags().String("kong-ingress-controller-image", "", "use a specific ingress controller container image for the Gateway (proxy)")
	environmentsCreateCmd.PersistentFlags().String("kong-gateway-image", "", "use a specific container image for the Gateway (proxy)")
	environmentsCreateCmd.PersistentFlags().String("kong-dbmode", "off", "indicate the backend dbmode to use for kong (default: \"off\" (DBLESS mode))")
}

var environmentsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "create a new testing environment",
	Run: func(cmd *cobra.Command, _ []string) {
		ctx, cancel := context.WithTimeout(context.Background(), EnvironmentCreateTimeout)
		defer cancel()

		// get the name for the environment (if not provided, a uuid will be generated)
		name, err := cmd.PersistentFlags().GetString("name")
		cobra.CheckErr(err)

		// get any addons for the cluster that were desired
		deployAddons, err := cmd.PersistentFlags().GetStringArray("addon")
		cobra.CheckErr(err)

		// verify whether the environment was flagged to use a generated name
		useGeneratedName, err := cmd.PersistentFlags().GetBool("generate-name")
		cobra.CheckErr(err)

		// check if a specific Kubernetes version was requested
		kubernetesVersion, err := cmd.PersistentFlags().GetString("kubernetes-version")
		cobra.CheckErr(err)

		// check if Calico CNI was requested
		useCalicoCNI, err := cmd.PersistentFlags().GetBool("cni-calico")
		cobra.CheckErr(err)

		// check if IPv6 was requested
		useIPv6Only, err := cmd.PersistentFlags().GetBool("ipv6-only")
		cobra.CheckErr(err)

		// setup the new environment
		builder := environments.NewBuilder()
		if !useGeneratedName {
			builder = builder.WithName(name)
		}
		if useCalicoCNI {
			builder = builder.WithCalicoCNI()
		}
		if useIPv6Only {
			builder = builder.WithIPv6Only()
		}
		if kubernetesVersion != "" {
			version, err := semver.Parse(strings.TrimPrefix(kubernetesVersion, "v"))
			cobra.CheckErr(err)
			builder = builder.WithKubernetesVersion(version)
		}

		// configure any addons that need to be deployed with the environment's cluster
		callbacks := configureAddons(cmd, builder, deployAddons)

		fmt.Printf("building new environment %s\n", builder.Name)
		env, err := builder.Build(ctx)
		cobra.CheckErr(err)

		addons := env.Cluster().ListAddons()
		for _, addon := range addons {
			fmt.Printf("waiting for addon %s to become ready...\n", addon.Name())
		}

		fmt.Println("waiting for environment to become ready (this can take some time)...")
		cobra.CheckErr(<-env.WaitForReady(ctx))

		fmt.Printf("environment %s was created successfully!\n", env.Name())
		for _, callback := range callbacks {
			callback()
		}
	},
}

func configureAddons(cmd *cobra.Command, builder *environments.Builder, addons []string) []func() {
	invalid, dedup := make([]string, 0), make(map[string]bool)
	// sometimes some addons which are configured for need to do something AFTER
	// the addon itself has successfully deployed, usually something like helpful
	// logging messages.
	callbacks := make([]func(), 0)

	for _, addon := range addons {
		// load any valid addons, and check for invalid addons
		switch addon {
		case "metallb":
			builder = builder.WithAddons(metallb.New())
		case "kong":
			builder = configureKongAddon(cmd, builder)
		case "istio":
			istioAddon := istio.NewBuilder().
				WithGrafana().
				WithJaeger().
				WithKiali().
				WithPrometheus().
				Build()
			builder = builder.WithAddons(istioAddon)
		case "httpbin":
			builder = builder.WithAddons(httpbin.New())
		case "cert-manager":
			builder = builder.WithAddons(certmanager.New())
		case "kuma":
			builder = builder.WithAddons(kuma.New())
		case "argocd":
			argoAddon := argocd.NewBuilder().Build()
			builder = builder.WithAddons(argoAddon)
		case "kong-argo":
			kongArgoAddon := kongargo.NewBuilder().Build()
			builder = builder.WithAddons(kongArgoAddon)
		case "registry":
			registryAddon := registry.NewBuilder().
				WithServiceTypeLoadBalancer().
				Build()
			builder = builder.WithAddons(registryAddon)
			registryInfoCallback := func() {
				fmt.Printf(`
Registry Addon HELP:

You have installed the registry addon deployed with an SSL certificate provided
by cert-manager. The default certificate used is a self-signed certificate.
As such if you try to push images to this registry with the standard:

  $ docker push ${REGISTRY_IP}/image

Without first adding its certificate to your local docker (or other client) chain
of trust it will fail. The following provides an example of how to add the certificate
using a standard docker installation on a Linux system where "/etc/docker" is the
configuration directory for docker:

  $ REGISTRY_IP="$(kubectl -n %s get svc registry -o=go-template='{{(index .status.loadBalancer.ingress 0).ip}}')"
  $ sudo mkdir -p /etc/docker/certs.d/${REGISTRY_IP}/
  $ kubectl -n %s get secrets registry-cert-secret -o=go-template='{{index .data "ca.crt"}}' | base64 -d | sudo tee /etc/docker/certs.d/${REGISTRY_IP}/ca.crt

Note that this generally is not going to work verbatim on all systems and the
above instructions should be considered just an example. Adjust for your own
system and docker installation. You may also need to change ".ip" for ".host"
if your service is provided a DNS name instead of an IP for its LB address.

Afterwards you should be able to push images to the registry, e.g.:

  $ docker pull kennethreitz/httpbin
  $ docker tag kennethreitz/httpbin ${REGISTRY_IP}/httpbin
  $ docker push ${REGISTRY_IP}/httpbin

Images pushed this way should be immediately usable in pod configurations
on the cluster as the certificate is automatically configured on the nodes.
`, registryAddon.Namespace(), registryAddon.Namespace())
			}
			callbacks = append(callbacks, registryInfoCallback)
		default:
			invalid = append(invalid, addon)
		}

		// fail if any duplicate addons were provided
		if _, ok := dedup[addon]; ok {
			cobra.CheckErr(fmt.Errorf("addon %s was provided more than once", addon))
		}
		dedup[addon] = true
	}

	if len(invalid) > 0 {
		cobra.CheckErr(fmt.Errorf("%d addons were invalid: %s", len(invalid), invalid))
	}

	return callbacks
}

func configureKongAddon(cmd *cobra.Command, envBuilder *environments.Builder) *environments.Builder {
	builder := kong.NewBuilder()

	disableController, err := cmd.PersistentFlags().GetBool("kong-disable-controller")
	cobra.CheckErr(err)

	if disableController {
		builder.WithControllerDisabled()
	}

	customGatewayImage, err := cmd.PersistentFlags().GetString("kong-gateway-image")
	cobra.CheckErr(err)

	if customGatewayImage != "" {
		imageParts := strings.Split(customGatewayImage, ":")
		if len(imageParts) == 1 {
			imageParts = append(imageParts, "latest")
		}
		if len(imageParts) != 2 { //nolint:mnd
			cobra.CheckErr(fmt.Errorf("malformed --kong-gateway-image: %s", customGatewayImage))
		}
		builder.WithProxyImage(imageParts[0], imageParts[1])
	}

	customControllerImage, err := cmd.PersistentFlags().GetString("kong-ingress-controller-image")
	cobra.CheckErr(err)

	if customControllerImage != "" {
		imageParts := strings.Split(customControllerImage, ":")
		if len(imageParts) == 1 {
			imageParts = append(imageParts, "latest")
		}
		if len(imageParts) != 2 { //nolint:mnd
			cobra.CheckErr(fmt.Errorf("malformed --kong-ingress-controller-image: %s", customControllerImage))
		}
		builder.WithControllerImage(imageParts[0], imageParts[1])
	}

	enableAdminSvcLB, err := cmd.PersistentFlags().GetBool("kong-admin-service-loadbalancer")
	cobra.CheckErr(err)

	if enableAdminSvcLB {
		builder.WithProxyAdminServiceTypeLoadBalancer()
	}

	dbmode, err := cmd.PersistentFlags().GetString("kong-dbmode")
	cobra.CheckErr(err)

	switch dbmode {
	case "off":
		builder.WithDBLess()
	case "postgres":
		builder.WithPostgreSQL()
	default:
		cobra.CheckErr(fmt.Errorf("%s is not a valid dbmode for kong, supported modes are \"off\" (DBLESS) or \"postgres\"", dbmode))
	}

	return envBuilder.WithAddons(builder.Build())
}

// -----------------------------------------------------------------------------
// Environments - Delete Subcommand
// -----------------------------------------------------------------------------

func init() { //nolint:gochecknoinits
	environmentsCmd.AddCommand(environmentsDeleteCmd)
	environmentsDeleteCmd.PersistentFlags().String("name", DefaultEnvironmentName, "name of the environment to delete")
}

var environmentsDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "delete a testing environment",
	Run: func(cmd *cobra.Command, _ []string) {
		ctx, cancel := context.WithTimeout(context.Background(), EnvironmentCreateTimeout)
		defer cancel()

		name, err := cmd.PersistentFlags().GetString("name")
		cobra.CheckErr(err)

		cluster, err := kind.NewFromExisting(name)
		cobra.CheckErr(err)

		fmt.Printf("deleting environment %s\n", name)
		cobra.CheckErr(cluster.Cleanup(ctx))

		fmt.Printf("environment %s has been successfully deleted!\n", name)
	},
}
