package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Panel rendering for the Study Desk design language.
//
// Every pane renders at exactly w×h. The perimeter is reserved space:
// unfocused panes show flat surface fill, focused panes draw a rounded
// accent border there. The content box is therefore always (w-2)×(h-2) —
// focus never reflows the layout.
//
// Panels sit on `surface`; the 1-cell gutters between panes show `base`.

const thinSpace = " " // U+2009 thin space

// letterspace expands caps section labels with thin spaces, word by word:
// "THIS WEEK" → "T H I S W E E K" (spaces stay single cells).
func letterspace(s string) string {
	words := strings.Fields(s)
	out := make([]string, 0, len(words))
	for _, word := range words {
		r := []rune(word)
		parts := make([]string, 0, len(r)*2-1)
		for i, c := range r {
			if i > 0 {
				parts = append(parts, thinSpace)
			}
			parts = append(parts, string(c))
		}
		out = append(out, strings.Join(parts, ""))
	}
	return strings.Join(out, " ")
}

// renderPanelBorderless renders body as a solid surface slab with no border
// and no title row — used by Home, which keeps flat panel rhythm with its
// inner panels carrying the single focus ring.
func renderPanelBorderless(m Model, w, h int, body []string) string {
	surface := lipgloss.Color(m.palette.Surface)
	lines := make([]string, 0, h)
	for i := 0; i < h; i++ {
		line := ""
		if i < len(body) {
			line = body[i]
		}
		lines = append(lines, clipStyled(line, w))
	}
	return paint(strings.Join(lines, "\n"), w, h, surface)
}

// renderPanel draws one pane. body must already be split into lines; each
// line is clipped/padded to the content width. focused draws the accent
// rounded border; title (optional, uppercase caps) sits in the top edge.
func renderPanel(m Model, title string, w, h int, focused bool, body []string) string {
	surface := m.styles.surface
	border := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Accent)).Background(lipgloss.Color(m.palette.Surface))

	innerW := w - 2
	innerH := h - 2
	if innerW < 1 {
		innerW = 1
	}
	if innerH < 1 {
		innerH = 1
	}

	// Content grid clipped to innerH rows; each row is padded to the inner
	// width on surface so the pane's own borders always land on the edge.
	surfaceFill := lipgloss.NewStyle().Background(lipgloss.Color(m.palette.Surface))
	grid := make([]string, 0, innerH)
	for i := 0; i < innerH; i++ {
		line := ""
		if i < len(body) {
			line = body[i]
		}
		line = clipStyled(line, innerW)
		if pad := innerW - lipgloss.Width(line); pad > 0 {
			line += surfaceFill.Render(strings.Repeat(" ", pad))
		}
		grid = append(grid, line)
	}

	lines := make([]string, 0, h)
	if focused {
		lines = append(lines, panelEdge(m, w, true, title, border, surface))
		for _, row := range grid {
			lines = append(lines, border.Render("│")+row+border.Render("│"))
		}
		lines = append(lines, panelEdge(m, w, false, "", border, surface))
	} else {
		cap := panelTitleRow(m, title, w, surface)
		lines = append(lines, cap)
		for _, row := range grid {
			lines = append(lines, surface.Render(" ")+row+surface.Render(" "))
		}
		lines = append(lines, surface.Render(strings.Repeat(" ", w)))
	}
	return paint(strings.Join(lines, "\n"), w, h, lipgloss.Color(m.palette.Surface))
}

// panelTitleRow renders the inner title line of an unfocused panel:
// letterspaced caps section label + hairline rule.
func panelTitleRow(m Model, title string, w int, surface lipgloss.Style) string {
	if strings.TrimSpace(title) == "" {
		return surface.Render(strings.Repeat(" ", w))
	}
	text := m.styles.sectionLabel.Render(letterspace(strings.ToUpper(title)))
	textW := lipgloss.Width(text)
	lead := 2
	ruleLen := w - lead - textW - 2
	if ruleLen < 1 {
		ruleLen = 1
	}
	return surface.Render(strings.Repeat(" ", lead)) + text +
		surface.Render(" "+strings.Repeat("─", ruleLen)+" ")
}

