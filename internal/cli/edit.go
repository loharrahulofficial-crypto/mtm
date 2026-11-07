package cli

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

// openEditor opens path in the configured editor, inheriting the terminal.
func openEditor(cfg interface{ EditorCmd() string }, path string) error {
	fields := strings.Fields(cfg.EditorCmd())
	if len(fields) == 0 {
		return fmt.Errorf("no editor configured (set $EDITOR or config editor)")
	}
	args := append(fields[1:], path)
	cmd := exec.Command(fields[0], args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func newEditCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "edit <id>",
		Short: "Edit a task file in $EDITOR",
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
			path, err := s.Path(t.ID)
			if err != nil {
				return err
			}
			before, _ := os.ReadFile(path)
			if err := openEditor(cfg, path); err != nil {
				return fmt.Errorf("editor %q: %w", cfg.EditorCmd(), err)
			}
			after, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if bytes.Equal(before, after) {
				fmt.Println("no changes")
				return nil
			}
			// Re-parse to validate; on failure leave the file untouched
			// and do not commit broken state.
			if err := s.Reparse(t); err != nil {
				note("could not parse %s after edit: %v", path, err)
				note("the file was left as you saved it; fix it and re-run `mtm edit %s`", args[0])
				return err
			}
			if err := s.Update(t); err != nil {
				return err
			}
			commitAndSync(cfg, s.Root, "edit: "+t.Title)
			fmt.Printf("updated %s  %s\n", t.ID, t.Title)
			return nil
		},
	}
}
