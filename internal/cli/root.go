// Package cli implements the mtm command line (Cobra).
// All business logic lives in internal/core and internal/store.
package cli

import (
	"fmt"
	"os"

	"github.com/loharrahulofficial-crypto/mtm/internal/appname"
	"github.com/spf13/cobra"
)

// version is overridden at build time via -ldflags "-X .../cli.version=x.y.z".
var version = "dev"

// Execute runs the root command and returns the process exit error.
func Execute() error {
	root := &cobra.Command{
		Use:           appname.BinName,
		Short:         appname.LongName + " — terminal task manager",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(
		newInitCmd(),
		newAddCmd(),
		newListCmd(),
		newShowCmd(),
		newEditCmd(),
		newDoneCmd(),
		newRmCmd(),
		newStatusCmd(),
		newProjectCmd(),
		newSyncCmd(),
	)
	if err := root.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", appname.BinName, err)
		return err
	}
	return nil
}
