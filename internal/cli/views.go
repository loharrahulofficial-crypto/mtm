package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/loharrahulofficial-crypto/mtm/internal/config"
	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/spf13/cobra"
)

// builtInViews are always available.
var builtInViews = []struct {
	Name        string
	Description string
}{
	{"today", "due today"},
	{"week", "due in the next 7 days"},
	{"overdue", "due date passed"},
}

func newViewsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "views",
		Short: "List built-in views and saved views from config",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			fmt.Println("built-in:")
			for _, v := range builtInViews {
				fmt.Printf("  %-12s %s  (mtm view %s)\n", v.Name, v.Description, v.Name)
			}
			if len(cfg.SavedViews) == 0 {
				fmt.Println("saved:     none — add them to ~/.config/mtm/config.yaml:")
				fmt.Println("             saved_views:\n               - name: college\n                 filter: project:college/dbms all")
				return nil
			}
			fmt.Println("saved:")
			for _, v := range cfg.SavedViews {
				fmt.Printf("  %-12s %s  (mtm view %s)\n", v.Name, v.Filter, v.Name)
			}
			return nil
		},
	}
}

func newViewCmd() *cobra.Command {
	var asJSON bool
	var all bool
	cmd := &cobra.Command{
		Use:   "view <name>",
		Short: "Run a built-in or saved view",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.ToLower(args[0])
			for _, v := range builtInViews {
				if v.Name == name {
					return runDueWindow(name, all, asJSON)
				}
			}
			cfg, s, err := openStore()
			if err != nil {
				return err
			}
			defer func() { _ = s.Close() }()
			printDiagnostics(s)
			var saved *config.SavedView
			for i := range cfg.SavedViews {
				if strings.EqualFold(cfg.SavedViews[i].Name, name) {
					saved = &cfg.SavedViews[i]
					break
				}
			}
			if saved == nil {
				names := make([]string, 0, len(cfg.SavedViews))
				for _, v := range cfg.SavedViews {
					names = append(names, v.Name)
				}
				if len(names) == 0 {
					return fmt.Errorf("no saved view %q (and no built-in named that). See `mtm views`", name)
				}
				return fmt.Errorf("no saved view %q. Saved: %s", name, strings.Join(names, ", "))
			}
			f, err := core.ParseViewExpr(saved.Filter)
			if err != nil {
				return fmt.Errorf("saved view %q: %w", name, err)
			}
			f.Now = s.Now()
			f.DoneCheck = func(p, st string) bool { return s.IsDoneStatus(p, st) }
			filtered := f.Apply(s.Tasks())
			if asJSON {
				return printTasksJSON(os.Stdout, filtered)
			}
			if len(filtered) == 0 {
				fmt.Printf("view %q: nothing\n", name)
				return nil
			}
			fmt.Printf("view %q\n", name)
			printTasks(os.Stdout, s.Tasks(), filtered, f.DoneCheck, cfg.Sort)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&all, "all", "a", false, "include done tasks (built-in views only)")
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	return cmd
}
