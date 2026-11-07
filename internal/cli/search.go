package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/spf13/cobra"
)

func newSearchCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "search <query...>",
		Short: "Full-text search over title, notes and tags",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.Join(args, " ")
			if strings.TrimSpace(query) == "" {
				return fmt.Errorf("a query is required, e.g. mtm search \"dbms\"")
			}
			_, s, err := openStore()
			if err != nil {
				return err
			}
			defer func() { _ = s.Close() }()
			printDiagnostics(s)

			needle := strings.ToLower(query)
			var matches []*core.Task
			for _, t := range s.Tasks() {
				haystack := strings.ToLower(strings.Join([]string{
					t.Title,
					t.Body,
					strings.Join(t.Tags, " "),
				}, "\n"))
				if strings.Contains(haystack, needle) {
					matches = append(matches, t)
				}
			}
			if asJSON {
				return printTasksJSON(os.Stdout, matches)
			}
			if len(matches) == 0 {
				fmt.Printf("nothing matches %q\n", query)
				return nil
			}
			fmt.Printf("%d match(es)\n", len(matches))
			printTasks(os.Stdout, s.Tasks(), matches, func(p, st string) bool { return s.IsDoneStatus(p, st) })
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	return cmd
}
