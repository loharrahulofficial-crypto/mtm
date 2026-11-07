package cli

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// confirm asks a yes/no question. Non-interactive stdin answers no.
func confirm(prompt string) error {
	fi, err := os.Stdin.Stat()
	if err != nil || fi.Mode()&os.ModeCharDevice == 0 {
		return errors.New("confirmation required — re-run with --force")
	}
	fmt.Fprintf(os.Stderr, "%s [y/N] ", prompt)
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return err
	}
	if strings.EqualFold(strings.TrimSpace(line), "y") || strings.EqualFold(strings.TrimSpace(line), "yes") {
		return nil
	}
	return errors.New("aborted")
}

func newRmCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "rm <id>",
		Short: "Remove a task (asks for confirmation)",
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
			if !force {
				if err := confirm(fmt.Sprintf("remove \"%s\"?", t.Title)); err != nil {
					return err
				}
			}
			if err := s.Remove(t); err != nil {
				return err
			}
			commitAndSync(cfg, s.Root, "rm: "+t.Title)
			fmt.Printf("removed %s  %s\n", t.ID, t.Title)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false, "remove without confirmation")
	return cmd
}
