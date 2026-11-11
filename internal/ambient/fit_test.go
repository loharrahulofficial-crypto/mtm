package ambient

import (
	"testing"
	"time"
)

// fit_test.go — §8.4: every scene must compose sanely at very different
// window shapes. Key elements (moon, sun, tree crown, cabin) may not be
// clipped by more than 10%; the horizon lies between 40% and 70% of the
// height.

func anchorClip(a Anchor, W, H float64) float64 {
	ix0 := a.X0
	if ix0 < 0 {
		ix0 = 0
	}
	iy0 := a.Y0
	if iy0 < 0 {
		iy0 = 0
	}
	ix1 := a.X1
	if ix1 > W {
		ix1 = W
	}
	iy1 := a.Y1
	if iy1 > H {
		iy1 = H
	}
	area := (a.X1 - a.X0) * (a.Y1 - a.Y0)
	if area <= 0 {
		return 0
	}
	if ix1 <= ix0 || iy1 <= iy0 {
		return 1
	}
	return 1 - (ix1-ix0)*(iy1-iy0)/area
}

func TestSceneFitAcrossShapes(t *testing.T) {
	sizes := [][2]int{{24, 10}, {30, 16}, {40, 14}, {64, 16}, {100, 30}, {200, 50}}
	times := map[string]time.Time{
		"day":   time.Date(2026, 7, 15, 13, 0, 0, 0, time.Local),
		"night": time.Date(2026, 11, 7, 21, 30, 0, 0, time.Local),
	}
	for _, name := range Registered() {
		for _, size := range sizes {
			for label, now := range times {
				w, h := size[0], size[1]
				W, H := float64(w), float64(h*2)
				st := stateFor(0.5, PhaseWork)
				st.Now = now
				anchors := SceneAnchors(name, 12*time.Second, w, h, st)
				for an, a := range anchors {
					if an == "horizon" {
						frac := ((a.Y0 + a.Y1) / 2) / H
						if frac < 0.40 || frac > 0.70 {
							t.Errorf("%s %dx%d %s: horizon at %.2f of the height, want [0.40, 0.70]", name, w, h, label, frac)
						}
						continue
					}
					if clip := anchorClip(a, W, H); clip > 0.10 {
						t.Errorf("%s %dx%d %s: %s clipped by %.0f%% (>10%%)", name, w, h, label, an, clip*100)
					}
				}
			}
		}
	}
}
