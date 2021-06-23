package ktf

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// -----------------------------------------------------------------------------
// Configuration
// -----------------------------------------------------------------------------

var cfgFile string

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		dir, name := determineConfigLocation()
		viper.SetConfigName(name)
		viper.AddConfigPath(dir)
	}
	viper.AutomaticEnv()
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}

func determineConfigLocation() (dir, name string) {
	// determine the home directory
	home, err := homedir.Dir()
	cobra.CheckErr(err)

	// set the desired defaults
	name = "ktf"
	dir = filepath.Join(home, ".config")

	// check to see if $HOME/.config actually exists
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			// if $HOME/.config doesn't exist, just use $HOME
			dir, name = home, ".ktf"
			return
		}
		cobra.CheckErr(err)
	}

	// if the $HOME/.config exists and is a directory, use it as the config dir
	if !info.IsDir() {
		// if $HOME/.config exists but is a file instead of a directory for some reason, just use $HOME.
		dir, name = home, ".ktf"
		return
	}

	return
}
