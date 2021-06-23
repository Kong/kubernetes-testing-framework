package ktf

import (
	"fmt"

	"github.com/kong/kubernetes-testing-framework/pkg/environment"
	"github.com/spf13/cobra"
)

// -----------------------------------------------------------------------------
// Environment
// -----------------------------------------------------------------------------

func init() {
	rootCmd.AddCommand(environmentCmd)
}

var environmentCmd = &cobra.Command{
	Use:   "environment",
	Short: "Deploy and manage testing environments",
}

// -----------------------------------------------------------------------------
// Environment - Create
// -----------------------------------------------------------------------------

func init() {
	environmentCmd.AddCommand(createEnvironmentCmd)
}

var createEnvironmentCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new environment",
	RunE: func(cmd *cobra.Command, args []string) error {
		env, err := state.CreateEnvironment(&environment.Builder{})
		if err != nil {
			return err
		}

		cmd.Printf("created new environment %s\n", env.Name())
		return nil
	},
}

// -----------------------------------------------------------------------------
// Environment - Describe
// -----------------------------------------------------------------------------

func init() {
	environmentCmd.AddCommand(describeEnvironmentCmd)
}

var describeEnvironmentCmd = &cobra.Command{
	Use:   "describe",
	Short: "Describe an existing environment",
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("unimplemented, args: %+v\n", args)
	},
}

// -----------------------------------------------------------------------------
// Environment - List
// -----------------------------------------------------------------------------

func init() {
	environmentCmd.AddCommand(listEnvironmentCmd)
}

var listEnvironmentCmd = &cobra.Command{
	Use:   "list",
	Short: "List all environments",
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("unimplemented, args: %+v\n", args)
	},
}

// -----------------------------------------------------------------------------
// Environment - Update
// -----------------------------------------------------------------------------

func init() {
	environmentCmd.AddCommand(updateEnvironmentCmd)
}

var updateEnvironmentCmd = &cobra.Command{
	Use:   "update",
	Short: "Update an existing environment",
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("unimplemented, args: %+v\n", args)
	},
}

// -----------------------------------------------------------------------------
// Environment - Delete
// -----------------------------------------------------------------------------

func init() {
	environmentCmd.AddCommand(deleteEnvironmentCmd)
}

var deleteEnvironmentCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete an existing environment",
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("unimplemented, args: %+v\n", args)
	},
}
