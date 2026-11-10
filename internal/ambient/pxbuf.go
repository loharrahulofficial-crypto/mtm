package ambient

import (
	"math"
)

// pxbuf.go — the supersampled float pixel buffer every scene renders into.
//
// The grid is the sub-cell raster of the current mode (halfblock: w x h*2,
// braille: w*2 x h*4, ascii: w x h), rasterized at 2x per axis. Edges are
// coverage-based (signed-distance falloff), so shapes anti-alias and
// sub-cell motion is smooth. Colors are premultiplied float RGBA. The base
// background is painted opaque before the scene runs — there are no
// unpainted cells.
type blendOp int

const (
	opNormal blendOp = iota
	opAdd            // additive glow (light adds to light)
)

type fpix struct{ r, g, b, a float32 }

func (p fpix) scale(s float32) fpix { return fpix{p.r * s, p.g * s, p.b * s, p.a * s} }
func (p fpix) add(q fpix) fpix      { return fpix{p.r + q.r, p.g + q.g, p.b + q.b, p.a + q.a} }

func minFloat32(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func maxFloat32(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

type fbuf struct {
	sw, sh int
	pw, ph int
	px     []fpix
}

const fSS = 2 // supersampling factor per axis

func newFBuffer(sw, sh int) *fbuf {
	if sw < 1 {
		sw = 1
	}
	if sh < 1 {
		sh = 1
	}
	pw, ph := sw*fSS, sh*fSS
	return &fbuf{sw: sw, sh: sh, pw: pw, ph: ph, px: make([]fpix, pw*ph)}
}

// rgbF resolves a palette index to float channels (0..1).
func (m *paletteModel) rgbF(i int) (float32, float32, float32) {
	c := m.table[i]
	return float32(c.r) / 255, float32(c.g) / 255, float32(c.b) / 255
}

// baseFill paints the whole raster with the base color, opaque.
func (b *fbuf) baseFill(r, g, bl float32) {
	for i := range b.px {
		b.px[i] = fpix{r, g, bl, 1}
	}
}

// blendPx composites a premultiplied source pixel at (x, y).
func (b *fbuf) blendPx(x, y int, src fpix, o blendOp) {
	if x < 0 || y < 0 || x >= b.pw || y >= b.ph {
		return
	}
	i := y*b.pw + x
	dst := b.px[i]
	if o == opAdd {
		// additive: dst + src*src.a (saturates at 1; HDR is pointless
		// when the output is 8-bit)
		add := src.scale(src.a)
		dst = dst.add(add)
		dst.r = minFloat32(maxFloat32(dst.r, 0), 1)
		dst.g = minFloat32(maxFloat32(dst.g, 0), 1)
		dst.b = minFloat32(maxFloat32(dst.b, 0), 1)
		if dst.a > 1 {
			dst.a = 1
		}
		b.px[i] = dst
		return
	}
	// normal blending on premultiplied pixels
	inv := 1 - src.a
	out := fpix{
		r: src.r*src.a + dst.r*inv,
		g: src.g*src.a + dst.g*inv,
		b: src.b*src.a + dst.b*inv,
		a: src.a + dst.a*inv,
	}
	b.px[i] = out
}

// fillRectF fills a rect in sub-cell coordinates with soft edges.
func (b *fbuf) fillRectF(x0, y0, x1, y1 float64, r, g, bl float32, alpha float64, o blendOp) {
	fx0, fy0 := x0*fSS, y0*fSS
	fx1, fy1 := x1*fSS, y1*fSS
	feather := float64(fSS) * 0.75
	px0 := int(math.Floor(fx0 - feather))
	px1 := int(math.Ceil(fx1 + feather))
	py0 := int(math.Floor(fy0 - feather))
	py1 := int(math.Ceil(fy1 + feather))
	for py := py0; py < py1; py++ {
		for px := px0; px < px1; px++ {
			cx, cy := float64(px)+0.5, float64(py)+0.5
			dx := math.Max(math.Max(fx0-cx, cx-fx1), 0)
			dy := math.Max(math.Max(fy0-cy, cy-fy1), 0)
			d := math.Hypot(dx, dy)
			cov := clampf((feather-d)/feather, 0, 1)
			if cov <= 0 {
				continue
			}
			b.blendPx(px, py, fpix{r, g, bl, float32(alpha * cov)}, o)
		}
	}
}

// fillRect paints a cell-aligned rect (used by glow/mark blocks).
func (b *fbuf) fillRect(x0, y0, x1, y1 int, r, g, bl float32, alpha float64, o blendOp) {
	b.fillRectF(float64(x0), float64(y0), float64(x1), float64(y1), r, g, bl, alpha, o)
}

// fillCircleF fills a circle in sub-cell coordinates with soft edges.
func (b *fbuf) fillCircleF(cx, cy, radius float64, r, g, bl float32, alpha float64, o blendOp) {
	px0 := int(math.Floor((cx - radius - 1) * fSS))
	px1 := int(math.Ceil((cx + radius + 1) * fSS))
	py0 := int(math.Floor((cy - radius - 1) * fSS))
	py1 := int(math.Ceil((cy + radius + 1) * fSS))
	fcx, fcy := cx*fSS, cy*fSS
	fr := radius * fSS
	feather := float64(fSS) * 0.7
	for py := py0; py < py1; py++ {
		for px := px0; px < px1; px++ {
			d := math.Hypot(float64(px)+0.5-fcx, float64(py)+0.5-fcy) - fr
			cov := clampf((feather-d)/feather, 0, 1)
			if cov <= 0 {
				continue
			}
			b.blendPx(px, py, fpix{r, g, bl, float32(alpha * cov)}, o)
		}
	}
}

// fillEllipseF fills an axis-aligned ellipse in sub-cell coordinates.
func (b *fbuf) fillEllipseF(cx, cy, rx, ry float64, r, g, bl float32, alpha float64, o blendOp) {
	fxcx, fxcy := cx*fSS, cy*fSS
	frx, fry := math.Max(rx*fSS, 0.5), math.Max(ry*fSS, 0.5)
	px0 := int(math.Floor(fxcx - frx - 2))
	px1 := int(math.Ceil(fxcx + frx + 2))
	py0 := int(math.Floor(fxcy - fry - 2))
	py1 := int(math.Ceil(fxcy + fry + 2))
	feather := float64(fSS) * 0.75
	scale := math.Min(frx, fry)
	for py := py0; py < py1; py++ {
		for px := px0; px < px1; px++ {
			dx := (float64(px) + 0.5 - fxcx) / frx
			dy := (float64(py) + 0.5 - fxcy) / fry
			d := (math.Hypot(dx, dy) - 1) * scale
			cov := clampf((feather-d)/feather, 0, 1)
			if cov <= 0 {
				continue
			}
			b.blendPx(px, py, fpix{r, g, bl, float32(alpha * cov)}, o)
		}
	}
}

// strokeLineF strokes a line with the given width (sub-cell units).
func (b *fbuf) strokeLineF(x0, y0, x1, y1, width float64, r, g, bl float32, alpha float64, o blendOp) {
	fx0, fy0, fx1, fy1 := x0*fSS, y0*fSS, x1*fSS, y1*fSS
	fw := width * fSS
	minX := int(math.Floor(math.Min(fx0, fx1) - fw - 1))
	maxX := int(math.Ceil(math.Max(fx0, fx1) + fw + 1))
	minY := int(math.Floor(math.Min(fy0, fy1) - fw - 1))
	maxY := int(math.Ceil(math.Max(fy0, fy1) + fw + 1))
	feather := float64(fSS) * 0.75
	dx, dy := fx1-fx0, fy1-fy0
	len2 := dx*dx + dy*dy
	if len2 == 0 {
		b.fillCircleF(x0, y0, width/2, r, g, bl, alpha, o)
		return
	}
	for py := minY; py < maxY; py++ {
		for px := minX; px < maxX; px++ {
			t := clampf(((float64(px)+0.5-fx0)*dx+(float64(py)+0.5-fy0)*dy)/len2, 0, 1)
			d := math.Hypot(float64(px)+0.5-(fx0+t*dx), float64(py)+0.5-(fy0+t*dy)) - fw/2
			cov := clampf((feather-d)/feather, 0, 1)
			if cov <= 0 {
				continue
			}
			b.blendPx(px, py, fpix{r, g, bl, float32(alpha * cov)}, o)
		}
	}
}

// boxBlur softens the raster in place (separable passes). Radius is in
// pixels (0 = no-op). Deterministic.
func (b *fbuf) boxBlur(radius int) {
	if radius <= 0 {
		return
	}
	dst := make([]fpix, len(b.px))
	// horizontal pass
	for y := 0; y < b.ph; y++ {
		for x := 0; x < b.pw; x++ {
			var acc fpix
			n := 0
			for k := -radius; k <= radius; k++ {
				xx := x + k
				if xx < 0 || xx >= b.pw {
					continue
				}
				acc = acc.add(b.px[y*b.pw+xx])
				n++
			}
			dst[y*b.pw+x] = acc.scale(1 / float32(n))
		}
	}
	// vertical pass
	for y := 0; y < b.ph; y++ {
		for x := 0; x < b.pw; x++ {
			var acc fpix
			n := 0
			for k := -radius; k <= radius; k++ {
				yy := y + k
				if yy < 0 || yy >= b.ph {
					continue
				}
				acc = acc.add(dst[yy*b.pw+x])
				n++
			}
			b.px[y*b.pw+x] = acc.scale(1 / float32(n))
		}
	}
}

// downsample box-averages each supersampled block into one subcell pixel.
func (b *fbuf) downsample() []fpix {
	out := make([]fpix, b.sw*b.sh)
	for sy := 0; sy < b.sh; sy++ {
		for sx := 0; sx < b.sw; sx++ {
			var acc fpix
			for dy := 0; dy < fSS; dy++ {
				for dx := 0; dx < fSS; dx++ {
					acc = acc.add(b.px[(sy*fSS+dy)*b.pw+(sx*fSS+dx)])
				}
			}
			out[sy*b.sw+sx] = acc.scale(1 / float32(fSS*fSS))
		}
	}
	return out
}
