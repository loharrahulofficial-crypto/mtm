package cli

import (
	"fmt"
	"strings"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/spf13/cobra"
)

func newSubCmd() *cobra.Command {
	var priority string
	cmd := &cobra.Command{
		Use:   "sub <parent-id> <title...>",
		Short: "Create a subtask under an existing task",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, s, err := openStore()
			if err != nil {
				return err
			}
			defer func() { _ = s.Close() }()

			parent, err := s.Resolve(args[0])
			if err != nil {
				return err
			}
			p, err := s.GetProject(parent.Project)
			if err != nil {
				return fmt.Errorf("project %q: %w", parent.Project, err)
			}
			priority = strings.ToLower(strings.TrimSpace(priority))
			if !core.ValidPriority(priority) {
				return fmt.Errorf("invalid priority %q (want low|medium|high|urgent)", priority)
			}
			first := "todo"
			if len(p.Statuses) > 0 {
				first = p.Statuses[0].Name
			}
			child := &core.Task{
				Title:    strings.Join(args[1:], " "),
				Project:  parent.Project,
				Status:   first,
				Priority: priority,
				Kind:     core.KindTask,
				Parent:   parent.ID,
			}
			if err := s.Create(child); err != nil {
				return err
			}
			commitAndSync(cfg, s.Root, "sub: "+child.Title)
			fmt.Printf("added %s  %s (sub of %s)\n", child.ID, child.Title, parent.ID)
			return nil
		},
	}
	cmd.Flags().StringVar(&priority, "priority", core.PriorityMedium, "low|medium|high|urgent")
	return cmd
}
