package tui

import "github.com/charmbracelet/lipgloss"

// fastWidth measures the display width of s in terminal cells.
//
// Hot-path replacement for lipgloss.Width (full ANSI parser + grapheme
// clustering). It walks the bytes directly and handles the sequences mtm
// emits: CSI color runs (ESC [ … final byte) and OSC runs (ESC ] … BEL or
// ST) are zero-width; printable ASCII is one cell; a control byte is zero.
// Anything else (non-ASCII text, grapheme clusters, wide runes) bails out
// to lipgloss.Width for correctness — scene lines are full of halfblock
// glyphs and pay that path, ordinary pane lines stay in the fast loop.
func fastWidth(s string) int {
	n := 0
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == 0x1b:
			i = skipEscape(s, i)
			continue
		case c < 0x20:
			i++
			continue
		case c < 0x7f:
			n++
			i++
			continue
		default:
			// Non-ASCII: grapheme clusters and wide runes need the
			// real library.
			return lipgloss.Width(s)
		}
	}
	return n
}

// skipEscape consumes one escape sequence starting at i (the ESC byte)
// and returns the index just past it.
func skipEscape(s string, i int) int {
	if i+1 >= len(s) {
		return len(s)
	}
	switch s[i+1] {
	case '[': // CSI: parameter bytes until a final byte in @..~
		for j := i + 2; j < len(s); j++ {
			if s[j] >= 0x40 && s[j] <= 0x7e {
				return j + 1
			}
		}
		return len(s)
	case ']': // OSC: until BEL or ST (ESC \)
		for j := i + 2; j < len(s); j++ {
			if s[j] == '\a' {
				return j + 1
			}
			if s[j] == 0x1b && j+1 < len(s) && s[j+1] == '\\' {
				return j + 2
			}
		}
		return len(s)
	default: // two-byte escape
		return i + 2
	}
}
