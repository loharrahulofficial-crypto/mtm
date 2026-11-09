package cli

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

// version builds the full version line: embedded version, VCS commit and
// build date. GenMan uses it too.
func versionInfo() (string, string, string) {
	return version, commit, buildDate
}

// VersionString reports the embedded version string (used by the man-page
// generator header).
func VersionString() string { return version }

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the embedded version, commit and build date",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			v, c, b := versionInfo()
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "mtm %s\n", v)
			fmt.Fprintf(out, "commit: %s\n", c)
			fmt.Fprintf(out, "built:  %s\n", b)
			fmt.Fprintf(out, "go:     %s\n", runtime.Version())
			return nil
		},
	}
}
