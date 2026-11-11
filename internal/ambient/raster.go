package ambient

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// raster.go — the downsampled pixel raster of a frame and the
// raster-space transitions built on it.
//
// Frames render into a supersampled float raster which is box-downsampled
// to one pixel per subcell before emission. A Raster carries exactly that
// downsampled data (w × 2h pixels for halfblock) so transitions blend
// pixels and quantize once, instead of parsing and re-emitting ANSI bytes.
//
// Rasters returned by FramePixels own their storage. Rasters handed to
// emitHalfblock are borrowed for the call only.
type Raster struct {
	W, H int
	Pix  []fpix
}

// Pixel returns the quantized 8-bit RGB of pixel (x, y).
func (r Raster) Pixel(x, y int) (uint8, uint8, uint8) {
	p := r.Pix[y*r.W+x]
	return quantize(p.r), quantize(p.g), quantize(p.b)
}

// BlendRasters blends two same-sized rasters pixel by pixel (t ∈ [0,1]).
// Mismatched sizes are resampled nearest-neighbour. Returns a new raster.
func BlendRasters(a, b Raster, t float64) Raster {
	if a.W != b.W || a.H != b.H {
		b = fitRaster(b, a.W, a.H)
	}
	out := Raster{W: a.W, H: a.H, Pix: make([]fpix, len(a.Pix))}
	f1 := float32(1 - t)
	f2 := float32(t)
	for i := range a.Pix {
		out.Pix[i] = fpix{
			r: a.Pix[i].r*f1 + b.Pix[i].r*f2,
			g: a.Pix[i].g*f1 + b.Pix[i].g*f2,
			b: a.Pix[i].b*f1 + b.Pix[i].b*f2,
			a: 1,
		}
	}
	return out
}

// fitRaster nearest-neighbour resamples src into a w×h raster.
func fitRaster(src Raster, w, h int) Raster {
	out := Raster{W: w, H: h, Pix: make([]fpix, w*h)}
	for y := 0; y < h; y++ {
		sy := y * src.H / h
		if sy >= src.H {
			sy = src.H - 1
		}
		for x := 0; x < w; x++ {
			sx := x * src.W / w
			if sx >= src.W {
				sx = src.W - 1
			}
			out.Pix[y*w+x] = src.Pix[sy*src.W+sx]
		}
	}
	return out
}

// BaseRaster returns a raster filled with the base color.
func BaseRaster(baseHex string, w, h int) Raster {
	baseF := mustBaseF(baseHex)
	out := Raster{W: w, H: h, Pix: make([]fpix, w*h)}
	for i := range out.Pix {
		out.Pix[i] = baseF
	}
	return out
}

func mustBaseF(baseHex string) fpix {
	c, ok := parseHex(baseHex)
	if !ok {
		c, _ = parseHex("#14282d")
	}
	return fpix{float32(c.r) / 255, float32(c.g) / 255, float32(c.b) / 255, 1}
}

// DissolveRaster wipes pixels to the base color on a seeded noise sweep
// (t ∈ [0,1]; pixels whose noise falls below t*1.35 dissolve). Returns a
// new raster.
func DissolveRaster(r Raster, t float64, baseHex string) Raster {
	base := mustBaseF(baseHex)
	noise := cachedPerlin(0xD155, 911)
	out := Raster{W: r.W, H: r.H, Pix: make([]fpix, len(r.Pix))}
	for y := 0; y < r.H; y++ {
		cellRow := y / 2 // two pixel rows per terminal cell
		for x := 0; x < r.W; x++ {
			n := noise.At(float64(x)*0.16+3.1, float64(cellRow)*0.16+7.7)*0.5 + 0.5
			if float64(t)*1.35 > n {
				out.Pix[y*r.W+x] = base
				continue
			}
			out.Pix[y*r.W+x] = r.Pix[y*r.W+x]
		}
	}
	return out
}

