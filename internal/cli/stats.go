package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/loharrahulofficial-crypto/mtm/internal/actions"
	"github.com/loharrahulofficial-crypto/mtm/internal/config"
	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/loharrahulofficial-crypto/mtm/internal/dates"
	"github.com/loharrahulofficial-crypto/mtm/internal/gauges"
	"github.com/loharrahulofficial-crypto/mtm/internal/text"
)

func newStatsCmd() *cobra.Command {
	var (
		week     bool
		month    bool
		semester bool
		since    string
	)
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Completion stats for a period (default: trailing 7 days)",
		Long: `Stats for a period: completed tasks, on-time rate, hours by course,
busiest day, estimate accuracy (only tasks with estimates; the percentage
hides below 3 estimated tasks), a per-course breakdown and current/longest
active-day streaks (a day is active with ≥1 completion or
stats.streak_min_minutes of logged time).

Scopes: --week (trailing 7 days, default) | --month (trailing 30 days) |
--semester (since ui.semester_start) | --since DATE (2006-01-02).`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, s, err := openStore()
			if err != nil {
				return err
			}
			defer func() { _ = s.Close() }()

			nown := s.Now()
			scopes := 0
			for _, b := range []bool{week, month, semester, since != ""} {
				if b {
					scopes++
				}
			}
			if scopes > 1 {
				return fmt.Errorf("--week, --month, --semester and --since are mutually exclusive")
			}
			var sinceDay time.Time
			period := "week"
			switch {
			case week, (!month && !semester && since == ""):
				sinceDay = nown.AddDate(0, 0, -6)
				period = "week"
			case month:
				sinceDay = nown.AddDate(0, 0, -29)
				period = "month"
			case semester:
				raw := strings.TrimSpace(cfg.UI.SemesterStart)
				if raw == "" {
					return fmt.Errorf("ui.semester_start is not configured — set it in config.yaml or use --since DATE")
				}
				parsed, err := time.ParseInLocation("2006-01-02", raw, nown.Location())
				if err != nil {
					return fmt.Errorf("ui.semester_start: %w", err)
				}
				sinceDay = parsed
				period = "semester"
			case since != "":
				parsed, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(since), nown.Location())
				if err != nil {
					return fmt.Errorf("--since: want 2006-01-02, got %q", since)
				}
				sinceDay = parsed
				period = "since"
			}
			rep, err := actions.CollectStats(s, nown, sinceDay, period, cfg.Stats.StreakMinMinutes)
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), renderStatsReport(cfg, rep))
			return nil
		},
	}
	cmd.Flags().BoolVar(&week, "week", false, "trailing 7 days (default)")
	cmd.Flags().BoolVar(&month, "month", false, "trailing 30 days")
	cmd.Flags().BoolVar(&semester, "semester", false, "since ui.semester_start from config")
	cmd.Flags().StringVar(&since, "since", "", "since DATE (2006-01-02)")
	return cmd
}

func renderStatsReport(cfg config.Config, rep *actions.StatsReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n", statsPeriodLabel(rep))

	fmt.Fprintf(&b, "completed   %d", rep.Completed)
	if rep.WithDue > 0 {
		fmt.Fprintf(&b, "  ·  on-time %.0f%% (%d/%d)", rep.OnTimePct(), rep.OnTime, rep.WithDue)
	} else {
		fmt.Fprint(&b, "  ·  on-time —")
	}
	if rep.Completed > 0 {
		fmt.Fprintf(&b, "  ·  busiest day %s (%d)", dates.HumanDay(rep.BusiestDay), rep.BusiestCount)
	} else {
		fmt.Fprint(&b, "  ·  busiest day —")
	}
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "hours       %.1fh", rep.TotalHours())
	if rep.Completed > 0 {
		fmt.Fprintf(&b, "  ·  est accuracy %s", estimateLabel(rep.Estimate))
	}
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "streak      %s current · longest %d\n", text.Count(rep.Streaks.Current, "day", "days"), rep.Streaks.Longest)
	if spark := gauges.Sparkline(rep.DayHours); spark != "" {
		fmt.Fprintf(&b, "activity    %s  (%s → %s)\n", spark, dates.HumanDay(rep.Since), dates.HumanDay(rep.Until))
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "by course")
	if len(rep.Courses) == 0 && rep.OtherCompleted == 0 {
		fmt.Fprintln(&b, "  no completed tasks in this period yet")
	}
	for _, c := range rep.Courses {
		if c.Completed == 0 && c.Hours == 0 {
			continue
		}
		fmt.Fprintf(&b, "  %-24s %-3s %5.1fh  %s%s\n",
			truncateRunes(c.Course, 24),
			fmt.Sprintf("%d done", c.Completed),
			c.Hours,
			onTimeShort(c.OnTime, c.WithDue),
			estimateSuffix(c.Estimate))
	}
	if rep.OtherCompleted > 0 {
		fmt.Fprintf(&b, "  %-24s %-3s %5.1fh  %s\n",
			"other projects",
			fmt.Sprintf("%d done", rep.OtherCompleted),
			rep.OtherHours,
			onTimeShort(rep.OtherOnTime, rep.OtherWithDue))
	}
	if len(rep.DayCompletions) > 0 {
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, "per day")
		for i, day := range rep.DayCompletions {
			d := rep.Since.AddDate(0, 0, i)
			fmt.Fprintf(&b, "  %-10s %d\n", dates.HumanDay(d), day)
		}
	}
	return b.String()
}

func statsPeriodLabel(rep *actions.StatsReport) string {
	switch rep.Period {
	case "week":
		return fmt.Sprintf("week of %s – %s (%s)", rep.Since.Format("2 Jan"), rep.Until.Format("2 Jan 2006"), text.Count(rep.Days, "day", "days"))
	case "month":
		return fmt.Sprintf("trailing 30 days (%s → %s)", rep.Since.Format("2 Jan 2006"), rep.Until.Format("2 Jan 2006"))
	case "semester":
		return fmt.Sprintf("semester since %s (%s)", rep.Since.Format("2 Jan 2006"), text.Count(rep.Days, "day", "days"))
	default:
		return fmt.Sprintf("since %s (%s)", rep.Since.Format("2 Jan 2006"), text.Count(rep.Days, "day", "days"))
	}
}

func onTimeShort(onTime, withDue int) string {
	if withDue == 0 {
		return "        "
	}
	return fmt.Sprintf("%2.0f%% on-time", float64(onTime)/float64(withDue)*100)
}

// estimateLabel is the full "N of M tasks estimated (P% of estimate)" form
// for the summary line; the percentage only appears with ≥3 estimates.
func estimateLabel(a core.EstimateAccuracy) string {
	if a.Total == 0 {
		return "no estimates"
	}
	label := fmt.Sprintf("%d of %d tasks estimated", a.Estimated, a.Total)
	if a.Enough {
		label += fmt.Sprintf(" (%.0f%% of estimate)", a.Pct())
	}
	return label
}

func estimateSuffix(a core.EstimateAccuracy) string {
	if a.Estimated == 0 {
		return ""
	}
	if a.Enough {
		return fmt.Sprintf("  est %.0f%% (%d of %d)", a.Pct(), a.Estimated, a.Total)
	}
	return fmt.Sprintf("  est %d of %d", a.Estimated, a.Total)
}
