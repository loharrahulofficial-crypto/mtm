package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/spf13/cobra"
)

// progressBar renders █/░ cells for a 0-100 percentage.
func progressBar(pct float64, width int) string {
	filled := int(pct * float64(width) / 100)
	if filled > width {
		filled = width
	}
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}

func newShowCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Show a task (id or unique prefix)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, s, err := openStore()
			if err != nil {
				return err
			}
			defer func() { _ = s.Close() }()

			t, err := s.Resolve(args[0])
			if err != nil {
				return err
			}
			if asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(t)
			}
			fmt.Printf("%s  %s\n", t.ID, t.Title)
			fmt.Println(strings.Repeat("-", 40))
			field := func(label, value string) {
				if value != "" {
					fmt.Printf("%-10s %s\n", label+":", value)
				}
			}
			field("project", t.Project)
			field("status", t.Status)
			field("priority", t.Priority)
			field("kind", t.Kind)
			field("due", t.Due)
			field("reminder", t.Reminder)
			field("recurrence", t.Recurrence)
			field("parent", t.Parent)
			if done, total, pct := core.SubtaskProgress(s.Tasks(), t.ID, func(p, st string) bool { return s.IsDoneStatus(p, st) }); total > 0 {
				fmt.Printf("%-10s %d/%d done (%.0f%%) %s\n", "subtasks:", done, total, pct, progressBar(pct, 10))
				for _, c := range s.Tasks() {
					if c.Parent == t.ID {
						fmt.Printf("  - %s [%s] %s\n", c.ID, c.Status, c.Title)
					}
				}
			}
			if len(t.Tags) > 0 {
				fmt.Printf("%-10s %s\n", "tags:", strings.Join(t.Tags, ", "))
			}
			field("created", t.Created)
			field("updated", t.Updated)
			field("completed", t.Completed)
			if t.EstimateMinutes > 0 {
				fmt.Printf("%-10s %d\n", "estimate:", t.EstimateMinutes)
			}
			if t.TimeLoggedMinutes > 0 {
				fmt.Printf("%-10s %d\n", "time:", t.TimeLoggedMinutes)
			}
			if body := strings.TrimRight(t.Body, "\n"); body != "" {
				fmt.Printf("\n%s\n", body)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	return cmd
}
