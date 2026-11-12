package ambient

import (
	"fmt"
	"math"
	"testing"
	"time"
)

// moon_test.go — the moon's physics: phase geometry vs. the lit fraction,
// the maria texture, earthshine above the sky, the halo's Gaussian-like
// falloff, and clipping.

type sphereSample struct {
	lit   bool // geometrically lit (n·l > epsilon)
	light float64
	px    float64
}

func sphereSamples(t *testing.T, phase float64) []sphereSample {
	t.Helper()
	p := paletteFor(DefaultPalette())
	b := newBuffer(ModeHalfblock, 48, 24, p, SubpixelAspect(ModeHalfblock, DefaultCellPixels()))
	cols := sphereCols(DefaultPalette().BrightText, DefaultPalette().Base, DefaultPalette().Base)
	mX, mY, mR := 24.0, 12.0, 9.0
	paintMoonSphere(b, mX, mY, mR, phase, 0, cols, [3]float32{1, 1, 1})
	illum := moonIllumination(phase)
	psi := math.Pi * (1 - illum)
	s := 1.0
	if !moonWaxing(phase) {
		s = -1.0
	}
	var out []sphereSample
	for y := 0; y < b.sh; y++ {
		for x := 0; x < b.sw; x++ {
			px := float64(x) + 0.5
			py := float64(y) + 0.5
			nx, ny := (px-mX)/mR, (py-mY)/mR
			d := math.Hypot(nx, ny)
			if d > 0.94 { // full-coverage interior only
				continue
			}
			z := math.Sqrt(math.Max(0, 1-nx*nx-ny*ny))
			ndotl := nx*math.Sin(psi)*s + z*math.Cos(psi)
			c := b.fb.px[(y*3+1)*b.fb.pw+x*3+1]
			out = append(out, sphereSample{ndotl > 0.02, fpixLum(c), px})
		}
	}
	return out
}

func classMeans(samples []sphereSample) (litMean, darkMean float64, nLit, nDark int) {
	var sumLit, sumDark float64
	for _, s := range samples {
		if s.lit {
			sumLit += s.light
			nLit++
		} else {
			sumDark += s.light
			nDark++
		}
	}
	if nLit > 0 {
		litMean = sumLit / float64(nLit)
	}
	if nDark > 0 {
		darkMean = sumDark / float64(nDark)
	}
	return
}

func TestMoonPhaseLitFractions(t *testing.T) {
	for _, tc := range []struct {
		phase     float64
		wantLit   float64
		leftFirst bool
	}{
		{0.0, 0.0, false},
		{0.25, 0.5, false},
		{0.5, 1.0, false},
		{0.75, 0.5, true},
		{1.0, 0.0, false},
	} {
		samples := sphereSamples(t, tc.phase)
		_, _, nLit, _ := classMeans(samples)
		total := float64(len(samples))
		got := float64(nLit) / total
		if math.Abs(got-tc.wantLit) > 0.05 {
			t.Errorf("phase %.2f: geometric lit fraction %.3f, want %.2f (±5%%)", tc.phase, got, tc.wantLit)
		}
		litMean, _, _, _ := classMeans(samples)
		// the ordering assertion is only meaningful at full: the whole
		// face must sit clearly above the earthshine reference
		if tc.phase == 0.5 {
			earthMean, _, _, _ := classMeans(sphereSamples(t, 0.05))
			if litMean <= earthMean*1.2 {
				t.Errorf("phase 0.5 (full): lit mean (%.3f) not clearly above the earthshine reference (%.3f)", litMean, earthMean)
			}
		}
	}
	// side check: waxing lights the right, waning the left (geometric mask)
	side := func(phase float64) (left, right int) {
		for _, s := range sphereSamples(t, phase) {
			if s.lit {
				if s.px < 24 {
					left++
				} else {
					right++
				}
			}
		}
		return left, right
	}
	if left, right := side(0.25); right <= left {
		t.Errorf("phase 0.25 (waxing): right side (%d) must exceed the left (%d)", right, left)
	}
	if left, right := side(0.75); left <= right {
		t.Errorf("phase 0.75 (waning): left side (%d) must exceed the right (%d)", left, right)
	}
}

