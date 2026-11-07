package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderFooterBar renders the bottom line: view tabs + status or the input bar.
// Layout rule: the middle message is truncated to fit first; the trailing
// right side (help hint, clock) is never truncated.
func (m Model) renderFooterBar(width int) string {
	s := m.styles

	if m.mode == modeInput {
		line := s.footerKey.Render(m.inputLabel) + " " + m.input.View()
		return s.footer.Width(width).Render(truncatePlain(line, width))
	}

	right := m.footerRight(width)
	left := m.footerLeft()

	// Measure the pieces exactly as they will be rendered (styled once,
	// measured after styling, never wrapped again).
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
	if strings.Contains(needles, "paused") || strings.Contains(needles, "failed") || strings.Contains(needles, "conflict") {
		middleStyle = s.syncBad
	}
	line := left + middleStyle.Render(middle) + right
	// Last resort: pad/trim to exactly width without touching the ends.
	if w := lipgloss.Width(line); w < width {
		padLeft := (width - w) / 2
		line = left + strings.Repeat(" ", padLeft) + middleStyle.Render(middle) +
			strings.Repeat(" ", width-w-padLeft) + right
	}
	return s.footer.Width(width).Render(line)
}

func (m Model) footerLeft() string {
	tabs := []string{}
	tabDefs := []struct {
		key, label string
		view       viewID
	}{
		{"1", "List", viewList},
		{"2", "Board", viewBoard},
		{"3", "Calendar", viewCalendar},
	}
	for _, td := range tabDefs {
		label := " " + td.key + " " + td.label + " "
		if m.view == td.view {
			tabs = append(tabs, m.styles.footerKey.Render(label))
		} else {
			tabs = append(tabs, m.styles.footer.Render(label))
		}
	}
	return strings.Join(tabs, "")
}

// footerRight builds the right-hand segment. The wall clock renders from
// the same Clock that stamps task frontmatter (D-011) so the two can never
// drift apart — but it is the first segment dropped when width is tight;
// sync state and the help hint always fit.
func (m Model) footerRight(width int) string {
	s := m.styles
	help := " " + m.keymap.Seq(aHelp) + " help "
	base := s.footer.Render(" local repo " + help)
	switch m.syncSt {
	case syncSyncing:
		base = s.footer.Render(m.spinner.View()+" syncing ") + s.footer.Render(help)
	case syncConflict:
		base = s.syncBad.Render(" ⚠ conflicts "+m.keymap.Seq(aConflicts)+" to fix ") + s.footer.Render(help)
	case syncOffline:
		base = s.footer.Render(" ○ offline ") + s.footer.Render(help)
	case syncOK:
		syncText := " ✓ synced"
		if !m.lastSync.IsZero() {
			syncText = " ✓ " + m.lastSync.Format("15:04")
		}
		if n := len(m.pendingCommits); n > 0 {
			syncText = " ✓ · " + itoa(n) + " pending"
		}
		base = s.syncOK.Render(syncText) + s.footer.Render(help)
	default:
		if m.hasRemote {
			base = s.footer.Render(" ○ … ") + s.footer.Render(help)
		}
	}
	clockSeg := s.footer.Render(" " + m.clock.Now().Format("15:04") + " ")
	if lipgloss.Width(m.footerLeft())+lipgloss.Width(base)+lipgloss.Width(clockSeg)+2 > width {
		return base // too tight — the clock yields, sync state and help stay
	}
	return clockSeg + base
}

// footerMiddle is the transient status / filter message and (in the redesign)
// the active timer. Never shown at the cost of the right side.
func (m Model) footerMiddle() string {
	msg := m.statusMsg
	if m.searchQuery != "" {
		if msg == "" {
			msg = "filter: " + m.searchQuery
		} else {
			msg = msg + " · filter: " + m.searchQuery
		}
	}
	return msg
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
