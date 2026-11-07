package tui

import (
	"fmt"
	"strings"

	"github.com/mattn/go-runewidth"

	"github.com/charmbracelet/lipgloss"
)

func renderSidebar(m Model, w, h int) string {
	s := m.styles
	var b strings.Builder
	rows := sidebarRows(m.sidebarItems)

	// Scroll so the selected item is visible.
	selRow := 0
	rowIdx := 0
	for i, r := range rows {
		if r.sel == m.sidebarSel {
			selRow = i
		}
		rowIdx = i + 1
	}
	_ = selRow
	_ = rowIdx

	top := 0
	if selRow >= h {
		top = selRow - h + 1
	}

	line := 0
	for i, r := range rows {
		if i < top {
			continue
		}
		if line >= h {
			break
		}
		if r.sel < 0 {
			b.WriteString(s.dim.Bold(true).Width(w).Render("  "+r.header) + "\n")
			line++
			continue
		}
		it := m.sidebarItems[r.sel]
		selected := r.sel == m.sidebarSel && m.focus == focusSidebar
		var left, right string
		switch it.Kind {
		case "smart":
			icon := "≡"
			switch it.Key {
			case "all":
				icon = "≡"
			case "today":
				icon = "◆"
			case "week":
				icon = "◇"
			case "overdue":
				icon = "▲"
			}
			iconStyle := s.accent
			if it.Key == "overdue" && it.Count > 0 {
				iconStyle = s.overdue
			}
			left = iconStyle.Render(icon) + " " + it.Name
			if it.Count > 0 {
				right = fmt.Sprint(it.Count)
			}
		case "view":
			left = s.dim.Render("▸") + " " + it.Name
		case "project":
			pct := 0
			if it.Total > 0 {
				pct = it.Done * 100 / it.Total
			}
			if it.Total == 0 {
				pct = 0
			}
			name := it.Name
			maxName := w - 12
			if runewidth.StringWidth(name) > maxName {
				name = truncatePlain(name, maxName)
			}
			left = "▸ " + name
			right = fmt.Sprintf("%d%%", pct)
		}
		pad := w - runewidth.StringWidth(left) - runewidth.StringWidth(right) - 2
		if pad < 1 {
			pad = 1
		}
		text := " " + left + strings.Repeat(" ", pad) + right
		if selected {
			b.WriteString(s.selected.Width(w).Render(text) + "\n")
		} else {
			b.WriteString(s.base.Width(w).Render(text) + "\n")
		}
		line++
	}
	for count := strings.Count(b.String(), "\n"); count < h; count++ {
		b.WriteString(strings.Repeat(" ", w) + "\n")
	}
	pane := lipgloss.NewStyle().Width(w).Render(b.String())
	return normalizeBlock(pane, w, h)
}

// normalizeBlock forces a rendered block to exactly w columns and h rows:
// every line is padded (styles preserved) and overflow rows are dropped.
// Every pane goes through this so JoinVertical/Horizontal never misalign the
// frame (bug: a one-line overflow scrolled the top row off the terminal).
func normalizeBlock(block string, w, h int) string {
	lines := strings.Split(block, "\n")
	// Keep the first h lines; pad or trim each to exactly w.
	if len(lines) > h {
		lines = lines[:h]
	}
	for i, l := range lines {
		if diff := w - lipgloss.Width(l); diff > 0 {
			lines[i] = l + strings.Repeat(" ", diff)
		} else if diff < 0 {
			lines[i] = truncatePlain(l, w)
		}
	}
	for len(lines) < h {
		lines = append(lines, strings.Repeat(" ", w))
	}
	return strings.Join(lines, "\n")
}
