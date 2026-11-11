package ambient

import (
	"fmt"
	"unicode/utf8"
)

// buffer is the sub-cell raster every scene paints into. Resolutions:
//
//	halfblock: w   × h*2   (two vertical subcells per terminal cell)
//	braille:   w*2 × h*4   (2×4 dots per terminal cell)
//	ascii:     w   × h
//
// Internally every frame renders into a supersampled float pixel raster
// (fbuf, 2x per axis) which is box-downsampled before output; each output
// cell then encodes two vertical subcells (halfblock), 2×4 dots (braille)
// or a luminance glyph (ascii). Colors are theme roles or blends; scenes
// never invent colors.
//
// Layers: scenes wrap bounded elements in beginLayer/endLayer with a tag.
// Every painted subcell records the tag of the layer that painted it last.
// The region union per tag is checked by tests (no layer paints outside
// its intended region).
type buffer struct {
	mode     string
	sw       int
	sh       int
	fb       *fbuf
	cells    []subcell
	layer    []uint8
	regions  []layerRegion
	curLayer uint8
	palette  *paletteModel
	bytes    int
	dither   bool
}

// layerRegion records the intended bounding rect of one painted layer.
type layerRegion struct {
	tag            uint8
	x0, y0, x1, y1 int // half-open, sub-cell coords
}

const (
	layerSky uint8 = 1 + iota
	layerCelestial
	layerStars
	layerSea
	layerLand
	layerCabin
	layerSmoke
	layerFoliage
	layerFire
	layerFog
)

func newBuffer(mode string, w, h int, p *paletteModel) *buffer {
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
	fb := newFBuffer(sw, sh)
	if p != nil {
		r, g, b := p.rgbF(p.base)
		fb.baseFill(r, g, b)
	}
	return &buffer{
		mode: mode, sw: sw, sh: sh, fb: fb,
		cells:   make([]subcell, sw*sh),
		layer:   make([]uint8, sw*sh),
		palette: p,
	}
}

// beginLayer starts a bounded layer: every painted subcell is tagged until
// endLayer is called, and (x0, y0, x1, y1) — half-open, sub-cell coords —
// records the layer's intended region for the region invariant test.
func (b *buffer) beginLayer(tag uint8, x0, y0, x1, y1 int) {
	b.curLayer = tag
	if x1 < x0 {
		x1 = x0
	}
	if y1 < y0 {
		y1 = y0
	}
	b.regions = append(b.regions, layerRegion{tag: tag, x0: x0, y0: y0, x1: x1, y1: y1})
}

// endLayer stops tagging cells (regions stay recorded).
func (b *buffer) endLayer() { b.curLayer = 0 }

// layerOf reports the tag of the layer that painted the cell (0 = none).
func (b *buffer) layerOf(i int) uint8 { return b.layer[i] }

// bytes reports the SGR byte count of the most recent render (§2.5).
func (b *buffer) bytesOut() int { return b.bytes }

// BytesPerFrame exposes the byte count of the last full frame render.
func (b *buffer) BytesPerFrame() int { return b.bytes }

func (b *buffer) rgbF(ci int) (float32, float32, float32) {
	return b.palette.rgbF(ci)
}

// mark writes a glyph cell (ascii mode) or a soft-edged ink cell.
func (b *buffer) mark(x, y int, ch rune, ci int) {
	if x < 0 || y < 0 || x >= b.sw || y >= b.sh {
		return
	}
	i := y*b.sw + x
	b.cells[i] = subcell{ch: ch, c: ci}
	r, g, bl := b.rgbF(ci)
	b.fb.fillRect(x, y, x+1, y+1, r, g, bl, 1, opNormal)
	if b.curLayer != 0 {
		b.layer[i] = b.curLayer
	}
}

