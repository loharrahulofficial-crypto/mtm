package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderHelpBox renders the keybinding help overlay. The box is windowed
// to `height` rows: j/k scroll through the entries. Without windowing,
// terminals shorter than the content would drop the bottom groups.
func renderHelpBox(km KeyMap, s styles, width, height, scroll int) string {
	type group struct {
		name    string
		actions []string
	}
	groups := []group{
		{"view", []string{aViewHome, aViewList, aViewBoard, aViewCalendar, aViewCourses, aViewStats, aViewIdeas, aCycleView, aFocus, aCycleScene, aEnter}},
		{"move", []string{aUp, aDown, aLeft, aRight, aJumpLeft, aJumpRight, aMonthPrev, aMonthNext, aTop, aBottom, aPageDown, aPageUp, aFocusLeft, aFocusRight, aSidebar}},
		{"filter", []string{aSearch, aSearchNext, aSearchPrev, aPalette}},
		{"task", []string{aAdd, aAddSubtask, aToggleDone, aDelete, aStatus, aPriority, aDue, aTags, aTitle, aArchive, aToggleCalendarDone, aGrade, aRate}},
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

	lines := []string{s.mAccent.Render("  keybindings"), ""}
	for _, g := range groups {
		lines = append(lines, s.mMuted.Render("  "+strings.ToUpper(g.name)))
		for _, action := range g.actions {
			e, ok := entries[action]
			if !ok {
				continue
			}
			key := padRight(e[2], 12)
			lines = append(lines, fmt.Sprintf("    %s %s", s.mAccent.Render(key), s.mText.Render(e[1])))
		}
	}

	// Window to the screen: content is allLines, scroll is the first row.
	maxContent := height - 6 // rounded border (2) + padding (2) + margin (2)
	if maxContent < 3 {
		maxContent = 3
	}
	start := scroll
	if maxStart := len(lines) - maxContent; start > maxStart {
		start = maxStart
	}
	if start < 0 {
		start = 0
	}
	visible := lines
	if len(lines) > maxContent {
		visible = lines[start : start+maxContent]
	}

	var b strings.Builder
	for _, l := range visible {
		b.WriteString(l + "\n")
	}
	if hint := helpScrollHint(len(lines), maxContent, start); hint != "" {
		b.WriteString("\n" + s.mMuted.Render(hint))
	} else {
		b.WriteString("\n" + s.mMuted.Render("  j/k scroll · any key closes · config: keys: in config.yaml"))
	}
	raised := lipgloss.Color(s.p.SurfaceRaised)
	block := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(s.p.Accent)).
		Background(raised).
		Padding(1, 2).
		Render(b.String())
	return paint(block, fastWidth(block), lipgloss.Height(block), raised)
}

func helpScrollHint(total, maxContent, start int) string {
	if total <= maxContent {
		return ""
	}
	return fmt.Sprintf("  j/k scroll · %d-%d of %d · esc closes", start+1, start+maxContent, total)
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
	return paint(block, fastWidth(block), lipgloss.Height(block), raised)
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
	return paint(block, fastWidth(block), lipgloss.Height(block), raised)
}
