package tui

import (
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/loharrahulofficial-crypto/mtm/internal/dates"
)

// Done-task display rules, shared by list, board, calendar, Home and the
// detail pane:
//   - strikethrough applies to the title's non-space glyphs only, never to
//     padding or blank runs;
//   - no overdue/urgency coloring — a muted "done 3d ago" instead;
//   - the detail pane shows "Completed Mon 2 Nov".

// strikeTitle renders a done task's title with strikethrough on the
// non-space glyphs only. Spaces keep the same fg/bg without strike so
// padding never looks struck.
func strikeTitle(fg, bg lipgloss.Color, title string) string {
	strike := lipgloss.NewStyle().Foreground(fg).Background(bg).Strikethrough(true)
	flat := lipgloss.NewStyle().Foreground(fg).Background(bg)
	var b strings.Builder
	for _, run := range glyphRuns(title) {
		if strings.Trim(run, " ") == "" {
			b.WriteString(flat.Render(run))
			continue
		}
		b.WriteString(strike.Render(run))
	}
	return b.String()
}

// glyphRuns splits s into alternating non-space and space runs, preserving
// order and exact bytes.
func glyphRuns(s string) []string {
	var out []string
	var cur strings.Builder
	curSpace := false
	started := false
	for _, r := range s {
		isSpace := r == ' '
		if !started {
			curSpace = isSpace
			started = true
		} else if isSpace != curSpace {
			out = append(out, cur.String())
			cur.Reset()
			curSpace = isSpace
		}
		cur.WriteRune(r)
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

// doneLabelFor is the muted completion marker for done tasks:
// "done 3d ago" (falling back to the updated stamp, then "done").
func doneLabelFor(t *core.Task, now time.Time) string {
	stamp := t.Completed
	if stamp == "" {
		stamp = t.Updated
	}
	if l := dates.DoneLabel(stamp, now); l != "" {
		return l
	}
	return "done"
}
