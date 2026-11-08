package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/spf13/cobra"
)

// printTasks renders filtered with subtask tree indentation, sorted per cfg.
// all is the full store slice (for subtask progress counts).
func printTasks(w io.Writer, all []*core.Task, filtered []*core.Task, isDone func(string, string) bool, sortKeys []string) {
	core.SortTasks(filtered, sortKeys, isDone)
	head := map[string][2]int{}
	for _, t := range all {
		if t.Parent == "" {
			continue
		}
		c := head[t.Parent]
		c[1]++
		done := core.IsDoneStatusFallback(t.Status)
		if isDone != nil {
			done = isDone(t.Project, t.Status)
		}
		if done {
			c[0]++
		}
		head[t.Parent] = c
	}
	inFiltered := map[string]bool{}
	for _, t := range filtered {
		inFiltered[t.ID] = true
	}
	children := map[string][]*core.Task{}
	var roots []*core.Task
	for _, t := range filtered {
		if t.Parent != "" && inFiltered[t.Parent] {
			children[t.Parent] = append(children[t.Parent], t)
			continue
		}
		roots = append(roots, t)
	}
	tab := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	for _, r := range roots {
		printNode(tab, head, children, r, 0)
	}
	_ = tab.Flush()
}

func printNode(tab *tabwriter.Writer, head map[string][2]int, children map[string][]*core.Task, t *core.Task, depth int) {
	indent := strings.Repeat("  ", depth)
	if depth > 0 {
		indent += "↳ "
	}
	due := t.Due
	if due == "" {
		due = "-"
	}
	title := t.Title
	if t.Archived {
		title += " [archived]"
	}
	if c, ok := head[t.ID]; ok {
		title = fmt.Sprintf("%s [%d/%d]", title, c[0], c[1])
	}
	fmt.Fprintf(tab, "%s%s\t%s\t%s\t%s\t%s\t%s\n", indent, t.ID, t.Priority, t.Status, due, title, t.Project)
	for _, c := range children[t.ID] {
		printNode(tab, head, children, c, depth+1)
	}
}

func printTasksJSON(w io.Writer, filtered []*core.Task) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(filtered)
}

// runDueWindow is shared by today/week/overdue.
func runDueWindow(window string, all bool, asJSON bool) error {
	cfg, s, err := openStore()
	if err != nil {
		return err
	}
	defer func() { _ = s.Close() }()
	printDiagnostics(s)
	f := core.Filter{
		Due:              window,
		IncludeCompleted: all,
		Now:              s.Now(),
		DoneCheck:        func(p, st string) bool { return s.IsDoneStatus(p, st) },
	}
	filtered := f.Apply(s.Tasks())
	if asJSON {
		return printTasksJSON(os.Stdout, filtered)
	}
	if len(filtered) == 0 {
		fmt.Printf("nothing %s\n", window)
		return nil
	}
	printTasks(os.Stdout, s.Tasks(), filtered, f.DoneCheck, cfg.Sort)
	return nil
}

func newListCmd() *cobra.Command {
	var (
		project  string
		tag      string
		status   string
		priority string
		due      string
		all      bool
		asJSON   bool
		archived bool
		weighted bool
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List tasks (hides done ones unless --all)",
		RunE: func(cmd *cobra.Command, args []string) error {
			due = strings.ToLower(strings.TrimSpace(due))
			if !core.ValidDue(due) {
				return fmt.Errorf("invalid --due %q (want today|week|overdue)", due)
			}
			cfg, s, err := openStoreOpts(archived)
			if err != nil {
				return err
			}
			defer func() { _ = s.Close() }()
			printDiagnostics(s)

			if archived {
				all = true // archived tasks are done; keep them visible
			}
			f := taskFilter(s, project, tag, status, priority, all)
			f.Due = due
			f.Now = s.Now()
			filtered := f.Apply(s.Tasks())
			if archived {
				filtered = archivedOnly(filtered)
			}
			if asJSON {
				return printTasksJSON(os.Stdout, filtered)
			}
			if len(filtered) == 0 {
				fmt.Println("no tasks")
				return nil
			}
			printTasks(os.Stdout, s.Tasks(), filtered, f.DoneCheck, cfg.Sort)
			printListProgress(os.Stdout, s.Tasks(), filtered, f.DoneCheck, weighted)
			return nil
		},
	}
	cmd.Flags().StringVarP(&project, "project", "p", "", "filter by project")
	cmd.Flags().StringVarP(&tag, "tag", "t", "", "filter by tag")
	cmd.Flags().StringVar(&status, "status", "", "filter by status")
	cmd.Flags().StringVar(&priority, "priority", "", "filter by priority")
	cmd.Flags().StringVar(&due, "due", "", "filter by due: today|week|overdue")
	cmd.Flags().BoolVarP(&all, "all", "a", false, "include done tasks")
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	cmd.Flags().BoolVar(&archived, "archived", false, "list archived tasks (instead of active ones)")
	cmd.Flags().BoolVar(&weighted, "weighted", false, "weight the project bars by assignment/exam grade weights")
	return cmd
}

