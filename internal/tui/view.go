package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// View renders the whole screen.
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "loading…"
	}
	baseColor := lipgloss.Color(m.palette.Base)
	if m.width < minUsableWidth || m.height < minUsableHeight {
		return tooSmallView(m.width, m.height, baseColor, m.keymap.Seq(aQuit))
	}
	if m.mode == modeConflict {
		return m.renderConflictScreen()
	}
	if m.mode == modeFocus {
		return normalizeBlock(m.renderFocus(), m.width, m.height, baseColor)
	}

	contentH := m.height - 1 // footer

	sidebarW := sideWidth(m.width, m.sidebarHidden)
	detailW := detailWidth(m.width, m.detailOn)
	if m.view == viewCourses {
		detailW = 0 // course dashboard is read-only; no detail pane
	}
	mainW, _ := mainPaneSize(m)

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
	middle = normalizeBlock(middle, m.width, contentH, baseColor)

	footer := m.renderFooterBar(m.width)
	screen := lipgloss.JoinVertical(lipgloss.Left, middle, footer)

	switch m.mode {
	case modeHelp:
		return overlay(m.width, m.height, screen, renderHelpBox(m.keymap, m.styles, m.width), baseColor)
	case modeConfirm:
		return overlay(m.width, m.height, screen, m.renderConfirmBox(), baseColor)
	case modePicker:
		return overlay(m.width, m.height, screen, m.renderPickerBox(), baseColor)
	case modePalette:
		return overlay(m.width, m.height, screen, m.renderPaletteBox(), baseColor)
	}
	return normalizeBlock(screen, m.width, m.height, baseColor)
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
		// Home gets no outer border; its inner panels carry the single
		// focus ring (renderHomeBody gives it to TODAY when focused).
		return renderPanelBorderless(m, w, h, renderHomeBody(m, w, h))
	case viewCourses:
		// Borderless slab like Home; the dashboard is read-only with scroll.
		return renderCoursesBody(m, w, h)
	case viewBoard:
		return panel("board", renderBoardBody(m, w-2, h-2))
	case viewCalendar:
		// The panel title carries the visible month (letterspaced
		// "CALENDAR" + normal-case month); the chevrons hint the J/K
		// month keys.
		return renderPanel(m, "CALENDAR · "+m.calMonth.Format("Jan 2006"), w, h, focused, renderCalendarBody(m, w-2, h-2), m.icon("chev_left")+" "+m.icon("chev_right"))
	default:
		body := renderListBody(m, w-2, h-2)
		chips := filterChipsText(m, (w*45)/100)
		return renderPanel(m, listPanelTitle(m), w, h, focused, body, "", chips)
	}
}

// overlay centers box over base. baseColor is the background the base block
// was painted on; box rows keep their own background.
func overlay(width, height int, base, box string, baseColor lipgloss.Color) string {
	box = strings.TrimRight(box, "\n")
	boxLines := strings.Split(box, "\n")
	boxW := 0
	for _, l := range boxLines {
		if w := lipgloss.Width(l); w > boxW {
			boxW = w
		}
	}
	x := (width - boxW) / 2
	if x < 0 {
		x = 0
	}
	y := (height - len(boxLines)) / 2
	if y < 0 {
		y = 0
	}
	baseLines := strings.Split(base, "\n")
	for i, bl := range boxLines {
		row := y + i
		if row >= len(baseLines) {
			break
		}
		baseLines[row] = spliceLine(baseLines[row], x, width, bl, baseColor)
	}
	return paint(strings.Join(baseLines, "\n"), width, height, baseColor)
}

// spliceLine splices boxLine into base at cell offset left, keeping base's
// prefix and suffix cells. Both sides stay exactly width cells.
func spliceLine(base string, left, width int, boxLine string, fill lipgloss.Color) string {
	prefix := clipStyled(base, left)
	if pad := left - lipgloss.Width(prefix); pad > 0 {
		prefix += lipgloss.NewStyle().Background(fill).Render(strings.Repeat(" ", pad))
	}
	boxW := lipgloss.Width(boxLine)
	suffixW := width - left - boxW
	suffix := ""
	if suffixW > 0 {
		suffix = clipStyled(ansi.Cut(base, left, ansi.StringWidth(base)), suffixW)
		if pad := suffixW - lipgloss.Width(suffix); pad > 0 {
			suffix += lipgloss.NewStyle().Background(fill).Render(strings.Repeat(" ", pad))
		}
		suffix = bgOpen(fill) + suffix
	}
	return prefix + boxLine + suffix
}

func paneBorderLeft(m *Model) lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(lipgloss.Color(m.palette.Muted))
}
