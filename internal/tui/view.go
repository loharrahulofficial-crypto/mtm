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
	if m.mode == modeConflict {
		return m.renderConflictScreen()
	}

	contentH := m.height - 1 // footer

	sidebarW := sideWidth(m.width, m.sidebarHidden)
	detailW := detailWidth(m.width, m.detailOn)
	mainW := m.width - sidebarW - detailW
	if mainW < 20 {
		mainW = 20
	}

	main := m.renderMain(mainW, contentH)

	middle := joinHorizontal(sidebarW, main, detailW, m, mainW, contentH)

	footer := m.renderFooterBar(m.width)
	screen := lipgloss.JoinVertical(lipgloss.Left, middle, footer)

	switch m.mode {
	case modeHelp:
		return overlay(m.width, m.height, screen, renderHelpBox(m.keymap, m.styles, m.width))
	case modeConfirm:
		return overlay(m.width, m.height, screen, m.renderConfirmBox())
	case modePicker:
		return overlay(m.width, m.height, screen, m.renderPickerBox())
	}
	return screen
}

func joinHorizontal(sidebarW int, main string, detailW int, m Model, mainW, contentH int) string {
	parts := []string{main}
	if sidebarW > 0 {
		sidebar := renderSidebar(m, sidebarW, contentH)
		parts = append([]string{sidebar}, parts...)
	}
	if detailW > 0 {
		detail := m.renderDetail(detailW, contentH)
		parts = append(parts, detail)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

func (m Model) renderMain(w, h int) string {
	if m.mode == modeNotes {
		return m.renderNotesEditor(w, h)
	}
	switch m.view {
	case viewBoard:
		return renderBoard(m, w, h)
	case viewCalendar:
		return renderCalendar(m, w, h)
	default:
		return renderList(m, w, h)
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
