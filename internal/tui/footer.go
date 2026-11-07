package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderFooterBar renders the statusline: view pills on the left, transient
// message + sync state in the middle, help and clock on the right.
// Layout rule: the middle message is truncated first; the right side
// (help > sync > clock) is never truncated — the clock drops first when
// width is tight, then the sync text; ? help never drops.
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

// footerRight builds the right-hand segment: sync state + help + clock.
// The wall clock renders from the same Clock that stamps task frontmatter
// (D-011) so the two can never drift apart — but it is the first segment
// dropped when width is tight; sync state and the help hint always fit.
func (m Model) footerRight(width int) string {
	s := m.styles
	help := " " + m.keymap.Seq(aHelp) + " help "
	base := s.footer.Render(" local repo " + help)
	switch m.syncSt {
	case syncSyncing:
		base = s.footer.Render(" "+m.spinner.View()+" syncing ") + s.footer.Render(help)
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
	if lipgloss.Width(m.footerLeft(width))+lipgloss.Width(base)+lipgloss.Width(clockSeg)+2 > width {
		return base // too tight — the clock yields, sync state and help stay
	}
	return clockSeg + base
}

// footerMiddle is the transient status / filter message. Never shown at the
// cost of the right side.
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
