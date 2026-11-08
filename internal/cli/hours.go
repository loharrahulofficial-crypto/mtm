package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/loharrahulofficial-crypto/mtm/internal/actions"
)

func newHoursCmd() *cobra.Command {
	var (
		course string
		week   bool
		asJSON bool
	)
	cmd := &cobra.Command{
		Use:   "hours",
		Short: "Study time rolled up per course",
		Long: `Time logged on tasks (e.g. by the pomodoro timer) rolled up per course.
Without a flag, totals cover all time logged. --week restricts to the
trailing 7 days, attributing a task's logged minutes to its completion day
(the same rule as the Home weekly chart). --course adds a per-task breakdown.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, s, err := openStore()
			if err != nil {
				return err
			}
			defer func() { _ = s.Close() }()
			rep, err := actions.CollectHours(s, s.Now())
			if err != nil {
				return err
			}
			if course != "" {
				name, err := actions.ResolveCourse(s, course)
				if err != nil {
					return err
				}
				for _, r := range rep.Courses {
					if r.Course == name {
						if asJSON {
							enc := json.NewEncoder(cmd.OutOrStdout())
							enc.SetIndent("", "  ")
							return enc.Encode(r)
						}
						printHoursCourse(r, week)
						for _, t := range r.Tasks {
							fmt.Printf("    %-8s %-40s %s\n", shortID(t.ID), truncateRunes(t.Title, 40), actions.FormatMinutes(t.TimeLoggedMinutes))
						}
						return nil
					}
				}
				fmt.Printf("%s — no time logged yet\n", name)
				return nil
			}
			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(rep)
			}
			weekTotal := 0
			loggedAny := false
			if week {
				fmt.Println("last 7 days")
			}
			for _, r := range rep.Courses {
				logged := r.LoggedMinutes
				if week {
					logged = r.WeekMinutes
					weekTotal += r.WeekMinutes
				}
				if logged == 0 && r.EstimateMinutes == 0 && !week {
					continue
				}
				if logged == 0 && week {
					continue
				}
				loggedAny = true
				fmt.Printf("%-24s %s logged", truncateRunes(r.Course, 24), actions.FormatMinutes(logged))
				if !week {
					fmt.Printf("  est %s", actions.FormatMinutes(r.EstimateMinutes))
				}
				fmt.Println()
			}
			other := rep.OtherLoggedMinutes
			otherLabel := actions.FormatMinutes(other)
			if week {
				other = rep.OtherWeekMinutes
				otherLabel = actions.FormatMinutes(other)
				weekTotal += other
			}
			if other > 0 {
				loggedAny = true
				fmt.Printf("%-24s %s logged\n", "other projects", otherLabel)
			}
			if !loggedAny {
				fmt.Println("no time logged yet — start a timer on a task (mtm timer start <id>)")
				return nil
			}
			if week {
				fmt.Printf("%-24s %s\n", "total", actions.FormatMinutes(weekTotal))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&course, "course", "", "limit to one course (adds a per-task breakdown)")
	cmd.Flags().BoolVar(&week, "week", false, "only the trailing 7 days (by completion day)")
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	return cmd
}

func printHoursCourse(r actions.HoursRollup, week bool) {
	if week {
		fmt.Printf("%s — %s this week", r.Course, actions.FormatMinutes(r.WeekMinutes))
		fmt.Printf(" (all time: %s)\n", actions.FormatMinutes(r.LoggedMinutes))
		return
	}
	fmt.Printf("%s — %s logged", r.Course, actions.FormatMinutes(r.LoggedMinutes))
	fmt.Printf("  est %s", actions.FormatMinutes(r.EstimateMinutes))
	if r.EstimateMinutes > 0 {
		pct := float64(r.LoggedMinutes) / float64(r.EstimateMinutes) * 100
		fmt.Printf(" (%.0f%% of estimate)", pct)
	}
	fmt.Println()
}
