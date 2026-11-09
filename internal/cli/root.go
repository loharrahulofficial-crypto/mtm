// Package cli implements the mtm command line (Cobra).
// All business logic lives in internal/core and internal/store.
package cli

import (
	"fmt"
	"os"

	"golang.org/x/term"

	"github.com/loharrahulofficial-crypto/mtm/internal/appname"
	"github.com/loharrahulofficial-crypto/mtm/internal/tui"
	"github.com/spf13/cobra"
)

// version is overridden at build time via -ldflags "-X .../cli.version=x.y.z"
var version = "dev"

// commit is overridden at build time via -ldflags "-X .../cli.commit=<hash>".
var commit = "unknown"

// buildDate is overridden at build time via -ldflags "-X .../cli.buildDate=<UTC stamp>".
var buildDate = "unknown"

// Execute runs the root command and returns the process exit error.
func Execute() error {
	root := NewRootCmd()
	if err := root.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", appname.BinName, err)
		return err
	}
	return nil
}

// NewRootCmd builds the full command tree. It is shared by Execute and by
// the man-page generator.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           appname.BinName,
		Short:         appname.LongName + " — terminal task manager",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Bare `mtm` launches the TUI, but only on a terminal; otherwise
			// print help (scripts keep the old behavior).
			if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())) {
				return cmd.Help()
			}
			return tui.Run()
		},
	}
	root.AddCommand(
		newInitCmd(),
		newAddCmd(),
		newArchiveCmd(),
		newListCmd(),
		newTodayCmd(),
		newWeekCmd(),
		newOverdueCmd(),
		newViewCmd(),
		newViewsCmd(),
		newShowCmd(),
		newEditCmd(),
		newDoneCmd(),
		newRmCmd(),
		newStatusCmd(),
		newNoteCmd(),
		newSubCmd(),
		newSearchCmd(),
		newProjectCmd(),
		newCourseCmd(),
		newExamCmd(),
		newGradeCmd(),
		newGradesCmd(),
		newHoursCmd(),
		newTimerCmd(),
		newPomoCmd(),
		newSyncCmd(),
		newDoctorCmd(),
		newSummaryCmd(),
		newStatsCmd(),
		newReviewCmd(),
		newExportCmd(),
		newIcalCmd(),
		newRemindCmd(),
		newSystemdCmd(),
		newWaybarCmd(),
		newCompletionCmd(root),
		newTuiCmd(),
		newDemoSeedCmd(),
		newVersionCmd(),
	)
	return root
}
