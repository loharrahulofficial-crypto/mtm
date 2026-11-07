package cli

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
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
		Short: "Add a task (flags; natural-language parsing lands in Phase 2)",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			title := strings.Join(args, " ")
			if strings.TrimSpace(title) == "" {
				return errors.New("a title is required, e.g. mtm add \"submit report\"")
			}
			cfg, s, err := openStore()
			if err != nil {
				return err
			}
			defer func() { _ = s.Close() }()

			if project == "" {
				project = cfg.DefaultProject
			}
			priority = strings.ToLower(strings.TrimSpace(priority))
			if !core.ValidPriority(priority) {
				return fmt.Errorf("invalid priority %q (want low|medium|high|urgent)", priority)
			}
			kind = strings.ToLower(strings.TrimSpace(kind))
			if !core.ValidKind(kind) {
				return fmt.Errorf("invalid kind %q (want task|assignment|exam|study|project)", kind)
			}
			normalizedDue, err := normalizeDue(due)
			if err != nil {
				return err
			}
			t := &core.Task{
				Title:    strings.TrimSpace(title),
				Project:  project,
				Status:   firstStatus(s, project),
				Priority: priority,
				Tags:     parseTags(tags),
				Due:      normalizedDue,
				Kind:     kind,
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
	cmd.Flags().StringVar(&priority, "priority", core.PriorityMedium, "low|medium|high|urgent")
	cmd.Flags().StringSliceVarP(&tags, "tag", "t", nil, "tags (comma-separated or repeated)")
	cmd.Flags().StringVar(&due, "due", "", "due date: YYYY-MM-DD or YYYY-MM-DDTHH:MM")
	cmd.Flags().StringVar(&kind, "kind", core.KindTask, "task|assignment|exam|study|project")
	return cmd
}

func firstStatus(s interface {
	GetProject(string) (*core.Project, error)
}, project string) string {
	if p, err := s.GetProject(project); err == nil && len(p.Statuses) > 0 {
		return p.Statuses[0].Name
	}
	return "todo"
}

// normalizeDue accepts date or datetime and stores a canonical form.
func normalizeDue(due string) (string, error) {
	due = strings.TrimSpace(due)
	if due == "" {
		return "", nil
	}
	for _, layout := range []string{core.TimeFormat, "2006-01-02", "2006-01-02 15:04"} {
		if t, err := time.ParseInLocation(layout, due, time.Local); err == nil {
			if layout == "2006-01-02" {
				return t.Format("2006-01-02"), nil
			}
			return t.Format(core.TimeFormat), nil
		}
	}
	return "", fmt.Errorf("invalid due %q (want YYYY-MM-DD or YYYY-MM-DDTHH:MM)", due)
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