// listProgressLines builds the per-project bar footer shown after `mtm
// list`. Only projects present in the filtered output get a line. Bars
// combine subtask completion; --weighted switches to weight-weighted
// fractions over the assignment/exam tasks of each project.
func listProgressLines(tasks, filtered []*core.Task, isDone func(string, string) bool, weighted bool) []string {
	seen := map[string]bool{}
	for _, t := range filtered {
		seen[t.Project] = true
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	var lines []string
	for _, name := range names {
		var proj []*core.Task
		for _, t := range tasks {
			if strings.EqualFold(t.Project, name) {
				proj = append(proj, t)
			}
		}
		p := core.ProgressFraction(proj, func(t *core.Task) bool { return isDone(t.Project, t.Status) }, false)
		label := name
		if weighted {
			var weightedTasks []*core.Task
			for _, t := range proj {
				if !strings.EqualFold(t.Kind, core.KindAssignment) && !strings.EqualFold(t.Kind, core.KindExam) {
					continue
				}
				if t.Weight == nil || *t.Weight <= 0 {
					continue
				}
				weightedTasks = append(weightedTasks, t)
			}
			if len(weightedTasks) == 0 {
				lines = append(lines, fmt.Sprintf("  %-24s - (no weighted assignments/exams)", truncateRunes(label, 24)))
				continue
			}
			wp := core.ProgressFraction(weightedTasks, func(t *core.Task) bool { return isDone(t.Project, t.Status) }, true)
			lines = append(lines, fmt.Sprintf("  %-24s %s", truncateRunes(label, 24), bar(wp, 24)+" (weighted)"))
			continue
		}
		lines = append(lines, fmt.Sprintf("  %-24s %s", truncateRunes(label, 24), bar(p, 24)))
	}
	return lines
}

func printListProgress(w io.Writer, all, filtered []*core.Task, isDone func(string, string) bool, weighted bool) {
	lines := listProgressLines(all, filtered, isDone, weighted)
	if len(lines) == 0 {
		return
	}
	fmt.Fprintln(w, "progress")
	for _, l := range lines {
		fmt.Fprintln(w, l)
	}
}

// archivedOnly keeps only tasks loaded from archive/.
func archivedOnly(in []*core.Task) []*core.Task {
	out := make([]*core.Task, 0, len(in))
	for _, t := range in {
		if t.Archived {
			out = append(out, t)
		}
	}
	return out
}

func newTodayCmd() *cobra.Command {
	var all bool
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "today",
		Short: "Tasks due today",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDueWindow("today", all, asJSON)
		},
	}
	cmd.Flags().BoolVarP(&all, "all", "a", false, "include done tasks")
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	return cmd
}

func newWeekCmd() *cobra.Command {
	var all bool
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "week",
		Short: "Tasks due in the next 7 days",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDueWindow("week", all, asJSON)
		},
	}
	cmd.Flags().BoolVarP(&all, "all", "a", false, "include done tasks")
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	return cmd
}

func newOverdueCmd() *cobra.Command {
	var all bool
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "overdue",
		Short: "Tasks whose due date passed",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDueWindow("overdue", all, asJSON)
		},
	}
	cmd.Flags().BoolVarP(&all, "all", "a", false, "include done tasks")
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	return cmd
}
