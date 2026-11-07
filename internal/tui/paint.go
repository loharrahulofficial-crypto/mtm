package tui

import (
	"regexp"
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

	lines := strings.Split(content, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	out := make([]string, 0, height)
	for _, line := range lines {
		if lipgloss.Width(line) > width {
			line = clipStyled(line, width)
		}
		line = reassertBG(line, open)
		if pad := width - lipgloss.Width(line); pad > 0 {
			line += fill.Render(strings.Repeat(" ", pad))
		}
		out = append(out, line)
	}
	for len(out) < height {
		out = append(out, fill.Render(strings.Repeat(" ", width)))
	}
	return strings.Join(out, "\n")
}

var sgrPattern = regexp.MustCompile("\x1b\\[[0-9;]*m")

// bgOpen returns the SGR sequence that sets bg as background, taken from
// lipgloss's own serialization so the color profile matches everywhere.
func bgOpen(bg lipgloss.Color) string {
	probe := lipgloss.NewStyle().Background(bg).Render(" ")
	seqs := sgrPattern.FindAllString(probe, -1)
	if len(seqs) == 0 {
		return ""
	}
	return seqs[0]
}

// reassertBG prefixes open and re-inserts it after every SGR sequence that
// drops the background (a full reset or a default-background code).
func reassertBG(line, open string) string {
	if open == "" {
		return line
	}
	var b strings.Builder
	b.Grow(len(line) + 32)
	b.WriteString(open)
	last := 0
	for _, loc := range sgrPattern.FindAllStringIndex(line, -1) {
		b.WriteString(line[last:loc[0]])
		seq := line[loc[0]:loc[1]]
		b.WriteString(seq)
		if sgrDropsBG(seq) {
			b.WriteString(open)
		}
		last = loc[1]
	}
	b.WriteString(line[last:])
	return b.String()
}

// sgrDropsBG reports whether an SGR sequence resets all attributes or sets
// the background back to default. Parameters are consumed the way ANSI parses
// them: 38/48/58 introduce an extended color and eat their arguments.
func sgrDropsBG(seq string) bool {
	params := strings.TrimSuffix(strings.TrimPrefix(seq, "\x1b["), "m")
	parts := strings.Split(params, ";")
	for i := 0; i < len(parts); i++ {
		switch p := parts[i]; p {
		case "", "0", "00", "000":
			return true
		case "38", "48", "58":
			switch {
			case i+2 < len(parts) && parts[i+1] == "2":
				i += 4
			case i+1 < len(parts) && parts[i+1] == "5":
				i += 2
			case i+1 < len(parts):
				i++
			}
		case "49":
			return true
		}
	}
	return false
}
