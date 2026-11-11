package ambient

import "math"

// gradient.go — linear gradients across the pixel raster. Scenes paint
// smooth sky/water washes with these; the halfblock output encodes the
// gradient as two vertical subcells per cell, so it never bands.

// fillGradientF fills the rect with a vertical interpolation between the
// two palette colors. Edges are feathered like fillRectF.
func (b *fbuf) fillGradientF(x0, y0, x1, y1 float64, r0, g0, b0, r1, g1, b1 float32, alpha float64, o blendOp) {
	fx0, fy0 := x0*float64(fSSV()), y0*float64(fSSV())
	fx1, fy1 := x1*float64(fSSV()), y1*float64(fSSV())
	span := fy1 - fy0
	if span <= 0 {
		span = 1
	}
	feather := float64(fSSV()) * 0.75
	px0, px1 := int(math.Floor(fx0-feather)), int(math.Ceil(fx1+feather))
	py0, py1 := int(math.Floor(fy0-feather)), int(math.Ceil(fy1+feather))
	for py := py0; py < py1; py++ {
		fy := float64(py) + 0.5 - fy0
		t := clampf(fy/span, 0, 1)
		t32 := float32(t)
		r := r0 + (r1-r0)*t32
		g := g0 + (g1-g0)*t32
		bl := b0 + (b1-b0)*t32
		cy := float64(py) + 0.5
		innerRow := cy >= fy0 && cy <= fy1
		for px := px0; px < px1; px++ {
			cx := float64(px) + 0.5
			// Interior pixels are fully covered: skip the SDF math
			// entirely and write the gradient color through.
			if innerRow && cx >= fx0 && cx <= fx1 && alpha == 1 && o == opNormal {
				i := py*b.pw + px
				if i >= 0 && i < len(b.px) {
					b.px[i] = fpix{r, g, bl, 1}
				}
				continue
			}
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
// (xterm-256). Ordered dithering is done between the two nearest palette
// entries only (see xterm256.go) — never per-channel. Applied when the
// render state asks for it (ui.ambient_dither: on, or auto-detected
// limited palette).

var bayer4 = [16]float64{
	0, 8, 2, 10,
	12, 4, 14, 6,
	3, 11, 1, 9,
	15, 7, 13, 5,
}

// bayerThreshold returns the ordered-dither threshold in [0,1) for (x, y).
func bayerThreshold(x, y int) float64 {
	return (bayer4[(y%4)*4+(x%4)] + 0.5) / 16
}
