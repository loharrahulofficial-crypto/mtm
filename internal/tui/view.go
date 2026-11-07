package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// View renders the whole screen.
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "loading…"
	}
	if m.width < minUsableWidth || m.height < minUsableHeight {
		return tooSmallView(m.width, m.height)
	}
	if m.mode == modeConflict {
		return m.renderConflictScreen()
	}
	if m.mode == modeFocus {
		return normalizeBlock(m.renderFocus(), m.width, m.height)
	}

	contentH := m.height - 1 // footer

	sidebarW := sideWidth(m.width, m.sidebarHidden)
	detailW := detailWidth(m.width, m.detailOn)
	gutters := 0
	if sidebarW > 0 {
		gutters++
	}
	if detailW > 0 {
		gutters++
	}
	mainW := m.width - sidebarW - detailW - gutters
	if mainW < 20 {
		mainW = 20
	}

	main := m.renderMain(mainW, contentH)

	parts := []string{}
	if sidebarW > 0 {
		parts = append(parts, renderSidebar(m, sidebarW, contentH), gutterColumn(contentH, m.styles.base))
	}
	parts = append(parts, main)
	if detailW > 0 {
		parts = append(parts, gutterColumn(contentH, m.styles.base), m.renderDetail(detailW, contentH))
	}
	middle := lipgloss.JoinHorizontal(lipgloss.Top, parts...)
	// The panes must tile the frame exactly; anything that overflows (a long
	// style, a wrap) would push the top row off the terminal, so normalize.
	middle = normalizeBlock(middle, m.width, contentH)

	footer := m.renderFooterBar(m.width)
	screen := lipgloss.JoinVertical(lipgloss.Left, middle, footer)

	switch m.mode {
	case modeHelp:
		return overlay(m.width, m.height, screen, renderHelpBox(m.keymap, m.styles, m.width))
	case modeConfirm:
		return overlay(m.width, m.height, screen, m.renderConfirmBox())
	case modePicker:
		return overlay(m.width, m.height, screen, m.renderPickerBox())
	case modePalette:
		return overlay(m.width, m.height, screen, m.renderPaletteBox())
	}
	return normalizeBlock(screen, m.width, m.height)
}

func (m Model) renderMain(w, h int) string {
	focused := m.focus == focusMain
	panel := func(title string, body []string) string {
		return renderPanel(m, title, w, h, focused, body)
	}
	if m.mode == modeNotes {
		content := m.renderNotesEditor(w-2, h-2)
		return panel("notes", strings.Split(content, "\n"))
	}
	switch m.view {
	case viewHome:
		return panel("", renderHomeBody(m, w-2, h-2))
	case viewBoard:
		return panel("board", renderBoardBody(m, w-2, h-2))
	case viewCalendar:
		return panel("", renderCalendarBody(m, w-2, h-2))
	default:
		return panel("", renderListBody(m, w-2, h-2))
	}
}

// overlay centers box over base.
func overlay(width, height int, base, box string) string {
	b := lipgloss.NewStyle().Width(width).Height(height).Render(base)
	placed := lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
	// lipgloss lacks compositing; draw the placed box over the base by
	// replacing base lines that the box touches.
	baseLines := strings.Split(b, "\n")
	boxLines := strings.Split(placed, "\n")
	for i, line := range boxLines {
		if i >= len(baseLines) {
			break
		}
		if strings.TrimRight(line, " ") != "" {
			baseLines[i] = line
		}
	}
	return strings.Join(baseLines, "\n")
}

func paneBorderLeft(m *Model) lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(lipgloss.Color(m.palette.Muted))
}
