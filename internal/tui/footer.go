package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderFooterBar renders the bottom line: view tabs + status or the input bar.
func (m Model) renderFooterBar(width int) string {
	s := m.styles

	if m.mode == modeInput {
		inputView := m.input.View()
		line := s.footerKey.Render(m.inputLabel) + " " + inputView
		pad := width - lipgloss.Width(line)
		if pad > 0 {
			line += strings.Repeat(" ", pad)
		}
		return s.footer.Render(line)
	}

	// Tabs (left).
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
		label := td.key + " " + td.label
		if m.view == td.view {
			tabs = append(tabs, s.footerKey.Render(label))
		} else {
			tabs = append(tabs, s.footer.Render(label))
		}
	}
	left := strings.Join(tabs, "")

	// Right: sync indicator + help hint.
	var right string
	switch m.syncSt {
	case syncSyncing:
		right = s.footer.Render(m.spinner.View() + " syncing")
	case syncConflict:
		right = s.syncBad.Render(" ⚠ conflicts ") + s.footer.Render(m.keymap.Seq(aConflicts)+" to fix ")
	case syncOffline:
		right = s.footer.Render(" ○ offline ")
	case syncOK:
		syncText := " ✓ synced"
		if !m.lastSync.IsZero() {
			syncText = " ✓ " + m.lastSync.Format("15:04")
		}
		if n := len(m.pendingCommits); n > 0 {
			syncText = " ✓ · " + itoa(n) + " pending"
		}
		right = s.syncOK.Render(syncText) + s.footer.Render(" "+m.keymap.Seq(aHelp)+" help ")
	default:
		if m.hasRemote {
			right = s.footer.Render(" ○ … ")
		} else {
			right = s.footer.Render(" local repo ")
		}
	}

	leftW := lipgloss.Width(left)
	rightW := lipgloss.Width(right)

	// Middle: status message or filter indicator.
	middle := m.statusMsg
	if m.searchQuery != "" && middle == "" {
		middle = "filter: " + m.searchQuery
	}
	middle = truncatePlain(middle, width-leftW-rightW-2)
	middleStyle := s.footer
	if strings.Contains(strings.ToLower(middle), "paused") || strings.Contains(strings.ToLower(middle), "failed") {
		middleStyle = s.syncBad
	}
	midW := lipgloss.Width(middle)
	padLeft := (width - leftW - rightW - midW) / 2
	if padLeft < 0 {
		padLeft = 0
	}
	padRight := width - leftW - rightW - midW - padLeft
	if padRight < 0 {
		padRight = 0
	}
	return s.footer.Render(left +
		strings.Repeat(" ", padLeft) +
		middleStyle.Render(middle) +
		strings.Repeat(" ", padRight) +
		right)
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
