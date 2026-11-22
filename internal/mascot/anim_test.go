package mascot

import (
	"testing"
	"time"

	"github.com/loharrahulofficial-crypto/mtm/internal/ambient"
	"github.com/loharrahulofficial-crypto/mtm/internal/theme"
)

func pal() theme.Palette { return theme.Defaults() }

func assertRastersEqual(t *testing.T, a, b ambient.Raster, ctx string) {
	t.Helper()
	if a.W != b.W || a.H != b.H {
		t.Fatalf("%s: size differs %dx%d vs %dx%d", ctx, a.W, a.H, b.W, b.H)
	}
	for y := 0; y < a.H; y++ {
		for x := 0; x < a.W; x++ {
			r1, g1, b1 := a.Pixel(x, y)
			r2, g2, b2 := b.Pixel(x, y)
			if r1 != r2 || g1 != g2 || b1 != b2 {
				t.Fatalf("%s: pixel (%d,%d) differs", ctx, x, y)
			}
		}
	}
}

func assertRastersDiffer(t *testing.T, a, b ambient.Raster, ctx string) {
	t.Helper()
	diff := 0
	for y := 0; y < a.H; y++ {
		for x := 0; x < a.W; x++ {
			r1, g1, b1 := a.Pixel(x, y)
			r2, g2, b2 := b.Pixel(x, y)
			if r1 != r2 || g1 != g2 || b1 != b2 {
				diff++
			}
		}
	}
	if diff == 0 {
		t.Fatalf("%s: rasters are identical, expected motion", ctx)
	}
}

func TestAnimateDeterminism(t *testing.T) {
	for p := 0; p < NumPoses(); p++ {
		s := State{Pose: p, Since: 3 * time.Second}
		assertRastersEqual(t, Animate(s, pal()), Animate(s, pal()), "deterministic "+PoseName(p))
	}
}

func TestAnimateMotionAcrossTime(t *testing.T) {
	for _, p := range []int{Wave, Celebrate, Alert, Pomo, Sleep, Think} {
		a := Animate(State{Pose: p, Since: 100 * time.Millisecond}, pal())
		b := Animate(State{Pose: p, Since: 830 * time.Millisecond}, pal())
		assertRastersDiffer(t, a, b, "pose "+PoseName(p))
	}
}

func TestBlinkCollapsesEyes(t *testing.T) {
	a := Anchor(Wave)
	if len(a.Eyes) < 2 || a.Eyes[0].Empty() || a.Eyes[1].Empty() {
		t.Skipf("wave eyes not anchored: %v", a.Eyes)
	}
	fired, found := false, false
	for t0 := time.Duration(0); t0 < 12*time.Second; t0 += time.Millisecond {
		if !blink(t0, Wave) {
			continue
		}
		fired = true
		r := Animate(State{Pose: Wave, Since: t0}, pal())
		r0, g0, b0 := r.Pixel(a.Eyes[0].X0, a.Eyes[0].Y0)
		if r0 > 230 && g0 > 210 && b0 > 180 {
			found = true // collapsed bright eye bar
		}
	}
	if !fired {
		t.Fatal("never hit blink window over 12s")
	}
	if !found {
		t.Fatal("blink never rendered as a bright eye bar")
	}
}

func TestLipSyncWhileSpeaking(t *testing.T) {
	a := Anchor(Wave)
	if a.Mouth.Empty() {
		t.Skip("wave mouth not anchored")
	}
	r1 := Animate(State{Pose: Wave, Since: 100 * time.Millisecond}, pal())
	r2 := Animate(State{Pose: Wave, Since: 100 * time.Millisecond, Speaking: true}, pal())
	assertRastersDiffer(t, r1, r2, "lip-sync")
}

func TestStarTwinkle(t *testing.T) {
	a := Anchor(Wave)
	if a.Star.Empty() {
		t.Skip("wave star not anchored")
	}
	// Some point in the twinkle cycle renders a bright star pixel.
	bright := false
	for ph := 0.0; ph < 1.0; ph += 0.001 {
		r := Animate(State{Pose: Wave, Since: time.Duration(ph * 4000 * float64(time.Millisecond))}, pal())
		for y := a.Star.Y0; y < a.Star.Y1; y++ {
			for x := a.Star.X0; x < a.Star.X1; x++ {
				r0, g0, b0 := r.Pixel(x, y)
				if r0 > 230 && g0 > 210 && b0 > 180 {
					bright = true
				}
			}
		}
	}
	if !bright {
		t.Fatal("star twinkle never rendered a bright star pixel")
	}
}

func TestFrozenStatic(t *testing.T) {
	s := State{Pose: Pomo, Prev: Wave, Since: 34 * time.Second, Frozen: true}
	r := Animate(s, pal())
	_, _, pix := Grid(Pomo)
	w, h := 40, 48
	g := copyGrid(w, h, pix)
	assertRastersEqual(t, r, gridToRaster(w, h, g, pal()), "frozen equals static")
}
