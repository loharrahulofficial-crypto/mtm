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

// letterspace expands caps section labels with thin spaces between letters,
// leaving THREE plain spaces between words so multi-word labels stay readable
// ("THIS WEEK" → "T H I S   W E E K", not "T H I S W E E K").
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
	return strings.Join(out, "   ")
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
// hint (optional, e.g. "3 active") is right-aligned in the title row / edge.
// bottomHint (optional) is centered on the bottom edge (e.g. "▼ more").
func renderPanel(m Model, title string, w, h int, focused bool, body []string, hint ...string) string {
	return renderPanelBottom(m, title, w, h, focused, body, "", "", hint...)
}

// renderPanelBottom is renderPanel with a muted centered indicator on the
// top and bottom edges. Used by panes whose content scrolls.
func renderPanelBottom(m Model, title string, w, h int, focused bool, body []string, centerHint, bottomHint string, hint ...string) string {
	surface := m.styles.surface
	border := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Accent)).Background(lipgloss.Color(m.palette.Surface))

	innerW := w - 2
	innerH := h - 2
	if innerW < 1 {
		innerW = 1
	}
	if innerH < 0 {
		innerH = 0
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
	hintText := ""
	if len(hint) > 0 {
		hintText = hint[0]
	}
	chipsText := ""
	if len(hint) > 1 {
		chipsText = hint[1]
	}
	if focused {
		lines = append(lines, panelEdgeCentered(m, w, true, title, border, surface, hintText, centerHint, chipsText))
		for _, row := range grid {
			lines = append(lines, border.Render("│")+row+border.Render("│"))
		}
		lines = append(lines, panelEdgeBottom(m, w, title, border, surface, bottomHint))
	} else {
		cap := panelTitleRowCentered(m, title, w, surface, centerHint, hintText, chipsText)
		lines = append(lines, cap)
		for _, row := range grid {
			lines = append(lines, surface.Render(" ")+row+surface.Render(" "))
		}
		lines = append(lines, panelBottomRow(m, w, surface, bottomHint))
	}
	return paint(strings.Join(lines, "\n"), w, h, lipgloss.Color(m.palette.Surface))
}

// panelEdgeBottom renders the bottom edge of a focused panel, embedding a
// centered muted hint (e.g. "▼ more") when set.
func panelEdgeBottom(m Model, w int, title string, border, surface lipgloss.Style, hint string) string {
	if hint == "" || w < 12 {
		return panelEdge(m, w, false, "", border, surface, "")
	}
	edge := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.MutedSoft)).Background(lipgloss.Color(m.palette.Surface)).Render(" " + hint + " ")
	edgeW := lipgloss.Width(edge)
	rest := w - 2 - edgeW
	if rest < 0 {
		return panelEdge(m, w, false, "", border, surface, "")
	}
	left := rest / 2
	fill := strings.Repeat("─", rest-left)
	out := border.Render("╰") + border.Render(strings.Repeat("─", left)) + edge + border.Render(fill) + border.Render("╯")
	return out
}

// panelBottomRow renders the last row of an unfocused panel: plain surface
// silence with the centered muted hint when set.
func panelBottomRow(m Model, w int, surface lipgloss.Style, hint string) string {
	if hint == "" {
		return surface.Render(strings.Repeat(" ", w))
	}
	fill := lipgloss.Color(m.palette.Surface)
	edge := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.MutedSoft)).Background(fill).Render(" " + hint + " ")
	edgeW := lipgloss.Width(edge)
	left := (w - edgeW) / 2
	if left < 0 {
		left = 0
	}
	right := w - left - edgeW
	if right < 0 {
		right = 0
	}
	return surface.Render(strings.Repeat(" ", left)) + edge + surface.Render(strings.Repeat(" ", right))
}

// panelTitleRow renders the inner title line of an unfocused panel:
// letterspaced caps section label + hairline rule, with an optional
// right-aligned hint ("3 active") after the rule.
func panelTitleRow(m Model, title string, w int, surface lipgloss.Style, hint ...string) string {
	return panelTitleRowCentered(m, title, w, surface, "", hint...)
}

// panelTitleRowCentered is panelTitleRow with a muted centered chip (e.g.
// "▲ more") embedded in the rule; the rule splits around the chip.
func panelTitleRowCentered(m Model, title string, w int, surface lipgloss.Style, centerHint string, hint ...string) string {
	if strings.TrimSpace(title) == "" {
		return surface.Render(strings.Repeat(" ", w))
	}
	text := sectionLabelText(m, title)
	textW := lipgloss.Width(text)
	lead := 2
	hintStr := ""
	hintW := 0
	if len(hint) > 0 && hint[0] != "" {
		hintStr = hint[0]
		hintW = lipgloss.Width(hintStr) + 1
	}
	chipsStr := ""
	chipsW := 0
	if len(hint) > 1 && hint[1] != "" {
		candidate := hint[1]
		// The label is never clipped to make room — drop the chips instead.
		if textW+lipgloss.Width(candidate)+1 <= w-lead-2-hintW {
			chipsStr = candidate
			chipsW = lipgloss.Width(chipsStr) + 1
		}
	}
	centerStr := ""
	centerW := 0
	if centerHint != "" {
		centerStr = lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.MutedSoft)).Background(lipgloss.Color(m.palette.Surface)).Render(" " + centerHint + " ")
		centerW = lipgloss.Width(centerStr) + 2
	}
	ruleLen := w - lead - textW - 2 - hintW - chipsW - centerW
	if ruleLen < 1 {
		ruleLen = 1
	}
	leftrule := ruleLen
	if centerStr != "" {
		leftrule = ruleLen / 2
	}
	rightrule := ruleLen - leftrule
	line := surface.Render(strings.Repeat(" ", lead)) + text
	if chipsStr != "" {
		line += surface.Render(" ") + chipsStr
	}
	line += surface.Render(" " + strings.Repeat("─", leftrule) + " ")
	if centerStr != "" {
		line += surface.Render("") + centerStr + surface.Render(" ")
	}
	line += surface.Render(strings.Repeat("─", rightrule) + " ")
	if hintStr != "" {
		line += surface.Render(hintStr + " ")
	}
	return line
}

