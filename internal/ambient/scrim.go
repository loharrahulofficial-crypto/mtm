package ambient

import "math"

// scrim.go — soft radial scrims painted into the scene after the paint
// pass (before downsample/emission). The splash sets them behind the
// wordmark, tagline and hint so the overlaid text stays legible with no
// hard backing plate: the scrim is a darkening toward the theme Base
// with a smooth quartic falloff to exactly zero at the rim, so cell
// backgrounds flow seamlessly into the surrounding scene.

// Scrim is one soft radial darkening region. Coordinates and radii are
// in terminal CELLS; the engine scales them per render mode (halfblock
// 1x2, quadrant 2x2, sextant 2x3), so an elliptical scrim always covers
// the same cells regardless of resolution.
type Scrim struct {
	CX, CY           float64 // center, cell coordinates
	RadiusX, RadiusY float64 // cell radii where the scrim reaches zero
	Strength         float64 // max darkening at the center (0..1)
}

// alpha computes the darkening at normalized squared distance q ∈ [0,1).
// Smooth at both ends: full dark at the center, exactly zero at the rim,
// monotonically decreasing in between.
func (sc Scrim) alpha(q float64) float64 {
	if q >= 1 || sc.Strength <= 0 {
		return 0
	}
	t := 1 - q
	return sc.Strength * t * t
}

// applyScrims paints the scrims into the supersampled pixel raster. Cell
// coordinates are converted with sxr/syr (subcells per cell).
func applyScrims(b *buffer, scrims []Scrim, sxr, syr float64) {
	ss := float64(fSSV())
	baseR, baseG, baseB := b.palette.rgbF(b.palette.base)
	for _, sc := range scrims {
		cx := sc.CX * sxr * ss
		cy := sc.CY * syr * ss
		rx := math.Max(sc.RadiusX*sxr*ss, 1)
		ry := math.Max(sc.RadiusY*syr*ss, 1)
		x0 := int(math.Floor(cx - rx))
		x1 := int(math.Ceil(cx + rx))
		y0 := int(math.Floor(cy - ry))
		y1 := int(math.Ceil(cy + ry))
		if x1 > b.fb.pw {
			x1 = b.fb.pw
		}
		if y1 > b.fb.ph {
			y1 = b.fb.ph
		}
		var src fpix
		src.r, src.g, src.b, src.a = baseR, baseG, baseB, 0
		for py := y0; py < y1; py++ {
			if py < 0 {
				continue
			}
			dy := (float64(py) + 0.5 - cy) / ry
			q2 := dy * dy
			for px := x0; px < x1; px++ {
				if px < 0 {
					continue
				}
				dx := (float64(px) + 0.5 - cx) / rx
				q := q2 + dx*dx
				a := sc.alpha(q)
				if a <= 0.003 {
					continue
				}
				src.a = float32(a)
				b.fb.blendPx(px, py, src, opNormal)
			}
		}
	}
}
