package tui

import (
	"strings"
)

// joinBlocksH concatenates same-height blocks side by side. Every pane is
// already normalized to exactly its target width×height (renderPanel,
// renderPanelBorderless and paint guarantee it), so this is byte-identical
// to lipgloss.JoinHorizontal(lipgloss.Top, parts...) for mtm's panes —
// but it does no grapheme-width parsing inside the frame loop and no
// strings.Split intermediates.
func joinBlocksH(parts []string) string {
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return parts[0]
	}
	maxRows := 0
	total := 0
	for _, p := range parts {
		if n := strings.Count(p, "\n") + 1; p != "" && n > maxRows {
			maxRows = n
		}
		total += len(p)
	}
	offs := make([]int, len(parts))
	var b strings.Builder
	b.Grow(total + maxRows + len(parts))
	for r := 0; r < maxRows; r++ {
		for i, p := range parts {
			o := offs[i]
			if o >= len(p) {
				continue // this part ran out of rows; nothing to add
			}
			end := strings.IndexByte(p[o:], '\n')
			var e int
			if end < 0 {
				e = len(p)
			} else {
				e = o + end
			}
			b.WriteString(p[o:e])
			if end < 0 {
				offs[i] = len(p)
			} else {
				offs[i] = e + 1
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
