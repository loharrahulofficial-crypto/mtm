package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/loharrahulofficial-crypto/mtm/internal/actions"
	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/loharrahulofficial-crypto/mtm/internal/store"
)

// WaybarPayload is the JSON shape waybar's custom/ modules expect.
type WaybarPayload struct {
	Text    string `json:"text"`
	Tooltip string `json:"tooltip"`
	Class   string `json:"class"`
}

// buildWaybarPayload assembles the bar JSON from the shared snapshot.
// Contract: text stays bar-sized (today's open count, an exam chip
// ✦<days> when the nearest exam is within 30 days, and an active-timer
// chip); class follows a fixed priority (overdue > exam ≤3d > timer >
// normal); tooltip carries the detail lines.
func buildWaybarPayload(snap actions.StatusSnapshot, timerLabel string) WaybarPayload {
	payload := WaybarPayload{}
	var tip strings.Builder

	fmt.Fprintf(&tip, "%d open today · %d overdue", snap.Today, snap.Overdue)
	payload.Text = fmt.Sprintf("%d", snap.Today)
	payload.Class = "normal"

	if snap.NextExam != nil {
		fmt.Fprintf(&tip, "\nexam %s in %dd", snap.NextExam.Title, snap.NextExamDays)
		if snap.MoreExams > 0 {
			fmt.Fprintf(&tip, " (+%d more)", snap.MoreExams)
		}
		payload.Text += fmt.Sprintf(" ✦%dd", snap.NextExamDays)
		if snap.NextExamDays <= 3 {
			payload.Class = "exam"
		}
	}
	if timerLabel != "" {
		fmt.Fprintf(&tip, "\ntimer %s", timerLabel)
		payload.Text += " ▸" + timerLabel
		if payload.Class == "normal" {
			payload.Class = "timer"
		}
	}
	if snap.Overdue > 0 {
		payload.Class = "overdue"
	}
	payload.Tooltip = tip.String()
	return payload
}

// renderWaybar resolves the timer's task title and builds the payload.
func renderWaybar(s *store.Store, snap actions.StatusSnapshot) WaybarPayload {
	timerLabel := ""
	if v := snap.Timer; v.Phase != core.TimerPhaseIdle {
		ref := ""
		if t, err := s.Resolve(v.TaskID); err == nil {
			ref = t.Title
		}
		switch {
		case v.Mode == core.TimerModePomodoro && v.Total > 0:
			timerLabel = formatClock(v.Remaining)
		default:
			timerLabel = actions.FormatMinutes(int(v.Accrued/time.Minute)) + " elapsed"
		}

		if ref != "" {
			timerLabel = fmt.Sprintf("%s on %s", timerLabel, truncateRunes(ref, 30))
		} else {
			timerLabel = fmt.Sprintf("%s on %s", timerLabel, shortID(v.TaskID))
		}
		if !v.Running {
			timerLabel += " (paused)"
		}
	}
	return buildWaybarPayload(snap, timerLabel)
}

func newWaybarCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "waybar",
		Short: "Print Waybar-compatible JSON for the custom module",
		Long: `Emits one JSON object with text, tooltip and class for a Waybar
custom/ module:

  text     today's open task count; plus an exam chip (✦<days>) when the
           nearest open exam is within 30 days, and an active-timer chip
  tooltip  today/overdue counts, the exam countdown, the timer line
  class    overdue > exam (≤3 days) > timer > normal

Reads the store through the index cache; built to run every few seconds.
Never exits non-zero and never prints anything but JSON: on error it
prints a minimal class"error" payload.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			_, s, err := openStore()
			if err != nil {
				return json.NewEncoder(out).Encode(WaybarPayload{Text: "", Tooltip: "mtm: " + err.Error(), Class: "error"})
			}
			defer func() { _ = s.Close() }()
			now := s.Now()
			snap := actions.LoadStatusSnapshot(s, now)
			fmt.Fprint(out, mustJSON(renderWaybar(s, snap))+"\n")
			return nil
		},
	}
	return cmd
}

func mustJSON(p WaybarPayload) string {
	data, err := json.Marshal(p)
	if err != nil {
		panic(err) // payload is plain strings and strings only
	}
	return string(data)
}
