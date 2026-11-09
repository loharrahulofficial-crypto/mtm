package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderHelpBox renders the keybinding help overlay.
func renderHelpBox(km KeyMap, s styles, width int) string {
	type group struct {
		name    string
		actions []string
	}
	groups := []group{
		{"view", []string{aViewHome, aViewList, aViewBoard, aViewCalendar, aViewCourses, aCycleView, aFocus, aEnter}},
		{"move", []string{aUp, aDown, aLeft, aRight, aJumpLeft, aJumpRight, aMonthPrev, aMonthNext, aTop, aBottom, aPageDown, aPageUp, aFocusLeft, aFocusRight, aSidebar}},
		{"filter", []string{aSearch, aSearchNext, aSearchPrev, aPalette}},
		{"task", []string{aAdd, aAddSubtask, aToggleDone, aDelete, aStatus, aPriority, aDue, aTags, aTitle, aArchive, aToggleCalendarDone, aGrade}},
		{"notes", []string{aNotes, aNotesEditor, aEditFile, aDetail}},
		{"sync", []string{aSync, aConflicts}},
		{"meta", []string{aHelp, aQuit, aPaletteColon}},
	}
	entries := map[string][3]string{}
	for _, e := range HelpEntries(km) {
		entries[e[0]] = e
	}

	boxW := width / 2
	if boxW > 64 {
		boxW = 64
	}
	if boxW < 44 {
		boxW = 44
	}

	var b strings.Builder
	raised := lipgloss.Color(s.p.SurfaceRaised)
	b.WriteString(s.mAccent.Render("  keybindings") + "\n\n")
	for _, g := range groups {
		b.WriteString(s.mMuted.Render("  "+strings.ToUpper(g.name)) + "\n")
		for _, action := range g.actions {
			e, ok := entries[action]
			if !ok {
				continue
			}
			key := padRight(e[2], 12)
			b.WriteString(fmt.Sprintf("    %s %s\n", s.mAccent.Render(key), s.mText.Render(e[1])))
		}
	}
	b.WriteString("\n" + s.mMuted.Render("  any key closes · config: keys: in config.yaml"))
	block := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(s.p.Accent)).
		Background(raised).
		Padding(1, 2).
		Render(b.String())
	return paint(block, lipgloss.Width(block), lipgloss.Height(block), raised)
}

// renderConfirmBox renders the delete/archive confirmation overlay.
func (m Model) renderConfirmBox() string {
	s := m.styles
	raised := lipgloss.Color(s.p.SurfaceRaised)
	var b strings.Builder
	b.WriteString(s.mAccent.Render("  "+m.confirmTitle) + "\n\n")
	b.WriteString(s.mText.Render("  "+m.confirmBody) + "\n\n")
	b.WriteString(s.mMuted.Render("  ") + s.mAccent.Render("y") + s.mMuted.Render(" confirm    ") + s.mAccent.Render("n/esc") + s.mMuted.Render(" cancel"))
	block := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(s.p.Red)).
		Background(raised).
		Padding(1, 2).
		Render(b.String())
	return paint(block, lipgloss.Width(block), lipgloss.Height(block), raised)
}

// renderPickerBox renders the status/priority picker overlay.
func (m Model) renderPickerBox() string {
	s := m.styles
	raised := lipgloss.Color(s.p.SurfaceRaised)
	var b strings.Builder
	b.WriteString(s.mAccent.Render("  "+m.pickerTitle) + "\n\n")
	for i, item := range m.pickerItems {
		marker := "  "
		style := s.mText
		if i == m.pickerSel {
			marker = "▸ "
			style = s.selected
		}
		b.WriteString(style.Render(marker+item) + "\n")
	}
	b.WriteString("\n" + s.mMuted.Render("  j/k choose · enter apply · esc cancel"))
	block := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(s.p.Accent)).
		Background(raised).
		Padding(1, 2).
		Render(b.String())
	return paint(block, lipgloss.Width(block), lipgloss.Height(block), raised)
}
