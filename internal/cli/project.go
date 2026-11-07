package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/spf13/cobra"
)

func newProjectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project",
		Short: "Manage projects and their custom statuses",
	}
	cmd.AddCommand(newProjectAddCmd(), newProjectListCmd(), newProjectEditCmd(), newProjectRmCmd())
	return cmd
}

func newProjectAddCmd() *cobra.Command {
	var (
		description string
		color       string
		statuses    string
		done        string
	)
	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Create a project (default statuses todo,doing,done)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			cfg, s, err := openStore()
			if err != nil {
				return err
			}
			defer func() { _ = s.Close() }()

			if s.ProjectExists(name) {
				return fmt.Errorf("project %q already exists", name)
			}
			statusList := splitCSV(statuses)
			if len(statusList) == 0 {
				statusList = []string{"todo", "doing", "done"}
			}
			doneList := splitCSV(done)
			if len(doneList) == 0 {
				doneList = []string{"done"}
			}
			doneSet := map[string]bool{}
			for _, d := range doneList {
				doneSet[strings.ToLower(d)] = true
			}
			p := &core.Project{Name: name, Description: description, Color: color}
			for _, st := range statusList {
				p.Statuses = append(p.Statuses, core.ProjectStatus{Name: st, Done: doneSet[strings.ToLower(st)]})
			}
			if err := p.Validate(); err != nil {
				return err
			}
			for _, d := range doneList {
				if !p.StatusExists(d) {
					return fmt.Errorf("done status %q is not in statuses (%s)", d, strings.Join(statusList, ", "))
				}
			}
			if err := s.SaveProject(p); err != nil {
				return err
			}
			commitAndSync(cfg, s.Root, "project add: "+name)
			fmt.Printf("project %q created with statuses %s\n", name, strings.Join(statusList, " → "))
			return nil
		},
	}
	cmd.Flags().StringVarP(&description, "description", "d", "", "project description")
	cmd.Flags().StringVar(&color, "color", "", "project color (any CSS-like value)")
	cmd.Flags().StringVar(&statuses, "statuses", "", "comma-separated ordered statuses (default todo,doing,done)")
	cmd.Flags().StringVar(&done, "done", "", "comma-separated statuses that count as done (default done)")
	return cmd
}

func newProjectListCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List projects with their statuses and task counts",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, s, err := openStore()
			if err != nil {
				return err
			}
			defer func() { _ = s.Close() }()
			projects, warnings := s.ListProjects()
			for _, w := range warnings {
				note("%s", w)
			}
			if asJSON {
				type row struct {
					Name        string   `json:"name"`
					Description string   `json:"description"`
					Color       string   `json:"color,omitempty"`
					Statuses    []string `json:"statuses"`
					Done        []string `json:"done_statuses"`
					Tasks       int      `json:"tasks"`
				}
				rows := make([]row, 0, len(projects))
				for _, p := range projects {
					r := row{Name: p.Name, Description: p.Description, Color: p.Color, Tasks: s.TaskCount(p.Name)}
					for _, st := range p.Statuses {
						r.Statuses = append(r.Statuses, st.Name)
						if st.Done {
							r.Done = append(r.Done, st.Name)
						}
					}
					rows = append(rows, r)
				}
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(rows)
			}
			if len(projects) == 0 {
				fmt.Println("no projects")
				return nil
			}
			for _, p := range projects {
				names := make([]string, len(p.Statuses))
				for i, st := range p.Statuses {
					names[i] = st.Name
					if st.Done {
						names[i] += "*"
					}
				}
				fmt.Printf("%-20s %3d tasks  %s\n", p.Name, s.TaskCount(p.Name), strings.Join(names, " → "))
			}
			fmt.Println("(* = counts as done)")
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	return cmd
}

func newProjectEditCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "edit <name>",
		Short: "Edit project.yaml in $EDITOR",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			cfg, s, err := openStore()
			if err != nil {
				return err
			}
			defer func() { _ = s.Close() }()
			if !s.ProjectExists(name) {
				return fmt.Errorf("project %q does not exist", name)
			}
			path := filepath.Join(s.Root, "projects", filepath.FromSlash(name), "project.yaml")
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
			// Validate by re-loading from disk; leave the file untouched on failure.
			if err := s.ReloadProject(name); err != nil {
				note("invalid project.yaml: %v", err)
				note("the file was left as you saved it; fix it and re-run `mtm project edit %s`", name)
				return err
			}
			commitAndSync(cfg, s.Root, "project edit: "+name)
			fmt.Printf("updated project %q\n", name)
			return nil
		},
	}
}

func newProjectRmCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "rm <name>",
		Short: "Remove a project directory and its tasks (asks for confirmation)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			cfg, s, err := openStore()
			if err != nil {
				return err
			}
			defer func() { _ = s.Close() }()
			if !s.ProjectExists(name) {
				return fmt.Errorf("project %q does not exist", name)
			}
			n := s.TaskCount(name)
			if !force {
				if err := confirm(fmt.Sprintf("remove project %q and its %d task(s)?", name, n)); err != nil {
					return err
				}
			}
			if err := s.RemoveProject(name); err != nil {
				return err
			}
			commitAndSync(cfg, s.Root, "project rm: "+name)
			fmt.Printf("removed project %q (%d task files deleted; git history retains them)\n", name, n)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false, "remove without confirmation")
	return cmd
}

func splitCSV(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
