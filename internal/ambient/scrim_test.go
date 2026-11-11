package ambient

import (
	"math"
	"testing"
	"time"
)

// scrim_test.go — the radial scrim mechanism: painted after the scene,
// fading smoothly to exactly zero at the rim.

func TestScrimAlphaProfile(t *testing.T) {
	sc := Scrim{CX: 10, CY: 10, RadiusX: 5, RadiusY: 3, Strength: 0.6}
	if got := sc.alpha(0); got != 0.6 {
		t.Errorf("alpha(0) = %v, want %v", got, 0.6)
	}
	prev := math.Inf(1)
	for q := 0.0; q <= 1.0; q += 0.05 {
		a := sc.alpha(q)
		if a > prev {
			t.Errorf("alpha(%.2f) = %v > previous %v — not monotonically decreasing", q, a, prev)
		}
		if q >= 1 && a != 0 {
			t.Errorf("alpha(%.2f) = %v, want exactly 0 at and beyond the rim", q, a)
		}
		prev = a
	}
	if got := (Scrim{Strength: 0}).alpha(0); got != 0 {
		t.Errorf("zero-strength scrim painted %v", got)
	}
}

func TestScrimDarkensCenterAndFadesToScene(t *testing.T) {
	st := stateFor(0.45, PhaseWork)
	sc := New("night", ModeHalfblock)
	with := st
	with.Scrims = []Scrim{{CX: 50, CY: 15, RadiusX: 12, RadiusY: 5, Strength: 0.55}}
	r0 := sc.FramePixels(time.Second, 100, 30, st)
	r1 := sc.FramePixels(time.Second, 100, 30, with)
	if r0.W != r1.W || r0.H != r1.H {
		t.Fatal("scrim changed the raster geometry")
	}
	lum := func(r Raster, x, y int) float64 {
		p := r.Pix[y*r.W+x]
		return 0.2126*float64(p.r) + 0.7152*float64(p.g) + 0.0722*float64(p.b)
	}
	// at the scrim center the scene must be darker than the unscrimmed render
	cx, cy := 50, 30 // subcells: 2y, y = 15
	if l0, l1 := lum(r0, cx, cy), lum(r1, cx, cy); l1 >= l0 {
		t.Errorf("scrim center luminance %.3f not darker than unscrimmed %.3f", l1, l0)
	}
	// far outside every scrim radius the renders are identical
	for _, pt := range [][2]int{{0, 0}, {99, 0}, {0, 59}, {99, 59}} {
		p0 := r0.Pix[pt[1]*r0.W+pt[0]]
		p1 := r1.Pix[pt[1]*r1.W+pt[0]]
		if p0 != p1 {
			t.Errorf("scrim leaked outside its radius at %v: %v vs %v", pt, p0, p1)
		}
	}
}

func TestScrimGradientOnUniformBackdrop(t *testing.T) {
	p := paletteFor(DefaultPalette())
	b := newBuffer(ModeHalfblock, 40, 10, p)
	b.fillRectHexF(0, 0, 40, 20, "#ffffff", 1, opNormal)
	scrim := Scrim{CX: 20, CY: 5, RadiusX: 8, RadiusY: 2, Strength: 0.55}
	applyScrims(b, []Scrim{scrim}, 1, 2)
	px := b.fb.downsample()
	lumPx := func(x, y int) float64 {
		q := px[y*40+x]
		return 0.2126*float64(q.r) + 0.7152*float64(q.g) + 0.0722*float64(q.b)
	}
	center := lumPx(20, 10)
	mid := lumPx(24, 10)
	rim := lumPx(28, 10)
	if !(center < mid && mid < rim) {
		t.Errorf("falloff must decrease smoothly: center %.3f mid %.3f rim %.3f", center, mid, rim)
	}
	if d := math.Abs(rim - 1.0); d > 0.02 {
		t.Errorf("at the rim the scrim must reach exactly zero: lum %.3f", rim)
	}
}
