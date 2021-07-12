package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kong/kubernetes-testing-framework/pkg/clusters/addons/kong"
	"github.com/kong/kubernetes-testing-framework/pkg/clusters/addons/metallb"
	"github.com/kong/kubernetes-testing-framework/pkg/clusters/types/kind"
	"github.com/kong/kubernetes-testing-framework/pkg/environments"
)

// -----------------------------------------------------------------------------
// Environments - Base Command
// -----------------------------------------------------------------------------

func init() {
	rootCmd.AddCommand(environmentsCmd)
}

var environmentsCmd = &cobra.Command{
	Use:   "environments",
	Short: "create and manage testing environments",
}

// -----------------------------------------------------------------------------
// Environments - Create Subcommand
// -----------------------------------------------------------------------------

func init() {
	environmentsCmd.AddCommand(environmentsCreateCmd)
	environmentsCreateCmd.PersistentFlags().String("name", DefaultEnvironmentName, "name to give the new testing environment")
	environmentsCreateCmd.PersistentFlags().StringArray("addon", nil, "a list of addons to deploy to the environment's cluster")
	environmentsCreateCmd.PersistentFlags().Bool("generate-name", false, "indicate whether or not to use a generated name for the environment")
}

var environmentsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "create a new testing environment",
	Run: func(cmd *cobra.Command, args []string) {
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

		// setup the new environment
		builder := environments.NewBuilder()
		if !useGeneratedName {
			builder = builder.WithName(name)
		}

		// configure any addons that need to be deployed with the environment's cluster
		configureAddons(builder, deployAddons)

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
	},
}

func configureAddons(builder *environments.Builder, addons []string) {
	invalid, dedup := make([]string, 0), make(map[string]bool)
	for _, addon := range addons {
		// load any valid addons, and check for invalid addons
		switch addon {
		case "metallb":
			builder = builder.WithAddons(metallb.New())
		case "kong":
			builder = builder.WithAddons(kong.New())
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
}

// -----------------------------------------------------------------------------
// Environments - Delete Subcommand
// -----------------------------------------------------------------------------

func init() {
	environmentsCmd.AddCommand(environmentsDeleteCmd)
	environmentsDeleteCmd.PersistentFlags().String("name", DefaultEnvironmentName, "name of the environment to delete")
}

var environmentsDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "delete a testing environment",
	Run: func(cmd *cobra.Command, args []string) {
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
