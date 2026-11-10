package ambient

import (
	"testing"
)

func TestAdditiveGlowBoosts(t *testing.T) {
	m := testPalette()
	mk := func() *fbuf {
		fb := newFBuffer(2, 2)
		r, g, b := m.rgbF(m.base)
		fb.baseFill(r, g, b)
		return fb
	}
	ar, ag, ab := m.rgbF3(m.lookup["#d3ae78"])
	acent := [3]float32{ar, ag, ab}
	single := mk()
	single.blendPx(1, 1, fpix{acent[0], acent[1], acent[2], 1}, opAdd)
	double := mk()
	double.blendPx(1, 1, fpix{acent[0], acent[1], acent[2], 1}, opAdd)
	double.blendPx(1, 1, fpix{acent[0], acent[1], acent[2], 1}, opAdd)
	a := single.px[1*mk().pw+1]
	b := double.px[1*mk().pw+1]
	if !(b.r > a.r) {
		t.Fatalf("second add did not boost the channel: single=%v double=%v", a, b)
	}
	if b.r > 1.0 {
		t.Fatalf("additive blew past 1.0: %v", b)
	}
}

func TestNormalBlendHalfCoverage(t *testing.T) {
	m := testPalette()
	fb := newFBuffer(1, 1)
	br, bg, bb := m.rgbF(m.base)
	fb.baseFill(br, bg, bb)
	r, g, bl := m.rgbF3(m.lookup["#dfc181"])
	for y := 0; y < fb.ph; y++ {
		for x := 0; x < fb.pw; x++ {
			fb.blendPx(x, y, fpix{r, g, bl, 0.5}, opNormal)
		}
	}
	ds := fb.downsample()
	got := ds[0]
	wantR := br*0.5 + r*0.5
	wantG := bg*0.5 + g*0.5
	wantB := bb*0.5 + bl*0.5
	if absF32(got.r-wantR) > 0.02 || absF32(got.g-wantG) > 0.02 || absF32(got.b-wantB) > 0.02 {
		t.Fatalf("50%% blend = %v, want ≈ (%.3f, %.3f, %.3f)", got, wantR, wantG, wantB)
	}
}

func TestBoxBlurSoftensImpulse(t *testing.T) {
	m := testPalette()
	fb := newFBuffer(7, 7)
	br, bg, bb := m.rgbF(m.base)
	fb.baseFill(br, bg, bb)
	r, g, bl := m.rgbF3(m.lookup["#dfc181"])
	fb.px[3*fb.pw+3] = fpix{r, g, bl, 1}
	before := len(fb.px)
	_ = before
	fb.boxBlur(2)
	center := fb.px[3*fb.pw+3]
	neighbor := fb.px[3*fb.pw+4]
	off := fb.px[3*fb.pw+6]
	if neighbor.r <= br || neighbor.g <= bg || neighbor.b <= bb {
		t.Fatal("blur did not spread the impulse to the neighbor pixel")
	}
	if center.r < neighbor.r {
		t.Fatal("blur amplified the edge over the center")
	}
	if off.r != br || off.g != bg || off.b != bb {
		t.Fatal("blur leaked outside its radius")
	}
}
