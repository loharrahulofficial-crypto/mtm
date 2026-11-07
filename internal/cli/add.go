package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/loharrahulofficial-crypto/mtm/internal/actions"
	"github.com/loharrahulofficial-crypto/mtm/internal/core"
)

func newAddCmd() *cobra.Command {
	var (
		project  string
		priority string
		tags     []string
		due      string
		kind     string
	)
	cmd := &cobra.Command{
		Use:   "add <title...>",
		Short: "Add a task (natural language or flags)",
		Long: `Quick add understands: dates and times (tomorrow, "friday 6pm", "next monday",
"in 3 days", "14 nov"), !priority (!low !med !high !urgent), #tags, @project,
~estimate (~2h), recurrence ("every week", "daily", "every monday").

  mtm add "submit report friday 6pm !high #college @dbms"
  mtm add "Submit report" --due 2026-11-14 --priority high
  mtm add buy milk            # unquoted words work too

Flags always win over parsed values. Remaining words become the title.`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			raw := strings.Join(args, " ")
			if strings.TrimSpace(raw) == "" {
				return errors.New("a title is required, e.g. mtm add \"submit report friday 6pm !high\"")
			}
			cfg, s, err := openStore()
			if err != nil {
				return err
			}
			defer func() { _ = s.Close() }()

			changed := func(name string) bool { return cmd.Flags().Changed(name) }
			ov := actions.QuickAddOverrides{Kind: kind}
			if changed("project") {
				ov.Project = project
			}
			if changed("priority") {
				ov.Priority = strings.ToLower(strings.TrimSpace(priority))
			}
			if changed("due") {
				ov.Due, ov.DueSet = due, true
			}
			if changed("tag") {
				ov.Tags = parseTags(tags)
			}
			t, err := actions.QuickAddTask(cfg, s, raw, ov)
			if err != nil {
				return err
			}
			commitAndSync(cfg, s.Root, "add: "+t.Title)
			fmt.Printf("added %s  %s\n", t.ID, t.Title)
			return nil
		},
	}
	cmd.Flags().StringVarP(&project, "project", "p", "", "project (default from config)")
	cmd.Flags().StringVar(&priority, "priority", "", "low|medium|high|urgent")
	cmd.Flags().StringSliceVarP(&tags, "tag", "t", nil, "tags (comma-separated or repeated)")
	cmd.Flags().StringVar(&due, "due", "", "due: YYYY-MM-DD or YYYY-MM-DDTHH:MM")
	cmd.Flags().StringVar(&kind, "kind", core.KindTask, "task|assignment|exam|study|project")
	return cmd
}

func parseTags(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, part := range in {
		for _, tag := range strings.Split(part, ",") {
			tag = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(tag, "#")))
			if tag == "" || seen[tag] {
				continue
			}
			seen[tag] = true
			out = append(out, tag)
		}
	}
	return out
}
