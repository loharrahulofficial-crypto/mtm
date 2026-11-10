package ambient

import "math"

// gradient.go — linear gradients across the pixel raster. Scenes paint
// smooth sky/water washes with these; the halfblock output encodes the
// gradient as two vertical subcells per cell, so it never bands.

// fillGradientF fills the rect with a vertical interpolation between the
// two palette colors. Edges are feathered like fillRectF.
func (b *fbuf) fillGradientF(x0, y0, x1, y1 float64, r0, g0, b0, r1, g1, b1 float32, alpha float64, o blendOp) {
	fx0, fy0 := x0*float64(fSS), y0*float64(fSS)
	fx1, fy1 := x1*float64(fSS), y1*float64(fSS)
	span := fy1 - fy0
	if span <= 0 {
		span = 1
	}
	feather := float64(fSS) * 0.75
	px0, px1 := int(math.Floor(fx0-feather)), int(math.Ceil(fx1+feather))
	py0, py1 := int(math.Floor(fy0-feather)), int(math.Ceil(fy1+feather))
	for py := py0; py < py1; py++ {
		fy := float64(py) + 0.5 - fy0
		t := clampf(fy/span, 0, 1)
		t32 := float32(t)
		r := r0 + (r1-r0)*t32
		g := g0 + (g1-g0)*t32
		bl := b0 + (b1-b0)*t32
		for px := px0; px < px1; px++ {
			cx, cy := float64(px)+0.5, float64(py)+0.5
			ddx := math.Max(math.Max(fx0-cx, cx-fx1), 0)
			ddy := math.Max(math.Max(fy0-cy, cy-fy1), 0)
			d2 := ddx*ddx + ddy*ddy
			if d2 >= feather*feather {
				continue
			}
			cov := clampf((feather-math.Sqrt(d2))/feather, 0, 1)
			b.blendPx(px, py, fpix{r, g, bl, float32(alpha * cov)}, o)
		}
	}
}

// fillGradient fills a cell-aligned vertical gradient of two palette colors.
func (b *buffer) fillGradient(x0, y0, x1, y1 int, topCI, botCI int, alpha float64, o blendOp) {
	r0, g0, bl0 := b.palette.rgbF(topCI)
	r1, g1, bl1 := b.palette.rgbF(botCI)
	b.fb.fillGradientF(float64(x0), float64(y0), float64(x1), float64(y1), r0, g0, bl0, r1, g1, bl1, alpha, o)
}

// dither.go — optional ordered dithering for limited-palette terminals
// (xterm-256). Applied only when the render state asks for it
// (ui.ambient_dither: on, or an auto-detected limited palette).

var bayer4 = [16]float64{
	0, 8, 2, 10,
	12, 4, 14, 6,
	3, 11, 1, 9,
	15, 7, 13, 5,
}

// xtermCube holds the 6 color-cube levels in [0,1].
var xtermCube = [6]float64{0, 95.0 / 255.0, 135.0 / 255.0, 175.0 / 255.0, 215.0 / 255.0, 1}

// bayerThreshold returns the ordered-dither threshold in [0,1) for (x, y).
func bayerThreshold(x, y int) float64 {
	return (bayer4[(y%4)*4+(x%4)] + 0.5) / 16
}

// ditherChannel rounds v (0..1) to the nearest xterm-256 cube level,
// choosing between the two adjacent levels by the Bayer threshold of (x, y).
func ditherChannel(v float64, x, y int) float64 {
	if v <= 0 {
		return 0
	}
	if v >= 1 {
		return 1
	}
	i := 1
	for i < len(xtermCube) && xtermCube[i] < v {
		i++
	}
	lo, hi := xtermCube[i-1], xtermCube[i]
	th := bayerThreshold(x, y)
	f := (v - lo) / (hi - lo)
	if f > th {
		return hi
	}
	return lo
}

// ditherQuantize snaps a pixel to the xterm-256 cube with ordered dithering.
func ditherQuantize(p fpix, x, y int) fpix {
	return fpix{
		float32(ditherChannel(float64(p.r), x, y)),
		float32(ditherChannel(float64(p.g), x+1, y+1)),
		float32(ditherChannel(float64(p.b), x+2, y+2)),
		p.a,
	}
}

// xtermValid256 reports whether v lies exactly on a cube level.
func xtermValid256(v float64) bool {
	for _, L := range xtermCube {
		if math.Abs(v-L) < 1e-6 {
			return true
		}
	}
	return false
}
