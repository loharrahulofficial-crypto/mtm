package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/loharrahulofficial-crypto/mtm/internal/actions"
)

func newExamCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "exam",
		Short: "Manage exams (kind:exam tasks tied to a course)",
	}
	cmd.AddCommand(newExamAddCmd(), newExamListCmd(), newExamPlanCmd())
	return cmd
}

func newExamAddCmd() *cobra.Command {
	var (
		course, date, topics string
		weight               string
	)
	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Create an exam under a course",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, s, err := openStore()
			if err != nil {
				return err
			}
			defer func() { _ = s.Close() }()
			name := strings.TrimSpace(args[0])
			if course == "" {
				return fmt.Errorf("--course is required")
			}
			if date == "" {
				now := s.Now()
				date = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Format("2006-01-02")
			}
			w, werr := parseWeight(weight)
			if werr != nil {
				return werr
			}
			_, err = actions.AddExam(s, course, name, date, w, splitCSV(topics))
			if err != nil {
				return err
			}
			commitAndSync(cfg, s.Root, "exam add: "+name)
			fmt.Printf("exam added under %s\n", course)
			return nil
		},
	}
	cmd.Flags().StringVar(&course, "course", "", "course (path or leaf name, e.g. dbms)")
	cmd.Flags().StringVar(&date, "date", "", "exam date 2006-01-02 (default: today)")
	cmd.Flags().StringVar(&weight, "weight", "", "grade weight % (optional)")
	cmd.Flags().StringVar(&topics, "topics", "", "comma-separated topics, e.g. ER,SQL,normalization")
	return cmd
}

func newExamListCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List upcoming exams with countdowns",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, s, err := openStore()
			if err != nil {
				return err
			}
			defer func() { _ = s.Close() }()
			exams := actions.ListExams(s)
			now := s.Clock.Now()
			if asJSON {
				type row struct {
					ID        string   `json:"id"`
					Course    string   `json:"course"`
					Title     string   `json:"title"`
					Due       string   `json:"due"`
					Weight    *float64 `json:"weight,omitempty"`
					Countdown string   `json:"countdown"`
				}
				rows := make([]row, 0, len(exams))
				for _, e := range exams {
					rows = append(rows, row{ID: e.ID, Course: e.Project, Title: e.Title, Due: e.Due, Weight: e.Weight, Countdown: countdownLabel(e.Due, now)})
				}
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(rows)
			}
			if len(exams) == 0 {
				fmt.Println("no upcoming exams — create one with `mtm exam add`")
				return nil
			}
			fmt.Printf("%-8s %-10s %-28s %-12s %s\n", "id", "course", "title", "due", "countdown")
			for _, e := range exams {
				fmt.Printf("%-8s %-10s %-28s %-12s %s\n", shortID(e.ID), e.Project, truncateRunes(e.Title, 28), e.Due, countdownLabel(e.Due, now))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	return cmd
}

func newExamPlanCmd() *cobra.Command {
	var est int
	cmd := &cobra.Command{
		Use:   "plan <id>",
		Short: "Generate kind:study tasks, one per exam topic, spread before the exam",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, s, err := openStore()
			if err != nil {
				return err
			}
			defer func() { _ = s.Close() }()
			created, err := actions.PlanExam(s, args[0], est)
			if err != nil {
				return err
			}
			if len(created) == 0 {
				fmt.Println("already planned (or nothing to plan) — re-running is a no-op")
				return nil
			}
			commitAndSync(cfg, s.Root, "exam plan: "+args[0])
			fmt.Printf("planned %d study session(s)\n", len(created))
			for _, t := range created {
				fmt.Printf("  %s  %s  due %s  est %d min\n", shortID(t.ID), t.Title, t.Due, t.EstimateMinutes)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&est, "est", 60, "estimate minutes per study session")
	return cmd
}

// parseWeight parses an optional weight string to *float64; empty -> nil.
// Anything else must parse as a non-negative number.
func parseWeight(s string) (*float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil, fmt.Errorf("weight %q must be a number", s)
	}
	if f < 0 || f > 100 {
		return nil, fmt.Errorf("weight %v must be between 0 and 100", f)
	}
	return &f, nil
}

// countdownLabel renders an always-relative countdown for the exam list:
// "today", "tomorrow", "in 6 days", "3 days overdue". Unlike gauges.DueLabel
// it never falls back to a bare date, because a countdown is the point of
// `mtm exam list`.
func countdownLabel(due string, now time.Time) string {
	loc := now.Location()
	day, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(due), loc)
	if err != nil {
		return due
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	days := int(day.Sub(today).Hours() / 24)
	switch {
	case days < -1:
		return fmt.Sprintf("%d days overdue", -days)
	case days == -1:
		return "1 day overdue"
	case days == 0:
		return "today"
	case days == 1:
		return "tomorrow"
	default:
		return fmt.Sprintf("in %d days", days)
	}
}

// shortID shows the first 6 chars of a task id.
func shortID(id string) string {
	if len(id) >= 6 {
		return id[:6]
	}
	return id
}
