package ambient

import (
	"fmt"
	"os"
	"strings"
)

// ppm.go — writes rendered frames as binary PPM (P6) images for review:
// each halfblock pixel becomes a rectangle. Output is a byte matrix, not
// an image library — zero dependencies, trivially convertible
// (pnm2png, ImageMagick, ...).

// RGBPix is one 8-bit RGB pixel (DecodeFrame output; row-major, w×h).
type RGBPix struct{ R, G, B uint8 }

// DecodeFrame converts rendered lines into a pixel grid for PPM output.
// Grid size: width x 2*height cells (vertical doubling, halfblock
// geometry). ' ' cells inherit the current SGR color (like a terminal);
// braille/ascii cells approximate the glyph with a density blend.
func DecodeFrame(lines []Line) (int, int, []RGBPix) {
	h := len(lines)
	if h == 0 {
		return 0, 0, nil
	}
	type cell struct {
		fg, bg RGBPix
		ch     rune
	}
	rows := make([][]cell, 0, h)
	for _, line := range lines {
		row := []cell{}
		// default terminal colors: white on near-black
		fg, bg := RGBPix{255, 255, 255}, RGBPix{12, 25, 30}
		rest := line
		for len(rest) > 0 {
			if strings.HasPrefix(rest, "\x1b[") {
				end := strings.Index(rest, "m")
				if end < 0 {
					break
				}
				params := strings.TrimPrefix(rest[:end+1], "\x1b[")
				params = strings.TrimSuffix(params, "m")
				fg, bg = applySGR(fg, bg, params)
				rest = rest[end+1:]
				continue
			}
			r, size := decodeRune(rest)
			if r == 0 {
				break
			}
			row = append(row, cell{fg, bg, r})
			rest = rest[size:]
		}
		rows = append(rows, row)
	}
	w := 0
	for _, r := range rows {
		if len(r) > w {
			w = len(r)
		}
	}
	pw, ph := w, h*2
	px := make([]RGBPix, pw*ph)
	for y, row := range rows {
		for x, c := range row {
			switch c.ch {
			case '▀':
				px[(2*y)*pw+x] = c.fg
				px[(2*y+1)*pw+x] = c.bg
			case '▄':
				px[(2*y)*pw+x] = c.bg
				px[(2*y+1)*pw+x] = c.fg
			case '█':
				px[(2*y)*pw+x] = c.fg
				px[(2*y+1)*pw+x] = c.fg
			case ' ':
				px[(2*y)*pw+x] = c.bg
				px[(2*y+1)*pw+x] = c.bg
			default:
				top, bot := inkCell(c.ch, c.fg, c.bg)
				px[(2*y)*pw+x] = top
				px[(2*y+1)*pw+x] = bot
			}
		}
	}
	return pw, ph, px
}

// inkCell approximates a braille/ascii glyph with a density blend between
// background and ink color.
func inkCell(ch rune, fg, bg RGBPix) (RGBPix, RGBPix) {
	density := 1.0
	switch {
	case ch >= 0x2800 && ch <= 0x28FF:
		bits := 0
		for _, r := range [8]int{0x01, 0x02, 0x04, 0x40, 0x08, 0x10, 0x20, 0x80} {
			if int(ch)&r != 0 {
				bits++
			}
		}
		density = 0.25 + 0.75*float64(bits)/8
	case ch == ' ' || ch == 0:
		return bg, bg
	case ch == '·' || ch == ':':
		density = 0.35
	case ch == '.':
		density = 0.25
	case ch == '-' || ch == '=' || ch == '~':
		density = 0.55
	case ch == '+' || ch == '*' || ch == '×':
		density = 0.7
	case ch == '▁' || ch == '▂':
		density = 0.2
	case ch == '▃' || ch == '▄':
		density = 0.5
	case ch == '▅' || ch == '▆':
		density = 0.75
	case ch == '▇' || ch == '█' || ch == '◆' || ch == '●' || ch == '✦':
		density = 1.0
	}
	mix := func(a, b RGBPix, t float64) RGBPix {
		return RGBPix{
			uint8(float64(a.R)*(1-t) + float64(b.R)*t),
			uint8(float64(a.G)*(1-t) + float64(b.G)*t),
			uint8(float64(a.B)*(1-t) + float64(b.B)*t),
		}
	}
	return mix(bg, fg, density), mix(bg, fg, density)
}

// applySGR parses one SGR parameter list and updates the current
// foreground/background. 38/48;2, 38/48;5 (and resets) are tracked.
func applySGR(fg, bg RGBPix, params string) (RGBPix, RGBPix) {
	parts := strings.Split(strings.TrimSuffix(params, "m"), ";")
	i := 0
	for i < len(parts) {
		p := strings.TrimSpace(parts[i])
		switch p {
		case "", "0", "00", "000":
			return RGBPix{255, 255, 255}, RGBPix{12, 25, 30}
		case "39":
			fg = RGBPix{255, 255, 255}
			i++
			continue
		case "49":
			bg = RGBPix{12, 25, 30}
			i++
			continue
		case "38", "48":
			target := &fg
			if p == "48" {
				target = &bg
			}
			if i+2 < len(parts) && parts[i+1] == "5" {
				if idx, ok := atoiSmall(parts[i+2]); ok && idx >= 0 && idx <= 255 {
					r, g, b := xterm256RGB(uint8(idx))
					*target = RGBPix{r, g, b}
				}
				i += 3
				continue
			}
			if i+4 < len(parts) && parts[i+1] == "2" {
				r, ok1 := atoiSmall(parts[i+2])
				g, ok2 := atoiSmall(parts[i+3])
				b, ok3 := atoiSmall(parts[i+4])
				if ok1 && ok2 && ok3 {
					*target = RGBPix{uint8(r), uint8(g), uint8(b)}
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

func atoiSmall(s string) (int, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	v := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, false
		}
		v = v*10 + int(r-'0')
		if v > 255 {
			return 0, false
		}
	}
	return v, true
}

// decodeRune returns the rune and its byte length at the start of s.
func decodeRune(s string) (rune, int) {
	for i, r := range s {
		return r, i + len(string(r))
	}
	return 0, 0
}

// WritePPM renders the frame to a binary PPM (P6). Each grid pixel
// becomes scale x scale pixels.
func WritePPM(path string, lines []Line, scale int) error {
	if scale < 1 {
		scale = 4
	}
	w, h, px := DecodeFrame(lines)
	if w == 0 || h == 0 {
		return fmt.Errorf("empty frame")
	}
	pw, ph := w*scale, h*scale
	header := fmt.Sprintf("P6\n%d %d\n255\n", pw, ph)
	pixCount := pw * ph * 3
	buf := make([]byte, 0, len(header)+pixCount)
	buf = append(buf, []byte(header)...)
	for y := 0; y < ph; y++ {
		sy := y / scale
		for x := 0; x < pw; x++ {
			sx := x / scale
			c := px[sy*w+sx]
			buf = append(buf, c.R, c.G, c.B)
		}
	}
	return os.WriteFile(path, buf, 0o644)
}
