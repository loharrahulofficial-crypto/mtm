package ambient

import (
	"math"
	"time"
)

// koiScene is a quiet koi pond: segment-jointed fish cruising lissajous
// paths, lily pads, drifting petals, and the occasional expanding ripple.
// Progress deepens the water color toward dusk.
type koiScene struct {
	mode string
}

func (s *koiScene) Name() string { return "koi" }

func (s *koiScene) Frame(t time.Duration, w, h int, state SceneState) []Line {
	return renderFrame(s, s.mode, t, w, h, state, false)
}

func (s *koiScene) Reward(t time.Duration, w, h int, state SceneState) []Line {
	return renderFrame(s, s.mode, t, w, h, state, true)
}

func (s *koiScene) paint(b *buffer, p *paletteModel, t time.Duration, w, h int, state SceneState) {
	// Water: base color with a few sunken depth lines (no flat fills)
	deep := cachedPerlin(state.Seed, 71)
	for y := sh(b) / 3; y < sh(b); y += sh(b)/4 + 1 {
		n := deep.At(0.3, float64(y)*0.11)
		if n > 0 {
			for x := 0; x < sw(b); x += 2 {
				b.mark(x, y, '·', mixIndex(p, cBlue, p.base, 0.55))
			}
		}
	}
	// Quiet caustic shimmer
	shimmer := cachedPerlin(state.Seed, 73)
	for y := 0; y < sh(b); y += 3 {
		for x := 0; x < sw(b); x += 2 {
			n := shimmer.At(float64(x)*0.14+tSecF(t)*0.18, float64(y)*0.3)
			if n > 0.62 {
				b.glow(x, y, mixIndex(p, cCyan, p.base, 0.75))
			}
		}
	}

	// Lily pads: seeded clusters on the surface
	for i := 0; i < 5; i++ {
		lr := newRNG(state.Seed, uint64(i)*412+19)
		px := lr.rangeF(0.06, 0.94) * float64(sw(b))
		py := lr.rangeF(0.1, 0.55) * float64(sh(b))
		for j := 0; j < 4; j++ {
			ci := mixIndex(p, cSuccess, p.base, 0.25+lr.float64()*0.35)
			b.mark(int(px+float64(j)-1.5), int(py), '▬', ci)
		}
		b.mark(int(px), int(py)-1, '▴', mixIndex(p, cSuccess, p.base, 0.15))
	}

	// Koi: two fish, segment chains along lissajous paths
	for i := 0; i < 2; i++ {
		var path func(tt float64) (float64, float64)
		if i == 0 {
			path = func(tt float64) (float64, float64) {
				return (0.5 + 0.34*math.Cos(tt*0.33+1.7)) * float64(sw(b)), (0.42 + 0.28*math.Sin(tt*0.47+0.4)) * float64(sh(b))
			}
		} else {
			path = func(tt float64) (float64, float64) {
				return (0.5 + 0.36*math.Cos(tt*0.27+4.2)) * float64(sw(b)), (0.55 + 0.26*math.Sin(tt*0.39+2.9)) * float64(sh(b))
			}
		}
		now := float64(wrapT(t, 60*time.Minute)) / float64(time.Second)
		segDelay := 0.42 // seconds behind the head per segment
		for segIdx := 5; segIdx >= 0; segIdx-- {
			pt := now - float64(segIdx)*segDelay
			fx, fy := path(pt)
			sway := math.Sin(pt*3.1+float64(segIdx)) * 0.35
			x := int(fx + sway)
			y := int(fy)
			ci := mixIndex(p, cOrange, p.base, 0.05+float64(segIdx)*0.09)
			if segIdx%2 == 0 {
				ci = mixIndex(p, cBrightText, p.base, 0.1+float64(segIdx)*0.09)
			}
			if segIdx == 0 {
				b.mark(x, y, '●', ci)
			} else if segIdx >= 4 {
				b.mark(x, y, '›', ci) // tail tapering
			} else {
				b.mark(x, y, '●', ci)
			}
		}
	}

	// Ripples: seeded expanding slots
	ripSlot := int64(wrapT(t, 24*time.Hour)/time.Millisecond) / 1400
	for k := 0; k < 3; k++ {
		slot := ripSlot - int64(k)
		rr := newRNG(state.Seed, uint64(slot)*521+31)
		frac := (float64(wrapT(t, 24*time.Hour)/time.Millisecond) / 1000.0) - float64(slot)*1.4
		if frac < 0 || frac >= 1.4 {
			continue
		}
		rx := rr.rangeF(0.2, 0.8) * float64(sw(b))
		ry := rr.rangeF(0.25, 0.8) * float64(sh(b))
		ci := mixIndex(p, cCyan, p.base, 0.45+frac*0.45)
		radius := frac / 1.4 * 4
		for dx := -4; dx <= 4; dx++ {
			d := absF(float64(dx))
			if d <= radius && d >= radius-0.75 {
				b.glow(int(rx)+dx, int(ry), ci)
			}
		}
	}

	// Drifting petals landing on the water
	for i := 0; i < 4; i++ {
		petalSlot := int64(wrapT(t, 24*time.Hour)/time.Millisecond)/2100 + int64(i)
		pfrac := (float64(wrapT(t, 24*time.Hour)/time.Millisecond) / 1000.0) - float64(petalSlot)*2.1
		if pfrac < 0 || pfrac >= 2.1 {
			continue
		}
		fr := newRNG(state.Seed^0xa41, uint64(i)*97+53)
		px := fr.rangeF(0.1, 0.9) * float64(sw(b))
		py := pfrac * float64(sh(b)) * 0.5
		ci := mixIndex(p, cOrange, p.base, 0.3)
		b.mark(int(px), int(py), ',', ci)
	}
}
