package ambient

import (
	"math"
	"testing"
	"time"
)

// aurora_test.go — the aurora curtains fade upward (bright foot, dim
// top), star density scales with area, and frames actually advance.

func TestAuroraCurtainFadesUpward(t *testing.T) {
	const w, h = 200, 50
	st := stateFor(0.45, PhaseWork)
	st.FullFrame = true
	sc := New("night", ModeHalfblock)
	r := sc.FramePixels(6*time.Second, w, h, st)
	H := float64(h * 2)
	W := float64(w)
	lay := layoutForState(st, W, H)
	_ = lay
	sil := cachedPerlin(st.Seed, 71)
	for ri, ribbon := range auroraRibbons(st) {
		footAt := func(x float64) float64 {
			fx := x / W
			return ribbon.baseY*H + sil.At(fx*ribbon.freq+ribbon.phase, 6*0.055+ribbon.phase*0.7)*ribbon.sway*H
		}
		var baseSum, topSum float64
		var n int
		for x := 0.0; x < W; x += 0.5 {
			foot := footAt(x)
			top := foot - ribbon.thick*H*0.8
			yB := int(math.Round(foot))
			yT := int(math.Round(top))
			if yT < 0 || yB >= r.H {
				continue
			}
			cB := r.Pix[yB*r.W+int(x)]
			cT := r.Pix[yT*r.W+int(x)]
			baseSum += fpixLum(cB)
			topSum += fpixLum(cT)
			n++
		}
		if n == 0 {
			t.Fatalf("ribbon %d: no sample columns", ri)
		}
		base, top := baseSum/float64(n), topSum/float64(n)
		if !(base > top) {
			t.Errorf("ribbon %d (%s): base luminance %.3f not higher than the top %.3f — curtain does not fade upward", ri, ribbon.col, base, top)
		}
	}
}

func TestNightStarCountScalesWithArea(t *testing.T) {
	c1 := len(nightStars(42, 100, 30))
	c2 := len(nightStars(42, 200, 50))
	c3 := len(nightStars(42, 40, 14))
	if got, want := float64(c2)/float64(c1), 4.0; math.Abs(got-want)/want > 0.20 {
		t.Errorf("200x50/100x30 star ratio %.2f, want within 20%% of %.2f", got, want)
	}
	if got, want := float64(c3)/float64(c1), float64(40*14)/float64(100*30); math.Abs(got-want)/want > 0.20 {
		t.Errorf("40x14/100x30 star ratio %.2f, want within 20%% of %.2f", got, want)
	}
}

func TestNightFramesDifferOverTime(t *testing.T) {
	st := stateFor(0.45, PhaseWork)
	st.FullFrame = true
	sc := New("night", ModeHalfblock)
	frame := func(ms int) string {
		return joinLines(sc.Frame(time.Duration(ms)*time.Millisecond, 100, 30, st))
	}
	a, b := frame(0), frame(8000)
	if a == b {
		t.Fatal("frames at t=0 and t=8s are identical — night scene does not animate")
	}
}

func joinLines(lines []Line) string {
	out := ""
	for i, l := range lines {
		if i > 0 {
			out += "\n"
		}
		out += l
	}
	return out
}
