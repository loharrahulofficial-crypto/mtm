package cli

import (
	"github.com/spf13/cobra"

	"github.com/loharrahulofficial-crypto/mtm/internal/clock"
	"github.com/loharrahulofficial-crypto/mtm/internal/demo"
)

// newDemoSeedCmd writes the demo dataset into a projects root, shifting
// every date relative to now. Hidden: used by contrib/demo.sh (make demo)
// and tests. Never touches the real data repo — the caller supplies the
// target directory inside a sandbox.
func newDemoSeedCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "demo-seed <projects-dir>",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		Short:  "Seed the example dataset with dates relative to now",
		RunE: func(cmd *cobra.Command, args []string) error {
			return demo.Seed(args[0], clock.Real().Now())
		},
	}
}