// titledBorder renders one edge of a rounded box, width cells. top edges
// embed the letterspaced caps title; the filler is computed from the display
// width of the FINAL title string (letterspacing, padding, icons included),
// never from len() or raw rune counts — a border that stops short of the
// corner breaks the frame's vertical rhythm. Optional hint sits at the
// right end of the top edge.
func titledBorder(m Model, title string, w int, top bool, edge lipgloss.Style, hint ...string) string {
	return titledBorderCentered(m, title, w, top, edge, "", hint...)
}

// titledBorderCentered is titledBorder with a muted centered chip (e.g.
// "▲ more") embedded in the top edge.
func titledBorderCentered(m Model, title string, w int, top bool, edge lipgloss.Style, centerHint string, hint ...string) string {
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
	cap := sectionLabelText(m, title)
	hintStr := ""
	hintW := 0
	if len(hint) > 0 && hint[0] != "" {
		hintStr = hint[0]
		hintW = lipgloss.Width(hintStr) + 2 // " hint "
	}
	chipsStr := ""
	chipsW := 0
	if len(hint) > 1 && hint[1] != "" {
		candidate := hint[1]
		// The label is never clipped to make room — drop the chips instead.
		if lipgloss.Width(cap)+lipgloss.Width(candidate)+1 <= w-3-2-hintW {
			chipsStr = candidate
			chipsW = lipgloss.Width(chipsStr) + 1
		}
	}
	centerStr := ""
	centerW := 0
	if centerHint != "" {
		centerStr = lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.MutedSoft)).Background(lipgloss.Color(m.palette.Surface)).Render(" " + centerHint + " ")
		centerW = lipgloss.Width(centerStr) + 2
	}
	// ╭─ TITLE chips ─── centerHint ─── hint ╮
	const pre, tail = 3, 2 // "╭─ " and " "+cornerR
	inner := w - pre - tail - hintW - chipsW - centerW
	if lipgloss.Width(cap) > inner {
		cap = clipStyled(cap, inner)
	}
	fillN := inner - lipgloss.Width(cap) - chipsW
	if fillN < 0 {
		fillN = 0
	}
	out := edge.Render("╭─ ") + cap
	if chipsStr != "" {
		out += edge.Render(" ") + chipsStr
	}
	if centerStr != "" {
		leftrule := fillN / 2
		rightrule := fillN - leftrule
		out += edge.Render(" "+strings.Repeat(fill, leftrule)+" ") + centerStr + edge.Render(strings.Repeat(fill, rightrule))
	} else {
		out += edge.Render(" " + strings.Repeat(fill, fillN))
	}
	if hintStr != "" {
		out += edge.Render(" " + hintStr + " ")
	}
	out += edge.Render(cornerR)
	return out
}

// panelEdge draws the top or bottom edge of a focused panel, embedding an
// optional letterspaced title in the top edge. The hint (optional) is
// right-aligned in the top edge.
func panelEdge(m Model, w int, top bool, title string, border, surface lipgloss.Style, hint string, chips ...string) string {
	return panelEdgeCentered(m, w, top, title, border, surface, hint, "", chips...)
}

// panelEdgeCentered is panelEdge with a muted centered chip on the top edge.
func panelEdgeCentered(m Model, w int, top bool, title string, border, surface lipgloss.Style, hint string, centerHint string, chips ...string) string {
	if w < 2 {
		return border.Render("│")
	}
	chipsText := ""
	if len(chips) > 0 {
		chipsText = chips[0]
	}
	if hint == "" && chipsText == "" && centerHint == "" {
		return titledBorderCentered(m, title, w, top, border, centerHint)
	}
	return titledBorderCentered(m, title, w, top, border, centerHint, hint, chipsText)
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

// sectionLabelText renders the shared letterspaced caps section label
// (styled); every panel, border and sidebar section label flows through
// here. Letterspacing applies to the label word only — anything after a
// "·" separator (view names, counts, filters) renders as normal-case
// text, never letterspaced or uppercased: "T A S K S · All".
func sectionLabelText(m Model, label string) string {
	base, rest := label, ""
	if i := strings.Index(label, "·"); i >= 0 {
		base = strings.TrimSpace(label[:i])
		rest = strings.TrimSpace(label[i+len("·"):])
	}
	out := m.styles.sectionLabel.Render(letterspace(strings.ToUpper(base)))
	if rest != "" {
		out += m.styles.sectionLabel.Render(" · " + rest)
	}
	return out
}

// sectionLabel renders a quiet caps label with a trailing hairline rule,
// used for sidebar sections.
func sectionLabel(m Model, label string, w int) string {
	text := sectionLabelText(m, label)
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
