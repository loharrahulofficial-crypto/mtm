package cli

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/loharrahulofficial-crypto/mtm/internal/parse"
	"github.com/spf13/cobra"
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

			parsed := parse.QuickAdd(raw, time.Now())
			title := parsed.Title
			if title == "" {
				return errors.New("no title left after parsing — did you mean to write some words of the task itself?")
			}

			// Flags win over parsed values.
			changed := func(name string) bool { return cmd.Flags().Changed(name) }

			if !changed("project") {
				project = parsed.Project
			}
			if project == "" {
				project = cfg.DefaultProject
			}

			if changed("priority") {
				priority = strings.ToLower(strings.TrimSpace(priority))
			} else {
				priority = parsed.Priority
			}
			if priority == "" {
				priority = core.PriorityMedium
			}
			if !core.ValidPriority(priority) {
				return fmt.Errorf("invalid priority %q (want low|medium|high|urgent)", priority)
			}

			var normalizedDue string
			if changed("due") {
				normalizedDue, err = core.DueInput(due)
				if err != nil {
					return err
				}
			} else {
				normalizedDue = parsed.Due
			}

			var finalTags []string
			if changed("tag") {
				finalTags = parseTags(tags)
			} else {
				finalTags = parsed.Tags
			}

			kind = strings.ToLower(strings.TrimSpace(kind))
			if !core.ValidKind(kind) {
				return fmt.Errorf("invalid kind %q (want task|assignment|exam|study|project)", kind)
			}

			p, err := s.GetProject(project)
			if err != nil {
				return fmt.Errorf("project %q: %w — run `mtm project add %s`", project, err, project)
			}
			t := &core.Task{
				Title:           title,
				Project:         project,
				Status:          firstStatusOf(p),
				Priority:        priority,
				Tags:            finalTags,
				Due:             normalizedDue,
				Recurrence:      parsed.Recurrence,
				EstimateMinutes: parsed.EstimateMinutes,
				Kind:            kind,
			}
			if err := s.Create(t); err != nil {
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

func firstStatusOf(p *core.Project) string {
	if p != nil && len(p.Statuses) > 0 {
		return p.Statuses[0].Name
	}
	return "todo"
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
