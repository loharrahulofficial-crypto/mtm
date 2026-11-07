package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
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
	leftW, rightW := lipgloss.Width(left), lipgloss.Width(right)
	budget := width - leftW - rightW - 2 // middle padding cells
	if budget < 0 {
		budget = 0
	}
	middle := m.footerMiddle()
	if lipgloss.Width(middle) > budget {
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
	if w := lipgloss.Width(line); w < width {
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
		{"0", "Home", m.icon("home"), viewHome},
		{"1", "List", m.icon("list"), viewList},
		{"2", "Board", m.icon("board"), viewBoard},
		{"3", "Calendar", m.icon("calendar"), viewCalendar},
	}
	compact := width < 90
	activeOnly := width < 56
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
