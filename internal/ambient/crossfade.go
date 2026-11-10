package ambient

import (
	"fmt"
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
