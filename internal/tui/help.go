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
		{"view", []string{aViewHome, aViewList, aViewBoard, aViewCalendar, aCycleView, aFocus}},
		{"move", []string{aUp, aDown, aLeft, aRight, aJumpLeft, aJumpRight, aTop, aBottom, aPageDown, aPageUp, aFocusLeft, aFocusRight, aSidebar}},
		{"filter", []string{aSearch, aSearchNext, aSearchPrev, aPalette}},
		{"task", []string{aAdd, aAddSubtask, aToggleDone, aDelete, aStatus, aPriority, aDue, aTags, aTitle, aArchive}},
		{"notes", []string{aNotes, aNotesEditor, aEditFile, aDetail}},
		{"sync", []string{aSync, aConflicts}},
		{"meta", []string{aHelp, aQuit, aCommand}},
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
	b.WriteString(s.accent.Render("  keybindings") + "\n\n")
	for _, g := range groups {
		b.WriteString(s.dim.Render("  "+strings.ToUpper(g.name)) + "\n")
		for _, action := range g.actions {
			e, ok := entries[action]
			if !ok {
				continue
			}
			key := padRight(e[2], 12)
			b.WriteString(fmt.Sprintf("    %s %s\n", s.accent.Render(key), s.base.Render(e[1])))
		}
	}
	b.WriteString("\n" + s.dim.Render("  any key closes · config: keys: in config.yaml"))
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(s.p.Accent)).
		Padding(1, 2).
		Render(b.String())
}

// renderConfirmBox renders the delete/archive confirmation overlay.
func (m Model) renderConfirmBox() string {
	s := m.styles
	var b strings.Builder
	b.WriteString(s.accent.Render("  "+m.confirmTitle) + "\n\n")
	b.WriteString(s.base.Render("  "+m.confirmBody) + "\n\n")
	b.WriteString(s.dim.Render("  ") + s.accent.Render("y") + s.dim.Render(" confirm    ") + s.accent.Render("n/esc") + s.dim.Render(" cancel"))
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(s.p.Red)).
		Padding(1, 2).
		Render(b.String())
}

// renderPickerBox renders the status/priority picker overlay.
func (m Model) renderPickerBox() string {
	s := m.styles
	var b strings.Builder
	b.WriteString(s.accent.Render("  "+m.pickerTitle) + "\n\n")
	for i, item := range m.pickerItems {
		marker := "  "
		style := s.base
		if i == m.pickerSel {
			marker = "▸ "
			style = s.selected
		}
		b.WriteString(style.Render(marker+item) + "\n")
	}
	b.WriteString("\n" + s.dim.Render("  j/k choose · enter apply · esc cancel"))
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(s.p.Accent)).
		Padding(1, 2).
		Render(b.String())
}
