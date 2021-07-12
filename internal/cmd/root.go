package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "ktf",
	Short: "Kong Kubernetes Testing Framework",
}

func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}
