package cli

import (
	"fmt"
	"strings"

	"github.com/loharrahulofficial-crypto/mtm/internal/actions"
	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status <id> <status>",
		Short: "Set a task's status (must exist in the project's workflow)",
		Args:  cobra.ExactArgs(2),
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
			canonical := p.CanonicalStatus(args[1])
			if canonical == "" {
				names := make([]string, len(p.Statuses))
				for i, st := range p.Statuses {
					names[i] = st.Name
				}
				return fmt.Errorf("unknown status %q in project %q (valid: %s)", args[1], t.Project, strings.Join(names, ", "))
			}
			spawnMsg, err := actions.SetStatus(s, t, canonical)
			if err != nil {
				return err
			}
			commitAndSync(cfg, s.Root, "status: "+t.Title)
			fmt.Printf("%s  %s -> %s\n", t.ID, t.Title, canonical)
			if spawnMsg != "" {
				fmt.Println(spawnMsg)
			}
			return nil
		},
	}
}
