package ambient

import (
	"fmt"
	"strings"
)

// buffer is the sub-cell raster every scene paints into. Resolutions:
//
//	halfblock: w   × h*2   (two vertical subcells per terminal cell)
//	braille:   w*2 × h*4   (2×4 dots per terminal cell)
//	ascii:     w   × h
//
// Scenes set cells with a glyph and a palette index; the mode decides how
// subcells become terminal glyphs.
type buffer struct {
	mode    string
	sw      int
	sh      int
	cells   []subcell
	palette *paletteModel
}

type subcell struct {
	ch rune // 0 = unmarked
	c  int  // palette index of the ink color
}

func newBuffer(mode string, w, h int) *buffer {
	sw, sh := w, h
	switch mode {
	case ModeHalfblock:
		sh = h * 2
	case ModeBraille:
		sw, sh = w*2, h*4
	}
	if sw < 1 {
		sw = 1
	}
	if sh < 1 {
		sh = 1
	}
	return &buffer{mode: mode, sw: sw, sh: sh, cells: make([]subcell, sw*sh)}
}

// mark writes a glyph cell (ascii mode) or a charged dot/block (others).
func (b *buffer) mark(x, y int, ch rune, ci int) {
	if x < 0 || y < 0 || x >= b.sw || y >= b.sh {
		return
	}
	c := &b.cells[y*b.sw+x]
	c.ch = ch
	c.c = ci
}

// glow charges a cell without a glyph (used for light/color washes).
func (b *buffer) glow(x, y int, ci int) {
	b.mark(x, y, 0, ci)
}

func (b *buffer) glowBlock(x0, y0, x1, y1 int, ci int) {
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			b.glow(x, y, ci)
		}
	}
}

// paintChar writes a block of glyph cells.
func (b *buffer) paintChar(x0, y0, x1, y1 int, ch rune, ci int) {
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			b.mark(x, y, ch, ci)
		}
	}
}

// render compiles the sub-raster into exactly h lines of w cells.
func (b *buffer) render(p *paletteModel, w, h int) []Line {
	b.palette = p
	out := make([]Line, 0, h)
	switch b.mode {
	case ModeHalfblock:
		for ty := 0; ty < h; ty++ {
			var sb strings.Builder
			base := p.hexAt(p.base)
			run := func(ch rune, fg, bg string) string {
				return fmt.Sprintf("\x1b[38;2;%sm\x1b[48;2;%sm%c", rgbTriplet(hexToRGB(fg)), rgbTriplet(hexToRGB(bg)), ch)
			}
			for tx := 0; tx < w; tx++ {
				top := b.sub(tx, ty*2)
				bot := b.sub(tx, ty*2+1)
				// Palette index 0 is the base color; a cell is marked
				// only when it carries a glyph or a non-base ink.
				topSet := top.ch != 0 || top.c != 0
				botSet := bot.ch != 0 || bot.c != 0
				fgTop := p.hexAt(top.c)
				fgBot := p.hexAt(bot.c)
				switch {
				case !topSet && !botSet:
					sb.WriteString(" ")
				case !botSet:
					sb.WriteString(run('▀', fgTop, base))
				case !topSet:
					sb.WriteString(run('▄', fgBot, base))
				case top.c == bot.c:
					sb.WriteString(run('█', fgTop, base))
				default:
					sb.WriteString(run('▀', fgTop, fgBot))
				}
			}
			sb.WriteString("\x1b[0m")
			out = append(out, sb.String())
		}
	case ModeBraille:
		for ty := 0; ty < h; ty++ {
			var sb strings.Builder
			for tx := 0; tx < w; tx++ {
				bits := 0
				best := -1 // brightest marked ink in the cell
				for dy := 0; dy < 4; dy++ {
					for dx := 0; dx < 2; dx++ {
						c := b.sub(tx*2+dx, ty*4+dy)
						if c.ch == 0 && c.c == 0 {
							continue
						}
						bits |= brailleDot(dx, dy)
						if best < 0 || p.lum[c.c] > p.lum[best] {
							best = c.c
						}
					}
				}
				if bits == 0 {
					sb.WriteString(" ")
					continue
				}
				sb.WriteString(fmt.Sprintf("\x1b[38;2;%sm%c", rgbTriplet(hexToRGB(p.hexAt(best))), rune(0x2800+bits)))
			}
			sb.WriteString("\x1b[0m")
			out = append(out, sb.String())
		}
	default:
		// ascii draws the raster 1:1.
		for ty := 0; ty < h; ty++ {
			var sb strings.Builder
			for tx := 0; tx < w; tx++ {
				c := b.sub(tx, ty)
				if c.ch == 0 && c.c == 0 {
					sb.WriteString(" ")
					continue
				}
				ch := c.ch
				if ch == 0 {
					ch = '·'
				}
				sb.WriteString(fmt.Sprintf("\x1b[38;2;%sm%c", rgbTriplet(hexToRGB(p.hexAt(c.c))), ch))
			}
			sb.WriteString("\x1b[0m")
			out = append(out, sb.String())
		}
	}
	return out
}

func (b *buffer) sub(x, y int) subcell {
	if x < 0 || y < 0 || x >= b.sw || y >= b.sh {
		return subcell{}
	}
	return b.cells[y*b.sw+x]
}

func hexToRGB(s string) (uint8, uint8, uint8) {
	c, ok := parseHex(s)
	if !ok {
		return 255, 0, 255
	}
	return c.r, c.g, c.b
}

func rgbTriplet(r, g, b uint8) string { return fmt.Sprintf("%d;%d;%d", r, g, b) }

// brailleDot maps a (col, row) in a 2×4 dots cell to the braille bit.
// col 0..1, row 0..3; rows 0..2 use bits 0x01..0x40, row 3 uses 0x80.
func brailleDot(dx, dy int) int {
	bits := []int{1, 8}
	if dy < 3 {
		return bits[dx] << uint(dy)
	}
	if dx == 0 {
		return 0x40
	}
	return 0x80
}
