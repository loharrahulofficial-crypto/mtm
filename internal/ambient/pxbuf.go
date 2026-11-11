package ambient

import (
	"math"
	"sync"
	"sync/atomic"
)

// pxbuf.go — the supersampled float pixel buffer every scene renders into.
//
// The grid is the sub-cell raster of the current mode (halfblock: w x h*2,
// braille: w*2 x h*4, quadrant: w*2 x h*2, sextant: w*2 x h*3, ascii: w x h),
// rasterized at 2x per axis. Edges are
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
	ink    []float32 // per-pixel ink coverage (non-background paint)
}

// fSSV() is the supersampling factor per axis. Adaptive quality (§4.7) may
// halve it to 1 at runtime; it is a package atomic, not a constant.
var fSS atomic.Int64

func init() { fSS.Store(3) }

// fSSV reads the current supersampling factor.
func fSSV() int { return int(fSS.Load()) }

// fbPool reuses the supersampled raster across frames (the app renders at
// most a handful of fixed sizes; holding them avoids ~1 MB of alloc
// churn per frame).
var fbPool sync.Pool

func newFBuffer(sw, sh int) *fbuf {
	if sw < 1 {
		sw = 1
	}
	if sh < 1 {
		sh = 1
	}
	pw, ph := sw*fSSV(), sh*fSSV()
	need := pw * ph
	if v, ok := fbPool.Get().(*fbuf); ok && cap(v.px) >= need {
		fb := &fbuf{sw: sw, sh: sh, pw: pw, ph: ph, px: v.px[:need]}
		for i := range fb.px {
			fb.px[i] = fpix{}
		}
		if cap(v.ink) >= need {
			fb.ink = v.ink[:need]
			for i := range fb.ink {
				fb.ink[i] = 0
			}
		} else {
			fb.ink = make([]float32, need)
		}
		return fb
	}
	return &fbuf{sw: sw, sh: sh, pw: pw, ph: ph, px: make([]fpix, need), ink: make([]float32, need)}
}

// release returns the raster to the pool.
func (b *fbuf) release() { fbPool.Put(b) }

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

// inkLum returns a 0..1 perceived luminance for a float pixel.
func inkLum(c fpix) float64 {
	return 0.2126*float64(c.r) + 0.7152*float64(c.g) + 0.0722*float64(c.b)
}

// asciiGlyphInk maps ink coverage to a legibility glyph.
func asciiGlyphInk(v float32) rune {
	const ramp = " ·:-=+*#%@"
	t := clampf(float64(v)/0.9, 0, 1)
	return rune(ramp[int(t*float64(len(ramp)-1)+0.5)])
}