// rasterFromLines normalizes already-rendered lines into a raster:
// '▀' is (fg, bg), '▄' is (bg, fg), '█' is (fg, fg), ' ' is (bg, bg);
// other glyphs become a density blend (ink over background) in both
// halves. Unset colors default to the frame's base. Grid is w × 2h.
func rasterFromLines(lines []Line, baseHex string) Raster {
	baseF := mustBaseF(baseHex)
	defHex := func(v string) string {
		if v == "" {
			return baseHex
		}
		return v
	}
	hexF1 := func(s string) (fpix, bool) {
		c, ok := parseHex(s)
		if !ok {
			return fpix{}, false
		}
		return fpix{float32(c.r) / 255, float32(c.g) / 255, float32(c.b) / 255, 1}, true
	}
	type cell struct{ fg, bg, glyph string }
	rows := make([][]cell, 0, len(lines))
	w := 0
	for _, line := range lines {
		row := []cell{}
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
			row = append(row, cell{defHex(fg), defHex(bg), string(r)})
			rest = rest[size:]
		}
		if len(row) > w {
			w = len(row)
		}
		rows = append(rows, row)
	}
	r := Raster{W: w, H: len(rows) * 2, Pix: make([]fpix, w*len(rows)*2)}
	for y, row := range rows {
		for x := 0; x < w; x++ {
			var c cell
			if x < len(row) {
				c = row[x]
			} else {
				c = cell{baseHex, baseHex, " "}
			}
			topF, ok := hexF1(c.fg)
			if !ok {
				topF = baseF
			}
			botF, ok := hexF1(c.bg)
			if !ok {
				botF = baseF
			}
			var top, bot fpix
			switch c.glyph {
			case "▀":
				top, bot = topF, botF
			case "▄":
				top, bot = botF, topF
			case "█":
				top, bot = topF, topF
			case " ":
				top, bot = botF, botF
			default:
				d := inkDensity([]rune(c.glyph)[0])
				top = fpix{
					r: botF.r*(1-float32(d)) + topF.r*float32(d),
					g: botF.g*(1-float32(d)) + topF.g*float32(d),
					b: botF.b*(1-float32(d)) + topF.b*float32(d),
					a: 1,
				}
				bot = top
			}
			r.Pix[(2*y)*w+x] = top
			r.Pix[(2*y+1)*w+x] = bot
		}
	}
	return r
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

// inkDensity maps a glyph to its ink coverage (mirrors inkCell in ppm.go).
func inkDensity(ch rune) float64 {
	switch {
	case ch >= 0x2800 && ch <= 0x28FF:
		bits := 0
		for _, r := range [8]int{0x01, 0x02, 0x04, 0x40, 0x08, 0x10, 0x20, 0x80} {
			if int(ch)&r != 0 {
				bits++
			}
		}
		return 0.25 + 0.75*float64(bits)/8
	case ch == ' ' || ch == 0:
		return 0
	case ch == '·' || ch == ':':
		return 0.35
	case ch == '.':
		return 0.25
	case ch == '-' || ch == '=' || ch == '~':
		return 0.55
	case ch == '+' || ch == '*' || ch == '×':
		return 0.7
	case ch == '▁' || ch == '▂':
		return 0.2
	case ch == '▃' || ch == '▄':
		return 0.5
	case ch == '▅' || ch == '▆':
		return 0.75
	case ch == '▇' || ch == '█' || ch == '◆' || ch == '●' || ch == '✦':
		return 1.0
	}
	return 1.0
}

// emitOpts controls how a raster becomes halfblock lines.
type emitOpts struct {
	pairOnly bool // transitions: always '▀' unless both halves sit on base (' ')
	limited  bool // emit 38;5;N via nearest xterm-256 (optionally Bayer-dithered)
	dither   bool // ordered dither between the two nearest xterm-256 entries
}

