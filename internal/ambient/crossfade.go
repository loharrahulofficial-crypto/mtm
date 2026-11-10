package ambient

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// crossfade.go — ANSI-aware frame blending for scene transitions
// (alt+s / :scene) and Pomodoro phase changes. Both frames are decoded
// into a cell grid; every cell's fg/bg blends toward the target by the
// eased progress; endpoints return the exact source frames.

// xCell is one terminal cell: colors as "#rrggbb" ("" = unset) and a
// single-cell glyph (all ambient glyphs are 1 cell wide).
type xCell struct {
	fg, bg, glyph string
}

// parseFrame decodes rendered lines into a uniform cell grid.
func parseFrame(lines []Line) [][]xCell {
	out := make([][]xCell, 0, len(lines))
	for _, line := range lines {
		row := []xCell{}
		fg, bg := "", ""
		rest := line
		for len(rest) > 0 {
			if strings.HasPrefix(rest, "\x1b[") {
				end := strings.Index(rest, "m")
				if end < 0 {
					break
				}
				fg, bg = applySGRStrings(fg, bg, rest[:end+1])
				rest = rest[end+1:]
				continue
			}
			r, size := decodeRune(rest)
			if r == 0 {
				break
			}
			row = append(row, xCell{fg, bg, string(r)})
			rest = rest[size:]
		}
		out = append(out, row)
	}
	return out
}

// applySGRStrings applies one SGR sequence to the current fg/bg hexes.
func applySGRStrings(fg, bg, seq string) (string, string) {
	params := strings.TrimSuffix(strings.TrimPrefix(seq, "\x1b["), "m")
	parts := strings.Split(params, ";")
	i := 0
	for i < len(parts) {
		p := strings.TrimSpace(parts[i])
		switch p {
		case "", "0", "00", "000":
			return "", ""
		case "39":
			fg = ""
			i++
			continue
		case "49":
			bg = ""
			i++
			continue
		case "38", "48":
			target := &fg
			if p == "48" {
				target = &bg
			}
			if i+4 < len(parts) && parts[i+1] == "2" {
				r, ok1 := atoiSmall(parts[i+2])
				g, ok2 := atoiSmall(parts[i+3])
				b, ok3 := atoiSmall(parts[i+4])
				if ok1 && ok2 && ok3 {
					*target = fmt.Sprintf("#%02x%02x%02x", r, g, b)
				}
				i += 5
				continue
			}
			i++
			continue
		default:
			i++
		}
	}
	return fg, bg
}

func lerpHex(a, b string, t float64) string {
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	ar, ag, ab := hexToRGB(a)
	br, bg, bb := hexToRGB(b)
	mix := func(x, y uint8) uint8 {
		return uint8(float64(x)*(1-t) + float64(y)*t + 0.5)
	}
	return fmt.Sprintf("#%02x%02x%02x", mix(ar, br), mix(ag, bg), mix(ab, bb))
}

// Dissolve wipes cells away on flip order of a seeded noise field — a
// cell turns to a space on the base background once the sweep passes it.
// t ∈ [0,1] sweeps 0..1; 0 = fully intact, 1 = fully dissolved.
func Dissolve(lines []Line, t float64) []Line {
	if t <= 0 {
		return lines
	}
	if t >= 1 {
		return BlackFrame(lines)
	}
	cfg := baseHexFromLines(lines)
	noise := cachedPerlin(0xD155, 911)
	A := parseFrame(lines)
	out := make([]Line, 0, len(A))
	for y, row := range A {
		var sb strings.Builder
		curFG, curBG := "", ""
		for x, c := range row {
			n := noise.At(float64(x)*0.16+3.1, float64(y)*0.16+7.7)*0.5 + 0.5
			// the wipe front: cells with n < t*1.35 dissolve
			if float64(t)*1.35 > n {
				glyph := " "
				emitBlank(&sb, &curFG, &curBG, cfg, glyph)
				continue
			}
			emitSGRStrings(&sb, &curFG, &curBG, c.fg, c.bg)
			sb.WriteString(c.glyph)
		}
		sb.WriteString("\x1b[0m")
		out = append(out, sb.String())
	}
	return out
}

