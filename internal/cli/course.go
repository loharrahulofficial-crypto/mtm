package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/loharrahulofficial-crypto/mtm/internal/actions"
	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/loharrahulofficial-crypto/mtm/internal/store"
)

func newCourseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "course",
		Short: "Manage academic courses (projects marked as courses)",
	}
	cmd.AddCommand(newCourseAddCmd(), newCourseListCmd(), newCourseShowCmd(), newCourseEditCmd())
	return cmd
}

func newCourseAddCmd() *cobra.Command {
	var (
		code, name, semester, targetGrade, color string
		credits                                  int
		examDates                                []string
		statuses, done                           string
	)
	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Create a course (a project with a course block in project.yaml)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			project := args[0]
			if strings.TrimSpace(code) == "" || strings.TrimSpace(name) == "" {
				return fmt.Errorf("--code and --name are required, e.g. mtm course add college/dbms --code DBMS --name \"Database Systems\"")
			}
			cfg, s, err := openStore()
			if err != nil {
				return err
			}
			defer func() { _ = s.Close() }()

			if s.ProjectExists(project) {
				return fmt.Errorf("project %q already exists", project)
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
			p := &core.Project{Name: project, Color: color}
			for _, st := range statusList {
				p.Statuses = append(p.Statuses, core.ProjectStatus{Name: st, Done: doneSet[strings.ToLower(st)]})
			}
			for _, d := range doneList {
				if !p.StatusExists(d) {
					return fmt.Errorf("done status %q is not in statuses (%s)", d, strings.Join(statusList, ", "))
				}
			}
			p.Course = &core.Course{
				Code:        strings.ToUpper(strings.TrimSpace(code)),
				Name:        strings.TrimSpace(name),
				Semester:    strings.TrimSpace(semester),
				Credits:     credits,
				TargetGrade: strings.TrimSpace(targetGrade),
				ExamDates:   splitCSV(strings.Join(examDates, ",")),
			}
			if err := p.Validate(); err != nil {
				return err
			}
			if err := s.SaveProject(p); err != nil {
				return err
			}
			commitAndSync(cfg, s.Root, "course add: "+project)
			fmt.Printf("course %q created (%s — %s)\n", project, p.Course.Code, p.Course.Name)
			return nil
		},
	}
	cmd.Flags().StringVar(&code, "code", "", "course code, e.g. DBMS")
	cmd.Flags().StringVar(&name, "name", "", "course title, e.g. \"Database Systems\"")
	cmd.Flags().StringVar(&semester, "semester", "", "semester label, e.g. 2026-Fall")
	cmd.Flags().IntVar(&credits, "credits", 0, "credit hours")
	cmd.Flags().StringVar(&targetGrade, "target-grade", "", "target grade (A, B+, … or a percentage)")
	cmd.Flags().StringArrayVar(&examDates, "exam-date", nil, "exam date YYYY-MM-DD (repeatable)")
	cmd.Flags().StringVar(&color, "color", "", "project color (any CSS-like value)")
	cmd.Flags().StringVar(&statuses, "statuses", "", "comma-separated ordered statuses (default todo,doing,done)")
	cmd.Flags().StringVar(&done, "done", "", "comma-separated statuses that count as done (default done)")
	return cmd
}

func newCourseListCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List courses with progress bars",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, s, err := openStore()
			if err != nil {
				return err
			}
			defer func() { _ = s.Close() }()
			courses, err := actions.ListCourses(s)
			if err != nil {
				return err
			}
			stats, err := collectAllCourseStats(s, courses)
			if err != nil {
				return err
			}
			if asJSON {
				return printCoursesJSON(stats)
			}
			if len(stats) == 0 {
				fmt.Println("no courses yet — create one with `mtm course add`")
				return nil
			}
			for _, st := range stats {
				c := st.Project.Course
				fmt.Printf("%-6s %-26s %-10s %2dcr  target %-4s  %s\n",
					c.Code, truncateRunes(c.Name, 26)+" ["+st.Project.Name+"]", semeOrDash(c.Semester), c.Credits,
					gradeOrDash(c.TargetGrade), bar(st.Overall, 12))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	return cmd
}

func newCourseShowCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "show <course>",
		Short: "Show a course: metadata, progress bars, workload",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, s, err := openStore()
			if err != nil {
				return err
			}
			defer func() { _ = s.Close() }()
			name, err := actions.ResolveCourse(s, args[0])
			if err != nil {
				return err
			}
			st, err := actions.CollectCourseStats(s, name)
			if err != nil {
				return err
			}
			if asJSON {
				return printCoursesJSON([]*actions.CourseStats{st})
			}
			printCourse(st)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	return cmd
}

func newCourseEditCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "edit <course>",
		Short: "Edit the course block (project.yaml) in $EDITOR",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, s, err := openStore()
			if err != nil {
				return err
			}
			defer func() { _ = s.Close() }()
			name, err := actions.ResolveCourse(s, args[0])
			if err != nil {
				return err
			}
			path := filepath.Join(s.Root, "projects", filepath.FromSlash(name), "project.yaml")
			changed, err := editorEdited(cfg, path, "project.yaml", func() error { return s.ReloadProject(name) })
			if err != nil {
				return err
			}
			if !changed {
				fmt.Println("no changes")
				return nil
			}
			commitAndSync(cfg, s.Root, "course edit: "+name)
			fmt.Printf("updated course %q\n", name)
			return nil
		},
	}
}

func collectAllCourseStats(s *store.Store, courses []*core.Project) ([]*actions.CourseStats, error) {
	out := make([]*actions.CourseStats, 0, len(courses))
	for _, c := range courses {
		st, err := actions.CollectCourseStats(s, c.Name)
		if err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, nil
}

func printCoursesJSON(stats []*actions.CourseStats) error {
	type row struct {
		Project          string   `json:"project"`
		Code             string   `json:"code"`
		Name             string   `json:"name"`
		Semester         string   `json:"semester,omitempty"`
		Credits          int      `json:"credits,omitempty"`
		TargetGrade      string   `json:"target_grade,omitempty"`
		ExamDates        []string `json:"exam_dates,omitempty"`
		Tasks            int      `json:"tasks"`
		TasksDone        int      `json:"tasks_done"`
		Progress         float64  `json:"progress"`
		WeightedProgress *float64 `json:"weighted_progress,omitempty"`
	}
	rows := make([]row, 0, len(stats))
	for _, st := range stats {
		c := st.Project.Course
		r := row{
			Project: st.Project.Name, Code: c.Code, Name: c.Name, Semester: c.Semester,
			Credits: c.Credits, TargetGrade: c.TargetGrade, ExamDates: c.ExamDates,
			Tasks: st.Overall.Total, TasksDone: st.Overall.Done, Progress: round3(st.Overall.Fraction),
		}
		if st.HasWeights {
			w := round3(st.OverallWeighted.Fraction)
			r.WeightedProgress = &w
		}
		rows = append(rows, r)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(rows)
}

func printCourse(st *actions.CourseStats) {
	p := st.Project
	c := p.Course
	fmt.Printf("%s — %s\n", p.Name, c.Name)
	fmt.Printf("code        %s\n", c.Code)
	if c.Semester != "" {
		fmt.Printf("semester    %s\n", c.Semester)
	}
	if c.Credits > 0 {
		fmt.Printf("credits     %d\n", c.Credits)
	}
	if c.TargetGrade != "" {
		if pct, err := core.GradeToPercent(c.TargetGrade); err == nil {
			fmt.Printf("target      %s (%.0f%%)\n", c.TargetGrade, pct)
		} else {
			fmt.Printf("target      %s\n", c.TargetGrade)
		}
	}
	if p.Description != "" {
		fmt.Printf("about       %s\n", p.Description)
	}
	if len(c.ExamDates) > 0 {
		fmt.Printf("exam dates  %s\n", strings.Join(c.ExamDates, ", "))
	}
	fmt.Printf("tasks       %d total · %d done\n", st.Overall.Total, st.Overall.Done)
	fmt.Printf("overall     %s\n", bar(st.Overall, 24))
	if st.HasWeights {
		fmt.Printf("weighted    %s\n", bar(st.OverallWeighted, 24))
	}
	for _, kind := range []string{core.KindAssignment, core.KindExam, core.KindStudy} {
		if k, ok := st.ByKind[kind]; ok {
			fmt.Printf("%-12s %s\n", kind+"s", bar(k, 24))
		}
	}
}

func semeOrDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

func gradeOrDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

func round3(f float64) float64 { return float64(int(f*1000+0.5)) / 1000 }

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 1 {
		return string(r[:n])
	}
	return string(r[:n-1]) + "…"
}
