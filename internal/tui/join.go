package tui

import "strings"

// joinBlocksH concatenates same-height blocks side by side. Every pane is
// already normalized to exactly its target width×height (renderPanel,
// renderPanelBorderless and paint guarantee it), so this is byte-identical
// to lipgloss.JoinHorizontal(lipgloss.Top, parts...) for mtm's panes —
// but it does no grapheme-width parsing inside the frame loop.
func joinBlocksH(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 {
		return parts[0]
	}
	lines := make([][]string, len(parts))
	maxRows := 0
	total := 0
	for i, p := range parts {
		lines[i] = strings.Split(p, "\n")
		if len(lines[i]) > maxRows {
			maxRows = len(lines[i])
		}
		total += len(p)
	}
	var b strings.Builder
	b.Grow(total + maxRows + len(parts))
	for r := 0; r < maxRows; r++ {
		for i := range parts {
			if r < len(lines[i]) {
				b.WriteString(lines[i][r])
			}
		}
		if r < maxRows-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// joinBlocksV stacks blocks vertically. Same reasoning as joinBlocksH;
// replaces lipgloss.JoinVertical for the frame loop.
func joinBlocksV(parts []string) string {
	total := 0
	for _, p := range parts {
		total += len(p)
	}
	var b strings.Builder
	b.Grow(total + len(parts))
	for i, p := range parts {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(p)
	}
	return b.String()
}