// glow paints an additive, soft blob centered in its cell (light washes).
func (b *buffer) glow(x, y int, ci int) {
	if x < 0 || y < 0 || x >= b.sw || y >= b.sh {
		return
	}
	i := y*b.sw + x
	c := &b.cells[i]
	if c.ch == 0 && c.c == 0 {
		c.c = ci // first writer wins for the glyph grid (ascii/braille)
	}
	r, g, bl := b.rgbF(ci)
	b.fb.fillEllipseF(float64(x)+0.5, float64(y)+0.5, 0.72, 0.72, r, g, bl, 0.85, opAdd)
	if b.curLayer != 0 {
		b.layer[i] = b.curLayer
	}
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

// fillRectF / fillRectHexF: float-plane primitives; hex variants parse on
// the spot (ramps produce hex strings).
func (b *buffer) fillRectF(x0, y0, x1, y1 float64, ci int, alpha float64, o blendOp) {
	r, g, bl := b.rgbF(ci)
	b.fb.fillRectF(x0, y0, x1, y1, r, g, bl, alpha, o)
}

func (b *buffer) fillRectHexF(x0, y0, x1, y1 float64, hex string, alpha float64, o blendOp) {
	r, g, bl, ok := hexF(hex)
	if !ok {
		return
	}
	b.fb.fillRectF(x0, y0, x1, y1, r, g, bl, alpha, o)
}

// fillCircleF / fillCircleHexF: float-coordinate primitives.
func (b *buffer) fillCircleF(cx, cy, radius float64, ci int, alpha float64, o blendOp) {
	r, g, bl := b.rgbF(ci)
	b.fb.fillCircleF(cx, cy, radius, r, g, bl, alpha, o)
}

func (b *buffer) fillCircleHexF(cx, cy, radius float64, hex string, alpha float64, o blendOp) {
	r, g, bl, ok := hexF(hex)
	if !ok {
		return
	}
	b.fb.fillCircleF(cx, cy, radius, r, g, bl, alpha, o)
}

// fillEllipseF / fillEllipseHexF: ellipse fill.
func (b *buffer) fillEllipseF(cx, cy, rx, ry float64, ci int, alpha float64, o blendOp) {
	r, g, bl := b.rgbF(ci)
	b.fb.fillEllipseF(cx, cy, rx, ry, r, g, bl, alpha, o)
}

func (b *buffer) fillEllipseHexF(cx, cy, rx, ry float64, hex string, alpha float64, o blendOp) {
	r, g, bl, ok := hexF(hex)
	if !ok {
		return
	}
	b.fb.fillEllipseF(cx, cy, rx, ry, r, g, bl, alpha, o)
}

// strokeLineF / strokeHexF: arbitrary-slope strokes.
func (b *buffer) strokeLineF(x0, y0, x1, y1, width float64, ci int, alpha float64, o blendOp) {
	r, g, bl := b.rgbF(ci)
	b.fb.strokeLineF(x0, y0, x1, y1, width, r, g, bl, alpha, o)
}

func (b *buffer) strokeHexF(x0, y0, x1, y1, width float64, hex string, alpha float64, o blendOp) {
	r, g, bl, ok := hexF(hex)
	if !ok {
		return
	}
	b.fb.strokeLineF(x0, y0, x1, y1, width, r, g, bl, alpha, o)
}

// strokeCircleF / strokeCircleHexF: ring stroke (ripples, halos).
func (b *buffer) strokeCircleF(cx, cy, radius, width float64, ci int, alpha float64, o blendOp) {
	r, g, bl := b.rgbF(ci)
	b.fb.strokeCircleF(cx, cy, radius, width, r, g, bl, alpha, o)
}

func (b *buffer) strokeCircleHexF(cx, cy, radius, width float64, hex string, alpha float64, o blendOp) {
	r, g, bl, ok := hexF(hex)
	if !ok {
		return
	}
	b.fb.strokeCircleF(cx, cy, radius, width, r, g, bl, alpha, o)
}

// fillGradientHex paints a vertical hex-to-hex gradient.
func (b *buffer) fillGradientHex(x0, y0, x1, y1 float64, topHex, botHex string, alpha float64, o blendOp) {
	tr, tg, tb, ok1 := hexF(topHex)
	br, bg, bb, ok2 := hexF(botHex)
	if !ok1 || !ok2 {
		return
	}
	b.fb.fillGradientF(x0, y0, x1, y1, tr, tg, tb, br, bg, bb, alpha, o)
}

// hexF parses #rrggbb into float channels.
func hexF(hex string) (float32, float32, float32, bool) {
	c, ok := parseHex(hex)
	if !ok {
		return 0, 0, 0, false
	}
	return float32(c.r) / 255, float32(c.g) / 255, float32(c.b) / 255, true
}

// sub exposes one subcell of the glyph grid.
func (b *buffer) sub(x, y int) subcell {
	if x < 0 || y < 0 || x >= b.sw || y >= b.sh {
		return subcell{}
	}
	return b.cells[y*b.sw+x]
}

type subcell struct {
	ch rune // 0 = unmarked
	c  int  // palette index of the ink color
}

// hexToRGB parses #rrggbb / #rgb into channel bytes.
func hexToRGB(s string) (uint8, uint8, uint8) {
	c, ok := parseHex(s)
	if !ok {
		return 255, 0, 255
	}
	return c.r, c.g, c.b
}

// quantize converts a float channel to a byte, rounding half away from zero.
func quantize(v float32) uint8 {
	if v <= 0 {
		return 0
	}
	if v >= 1 {
		return 255
	}
	return uint8(v*255 + 0.5)
}

// baseNear reports whether p matches the theme base within one 8-bit step.
func baseNear(p *paletteModel, c fpix) bool {
	br, bg, bb := p.rgbF(p.base)
	const step = float32(1.0 / 255.0)
	return abs32(c.r-br) <= step && abs32(c.g-bg) <= step && abs32(c.b-bb) <= step
}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

// tripack quantizes p into a 24-bit color key and its channel bytes —
// the whole halfblock render compares and emits with integers, never
// hex strings.
func tripack(p fpix) (key uint32, r, g, b uint8) {
	r, g, b = quantize(p.r), quantize(p.g), quantize(p.b)
	return uint32(r)<<16 | uint32(g)<<8 | uint32(b), r, g, b
}

// appendSGR appends "\x1b[38;2;r;g;bm" (fg=true) or "\x1b[48;2;…" —
// byte-identical to fmt.Sprintf("\x1b[3%d;2;%d;%d;%dm", …) but with no
// format parsing.
func appendSGR(dst []byte, fg bool, r, g, b uint8) []byte {
	dst = append(dst, 0x1b, '[')
	if fg {
		dst = append(dst, '3', '8', ';', '2', ';')
	} else {
		dst = append(dst, '4', '8', ';', '2', ';')
	}
	dst = appendByte(dst, r)
	dst = append(dst, ';')
	dst = appendByte(dst, g)
	dst = append(dst, ';')
	dst = appendByte(dst, b)
	return append(dst, 'm')
}

func appendByte(dst []byte, v uint8) []byte {
	switch {
	case v >= 100:
		return append(dst, '0'+v/100, '0'+(v/10)%10, '0'+v%10)
	case v >= 10:
		return append(dst, '0'+v/10, '0'+v%10)
	default:
		return append(dst, '0'+v)
	}
}

// render compiles the raster into exactly h lines of w cells.
func (b *buffer) render(p *paletteModel, w, h int) []Line {
	b.palette = p
	px := b.fb.downsample()
	ink := b.fb.downsampleInk()
	for i := range px {
		px[i] = p.clampHull(px[i])
	}
	if b.dither {
		for y := 0; y < b.sh; y++ {
			for x := 0; x < b.sw; x++ {
				px[y*b.sw+x] = ditherQuantize(px[y*b.sw+x], x, y)
			}
		}
	}
	out := make([]Line, 0, h)
	switch b.mode {
	case ModeHalfblock:
		br, bgc, bb := hexToRGB(p.hexAt(p.base))
		baseKey := uint32(br)<<16 | uint32(bgc)<<8 | uint32(bb)
		lineBuf := make([]byte, 0, w*18)
		for ty := 0; ty < h; ty++ {
			lineBuf = lineBuf[:0]
			curFG, curBG := uint32(0), uint32(0)
			haveFG, haveBG := false, false
			for tx := 0; tx < w; tx++ {
				top := px[(ty*2)*b.sw+tx]
				bot := px[(ty*2+1)*b.sw+tx]
				topKey, tr, tg, tb := tripack(top)
				botKey, dr, dg, db := tripack(bot)
				var fgKey, bgKey uint32
				var fr, fg, fb, br2, bg2, bb2 uint8
				var glyph rune
				switch {
				case baseNear(p, top) && baseNear(p, bot):
					fgKey, fr, fg, fb = baseKey, br, bgc, bb
					bgKey, br2, bg2, bb2 = baseKey, br, bgc, bb
					glyph = ' '
				case topKey == botKey:
					fgKey, fr, fg, fb = topKey, tr, tg, tb
					bgKey, br2, bg2, bb2 = baseKey, br, bgc, bb
					glyph = '█'
				default:
					fgKey, fr, fg, fb = topKey, tr, tg, tb
					bgKey, br2, bg2, bb2 = botKey, dr, dg, db
					glyph = '▀'
				}
				if !haveFG || curFG != fgKey {
					lineBuf = appendSGR(lineBuf, true, fr, fg, fb)
					curFG, haveFG = fgKey, true
				}
				if !haveBG || curBG != bgKey {
					lineBuf = appendSGR(lineBuf, false, br2, bg2, bb2)
					curBG, haveBG = bgKey, true
				}
				lineBuf = utf8.AppendRune(lineBuf, glyph)
			}
			lineBuf = append(lineBuf, "\x1b[0m"...)
			out = append(out, string(lineBuf))
		}
	case ModeBraille:
		bbuf := make([]byte, 0, w*14)
		for ty := 0; ty < h; ty++ {
			bbuf = bbuf[:0]
			for tx := 0; tx < w; tx++ {
				bits := 0
				bestLum := -1.0
				var bestRGB rgbPix
				for dy := 0; dy < 4; dy++ {
					for dx := 0; dx < 2; dx++ {
						i2 := (ty*4+dy)*b.sw + tx*2 + dx
						if ink[i2] <= 0.22 {
							continue
						}
						c := px[i2]
						bits |= brailleDot(dx, dy)
						if l := inkLum(c); l > bestLum {
							bestLum = l
							bestRGB = rgbPix{quantize(c.r), quantize(c.g), quantize(c.b)}
						}
					}
				}
				if bits == 0 {
					bbuf = append(bbuf, ' ')
					continue
				}
				bbuf = appendSGR(bbuf, true, bestRGB.r, bestRGB.g, bestRGB.b)
				bbuf = utf8.AppendRune(bbuf, rune(0x2800+bits))
			}
			bbuf = append(bbuf, "\x1b[0m"...)
			out = append(out, string(bbuf))
		}
	default: // ascii 1:1
		abuf := make([]byte, 0, w*14)
		for ty := 0; ty < h; ty++ {
			abuf = abuf[:0]
			for tx := 0; tx < w; tx++ {
				i2 := ty*b.sw + tx
				c := px[i2]
				if ink[i2] <= 0.07 {
					abuf = append(abuf, ' ')
					continue
				}
				ch := asciiGlyphInk(ink[i2])
				abuf = appendSGR(abuf, true, quantize(c.r), quantize(c.g), quantize(c.b))
				abuf = utf8.AppendRune(abuf, ch)
			}
			abuf = append(abuf, "\x1b[0m"...)
			out = append(out, string(abuf))
		}
	}
	total := 0
	for _, l := range out {
		total += len(l) + 1
	}
	b.bytes = total
	b.fb.release()
	releaseDownsample(px, ink)
	return out
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