// inkAt adds coverage to the ink raster at pixel (x, y).
func (b *fbuf) inkAt(x, y int, cov float32) {
	if x < 0 || y < 0 || x >= b.pw || y >= b.ph {
		return
	}
	i := y*b.pw + x
	v := b.ink[i] + cov
	if v > 1 {
		v = 1
	}
	b.ink[i] = v
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
	fx0, fy0 := x0*float64(fSSV()), y0*float64(fSSV())
	fx1, fy1 := x1*float64(fSSV()), y1*float64(fSSV())
	feather := float64(fSSV()) * 0.75
	px0 := int(math.Floor(fx0 - feather))
	px1 := int(math.Ceil(fx1 + feather))
	py0 := int(math.Floor(fy0 - feather))
	py1 := int(math.Ceil(fy1 + feather))
	for py := py0; py < py1; py++ {
		cy := float64(py) + 0.5
		innerRow := cy >= fy0 && cy <= fy1
		for px := px0; px < px1; px++ {
			cx := float64(px) + 0.5
			// Interior pixels are fully covered: skip the SDF math.
			// px must be inside the raster: py*b.pw + px can be a valid
			// index yet land in the wrong row.
			if innerRow && cx >= fx0 && cx <= fx1 && alpha == 1 && o == opNormal {
				if px >= 0 && px < b.pw && py >= 0 && py < b.ph {
					b.px[py*b.pw+px] = fpix{r, g, bl, 1}
					b.inkAt(px, py, 1)
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
			b.inkAt(px, py, float32(alpha*cov))
		}
	}
}

// fillRect paints a cell-aligned rect (used by glow/mark blocks).
func (b *fbuf) fillRect(x0, y0, x1, y1 int, r, g, bl float32, alpha float64, o blendOp) {
	b.fillRectF(float64(x0), float64(y0), float64(x1), float64(y1), r, g, bl, alpha, o)
}

// fillCircleF fills a circle in sub-cell coordinates with soft edges.
func (b *fbuf) fillCircleF(cx, cy, radius float64, r, g, bl float32, alpha float64, o blendOp) {
	px0 := int(math.Floor((cx - radius - 1) * float64(fSSV())))
	px1 := int(math.Ceil((cx + radius + 1) * float64(fSSV())))
	py0 := int(math.Floor((cy - radius - 1) * float64(fSSV())))
	py1 := int(math.Ceil((cy + radius + 1) * float64(fSSV())))
	fcx, fcy := cx*float64(fSSV()), cy*float64(fSSV())
	fr := radius * float64(fSSV())
	feather := float64(fSSV()) * 0.7
	for py := py0; py < py1; py++ {
		for px := px0; px < px1; px++ {
			dx := float64(px) + 0.5 - fcx
			dy := float64(py) + 0.5 - fcy
			d2 := dx*dx + dy*dy
			cov := clampf((feather-(math.Sqrt(d2)-fr))/feather, 0, 1)
			if cov <= 0 {
				continue
			}
			b.blendPx(px, py, fpix{r, g, bl, float32(alpha * cov)}, o)
			b.inkAt(px, py, float32(alpha*cov))
		}
	}
}

// fillEllipseF fills an axis-aligned ellipse in sub-cell coordinates.
func (b *fbuf) fillEllipseF(cx, cy, rx, ry float64, r, g, bl float32, alpha float64, o blendOp) {
	fxcx, fxcy := cx*float64(fSSV()), cy*float64(fSSV())
	frx, fry := math.Max(rx*float64(fSSV()), 0.5), math.Max(ry*float64(fSSV()), 0.5)
	px0 := int(math.Floor(fxcx - frx - 2))
	px1 := int(math.Ceil(fxcx + frx + 2))
	py0 := int(math.Floor(fxcy - fry - 2))
	py1 := int(math.Ceil(fxcy + fry + 2))
	feather := float64(fSSV()) * 0.75
	scale := math.Min(frx, fry)
	for py := py0; py < py1; py++ {
		for px := px0; px < px1; px++ {
			dx := (float64(px) + 0.5 - fxcx) / frx
			dy := (float64(py) + 0.5 - fxcy) / fry
			d := (math.Sqrt(dx*dx+dy*dy) - 1) * scale
			if d >= feather {
				continue
			}
			cov := clampf((feather-d)/feather, 0, 1)
			b.blendPx(px, py, fpix{r, g, bl, float32(alpha * cov)}, o)
			b.inkAt(px, py, float32(alpha*cov))
		}
	}
}

// strokeCircleF strokes a ring (ripples, halos): an annulus SDF stroke.
func (b *fbuf) strokeCircleF(cx, cy, radius, width float64, r, g, bl float32, alpha float64, o blendOp) {
	fcx, fcy := cx*float64(fSSV()), cy*float64(fSSV())
	fr := radius * float64(fSSV())
	fw := math.Max(width*float64(fSSV()), 0.5)
	feather := float64(fSSV()) * 0.75
	px0 := int(math.Floor(fcx - fr - fw/2 - feather))
	px1 := int(math.Ceil(fcx + fr + fw/2 + feather))
	py0 := int(math.Floor(fcy - fr - fw/2 - feather))
	py1 := int(math.Ceil(fcy + fr + fw/2 + feather))
	for py := py0; py < py1; py++ {
		for px := px0; px < px1; px++ {
			dx, dy := float64(px)+0.5-fcx, float64(py)+0.5-fcy
			dd := math.Abs(math.Sqrt(dx*dx+dy*dy)-fr) - fw/2
			if dd >= feather {
				continue
			}
			cov := clampf((feather-dd)/feather, 0, 1)
			b.blendPx(px, py, fpix{r, g, bl, float32(alpha * cov)}, o)
			b.inkAt(px, py, float32(alpha*cov))
		}
	}
}

// strokeLineF strokes a line with the given width (sub-cell units).
func (b *fbuf) strokeLineF(x0, y0, x1, y1, width float64, r, g, bl float32, alpha float64, o blendOp) {
	fx0, fy0, fx1, fy1 := x0*float64(fSSV()), y0*float64(fSSV()), x1*float64(fSSV()), y1*float64(fSSV())
	fw := width * float64(fSSV())
	minX := int(math.Floor(math.Min(fx0, fx1) - fw - 1))
	maxX := int(math.Ceil(math.Max(fx0, fx1) + fw + 1))
	minY := int(math.Floor(math.Min(fy0, fy1) - fw - 1))
	maxY := int(math.Ceil(math.Max(fy0, fy1) + fw + 1))
	feather := float64(fSSV()) * 0.75
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
			b.inkAt(px, py, float32(alpha*cov))
		}
	}
}

var downPool sync.Pool
var inkPool sync.Pool

// downsample box-averages each supersampled block into one subcell pixel.
// The returned slice is borrowed — callers must return it to the pool.
func (b *fbuf) downsample() []fpix {
	v, ok := downPool.Get().([]fpix)
	need := b.sw * b.sh
	if !ok || cap(v) < need {
		v = make([]fpix, need)
	}
	out := v[:need]
	for sy := 0; sy < b.sh; sy++ {
		for sx := 0; sx < b.sw; sx++ {
			var acc fpix
			for dy := 0; dy < fSSV(); dy++ {
				for dx := 0; dx < fSSV(); dx++ {
					acc = acc.add(b.px[(sy*fSSV()+dy)*b.pw+(sx*fSSV()+dx)])
				}
			}
			out[sy*b.sw+sx] = acc.scale(1 / float32(fSSV()*fSSV()))
		}
	}
	return out
}

// downsampleInk box-averages the ink raster (per subcell).
func (b *fbuf) downsampleInk() []float32 {
	v, ok := inkPool.Get().([]float32)
	need := b.sw * b.sh
	if !ok || cap(v) < need {
		v = make([]float32, need)
	}
	out := v[:need]
	for sy := 0; sy < b.sh; sy++ {
		for sx := 0; sx < b.sw; sx++ {
			var acc float32
			for dy := 0; dy < fSSV(); dy++ {
				for dx := 0; dx < fSSV(); dx++ {
					acc += b.ink[(sy*fSSV()+dy)*b.pw+(sx*fSSV()+dx)]
				}
			}
			out[sy*b.sw+sx] = acc / float32(fSSV()*fSSV())
		}
	}
	return out
}

// releaseDownsample returns the downsample scratch buffers to the pool.
func releaseDownsample(px []fpix, ink []float32) {
	downPool.Put(px)
	inkPool.Put(ink)
}
