package ambient

import "unicode/utf8"

// quadrant.go — the quadrant render path: 2×2 subcells per terminal cell
// (square pixels, 4× the subcell count of halfblock at the same grid),
// encoded with the 16 Unicode quadrant/half-block glyphs.
//
// Same discipline as emitHalfblock: no fmt.Sprintf in the hot loop,
// SGR change detection per line, byte-int writes for numbers, borrowed
// buffers released by the caller, preallocated line buffer.
//
// Per cell the four subcells are split by luminance at the midpoint of
// their min/max (0.5 when all four are equal). The higher side becomes
// fg, the lower side bg, and the glyph selects the fg-group dots
// (TL=1, TR=2, BL=4, BR=8). With one side empty the non-empty side's
// mean becomes fg and the glyph is '█'. Subcells at or below the ink
// threshold are unpainted base — they land in the lower group; correct
// by construction.
var quadrantGlyphs = [16]rune{
	' ', '▘', '▝', '▀',
	'▖', '▌', '▞', '▛',
	'▗', '▚', '▐', '▜',
	'▄', '▙', '▟', '█',
}

// emitQuadrant compiles a 2w×2h downsampled raster (plus ink coverage)
// into exactly h lines of w cells. Cells whose four subcells all sit at
// or below the ink threshold are skipped (' '), like ascii/braille skip
// unpainted cells; every other case emits a quadrant glyph.
func emitQuadrant(px []fpix, ink []float32, w, h int, baseHex string, o emitOpts) []Line {
	baseC, ok := parseHex(baseHex)
	if !ok {
		baseC, _ = parseHex("#14282d")
	}
	br, bgc, bb := baseC.r, baseC.g, baseC.b
	baseF := fpix{float32(br) / 255, float32(bgc) / 255, float32(bb) / 255, 1}
	const step = float32(1.0 / 255.0)
	nearBase := func(c fpix) bool {
		if c.a == 0 {
			return true // never painted — treat as base, like halfblock's nearBase cell
		}
		return abs32(c.r-baseF.r) <= step && abs32(c.g-baseF.g) <= step && abs32(c.b-baseF.b) <= step
	}
	baseKey := uint32(br)<<16 | uint32(bgc)<<8 | uint32(bb)
	stride := w * 2
	out := make([]Line, 0, h)
	lineBuf := make([]byte, 0, w*18)
	for ty := 0; ty < h; ty++ {
		lineBuf = lineBuf[:0]
		curFG, curBG := uint32(0), uint32(0)
		haveFG, haveBG := false, false
		row0 := (ty * 2) * stride
		row1 := row0 + stride
		for tx := 0; tx < w; tx++ {
			i0 := row0 + tx*2
			i1 := row1 + tx*2
			// Ink-free cells still carry colors (gradient fills mark no
			// ink) — only a fully base-colored cell is skipped, like
			// halfblock's nearBase shortcut.
			ids := [4]int{i0, i0 + 1, i1, i1 + 1}
			near := 0
			for _, j := range ids {
				if ink[j] <= 0.22 && nearBase(px[j]) {
					near++
				}
			}
			if near == 4 {
				if !haveFG || curFG != baseKey {
					if o.limited {
						bi := nearestXterm256Dither(br, bgc, bb, tx, ty*2, false)
						lineBuf = appendXtermSGR(lineBuf, true, uint8(bi))
					} else {
						lineBuf = appendSGR(lineBuf, true, br, bgc, bb)
					}
					curFG, haveFG = baseKey, true
				}
				if !haveBG || curBG != baseKey {
					if o.limited {
						bi := nearestXterm256Dither(br, bgc, bb, tx, ty*2+1, false)
						lineBuf = appendXtermSGR(lineBuf, false, uint8(bi))
					} else {
						lineBuf = appendSGR(lineBuf, false, br, bgc, bb)
					}
					curBG, haveBG = baseKey, true
				}
				lineBuf = append(lineBuf, ' ')
				continue
			}
			cs := [4]fpix{px[i0], px[i0+1], px[i1], px[i1+1]}
			ls := [4]float64{inkLum(cs[0]), inkLum(cs[1]), inkLum(cs[2]), inkLum(cs[3])}
			minL, maxL := ls[0], ls[0]
			for i := 1; i < 4; i++ {
				if ls[i] < minL {
					minL = ls[i]
				}
				if ls[i] > maxL {
					maxL = ls[i]
				}
			}
			split := 0.5
			if maxL != minL {
				split = (minL + maxL) * 0.5
			}
			var fgSum, bgSum [3]float64
			fgBits, fgN, bgN := 0, 0, 0
			for i := 0; i < 4; i++ {
				c := cs[i]
				if ls[i] > split {
					fgSum[0] += float64(c.r)
					fgSum[1] += float64(c.g)
					fgSum[2] += float64(c.b)
					fgBits |= 1 << uint(i)
					fgN++
				} else {
					bgSum[0] += float64(c.r)
					bgSum[1] += float64(c.g)
					bgSum[2] += float64(c.b)
					bgN++
				}
			}
			var glyph rune
			var fgR, fgG, fgB, bgR, bgG, bgB uint8
			var fgKey, bgKey uint32
			if fgN == 0 || bgN == 0 {
				// One side is empty — paint the whole cell with the
				// non-empty side's mean; bg is irrelevant under '█'.
				sum, n := bgSum, bgN
				if fgN > 0 {
					sum, n = fgSum, fgN
				}
				fn := float64(n)
				fgR, fgG, fgB = quantize(float32(sum[0]/fn)), quantize(float32(sum[1]/fn)), quantize(float32(sum[2]/fn))
				fgKey = uint32(fgR)<<16 | uint32(fgG)<<8 | uint32(fgB)
				bgKey, bgR, bgG, bgB = baseKey, br, bgc, bb
				glyph = '█'
			} else {
				fgN64, bgN64 := float64(fgN), float64(bgN)
				fgR, fgG, fgB = quantize(float32(fgSum[0]/fgN64)), quantize(float32(fgSum[1]/fgN64)), quantize(float32(fgSum[2]/fgN64))
				bgR, bgG, bgB = quantize(float32(bgSum[0]/bgN64)), quantize(float32(bgSum[1]/bgN64)), quantize(float32(bgSum[2]/bgN64))
				fgKey = uint32(fgR)<<16 | uint32(fgG)<<8 | uint32(fgB)
				bgKey = uint32(bgR)<<16 | uint32(bgG)<<8 | uint32(bgB)
				glyph = quadrantGlyphs[fgBits]
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
