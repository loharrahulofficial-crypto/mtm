package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/loharrahulofficial-crypto/mtm/internal/clock"
	"github.com/loharrahulofficial-crypto/mtm/internal/config"
	"github.com/loharrahulofficial-crypto/mtm/internal/doctor"
	"github.com/loharrahulofficial-crypto/mtm/internal/paths"
	"github.com/loharrahulofficial-crypto/mtm/internal/theme"
)

func newDoctorCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Read-only health checks for the data repo (marker, git, remote, identity, frontmatter, sync, timer, render)",
		Long: `Checks the data repo without changing anything:

  git          data repo is a git repository
  marker       .mtm-data present, readable, kind real|demo
  remote       at least one git remote configured
  identity     git user.name and user.email are set
  frontmatter  every task file under projects/ and archive/ parses
  demo-data    task IDs or commit messages matching the demo corpus
               (od0001, lt0001, dn0055, ...) — flagged when the marker
               says real (or there is no marker)
  sync         paused/conflicted sync state, rebase in progress
  timer        stale or corrupt timer state in the state dir
  theme        malformed role(s) in colors.toml — falling back to the
               defaults per role (neutral warning, not a failure)
  terminal     detected terminal and color mode (truecolor | xterm-256),
               and where the override came from (env, config or auto)
  render       ui.ambient_render setting (auto | halfblock | braille |
               ascii | quadrant | sextant) and the resolution the ambient
               animations run at; ambient numbers are measured with it
               (set sextant for 2×3 subcells)

Exits non-zero when any check fails. --json prints the full report.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			rep := doctor.Run(doctor.Options{
				Root:          cfg.DataRepoPath(),
				StateDir:      paths.StateDir(),
				ThemeDir:      theme.StateDir(),
				TrueColor:     cfg.UI.TrueColor,
				AmbientRender: cfg.UI.AmbientRender,
				Now:           clock.Real().Now(),
			})
			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				if err := enc.Encode(rep); err != nil {
					return err
				}
				if !rep.OK {
					return errDoctorFailed
				}
				return nil
			}
			for _, f := range rep.Findings {
				where := ""
				if f.Path != "" && f.Path != rep.Root {
					where = "  " + f.Path
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-4s  %-11s  %s%s\n", f.Level, f.Check, f.Message, where)
			}
			fails, warns := 0, 0
			for _, f := range rep.Findings {
				switch f.Level {
				case doctor.LevelFail:
					fails++
				case doctor.LevelWarn:
					warns++
				}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "doctor: %d fail, %d warn (%s)\n", fails, warns, rep.Root)
			if !rep.OK {
				return errDoctorFailed
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print the report as JSON")
	return cmd
}

// errDoctorFailed makes `mtm doctor` exit non-zero when checks fail.
// The as-bytes sentinel avoids printing twice via Execute's error line.
var errDoctorFailed = fmt.Errorf("doctor: checks failed")