// titledBorder renders one edge of a rounded box, width cells. top edges
// embed the letterspaced caps title; the filler is computed from the display
// width of the FINAL title string (letterspacing, padding, icons included),
// never from len() or raw rune counts — a border that stops short of the
// corner breaks the frame's vertical rhythm.
func titledBorder(title string, w int, top bool, edge, titleStyle lipgloss.Style) string {
	cornerL, cornerR, fill := "╭", "╮", "─"
	if !top {
		cornerL, cornerR = "╰", "╯"
	}
	if !top || strings.TrimSpace(title) == "" || w < 5 {
		if w < 2 {
			return edge.Render(cornerL)
		}
		return edge.Render(cornerL + strings.Repeat(fill, w-2) + cornerR)
	}
	cap := titleStyle.Render(letterspace(strings.ToUpper(title)))
	// ╭─ TITLE ───╮
	const pre, tail = 3, 2 // "╭─ " and " "+cornerR
	inner := w - pre - tail
	if lipgloss.Width(cap) > inner {
		cap = clipStyled(cap, inner)
	}
	fillN := inner - lipgloss.Width(cap)
	if fillN < 0 {
		fillN = 0
	}
	return edge.Render("╭─ ") + cap + edge.Render(" "+strings.Repeat(fill, fillN)+cornerR)
}

// panelEdge draws the top or bottom edge of a focused panel, embedding an
// optional letterspaced title in the top edge.
func panelEdge(m Model, w int, top bool, title string, border, surface lipgloss.Style) string {
	if w < 2 {
		return border.Render("│")
	}
	return titledBorder(title, w, top, border, m.styles.sectionLabel)
}

// clipStyled truncates a line that may contain ANSI escape sequences to
// exactly width cells (lipgloss MaxWidth is escape-aware).
func clipStyled(line string, width int) string {
	return lipgloss.NewStyle().MaxWidth(width).Render(line)
}

// gutterColumn is the 1-cell base-colored spacer between panes.
func gutterColumn(h int, base lipgloss.Style) string {
	return strings.Join(repeatStr(base.Render(" "), h), "\n")
}

func repeatStr(s string, n int) []string {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = s
	}
	return lines
}

// sectionLabel renders a quiet caps label with a trailing hairline rule,
// used for sidebar sections.
func sectionLabel(m Model, label string, w int) string {
	text := m.styles.sectionLabel.Render(letterspace(strings.ToUpper(label)))
	ruleLen := w - lipgloss.Width(text) - ContentInset - 2
	if ruleLen < 1 {
		ruleLen = 1
	}
	return m.styles.muted.Render(strings.Repeat(" ", ContentInset) + text + " " + strings.Repeat("─", ruleLen))
}

// normalizeBlock forces a rendered block to exactly w columns and h rows:
// every line is clipped/padded to width and every pixel gets bg. Every pane
// goes through this so JoinVertical/Horizontal never misalign the frame
// (bug: a one-line overflow scrolled the top row off the terminal).
func normalizeBlock(block string, w, h int, bg lipgloss.Color) string {
	return paint(block, w, h, bg)
}

// minUsableWidth/minUsableHeight bound the smallest terminal mtm will lay out
// in. Below this the whole frame collapses to the too-small message instead
// of a broken pane salad.
const (
	minUsableWidth  = 60
	minUsableHeight = 15
)

// tooSmallView renders the single-purpose "terminal too small" screen.
func tooSmallView(w, h int, bg lipgloss.Color) string {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	lines := []string{}
	box := []string{
		"mtm needs a bigger terminal",
		fmt.Sprintf("currently %d×%d — need at least %d×%d", w, h, minUsableWidth, minUsableHeight),
		"q quits",
	}
	contentW := 0
	for _, l := range box {
		if lipgloss.Width(l) > contentW {
			contentW = lipgloss.Width(l)
		}
	}
	if contentW > w-2 {
		// Extremely narrow: hard-wrap the text.
		box = []string{}
		for _, l := range []string{
			"mtm needs a", "bigger terminal",
			fmt.Sprintf("need %d×%d", minUsableWidth, minUsableHeight),
			"q quits",
		} {
			box = append(box, l)
		}
		contentW = lipgloss.Width("bigger terminal")
	}
	topPad := (h - len(box) - 2) / 2
	if topPad < 0 {
		topPad = 0
	}
	blank := strings.Repeat(" ", w)
	for i := 0; i < topPad; i++ {
		lines = append(lines, blank)
	}
	for _, l := range box {
		pad := (w - lipgloss.Width(l)) / 2
		if pad < 0 {
			pad = 0
		}
		lines = append(lines, strings.Repeat(" ", pad)+l)
	}
	for len(lines) < h {
		lines = append(lines, blank)
	}
	return normalizeBlock(strings.Join(lines, "\n"), w, h, bg)
}
