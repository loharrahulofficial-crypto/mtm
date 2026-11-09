package cli

import (
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/loharrahulofficial-crypto/mtm/internal/clock"
	"github.com/loharrahulofficial-crypto/mtm/internal/demo"
	"github.com/loharrahulofficial-crypto/mtm/internal/paths"
)

// newDemoSeedCmd writes the demo dataset into a projects root, shifting
// every date relative to now. Hidden: used by contrib/demo.sh (make demo)
// and tests.
//
// Refuses to run unless the process is sandboxed: MTM_SANDBOX=1 plus all
// mtm roots AND the target resolving under a temp dir (the same admission
// `tui --dump` uses), and never writes into a repo whose .mtm-data marker
// says kind: real or whose path is under the real home directory.
func newDemoSeedCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "demo-seed <projects-dir>",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		Short:  "Seed the example dataset with dates relative to now (sandbox only)",
		RunE: func(cmd *cobra.Command, args []string) error {
			target, err := filepath.Abs(args[0])
			if err != nil {
				return err
			}
			if err := paths.AdmitSandbox(target); err != nil {
				return err
			}
			return demo.Seed(target, clock.Real().Now())
		},
	}
}
