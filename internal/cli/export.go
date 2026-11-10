package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/loharrahulofficial-crypto/mtm/internal/actions"
)

func newExportCmd() *cobra.Command {
	var (
		format  string
		out     string
		project string
		course  string
		since   string
	)
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export tasks, courses, exams, grades and hours (md|csv|json)",
		Long: `Export the data you own. JSON carries the full report with a
top-level schema_version (2) and is the round-trip format. Markdown is a
readable report grouped by course with grades, exams and hours inline.
CSV is a flat task table (one row per task, stable column order) for
spreadsheets.

  --project X   restrict to one project
  --course X    restrict to one course
  --since DATE  only tasks touched on/after DATE (2006-01-02)
  --out FILE    write to FILE instead of stdout (mode 0600)

Ideas and milestones land as reserved top-level keys in a later schema.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, s, err := openStore()
			if err != nil {
				return err
			}
			_ = cfg
			defer func() { _ = s.Close() }()

			var sinceT time.Time
			if strings.TrimSpace(since) != "" {
				parsed, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(since), s.Now().Location())
				if err != nil {
					return fmt.Errorf("--since: want 2006-01-02, got %q", since)
				}
				sinceT = parsed
			}
			now := s.Now()
			rep, err := actions.CollectExport(s, now, actions.ExportOptions{
				Project: strings.TrimSpace(project),
				Course:  strings.TrimSpace(course),
				Since:   sinceT,
			})
			if err != nil {
				return err
			}
			var data []byte
			switch strings.ToLower(strings.TrimSpace(format)) {
			case "json":
				data, err = actions.ExportJSON(rep)
			case "csv":
				data, err = actions.ExportCSV(rep)
			case "md", "markdown":
				data = []byte(actions.ExportMarkdown(rep, now))
			default:
				return fmt.Errorf("unknown --format %q (want md|csv|json)", format)
			}
			if err != nil {
				return err
			}
			if strings.TrimSpace(out) != "" {
				if err := os.WriteFile(out, data, 0o600); err != nil {
					return fmt.Errorf("write %s: %w", out, err)
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "wrote %s (%d bytes)\n", out, len(data))
				return nil
			}
			_, err = cmd.OutOrStdout().Write(data)
			if !bytes1HasSuffix(data, '\n') {
				if _, werr := cmd.OutOrStdout().Write([]byte{'\n'}); werr != nil {
					_ = werr
				}
			}
			return err
		},
	}
	cmd.Flags().StringVar(&format, "format", "md", "output format: md|csv|json")
	cmd.Flags().StringVar(&out, "out", "", "write to FILE instead of stdout (mode 0600)")
	cmd.Flags().StringVar(&project, "project", "", "restrict to one project")
	cmd.Flags().StringVar(&course, "course", "", "restrict to one course")
	cmd.Flags().StringVar(&since, "since", "", "only tasks touched on/after DATE (2006-01-02)")
	return cmd
}

func bytes1HasSuffix(b []byte, c byte) bool {
	return len(b) > 0 && b[len(b)-1] == c
}