func TestMoonPhasesAreDistinct(t *testing.T) {
	seen := map[string]bool{}
	for _, phase := range []float64{0, 0.25, 0.5, 0.75, 1} {
		samples := sphereSamples(t, phase)
		_, _, nLit, _ := classMeans(samples)
		seen[fmt.Sprintf("%d", nLit)] = true
	}
	// 0/1 → 0, 0.25/0.75 → half, 0.5 → all
	if len(seen) != 3 {
		t.Errorf("phases produced %d distinct lit counts (%v), want 3", len(seen), seen)
	}
}

func fpixLum(c fpix) float64 {
	return 0.2126*float64(c.r) + 0.7152*float64(c.g) + 0.0722*float64(c.b)
}

func TestMoonEarthshineAboveSky(t *testing.T) {
	p := paletteFor(DefaultPalette())
	skyBuf := newBuffer(ModeHalfblock, 48, 24, p, SubpixelAspect(ModeHalfblock, DefaultCellPixels())) // nothing painted: pure base fill
	skyLum := fpixLum(skyBuf.fb.px[0])
	// near-new: almost entirely the dark side
	b2 := newBuffer(ModeHalfblock, 48, 24, paletteFor(DefaultPalette()), SubpixelAspect(ModeHalfblock, DefaultCellPixels()))
	paintMoonSphere(b2, 24, 12, 9, 0.05, 0, sphereCols(DefaultPalette().BrightText, DefaultPalette().Base, DefaultPalette().Base), [3]float32{1, 1, 1})
	minLum := math.Inf(1)
	for y := 0; y < b2.sh; y++ {
		for x := 0; x < b2.sw; x++ {
			px := float64(x) + 0.5
			py := float64(y) + 0.5
			if math.Hypot(px-24, py-12) > 8.5 {
				continue
			}
			c := b2.fb.px[(y*3+1)*b2.fb.pw+x*3+1]
			l := fpixLum(c)
			if l < minLum {
				minLum = l
			}
		}
	}
	if minLum <= skyLum {
		t.Errorf("earthshine (%.4f) not above the sky (%.4f) — the dark side reads as a black sticker", minLum, skyLum)
	}
}

func TestMoonGlowFalloff(t *testing.T) {
	sigma, amp := 8.0, 0.4
	// unit test of the falloff shape: monotonically decreasing to zero
	prev := math.Inf(1)
	for d := 0.0; d <= sigma*3; d += 0.25 {
		a := MoonGlowAlpha(d, sigma, amp)
		if a > prev {
			t.Errorf("glow alpha increased with distance: d=%v a=%.4f (was %.4f)", d, a, prev)
		}
		if a < 0 {
			t.Errorf("glow alpha negative at d=%v", d)
		}
		prev = a
	}
	// frame level: ring luminance decreases monotonically outward
	p := paletteFor(DefaultPalette())
	b := newBuffer(ModeHalfblock, 48, 24, p, SubpixelAspect(ModeHalfblock, DefaultCellPixels()))
	cols := sphereCols(DefaultPalette().BrightText, DefaultPalette().Base, DefaultPalette().Base)
	paintMoonSphere(b, 24, 12, 7, 0.5, 0, cols, haloCol(DefaultPalette().BrightText, DefaultPalette().Accent))
	lumAt := func(x, y int) float64 {
		c := b.fb.px[(y*3+1)*b.fb.pw+x*3+1]
		return fpixLum(c)
	}
	prev = lumAt(31, 12)
	for x := 32; x < 46; x++ {
		l := lumAt(x, 12)
		if l > prev+0.02 {
			t.Errorf("halo luminance rose away from the moon at x=%d: %.4f > %.4f", x, l, prev)
		}
		prev = l
	}
}

func TestMoonNotClipped(t *testing.T) {
	for _, size := range [][2]int{{24, 10}, {100, 30}, {200, 50}} {
		for _, prog := range []float64{0, 0.25, 0.5, 0.75, 1} {
			for _, scene := range []string{"night", "ocean", "snow"} {
				w, h := size[0], size[1]
				st := stateFor(prog, PhaseWork)
				st.FullFrame = true
				st.Now = time.Date(2026, 11, 7, 21, 30, 0, 0, time.Local)
				anchors := SceneAnchors(scene, 3*time.Second, w, h, st)
				a, ok := anchors["moon"]
				if !ok {
					continue
				}
				if clip := anchorClip(a, float64(w), float64(h*2)); clip > 0 {
					t.Errorf("%s %dx%d prog %.2f: moon clipped by %.1f%% (anchor %v)", scene, w, h, prog, clip*100, a)
				}
			}
		}
	}
}
