package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newNoteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "note <id> <text...>",
		Short: "Append a line of notes to a task",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			text := strings.Join(args[1:], " ")
			cfg, s, err := openStore()
			if err != nil {
				return err
			}
			defer func() { _ = s.Close() }()

			t, err := s.Resolve(args[0])
			if err != nil {
				return err
			}
			body := t.Body
			if body != "" && !strings.HasSuffix(body, "\n") {
				body += "\n"
			}
			t.Body = body + "- " + strings.TrimSpace(text) + "\n"
			if err := s.Update(t); err != nil {
				return err
			}
			commitAndSync(cfg, s.Root, "note: "+t.Title)
			fmt.Printf("noted %s  %s\n", t.ID, t.Title)
			return nil
		},
	}
}
