package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
)

// Active-filter chips shown next to the panel title (list view). Chips are
// compact pills on SurfaceStrong with BrightText — they sit in the title
// row between the label and the hairline rule.

// listFilterChips returns the active filter tokens (max width maxW, cells):
// parsed search tokens as "key:value" (or "#tag"), plus the raw free text
// as "“text”". When the tokens exceed maxW, the last visible chip becomes
// "+N".
func listFilterChips(m Model, maxW int) []string {
	q := strings.TrimSpace(m.searchQuery)
	if q == "" {
		return nil
	}
	type chip struct{ text string }
	var chips []string
	if f, needle, err := core.ParseSearchQuery(q); err == nil {
		if f.Project != "" {
			chips = append(chips, "project:"+leafName(f.Project))
		}
		if f.Tag != "" {
			chips = append(chips, "#"+f.Tag)
		}
		if f.Status != "" {
			chips = append(chips, "status:"+f.Status)
		}
		if f.Priority != "" {
			chips = append(chips, "priority:"+f.Priority)
		}
		if f.Due != "" {
			chips = append(chips, "due:"+f.Due)
		}
		if needle != "" {
			chips = append(chips, fmt.Sprintf("“%s”", truncatePlain(needle, 24)))
		}
	} else {
		chips = append(chips, fmt.Sprintf("“%s”", truncatePlain(q, 24)))
	}
	if len(chips) == 0 {
		return nil
	}
	// Budget the row: append chips until the budget runs out, then fold
	// the remainder into a trailing "+N" chip (never eating a chip that
	// already fit).
	out := []string{}
	width := 0
	for i, c := range chips {
		w := lipgloss.Width(c) + 2 // one leading + trailing space per chip
		if i > 0 {
			w++ // inter-chip gap
		}
		if maxW > 0 && width+w > maxW && len(out) > 0 {
			break
		}
		out = append(out, c)
		width += w
	}
	if len(out) < len(chips) {
		more := fmt.Sprintf("+%d", len(chips)-len(out))
		moreW := lipgloss.Width(more) + 2
		switch {
		case len(out) == 0:
			out = []string{more}
		case width+1+moreW <= maxW || maxW <= 0:
			out = append(out, more)
		default:
			out[len(out)-1] = more
		}
	}
	return out
}

// filterChipsText renders the chips as pills joined by a space, sized to
// maxW cells (0 = no limit). Returns "" when nothing is active.
func filterChipsText(m Model, maxW int) string {
	chips := listFilterChips(m, maxW)
	if len(chips) == 0 {
		return ""
	}
	chipStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(m.palette.BrightText)).
		Background(lipgloss.Color(m.palette.SurfaceStrong))
	parts := make([]string, 0, len(chips))
	for _, c := range chips {
		parts = append(parts, chipStyle.Render(" "+c+" "))
	}
	return strings.Join(parts, " ")
}

// listPanelTitle builds the list panel title from the active view:
// "TASKS · <view>" with the saved view / project / smart view name.
func listPanelTitle(m Model) string {
	sel := m.sidebarSelection()
	label := "ALL"
	if sel != nil {
		switch sel.Kind {
		case "smart":
			label = strings.ToUpper(sel.Key)
		case "view":
			label = strings.ToUpper(sel.Key)
		case "project":
			label = strings.ToUpper(leafName(sel.Key))
		case "home":
			label = "ALL"
		}
	}
	if label == "" {
		label = "ALL"
	}
	return "TASKS · " + label
}
