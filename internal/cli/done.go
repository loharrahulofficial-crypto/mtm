package cli

import (
	"fmt"

	"github.com/loharrahulofficial-crypto/mtm/internal/actions"
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
			spawnMsg, err := actions.CompleteTask(s, t)
			if err != nil {
				return err
			}
			commitAndSync(cfg, s.Root, "done: "+t.Title)
			fmt.Printf("done %s  %s\n", t.ID, t.Title)
			if spawnMsg != "" {
				fmt.Println(spawnMsg)
			}
			return nil
		},
	}
}
