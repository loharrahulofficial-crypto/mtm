package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/loharrahulofficial-crypto/mtm/internal/actions"
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

			now := s.Now()
			snap := actions.LoadStatusSnapshot(s, now)

			line1 := fmt.Sprintf("%d today · %d overdue", snap.Today, snap.Overdue)
			if snap.NextExam != nil {
				line1 += fmt.Sprintf(" · exam in %dd: %s", snap.NextExamDays, snap.NextExam.Title)
				if snap.MoreExams > 0 {
					line1 += fmt.Sprintf(" (+%d more)", snap.MoreExams)
				}
			}
			// Active timer (read-only peek; phase advances happen on TUI/pomo ticks).
			if v := snap.Timer; v.Phase != core.TimerPhaseIdle {
				ref := shortID(v.TaskID)
				if t, err := s.Resolve(v.TaskID); err == nil {
					ref = shortID(v.TaskID) + " " + t.Title
				}
				var chip string
				if v.Mode == core.TimerModePomodoro && v.Total > 0 {
					chip = fmt.Sprintf("pomo %s left · %s %d · %s", formatClock(v.Remaining), v.Phase, v.PhaseIndex, ref)
				} else {
					chip = fmt.Sprintf("timer %s elapsed · %s", actions.FormatMinutes(int(v.Accrued/time.Minute)), ref)
				}
				if !v.Running {
					chip += " (paused)"
				}
				line1 += " · " + chip
			}

			// Open ideas (only when non-zero) and the next milestone.
			if snap.OpenIdeas > 0 {
				line1 += fmt.Sprintf(" · %d ideas", snap.OpenIdeas)
			}
			if m := snap.NextMilestone; m != nil {
				days := snap.NextMilestoneDays
				chip := "today"
				switch {
				case days > 0:
					chip = fmt.Sprintf("in %dd", days)
				case days < 0:
					chip = fmt.Sprintf("%dd overdue", -days)
				}
				line1 += fmt.Sprintf(" · ▸ %s · %s", truncateRunes(m.Title, 24), chip)
			}

			// Per-project progress.
			stats := snap.ProjectProgress
			parts := make([]string, 0, len(stats))
			for i, st := range stats {
				if i >= 6 {
					parts = append(parts, fmt.Sprintf("+%d more", len(stats)-6))
					break
				}
				pct := 0
				if st.Total > 0 {
					pct = st.Done * 100 / st.Total
				}
				parts = append(parts, fmt.Sprintf("%s %d/%d (%d%%)", st.Project, st.Done, st.Total, pct))
			}

			fmt.Println(line1)
			if len(parts) > 0 {
				fmt.Println(strings.Join(parts, " · "))
			}
			return nil
		},
	}
}
