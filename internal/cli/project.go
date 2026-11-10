package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/loharrahulofficial-crypto/mtm/internal/actions"
	"github.com/loharrahulofficial-crypto/mtm/internal/config"
	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/spf13/cobra"
)

func newProjectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project",
		Short: "Manage projects and their custom statuses",
	}
	cmd.AddCommand(newProjectAddCmd(), newProjectListCmd(), newProjectEditCmd(), newProjectRmCmd(), newProjectLinkCmd())
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
			// Validate by re-loading from disk; leave the file untouched on failure.
			changed, err := editorEdited(cfg, path, "project.yaml", func() error { return s.ReloadProject(name) })
			if err != nil {
				return err
			}
			if !changed {
				fmt.Println("no changes")
				return nil
			}
			commitAndSync(cfg, s.Root, "project edit: "+name)
			fmt.Printf("updated project %q\n", name)
			return nil
		},
	}
}

func newProjectLinkCmd() *cobra.Command {
	var (
		repo string
		path string
	)
	cmd := &cobra.Command{
		Use:   "link <project>",
		Short: "Link a project to a git repo for read-only stats (never writes to the repo)",
		Long: `Link a project to the repository that implements it. The data repo is
never written — mtm only reads: git log for a 12-week commit sparkline, the
last commit time, the last 5 subjects and a commit streak. When the linked
repo has an 'owner/name' slug and gh is installed and authenticated, mtm
also shows open issue and PR counts and the latest few (cached in the
cache dir for 15 minutes).

  mtm project link college/dbms --path ~/Projects/dbms        # local only
  mtm project link college/dbms --repo you/dbms --path ~/Projects/dbms`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, s, err := openStore()
			if err != nil {
				return err
			}
			defer func() { _ = s.Close() }()

			project, err := actions.ResolveProject(s, args[0])
			if err != nil {
				return err
			}
			repo = strings.TrimSpace(repo)
			path = strings.TrimSpace(path)
			if repo == "" && path == "" {
				return fmt.Errorf("nothing to link — pass --repo owner/name and/or --path <dir>")
			}
			if repo != "" && !repoSlug.MatchString(repo) {
				return fmt.Errorf("--repo %q must look like owner/name (letters, digits, . _ -)", repo)
			}
			if path != "" {
				expanded := config.ExpandPath(path)
				if info, err := os.Stat(expanded); err != nil || !info.IsDir() {
					return fmt.Errorf("--path %q is not a directory", path)
				}
				gitDir := filepath.Join(expanded, ".git")
				if info, err := os.Stat(gitDir); err != nil || !info.IsDir() {
					// Worktrees and submodules have .git as a file.
					if info, err := os.Stat(gitDir); err != nil || !info.Mode().IsRegular() {
						return fmt.Errorf("--path %q is not a git repository (no .git found)", path)
					}
				}
			}

			p, err := s.GetProject(project)
			if err != nil {
				return err
			}
			if repo != "" {
				p.GitHub = repo
			}
			if path != "" {
				p.RepoPath = path // stored as given (~ expands on read)
			}
			if err := s.SaveProject(p); err != nil {
				return err
			}
			commitAndSync(cfg, s.Root, "project link: "+project)

			fmt.Printf("project %q linked\n", project)
			if p.GitHub != "" {
				fmt.Printf("  github:    %s\n", p.GitHub)
			}
			if p.RepoPath != "" {
				fmt.Printf("  repo_path: %s\n", p.RepoPath)
			}
			// Summarize what the link unlocks (local always; remote best effort).
			now := s.Now()
			if info, err := actions.LoadGitHub(s, project, now, true); err == nil {
				if info.LocalOK {
					total := 0
					for _, n := range info.Weeks {
						total += int(n)
					}
					fmt.Printf("local:     %d commits/12w · last %s · %dd streak\n", total, relativeStamp(now, info.LastCommit), info.Streak)
				} else if info.LocalNote != "" {
					fmt.Printf("local:     %s\n", info.LocalNote)
				}
				if info.RemoteOK {
					fmt.Printf("remote:    %d open issues · %d open PRs\n", info.OpenIssues, info.OpenPRs)
				} else if info.RemoteNote != "" {
					fmt.Printf("remote:    %s\n", info.RemoteNote)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&repo, "repo", "", "owner/name slug (enables open issue/PR counts via gh)")
	cmd.Flags().StringVar(&path, "path", "", "local path of the repository (enables commit reads; ~ expands)")
	return cmd
}

var repoSlug = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*/[A-Za-z0-9][A-Za-z0-9._-]*$`)

// relativeStamp renders a compact "2d ago"-style label for link output.
func relativeStamp(now, ts time.Time) string {
	if ts.IsZero() {
		return "never"
	}
	days := int(now.Sub(ts).Hours() / 24)
	switch {
	case days <= 0:
		return "today"
	case days == 1:
		return "yesterday"
	default:
		return fmt.Sprintf("%dd ago", days)
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
