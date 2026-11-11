package ambient

import "unicode/utf8"

// sextant.go — the sextant render path: 2×3 subcells per terminal cell
// (2× the subcell count of quadrant at the same grid), encoded with the
// 60 Unicode 15 "Symbols for Legacy Computing" sextant glyphs
// (U+1FB00..U+1FB3B) plus '▌'/'▐' (left/right columns, not encoded as
// sextants by Unicode), '█' and ' '.
//
// The Unicode block does NOT enumerate the 2×3 patterns in numeric
// order, and skips four mosaics entirely: empty (' '), the left column
// (mask 21, '▌'), the right column (mask 42, '▐') and full ('█'). The
// table below is the exact permutation verified against the Unicode 15
// code charts (SEXTANT-1..SEXTANT-23456 in order), not an arithmetic
// formula. Cell bit layout: TL=1, TR=2, ML=4, MR=8, BL=16, BR=32.
// lumEps is the luminance spread treated as uniform. 1e-5 is ~1/40th
// of one opaque 8-bit channel step, far below what a terminal can show.
const lumEps = 1e-5

var sextantGlyphs = [64]rune{
	' ', 0x1FB00, 0x1FB01, 0x1FB02, 0x1FB03, 0x1FB04, 0x1FB05, 0x1FB06,
	0x1FB07, 0x1FB08, 0x1FB09, 0x1FB0A, 0x1FB0B, 0x1FB0C, 0x1FB0D, 0x1FB0E,
	0x1FB0F, 0x1FB10, 0x1FB11, 0x1FB12, 0x1FB13, '▌', 0x1FB14, 0x1FB15,
	0x1FB16, 0x1FB17, 0x1FB18, 0x1FB19, 0x1FB1A, 0x1FB1B, 0x1FB1C, 0x1FB1D,
	0x1FB1E, 0x1FB1F, 0x1FB20, 0x1FB21, 0x1FB22, 0x1FB23, 0x1FB24, 0x1FB25,
	0x1FB26, 0x1FB27, '▐', 0x1FB28, 0x1FB29, 0x1FB2A, 0x1FB2B, 0x1FB2C,
	0x1FB2D, 0x1FB2E, 0x1FB2F, 0x1FB30, 0x1FB31, 0x1FB32, 0x1FB33, 0x1FB34,
	0x1FB35, 0x1FB36, 0x1FB37, 0x1FB38, 0x1FB39, 0x1FB3A, 0x1FB3B, '█',
}

// Same discipline as emitHalfblock: no fmt.Sprintf in the hot loop,
// SGR change detection per line, byte-int writes for numbers, borrowed
// buffers released by the caller, preallocated line buffer.
//
// Per cell the six subcells are split by luminance at the midpoint of
// their min/max (0.5 when all six are equal). The higher side becomes
// fg, the lower side bg, and the glyph selects the fg-group dots. With
// one side empty the non-empty side's mean becomes fg and the glyph is
// '█'. Subcells at or below the ink threshold are unpainted base — they
// land in the lower group; correct by construction.

// sextantGlyphMask maps every non-space sextant glyph (including '▌'
// mask 21 / '▐' mask 42 and '█' mask 63) to its 2×3 fill bits. The
// ppm/raster decoders use it to reconstruct the exact 2×3 structure.
var sextantGlyphMask = map[rune]uint8{}

// sextantDistinctive are the glyphs ONLY the sextant renderer emits:
// U+1FB00..U+1FB3B. ('▌'/'▐'/'█' also exist in halfblock/quadrant output.)
// Their presence triples DecodeFrame's height and doubles its width so
// the 2×3 structure survives.
var sextantDistinctive = map[rune]bool{}

func init() {
	for i := 1; i < 64; i++ {
		sextantGlyphMask[sextantGlyphs[i]] = uint8(i)
		if g := sextantGlyphs[i]; g >= 0x1FB00 && g <= 0x1FB3B {
			sextantDistinctive[g] = true
		}
	}
}

// emitSextant compiles a 2w×3h downsampled raster (plus ink coverage)
// into exactly h lines of w cells. Cells whose six subcells all sit at
// or below the ink threshold are skipped (' '), like ascii/braille skip
// unpainted cells; every other case emits a sextant glyph.
func emitSextant(px []fpix, ink []float32, w, h int, baseHex string, o emitOpts) []Line {
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
		row0 := (ty * 3) * stride
		row1 := row0 + stride
		row2 := row1 + stride
		for tx := 0; tx < w; tx++ {
			i0 := row0 + tx*2
			i1 := row1 + tx*2
			i2 := row2 + tx*2
			// Ink-free cells still carry colors (gradient fills mark no
			// ink) — only a fully base-colored cell is skipped, like
			// halfblock's nearBase shortcut.
			ids := [6]int{i0, i0 + 1, i1, i1 + 1, i2, i2 + 1}
			near := 0
			for _, j := range ids {
				if ink[j] <= 0.22 && nearBase(px[j]) {
					near++
				}
			}
			if near == 6 {
				if !haveFG || curFG != baseKey {
					if o.limited {
						bi := nearestXterm256Dither(br, bgc, bb, tx, ty*3, false)
						lineBuf = appendXtermSGR(lineBuf, true, uint8(bi))
					} else {
						lineBuf = appendSGR(lineBuf, true, br, bgc, bb)
					}
					curFG, haveFG = baseKey, true
				}
				if !haveBG || curBG != baseKey {
					if o.limited {
						bi := nearestXterm256Dither(br, bgc, bb, tx, ty*3+2, false)
						lineBuf = appendXtermSGR(lineBuf, false, uint8(bi))
					} else {
						lineBuf = appendSGR(lineBuf, false, br, bgc, bb)
					}
					curBG, haveBG = baseKey, true
				}
				lineBuf = append(lineBuf, ' ')
				continue
			}
			cs := [6]fpix{px[i0], px[i0+1], px[i1], px[i1+1], px[i2], px[i2+1]}
			ls := [6]float64{inkLum(cs[0]), inkLum(cs[1]), inkLum(cs[2]), inkLum(cs[3]), inkLum(cs[4]), inkLum(cs[5])}
			minL, maxL := ls[0], ls[0]
			for i := 1; i < 6; i++ {
				if ls[i] < minL {
					minL = ls[i]
				}
				if ls[i] > maxL {
					maxL = ls[i]
				}
			}
			// Sub-8-bit luminance spreads (fraction-painter float noise)
			// must read as uniform — otherwise a 1e-9 gap flips whole columns
			// into the fg group on solid fills (barcode artifact).
			split := 0.5
			if maxL-minL > lumEps {
				split = (minL + maxL) * 0.5
			}
			var fgSum, bgSum [3]float64
			fgBits, fgN, bgN := 0, 0, 0
			for i := 0; i < 6; i++ {
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
				glyph = sextantGlyphs[fgBits]
			}
			if o.limited {
				// Dither phase locks to the cell's first/last subrow so
				// the Bayer matrix stays put across the 3 subrows.
				fi := nearestXterm256Dither(fgR, fgG, fgB, tx, ty*3, o.dither)
				fgKey = uint32(fi)
				fgR, fgG, fgB = xterm256RGB(fi)
				bi := nearestXterm256Dither(bgR, bgG, bgB, tx, ty*3+2, o.dither)
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
