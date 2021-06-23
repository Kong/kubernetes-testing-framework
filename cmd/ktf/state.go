package ktf

import (
	"github.com/spf13/cobra"

	internalstate "github.com/kong/kubernetes-testing-framework/internal/state"
)

// state is the KTF state read from disk.
var state *internalstate.State

func init() {
	f, err := internalstate.GetStateFile()
	cobra.CheckErr(err)
	state, err = internalstate.NewFromFile(f)
	cobra.CheckErr(err)
}
