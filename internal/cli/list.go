package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	var (
		project  string
		tag      string
		status   string
		priority string
		all      bool
		asJSON   bool
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List tasks (hides done ones unless --all)",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, s, err := openStore()
			if err != nil {
				return err
			}
			defer func() { _ = s.Close() }()
			printDiagnostics(s)

			filtered := taskFilter(s, project, tag, status, priority, all).Apply(s.Tasks())
			if asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(filtered)
			}
			if len(filtered) == 0 {
				fmt.Println("no tasks")
				return nil
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			for _, t := range filtered {
				due := t.Due
				if due == "" {
					due = "-"
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", t.ID, t.Priority, t.Status, due, t.Title, t.Project)
			}
			return w.Flush()
		},
	}
	cmd.Flags().StringVarP(&project, "project", "p", "", "filter by project")
	cmd.Flags().StringVarP(&tag, "tag", "t", "", "filter by tag")
	cmd.Flags().StringVar(&status, "status", "", "filter by status")
	cmd.Flags().StringVar(&priority, "priority", "", "filter by priority")
	cmd.Flags().BoolVarP(&all, "all", "a", false, "include done tasks")
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	return cmd
}
