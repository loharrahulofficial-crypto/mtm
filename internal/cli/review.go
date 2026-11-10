package cli

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/loharrahulofficial-crypto/mtm/internal/actions"
	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/loharrahulofficial-crypto/mtm/internal/dates"
	"github.com/loharrahulofficial-crypto/mtm/internal/store"
	"github.com/loharrahulofficial-crypto/mtm/internal/text"
)

func newReviewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "review",
		Short: "Weekly review: what finished, what slipped, what's next — overdue items one by one",
		Long: `Weekly review for the trailing 7 days:
  • what you finished (with completion day)
  • what slipped (overdue tasks)
  • the next 7 days, exams included
Each overdue item is handled one by one: [r]eschedule, [a]rchive, [k]eep
or [q]uit. Nothing changes without your keypress; an empty answer keeps.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, s, err := openStore()
			if err != nil {
				return err
			}
			defer func() { _ = s.Close() }()

			now := s.Now()
			rep, err := actions.CollectReview(s, now)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			printReviewSections(out, now, rep)

			if len(rep.Overdue) == 0 {
				fmt.Fprintln(out, "\nno overdue items — nothing to decide.")
				return nil
			}
			fmt.Fprintf(out, "\nact on the %d overdue %s — nothing changes without your keypress\n", len(rep.Overdue), text.Plural(len(rep.Overdue), "item", "items"))
			d := runReviewPrompts(s, now, rep.Overdue, cmd.InOrStdin(), out)
			if d.rescheduled+d.archived > 0 {
				commitAndSync(cfg, s.Root, fmt.Sprintf("review: %d rescheduled, %d archived", d.rescheduled, d.archived))
			}
			if d.stopped {
				fmt.Fprintf(out, "review stopped — %d rescheduled, %d archived, %d kept, rest unchanged\n", d.rescheduled, d.archived, d.kept)
				return nil
			}
			fmt.Fprintf(out, "review done — %d rescheduled, %d archived, %d kept\n", d.rescheduled, d.archived, d.kept)
			return nil
		},
	}
}

// printReviewSections renders the three read-only sections.
func printReviewSections(out io.Writer, now time.Time, rep *actions.ReviewReport) {
	fmt.Fprintf(out, "weekly review — %s – %s\n\n", rep.Since.Format("2 Jan"), rep.Until.Format("2 Jan 2006"))

	fmt.Fprintf(out, "finished this week (%d)\n", len(rep.Finished))
	if len(rep.Finished) == 0 {
		fmt.Fprintln(out, "  nothing yet — the week is young")
	}
	for i, t := range rep.Finished {
		if i >= 10 {
			fmt.Fprintf(out, "  … +%d more\n", len(rep.Finished)-10)
			break
		}
		fmt.Fprintf(out, "  %s%s\n", actions.ReviewRow(t, now), examMarkSuffix(t))
	}

	fmt.Fprintf(out, "\nslipped — overdue (%d)\n", len(rep.Overdue))
	if len(rep.Overdue) == 0 {
		fmt.Fprintln(out, "  ✦ nothing overdue")
	}
	for i, t := range rep.Overdue {
		fmt.Fprintf(out, "  %d. %s%s (due %s)\n", i+1, examMark(t), t.Title, dueLabel(t, now))
	}

	fmt.Fprintf(out, "\nnext 7 days (including exams)\n")
	for i := 0; i <= 6; i++ {
		day := now.AddDate(0, 0, i)
		dayStr := day.Format("2006-01-02")
		var rows []*core.Task
		for _, t := range rep.Upcoming {
			if strings.HasPrefix(t.Due, dayStr) {
				rows = append(rows, t)
			}
		}
		dayLabel := dates.HumanDay(day)
		if i == 0 {
			dayLabel += " (today)"
		}
		if len(rows) == 0 {
			fmt.Fprintf(out, "  %-18s —\n", dayLabel)
			continue
		}
		for _, t := range rows {
			fmt.Fprintf(out, "  %-18s %s%s\n", dayLabel, examMark(t), t.Title)
		}
	}
	if rep.ParseFailures > 0 {
		fmt.Fprintf(out, "\nnote: %d task(s) with unparseable due dates were skipped\n", rep.ParseFailures)
	}
}

type reviewDecisions struct {
	rescheduled, archived, kept int
	stopped                     bool
}

// runReviewPrompts walks the overdue list one by one. Every mutation is a
// single explicit keypress; an empty answer or EOF keeps the item.
func runReviewPrompts(s *store.Store, now time.Time, overdue []*core.Task, in io.Reader, out io.Writer) reviewDecisions {
	var d reviewDecisions
	r := bufio.NewReader(in)
	read := func() (string, bool) {
		line, err := r.ReadString('\n')
		line = strings.TrimSpace(line)
		if err != nil && line == "" {
			return "", false
		}
		return line, true
	}
	for i, t := range overdue {
		fmt.Fprintf(out, "%d/%d  %s%s (due %s)\n", i+1, len(overdue), examMark(t), t.Title, dueLabel(t, now))
		fmt.Fprint(out, "     [r]eschedule · [a]rchive · [k]eep › ")
		choice, ok := read()
		if !ok {
			fmt.Fprintln(out, "\nno input — keeping the rest")
			d.kept += len(overdue) - i
			return d
		}
		switch strings.ToLower(choice) {
		case "r":
			fmt.Fprint(out, "     new due (2026-11-15 or 2026-11-15 14:00) › ")
			value, ok := read()
			if !ok {
				fmt.Fprintln(out, "\nno input — kept")
				d.kept++
				continue
			}
			if err := actions.ReviewReschedule(s, t, value); err != nil {
				fmt.Fprintf(out, "     %v — kept unchanged\n", err)
				d.kept++
				continue
			}
			fmt.Fprintf(out, "     due → %s\n", t.Due)
			d.rescheduled++
		case "a":
			fmt.Fprint(out, "     archive this task? [y/N] › ")
			value, ok := read()
			if !ok {
				fmt.Fprintln(out, "\nno input — kept")
				d.kept++
				continue
			}
			if strings.HasPrefix(strings.ToLower(value), "y") {
				if err := actions.ReviewArchive(s, t); err != nil {
					fmt.Fprintf(out, "     %v — kept\n", err)
					d.kept++
					continue
				}
				fmt.Fprintf(out, "     archived → archive/%s\n", t.Project)
				d.archived++
				continue
			}
			fmt.Fprintln(out, "     kept")
			d.kept++
		case "q":
			fmt.Fprintln(out, "     stopped — the remaining items are unchanged")
			d.kept += len(overdue) - i
			d.stopped = true
			return d
		case "k", "":
			fmt.Fprintln(out, "     kept")
			d.kept++
		default:
			fmt.Fprintln(out, "     ? unrecognized — kept")
			d.kept++
		}
	}
	return d
}

// examMark is the ✦ exam marker used across the TUI.
func examMark(t *core.Task) string {
	if strings.EqualFold(t.Kind, core.KindExam) {
		return "✦ "
	}
	return ""
}

// examMarkSuffix is examMark for lines that lead with a title instead of a
// timestamp (finished rows).
func examMarkSuffix(t *core.Task) string {
	if strings.EqualFold(t.Kind, core.KindExam) {
		return "  ✦"
	}
	return ""
}

// dueLabel renders a task due date as "Thu 5 Nov" or "Thu 5 Nov 09:00".
func dueLabel(t *core.Task, now time.Time) string {
	if len(t.Due) > len("2006-01-02") {
		if ts, err := time.ParseInLocation(core.TimestampFormat, t.Due, now.Location()); err == nil {
			return ts.Format("Mon 2 Jan 15:04")
		}
		return t.Due
	}
	if d, err := time.ParseInLocation("2006-01-02", t.Due, now.Location()); err == nil {
		return d.Format("Mon 2 Jan")
	}
	return t.Due
}