// baseHexFromLines returns the dominant background hex of the frame
// (default: the mtm default). Scene frames are opaque, so any interior
// cell's bg works; ties break by hex value so the result is deterministic
// across runs (map iteration order is randomized).
func baseHexFromLines(lines []Line) string {
	counts := map[string]int{}
	for _, line := range lines {
		for _, mm := range sgrBg.FindAllStringSubmatch(line, -1) {
			counts["#"+mm[1]]++
		}
	}
	best := "#14282d"
	bestN := 0
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if counts[k] > bestN {
			best, bestN = k, counts[k]
		}
	}
	return best
}

var sgrBg = regexp.MustCompile(`48;2;([0-9]+;[0-9]+;[0-9]+)`)

// emitBlank writes a base-colored space, reasserting SGR only on changes.
func emitBlank(sb *strings.Builder, curFG, curBG *string, baseHex string, glyph string) {
	bg := baseHex
	if bg == "" {
		bg = "#14282d"
	}
	emitSGRStrings(sb, curFG, curBG, "", bg)
	sb.WriteString(glyph)
}

// emitSGRStrings writes fg/bg SGR only when they changed.
func emitSGRStrings(sb *strings.Builder, curFG, curBG *string, fg, bg string) {
	if fg != *curFG {
		if fg == "" {
			sb.WriteString("\x1b[39m")
		} else {
			sb.WriteString(fmt.Sprintf("\x1b[38;2;%sm", rgbTriplet(hexToRGB(fg))))
		}
		*curFG = fg
	}
	if bg != *curBG {
		if bg == "" {
			sb.WriteString("\x1b[49m")
		} else {
			sb.WriteString(fmt.Sprintf("\x1b[48;2;%sm", rgbTriplet(hexToRGB(bg))))
		}
		*curBG = bg
	}
}

// BlackFrame renders an all-base frame with the same cell grid as lines.
func BlackFrame(lines []Line) []Line {
	base := baseHexFromLines(lines)
	var sb strings.Builder
	s := fmt.Sprintf("\x1b[48;2;%sm", rgbTriplet(hexToRGB(base)))
	out := make([]Line, 0, len(lines))
	for _, l := range lines {
		sb.Reset()
		sb.WriteString(s)
		n := len([]rune(stripEscapes(l)))
		sb.WriteString(strings.Repeat(" ", n))
		sb.WriteString("\x1b[0m")
		out = append(out, sb.String())
	}
	return out
}

// stripEscapes removes ANSI sequences (for width counting).
func stripEscapes(s string) string {
	var sb strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\x1b' {
			for j := i; j < len(s); j++ {
				if s[j] >= 'a' && s[j] <= 'z' || s[j] >= 'A' && s[j] <= 'Z' {
					i = j
					break
				}
			}
			continue
		}
		sb.WriteByte(s[i])
	}
	return sb.String()
}

// Crossfade blends from→to by eased t ∈ [0,1]. Endpoints return the exact
// source lines.
func Crossfade(from, to []Line, t float64) []Line {
	if t <= 0 {
		return from
	}
	if t >= 1 {
		return to
	}
	e := EaseInOutCubic(t)
	A := parseFrame(from)
	B := parseFrame(to)
	rows := len(A)
	if len(B) < rows {
		rows = len(B)
	}
	out := make([]Line, 0, rows)
	for y := 0; y < rows; y++ {
		ra := A[y]
		rb := B[y]
		w := len(ra)
		if len(rb) < w {
			w = len(rb)
		}
		var sb strings.Builder
		curFG, curBG := "", ""
		emitSGR := func(fg, bg string) {
			if fg != curFG {
				if fg == "" {
					sb.WriteString("\x1b[39m")
				} else {
					sb.WriteString(fmt.Sprintf("\x1b[38;2;%sm", rgbTriplet(hexToRGB(fg))))
				}
				curFG = fg
			}
			if bg != curBG {
				if bg == "" {
					sb.WriteString("\x1b[49m")
				} else {
					sb.WriteString(fmt.Sprintf("\x1b[48;2;%sm", rgbTriplet(hexToRGB(bg))))
				}
				curBG = bg
			}
		}
		for x := 0; x < w; x++ {
			fg := lerpHex(ra[x].fg, rb[x].fg, e)
			bg := lerpHex(ra[x].bg, rb[x].bg, e)
			emitSGR(fg, bg)
			if e < 0.5 {
				sb.WriteString(ra[x].glyph)
			} else {
				sb.WriteString(rb[x].glyph)
			}
		}
		sb.WriteString("\x1b[0m")
		out = append(out, sb.String())
	}
	return out
}
