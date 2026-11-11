package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// paint renders content as a solid width×height block on bg. It is the one
// place that guarantees background continuity after nested styled segments:
//
//  1. every SGR sequence that resets all attributes (or defaults the
//     background) is immediately followed by a re-assertion of bg;
//  2. every line is clipped/padded to exactly width cells with bg;
//  3. the block is padded to exactly height rows with full-width bg lines.
//
// Explicit backgrounds inside the content (chips, highlights, selected rows)
// are kept. Everything else lands on bg, so panes never show gaps, stripes or
// default-background cells.
func paint(content string, width, height int, bg lipgloss.Color) string {
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}
	open := bgOpen(bg)
	fill := lipgloss.NewStyle().Background(bg)

	out := make([]string, 0, height)
	// Walk lines without strings.Split — the split+slice churn was one of
	// the hottest alloc sites in a full redraw. Split semantics preserved:
	// a trailing newline yields one final empty segment.
	rest := content
	endedWithNL := false
	for len(out) < height {
		var line string
		if i := strings.IndexByte(rest, '\n'); i >= 0 {
			line, rest = rest[:i], rest[i+1:]
			if rest == "" {
				endedWithNL = true
			}
		} else {
			if rest == "" && !endedWithNL {
				break
			}
			line, rest, endedWithNL = rest, "", false
		}
		if fastWidth(line) > width {
			line = clipStyled(line, width)
		}
		line = reassertBG(line, open)
		if pad := width - fastWidth(line); pad > 0 {
			line += fill.Render(strings.Repeat(" ", pad))
		}
		out = append(out, line)
	}
	for len(out) < height {
		out = append(out, fill.Render(strings.Repeat(" ", width)))
	}
	return strings.Join(out, "\n")
}

// sgrSeq is one parsed SGR sequence in a line: byte offsets and the raw
// sequence text.
type sgrSeq struct {
	start int
	end   int // one past the closing 'm'
	seq   string
}

// scanSGR walks line and reports every CSI … m sequence. Manual scan —
// the regexp version cost ~15% of all CPU in a full redraw.
func scanSGR(line string) []sgrSeq {
	var out []sgrSeq
	i := 0
	for i < len(line) {
		if line[i] == 0x1b && i+1 < len(line) && line[i+1] == '[' {
			j := i + 2
			valid := true
			for ; j < len(line); j++ {
				if line[j] == 'm' {
					break
				}
				if line[j] != ';' && line[j] != ':' && !(line[j] >= '0' && line[j] <= '9') {
					valid = false
					break
				}
			}
			if valid && j < len(line) && line[j] == 'm' {
				out = append(out, sgrSeq{start: i, end: j + 1, seq: line[i : j+1]})
				i = j + 1
				continue
			}
			i += 2 // not an SGR sequence; scan the payload as plain bytes
			continue
		}
		i++
	}
	return out
}

// bgOpen returns the SGR sequence that sets bg as background, taken from
// lipgloss's own serialization so the color profile matches everywhere.
func bgOpen(bg lipgloss.Color) string {
	probe := lipgloss.NewStyle().Background(bg).Render(" ")
	for _, s := range scanSGR(probe) {
		return s.seq
	}
	return ""
}

// reassertBG prefixes open and re-inserts it after every SGR sequence that
// drops the background (a full reset or a default-background code).
// Fast path: when nothing drops, open+line is returned directly —
// byte-identical to the builder path, no builder allocations.
func reassertBG(line, open string) string {
	if open == "" {
		return line
	}
	// Fast path — byte-identical to the builder path when no sequence
	// drops the background (prefix open + the line, nothing else).
	if !dropsBG(line) {
		return open + line
	}
	var b strings.Builder
	b.Grow(len(line) + 32)
	b.WriteString(open)
	last := 0
	for _, loc := range scanSGR(line) {
		b.WriteString(line[last:loc.start])
		b.WriteString(loc.seq)
		if sgrDropsBG(loc.seq) {
			b.WriteString(open)
		}
		last = loc.end
	}
	b.WriteString(line[last:])
	return b.String()
}

// dropsBG reports whether line contains at least one SGR sequence that
// drops the background. Zero allocations; the scanner mirrors scanSGR —
// keep the two in sync.
func dropsBG(line string) bool {
	i := 0
	for i < len(line) {
		if line[i] == 0x1b && i+1 < len(line) && line[i+1] == '[' {
			j := i + 2
			valid := true
			for ; j < len(line); j++ {
				if line[j] == 'm' {
					break
				}
				if line[j] != ';' && line[j] != ':' && !(line[j] >= '0' && line[j] <= '9') {
					valid = false
					break
				}
			}
			if valid && j < len(line) && line[j] == 'm' {
				if sgrDropsBG(line[i : j+1]) {
					return true
				}
				i = j + 1
				continue
			}
			i += 2
			continue
		}
		i++
	}
	return false
}

// sgrDropsBG reports whether an SGR sequence resets all attributes or sets
// the background back to default. Parameters are scanned byte-wise — no
// splitting; called for every sequence on every line on every frame.
// Assumes the same shape as the parser: [38|48|58][2|5] extended colors
// consume their arguments, 49/0/1 reset the background.
func sgrDropsBG(seq string) bool {
	params := seq[2 : len(seq)-1]
	if params == "" {
		return true
	}
	i := 0
	for i < len(params) {
		start := i
		for i < len(params) && params[i] != ';' {
			i++
		}
		p := params[start:i]
		switch p {
		case "", "0", "00", "000":
			return true
		case "49":
			return true
		case "38", "48", "58":
			// Extended color: 38;2;r;g;b or 38;5;n — consume the
			// argument params so they are not misparsed as resets.
			i++ // skip ';' after this param
			if i < len(params) {
				mode := params[i]
				switch {
				case mode == '2': // truecolor: consume 4 more params (r,g,b or ;g;b)
					for k := 0; k < 4 && i < len(params); k++ {
						i = skipSGRParam(params, i)
					}
				case mode == '5': // 256-color: consume 2 more params (;n)
					i = skipSGRParam(params, i)
					i = skipSGRParam(params, i)
				default:
					i = skipSGRParam(params, i)
				}
			}
			continue
		}
		if i < len(params) {
			i++ // skip ';' between params
		}
	}
	return false
}

// skipSGRParam skips from i past the next ';' (or the end), returning the
// index just past it — i.e. the start of the following parameter.
func skipSGRParam(params string, i int) int {
	for i < len(params) {
		for i < len(params) && params[i] != ';' {
			i++
		}
		if i < len(params) && params[i] == ';' {
			return i + 1
		}
		return len(params)
	}
	return len(params)
}
