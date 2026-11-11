package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
)

// renderFooterBar renders the statusline: view pills on the left, the
// transient message + labeled sync state in the middle, and the clock
// followed by ? help on the right. Layout rule: the middle yields space
// first (truncated); the right side (clock + help) is never truncated.
func (m Model) renderFooterBar(width int) string {
	s := m.styles

	if m.mode == modeInput {
		line := s.footerKey.Render(m.inputLabel) + " " + m.input.View()
		return paint(truncatePlain(line, width), width, 1, lipgloss.Color(m.palette.Base))
	}

	right := m.footerRight(width)
	left := m.footerLeft(width)

	// Measure the pieces exactly as they will be rendered.
	leftW, rightW := fastWidth(left), fastWidth(right)
	budget := width - leftW - rightW - 2 // middle padding cells
	if budget < 0 {
		budget = 0
	}
	middle := m.footerMiddle()
	if fastWidth(middle) > budget {
		middle = truncatePlain(middle, budget)
	}
	middleStyle := s.footer
	needles := strings.ToLower(middle)
	if strings.Contains(needles, "paused") || strings.Contains(needles, "failed") || strings.Contains(needles, "conflict") || strings.Contains(needles, "sync error") {
		middleStyle = s.syncBad
	}
	fill := func() string { return middleStyle.Render(middle) }

	line := left + fill() + right
	// Last resort: pad/trim to exactly width without touching the ends.
	if w := fastWidth(line); w < width {
		padLeft := (width - w) / 2
		line = left + s.footer.Render(strings.Repeat(" ", padLeft)) + fill() +
			s.footer.Render(strings.Repeat(" ", width-w-padLeft)) + right
	}
	return paint(line, width, 1, lipgloss.Color(m.palette.Base))
}

type pillSeg struct {
	view     viewID
	rendered string
}

// footerPillSegments is the single source for the pill row — the statusline
// and the mouse hit-testing both consume it.
func (m Model) footerPillSegments(width int) []pillSeg {
	s := m.styles
	if width < 30 {
		return nil
	}
	pills := []struct {
		key   string
		label string
		icon  string
		view  viewID
	}{
		{m.keymap.Seq(aViewHome), "Home", m.icon("home"), viewHome},
		{m.keymap.Seq(aViewList), "List", m.icon("list"), viewList},
		{m.keymap.Seq(aViewBoard), "Board", m.icon("board"), viewBoard},
		{m.keymap.Seq(aViewCalendar), "Calendar", m.icon("calendar"), viewCalendar},
		{m.keymap.Seq(aViewCourses), "Courses", m.icon("course"), viewCourses},
		{m.keymap.Seq(aViewStats), "Stats", m.icon("stats"), viewStats},
		{m.keymap.Seq(aViewIdeas), "Ideas", m.icon("idea"), viewIdeas},
	}
	compact := width < 115   // icons + 7 pills + clock + help don't fit below this
	activeOnly := width < 90 // the pill row yields to clock+help below this
	out := []pillSeg{}
	for _, p := range pills {
		active := m.view == p.view
		if activeOnly && !active {
			continue
		}
		text := p.key + " " + p.label
		if p.icon != "" && !compact {
			text = p.icon + " " + text
		}
		var rendered string
		if active {
			rendered = s.pillActive.Render(" " + text + " ")
		} else {
			rendered = s.pillInactive.Render(" " + text + " ")
		}
		out = append(out, pillSeg{p.view, rendered})
	}
	return out
}

// footerLeft renders the view pills. Compact forms drop detail when space
// is tight (rule: right side never truncates).
func (m Model) footerLeft(width int) string {
	segs := m.footerPillSegments(width)
	out := make([]string, 0, len(segs))
	for _, s := range segs {
		out = append(out, s.rendered)
	}
	return strings.Join(out, "")
}

// footerRight builds the pinned right-hand segment: the wall clock followed
// by the help hint. Exactly one clock, always present, right-aligned — the
// left pills and the middle message yield space first.
func (m Model) footerRight(width int) string {
	s := m.styles
	help := " " + m.keymap.Seq(aHelp) + " help "
	clockSeg := s.footer.Render(" " + m.clock.Now().Format("15:04") + " ")
	return clockSeg + s.footer.Render(help)
}

// syncStateLabel renders the labeled sync state that lives in the middle of
// the statusline: "synced 10:32" / "syncing…" / "offline" / "⚠ conflicts".
func (m Model) syncStateLabel() string {
	switch m.syncSt {
	case syncSyncing:
		return m.spinner.View() + " syncing…"
	case syncConflict:
		return "⚠ conflicts · " + m.keymap.Seq(aConflicts) + " to fix"
	case syncOffline:
		return "offline"
	case syncOK:
		if !m.lastSync.IsZero() {
			return "synced " + m.lastSync.Format("15:04")
		}
		if n := len(m.pendingCommits); n > 0 {
			return "synced · " + itoa(n) + " pending"
		}
		return "synced"
	default:
		if m.hasRemote {
			return "syncing…"
		}
		return "local repo"
	}
}

// timerChip renders the compact countdown for the statusline, e.g.
// "24:59 work", "break 4:32", "12m timer". Empty when no timer runs.
func timerChip(v core.TimerView) string {
	if v.Phase == core.TimerPhaseIdle {
		return ""
	}
	var chip string
	switch {
	case v.Mode == core.TimerModePomodoro && v.Total > 0:
		chip = fmt.Sprintf("%02d:%02d %s", int(v.Remaining.Minutes()), int(v.Remaining.Seconds())%60, v.Phase)
	default:
		chip = fmt.Sprintf("%dm timer", int(v.Accrued.Round(time.Minute).Minutes()))
	}
	if !v.Running {
		chip += " paused"
	}
	if v.Stale {
		chip = "stale " + chip
	}
	return chip
}

// footerMiddle is the transient status / filter message followed by the
// labeled sync state. The left pills and this middle always yield space to
// the right side.
func (m Model) footerMiddle() string {
	msg := m.statusMsg
	if m.searchQuery != "" {
		filter := "filter: " + m.searchQuery
		if msg == "" {
			msg = filter
		} else {
			msg = msg + " · " + filter
		}
	}
	if chip := timerChip(m.timer); chip != "" {
		if msg == "" {
			msg = chip
		} else {
			msg = chip + " · " + msg
		}
	}
	if msg == "" {
		return m.syncStateLabel()
	}
	return msg + " · " + m.syncStateLabel()
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := ""
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}
	return digits
}
