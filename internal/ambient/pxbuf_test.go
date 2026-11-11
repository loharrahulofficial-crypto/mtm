package ambient

import (
	"math"
	"testing"
)

// testPalette returns a small palette built from the default theme roles.
func testPalette() *paletteModel { return buildPalette(DefaultPalette()) }

func lumOf(c rgb) int { return int(0.2126*float64(c.r) + 0.7152*float64(c.g) + 0.0722*float64(c.b)) }

// rgbF3 unpacks m.rgbF as an unnamed triple for literals.
func (m *paletteModel) rgbF3(i int) (float32, float32, float32) {
	c := m.table[i]
	return float32(c.r) / 255, float32(c.g) / 255, float32(c.b) / 255
}

func absF32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

func TestAntiAliasEdgeCoverage(t *testing.T) {
	m := testPalette()
	fb := newFBuffer(8, 8)
	r, g, bl := m.rgbF(m.lookup["#d3ae78"]) // accent
	baseR, baseG, baseB := m.rgbF(m.base)
	fb.baseFill(baseR, baseG, baseB)
	// circle centered at (4,4) r=2.5 subcells — edges straddle pixels
	fb.fillCircleF(4, 4, 2.5, r, g, bl, 1, opNormal)
	ds := fb.downsample()
	// find pixels that are neither fully base nor fully accent — AA fringes
	mid := 0
	baseN := 0
	fullN := 0
	af := func(v float32) float64 { return math.Abs(float64(v)) }
	for _, p := range ds {
		ar, ag, ab := p.r, p.g, p.b
		baseD := af(ar-baseR) + af(ag-baseG) + af(ab-baseB)
		fullD := af(ar-r) + af(ag-g) + af(ab-bl)
		switch {
		case baseD < 0.02:
			baseN++
		case fullD < 0.02:
			fullN++
		default:
			mid++
		}
	}
	if baseN == 0 || fullN == 0 {
		t.Fatalf("circle painted nothing (base=%d full=%d)", baseN, fullN)
	}
	if mid == 0 {
		t.Fatal("no anti-aliased fringe pixels — circle edges snap to the pixel grid")
	}
}

func TestDownsampleBoxAverage(t *testing.T) {
	m := testPalette()
	fss := fSSV()
	fb := newFBuffer(2, 2) // (2·fSS)² pixels
	baseR, baseG, baseB := m.rgbF(m.base)
	fb.baseFill(baseR, baseG, baseB)
	// one full fSS² supersampled block of accent at subcell (0,0)
	r, g, bl := m.rgbF(m.lookup["#d3ae78"])
	for y := 0; y < fss; y++ {
		for x := 0; x < fss; x++ {
			fb.px[y*fb.pw+x] = fpix{r, g, bl, 1}
		}
	}
	ds := fb.downsample()
	const eps = 1e-6
	close := func(a, b float32) bool {
		d := a - b
		if d < 0 {
			d = -d
		}
		return d < eps
	}
	same := func(p, q fpix) bool {
		return p.a == q.a && close(p.r, q.r) && close(p.g, q.g) && close(p.b, q.b)
	}
	if ds[0].a != 1 || !close(ds[0].r, r) || !close(ds[0].g, g) || !close(ds[0].b, bl) {
		t.Fatalf("block did not survive downsample: %+v", ds[0])
	}
	for i := 1; i < len(ds); i++ {
		if !same(ds[i], fpix{baseR, baseG, baseB, 1}) {
			t.Fatalf("untouched subcell changed: idx=%d %+v", i, ds[i])
		}
	}
}
