package ambient

import (
	"math"
	"testing"
)

// glow_test.go — soft shapes: radial falloff is monotonic with no steps
// bigger than the gradient allows (no rings), sun/moon disc is darker at
// the limb than at the center, and clouds show no abrupt luminance step
// (no outlines, even when overlapping).

func subLum(b *buffer, x, y int) float64 {
	c := b.fb.px[y*3*b.fb.pw+x*3+1] // center-ish superpixel of the subcell
	return 0.2126*float64(c.r) + 0.7152*float64(c.g) + 0.0722*float64(c.b)
}

func maxStep(vals []float64) (maxDelta float64, worst int) {
	maxDelta = 0
	worst = -1
	for i := 1; i < len(vals); i++ {
		d := math.Abs(vals[i] - vals[i-1])
		if d > maxDelta {
			maxDelta = d
			worst = i
		}
	}
	return maxDelta, worst
}

func TestGlowRadialFalloffMonotonicNoRings(t *testing.T) {
	p := paletteFor(DefaultPalette())
	b := newBuffer(ModeHalfblock, 60, 20, p, SubpixelAspect(ModeHalfblock, DefaultCellPixels()))
	b.fillGlowHexF(30, 10, 5, 0.7, "#e8dfc4", opAdd)
	// along the horizontal radius, outward from the center
	vals := make([]float64, 0, 24)
	for x := 30; x <= 53; x++ {
		vals = append(vals, subLum(b, x, 10))
	}
	for i := 1; i < len(vals); i++ {
		if vals[i] > vals[i-1]+1e-6 {
			t.Errorf("radius luminance rose at subcell %d: %.4f > %.4f (not monotonic)", i, vals[i], vals[i-1])
		}
	}
	// a Gaussian's steepest step is ~peak/sigma * e^{-0.5} at d = sigma;
	// any step steeper than that is a rim, not a falloff.
	limit := 0.7*0.61/5 + 0.005
	if d, _ := maxStep(vals); d > limit {
		t.Errorf("largest step between adjacent subpixels on the glow radius: %.4f (want <= %.4f, no rings)", d, limit)
	}
	// vertical radius too
	vals = vals[:0]
	for y := 10; y <= 19; y++ {
		vals = append(vals, subLum(b, 30, y))
	}
	for i := 1; i < len(vals); i++ {
		if vals[i] > vals[i-1]+1e-6 {
			t.Errorf("vertical radius luminance rose at subcell %d: %.4f > %.4f", i, vals[i], vals[i-1])
		}
	}
	limit = 0.7*0.61/5 + 0.005
	if d, _ := maxStep(vals); d > limit {
		t.Errorf("largest step between adjacent subpixels on the vertical glow radius: %.4f (want <= %.4f)", d, limit)
	}
}

func TestGlowFalloffIsExponentialNotDisc(t *testing.T) {
	// a disc stays flat then drops to nothing at its rim; an exponential
	// keeps falling — at 2 sigma the level is a fraction of the 1-sigma
	// level.
	p := paletteFor(DefaultPalette())
	b := newBuffer(ModeHalfblock, 60, 20, p, SubpixelAspect(ModeHalfblock, DefaultCellPixels()))
	b.fillGlowHexF(30, 10, 4, 0.8, "#ffffff", opAdd)
	// measure the lift above the local background, not the absolute
	// luminance (the base fill lifts both levels equally)
	at1 := subLum(b, 34, 10) - subLum(b, 45, 10)
	at2 := subLum(b, 38, 10) - subLum(b, 45, 10)
	if at1 <= 0 || at2 <= 0 {
		t.Fatal("no glow painted")
	}
	if ratio := at2 / at1; ratio >= 0.35 {
		t.Errorf("glow lift at 2 sigma / at 1 sigma = %.3f, want < 0.35 (exponential decay, not a disc)", ratio)
	}
}

func TestSunSphereLimbDarkening(t *testing.T) {
	p := paletteFor(DefaultPalette())
	b := newBuffer(ModeHalfblock, 60, 20, p, SubpixelAspect(ModeHalfblock, DefaultCellPixels()))
	paintSunSphere(b, 30, 10, 5, 0, "#b0623f", "#ffd9a0", haloCol(DefaultPalette().BrightText, DefaultPalette().Yellow), 0.4)
	center := subLum(b, 30, 10)
	ridge := subLum(b, 30, 17) // 7 subcells below the center; near the limb
	if center <= ridge {
		t.Errorf("sun center luminance %.4f not brighter than limb %.4f (no limb darkening)", center, ridge)
	}
}

func TestCloudHasNoAbruptStep(t *testing.T) {
	p := paletteFor(DefaultPalette())
	b := newBuffer(ModeHalfblock, 80, 24, p, SubpixelAspect(ModeHalfblock, DefaultCellPixels()))
	cloud := cachedPerlin(42, 211)
	paintCloudMass(b, 40, 12, 12, 6, 1.5, cloud, "#cfd3c0", "#5f6b64", 0.75)
	vals := make([]float64, 0, 60)
	for x := 0; x < 80; x++ {
		vals = append(vals, subLum(b, x, 12))
	}
	// 0.05 per subcell at fSS=3: per superpixel ~0.017 — visually smooth
	if d, _ := maxStep(vals); d > 0.05 {
		t.Errorf("largest adjacent luminance step across the cloud center row: %.4f (want <= 0.05)", d)
	}
	vals = vals[:0]
	for y := 3; y < 22; y++ {
		vals = append(vals, subLum(b, 40, y))
	}
	if d, _ := maxStep(vals); d > 0.05 {
		t.Errorf("largest adjacent luminance step through the cloud column: %.4f (want <= 0.05)", d)
	}
}

func TestCloudOverlapHasNoOutline(t *testing.T) {
	p := paletteFor(DefaultPalette())
	b := newBuffer(ModeHalfblock, 80, 24, p, SubpixelAspect(ModeHalfblock, DefaultCellPixels()))
	cloud := cachedPerlin(42, 211)
	paintCloudMass(b, 32, 12, 11, 5, 0.5, cloud, "#cfd3c0", "#5f6b64", 0.7)
	paintCloudMass(b, 46, 12, 11, 5, 3.5, cloud, "#cfd3c0", "#5f6b64", 0.7)
	vals := make([]float64, 0, 80)
	for x := 0; x < 80; x++ {
		vals = append(vals, subLum(b, x, 12))
	}
	// same 0.05 bound as the single-cloud test: overlap must not
	// add a rim either (no outlines at the boundary)
	if d, i := maxStep(vals); d > 0.05 {
		t.Errorf("largest adjacent luminance step across the overlap: %.4f at subcell %d (want <= 0.05, no outlines)", d, i)
	}
}

func TestCloudIsNotAUniformBlob(t *testing.T) {
	p := paletteFor(DefaultPalette())
	b := newBuffer(ModeHalfblock, 80, 24, p, SubpixelAspect(ModeHalfblock, DefaultCellPixels()))
	cloud := cachedPerlin(42, 211)
	paintCloudMass(b, 40, 12, 12, 6, 1.5, cloud, "#cfd3c0", "#5f6b64", 0.75)
	minL, maxL := math.Inf(1), 0.0
	for _, x := range []int{28, 32, 36, 40, 44, 48, 52, 56} {
		l := subLum(b, x, 12)
		minL = math.Min(minL, l)
		maxL = math.Max(maxL, l)
	}
	if maxL-minL < 0.02 {
		t.Errorf("cloud interior is uniform (spread %.4f) — no FBM structure visible", maxL-minL)
	}
}