// emitHalfblock compiles a raster into r.H/2 lines of r.W cells.
func (r Raster) emitHalfblock(baseHex string, o emitOpts) []Line {
	baseC, ok := parseHex(baseHex)
	if !ok {
		baseC, _ = parseHex("#14282d")
	}
	br, bgc, bb := baseC.r, baseC.g, baseC.b
	baseKey := uint32(br)<<16 | uint32(bgc)<<8 | uint32(bb)
	baseF := fpix{float32(br) / 255, float32(bgc) / 255, float32(bb) / 255, 1}
	const step = float32(1.0 / 255.0)
	nearBase := func(c fpix) bool {
		return abs32(c.r-baseF.r) <= step && abs32(c.g-baseF.g) <= step && abs32(c.b-baseF.b) <= step
	}
	w, h := r.W, r.H/2
	out := make([]Line, 0, h)
	lineBuf := make([]byte, 0, w*18)
	for ty := 0; ty < h; ty++ {
		lineBuf = lineBuf[:0]
		curFG, curBG := uint32(0), uint32(0)
		haveFG, haveBG := false, false
		for tx := 0; tx < w; tx++ {
			top := r.Pix[(ty*2)*w+tx]
			bot := r.Pix[(ty*2+1)*w+tx]
			topKey, tr, tg, tb := tripack(top)
			botKey, dr, dg, db := tripack(bot)
			var fgKey, bgKey uint32
			var fgR, fgG, fgB, bgR, bgG, bgB uint8
			var glyph rune
			topNear, botNear := nearBase(top), nearBase(bot)
			switch {
			case topNear && botNear:
				fgKey, fgR, fgG, fgB = baseKey, br, bgc, bb
				bgKey, bgR, bgG, bgB = baseKey, br, bgc, bb
				glyph = ' '
			case topKey == botKey && !o.pairOnly:
				fgKey, fgR, fgG, fgB = topKey, tr, tg, tb
				bgKey, bgR, bgG, bgB = baseKey, br, bgc, bb
				glyph = '█'
			default:
				fgKey, fgR, fgG, fgB = topKey, tr, tg, tb
				bgKey, bgR, bgG, bgB = botKey, dr, dg, db
				glyph = '▀'
			}
			if o.limited {
				fi := nearestXterm256Dither(fgR, fgG, fgB, tx, ty*2, o.dither)
				fgKey = uint32(fi)
				fgR, fgG, fgB = xterm256RGB(fi)
				bi := nearestXterm256Dither(bgR, bgG, bgB, tx, ty*2+1, o.dither)
				bgKey = uint32(bi)
				bgR, bgG, bgB = xterm256RGB(bi)
			}
			if !haveFG || curFG != fgKey {
				if o.limited {
					lineBuf = appendXtermSGR(lineBuf, true, uint8(fgKey))
				} else {
					lineBuf = appendSGR(lineBuf, true, fgR, fgG, fgB)
				}
				curFG, haveFG = fgKey, true
			}
			if !haveBG || curBG != bgKey {
				if o.limited {
					lineBuf = appendXtermSGR(lineBuf, false, uint8(bgKey))
				} else {
					lineBuf = appendSGR(lineBuf, false, bgR, bgG, bgB)
				}
				curBG, haveBG = bgKey, true
			}
			lineBuf = utf8.AppendRune(lineBuf, glyph)
		}
		lineBuf = append(lineBuf, "\x1b[0m"...)
		out = append(out, string(lineBuf))
	}
	return out
}

// EmitHalfblock exposes the halfblock emitter for already-blended rasters.
func (r Raster) EmitHalfblock(baseHex string, pairOnly, limited, dither bool) []Line {
	return r.emitHalfblock(baseHex, emitOpts{pairOnly: pairOnly, limited: limited, dither: dither})
}

// EmitTheme renders the raster with the scene-state color policy:
// truecolor by default, xterm-256 when PaletteLimited (optionally with
// ordered dither). Used by transitions, which always emit '▀'.
func (r Raster) EmitTheme(st SceneState, pairOnly bool) []Line {
	return r.emitHalfblock(st.Palette.Base, emitOpts{pairOnly: pairOnly, limited: st.PaletteLimited, dither: st.Dither})
}
