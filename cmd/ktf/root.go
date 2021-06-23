package ktf

import (
	"github.com/spf13/cobra"
)

// -----------------------------------------------------------------------------
// Root Cmd
// -----------------------------------------------------------------------------

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.ktf.yaml)")
}

var rootCmd = &cobra.Command{
	Use:   "ktf",
	Short: "Kong Kubernetes Testing Framework",
}

func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}
