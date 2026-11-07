package cli

import (
	"fmt"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/spf13/cobra"
)

func newDoneCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "done <id>",
		Short: "Mark a task done (project's first done status)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, s, err := openStore()
			if err != nil {
				return err
			}
			defer func() { _ = s.Close() }()

			t, err := s.Resolve(args[0])
			if err != nil {
				return err
			}
			p, err := s.GetProject(t.Project)
			if err != nil {
				return fmt.Errorf("project %q: %w", t.Project, err)
			}
			doneStatus := p.DoneStatus()
			if doneStatus == "" {
				return fmt.Errorf("project %q has no status marked done", t.Project)
			}
			t.Status = doneStatus
			t.Completed = core.NowLocal()
			spawnMsg := spawnNextOccurrence(s, t)
			if err := s.Update(t); err != nil {
				return err
			}
			commitAndSync(cfg, s.Root, "done: "+t.Title)
			fmt.Printf("done %s  %s\n", t.ID, t.Title)
			fmt.Print(spawnMsg)
			return nil
		},
	}
}
