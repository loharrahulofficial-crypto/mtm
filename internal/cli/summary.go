package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/spf13/cobra"
)

func newSummaryCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "summary",
		Short: "Compact status block for shell startup (<50ms)",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, s, err := openStore()
			if err != nil {
				fmt.Printf("mtm: %v\n", err)
				return nil
			}
			defer func() { _ = s.Close() }()

			now := time.Now()
			today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
			isDone := func(p, st string) bool { return s.IsDoneStatus(p, st) }
			all := s.Tasks()

			base := core.Filter{Now: now, DoneCheck: isDone}
			todayFilter := base
			todayFilter.Due = "today"
			overdueFilter := base
			overdueFilter.Due = "overdue"
			nToday := len(todayFilter.Apply(all))
			nOverdue := len(overdueFilter.Apply(all))

			// Nearest upcoming exam (kind=exam, future due, not done).
			type examHit struct {
				task *core.Task
				days int
			}
			var exams []examHit
			for _, t := range all {
				if !strings.EqualFold(t.Kind, core.KindExam) || t.Due == "" {
					continue
				}
				d, err := core.ParseDue(t.Due, now.Location())
				if err != nil {
					continue
				}
				examDay := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, now.Location())
				days := int(examDay.Sub(today).Hours() / 24)
				if days < 0 || isDone(t.Project, t.Status) {
					continue
				}
				exams = append(exams, examHit{t, days})
			}
			sort.Slice(exams, func(i, j int) bool { return exams[i].days < exams[j].days })

			line1 := fmt.Sprintf("%d today · %d overdue", nToday, nOverdue)
			if len(exams) > 0 {
				e := exams[0]
				line1 += fmt.Sprintf(" · exam in %dd: %s", e.days, e.task.Title)
				if len(exams) > 1 {
					line1 += fmt.Sprintf(" (+%d more)", len(exams)-1)
				}
			}

			// Per-project progress.
			type projStat struct {
				name      string
				done, all int
			}
			byProject := map[string]*projStat{}
			for _, t := range all {
				st := byProject[t.Project]
				if st == nil {
					st = &projStat{name: t.Project}
					byProject[t.Project] = st
				}
				st.all++
				if isDone(t.Project, t.Status) {
					st.done++
				}
			}
			stats := make([]*projStat, 0, len(byProject))
			for _, st := range byProject {
				stats = append(stats, st)
			}
			sort.Slice(stats, func(i, j int) bool { return stats[i].name < stats[j].name })

			parts := make([]string, 0, len(stats))
			for i, st := range stats {
				if i >= 6 {
					parts = append(parts, fmt.Sprintf("+%d more", len(stats)-6))
					break
				}
				pct := 0
				if st.all > 0 {
					pct = st.done * 100 / st.all
				}
				parts = append(parts, fmt.Sprintf("%s %d/%d (%d%%)", st.name, st.done, st.all, pct))
			}

			fmt.Println(line1)
			if len(parts) > 0 {
				fmt.Println(strings.Join(parts, " · "))
			}
			return nil
		},
	}
}
