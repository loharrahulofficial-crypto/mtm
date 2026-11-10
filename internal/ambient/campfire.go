package ambient

import (
	"math"
	"time"
)

// campfireScene is a small fire under the stars: rising flame tongues
// (noise-shaped columns), popping sparks, a leaning smoke column, stones,
// crossed logs, and a warm radial glow that breathes with the flame.
type campfireScene struct {
	mode string
}

func (s *campfireScene) Name() string { return "campfire" }

func (s *campfireScene) Frame(t time.Duration, w, h int, state SceneState) []Line {
	return renderFrame(s, s.mode, t, w, h, state, false)
}

func (s *campfireScene) Reward(t time.Duration, w, h int, state SceneState) []Line {
	return renderFrame(s, s.mode, t, w, h, state, true)
}

func (s *campfireScene) paint(b *buffer, p *paletteModel, t time.Duration, w, h int, state SceneState) {
	tSec := tSecF(t)
	// Night sky with stars and a dim moon
	for y := 0; y < sh(b)*62/100; y++ {
		f := float64(y) / float64(maxInt(sh(b)*62/100-1, 1))
		b.glowBlock(0, y, sw(b), y+1, mixIndex(p, cBase, p.base, f*0.15))
	}
	for _, st := range sceneStars(state.Seed, sw(b), sh(b)) {
		if st.layer >= 1 && st.mag > 0.6 {
			b.mark(st.x, st.y, '·', mixIndex(p, cBrightText, p.base, 0.4))
		}
	}
	phase := moonPhase(state.Now)
	paintMoon(b, sw(b)/4, maxInt(sh(b)/6, 1), maxInt(minInt(sw(b), sh(b))/9, 1), phase, p)

	// Ground line only — the fire glow carries the rest
	fireY := sh(b) * 78 / 100
	for x := 0; x < sw(b); x++ {
		b.glow(x, fireY+2, mixIndex(p, cBrown, p.base, 0.5))
		b.glow(x, fireY+3, mixIndex(p, cBrown, p.base, 0.35))
	}

	// Warm radial glow around the fire, breathing with noise
	glowN := newPerlin(state.Seed, 701)
	fx0 := sw(b) / 2
	breath := 0.85 + 0.3*glowN.At(tSec*0.55, 0.5)
	for y := fireY - sh(b)*45/100; y < fireY+2; y++ {
		for x := 0; x < sw(b); x++ {
			dx := float64(x-fx0) / float64(maxInt(sw(b)/2, 1))
			dy := float64(y-fireY) / float64(maxInt(sh(b)/2, 1))
			d2 := dx*dx*1.4 + dy*dy*2.6
			intensity := clampf((1.15-d2)*breath*0.9, 0, 1)
			if intensity > 0.05 {
				b.glow(x, y, mixIndex(p, cWarning, p.base, 1-intensity*0.85))
			}
		}
	}

	// Stone ring
	stones := [][2]int{{-7, 1}, {-5, 2}, {-3, 2}, {-1, 3}, {1, 3}, {3, 2}, {5, 2}, {7, 1}}
	for _, sPos := range stones {
		x := fx0 + sPos[0]
		y := fireY + sPos[1] - 2
		if x >= 0 && x < sw(b) && y >= 0 && y < sh(b) {
			b.mark(x, y, 'o', mixIndex(p, cSurfaceRaised, p.base, 0.25))
		}
	}
	// Crossed logs
	logCols := mixIndex(p, cBrown, p.base, 0.15)
	for dx := -4; dx <= 4; dx++ {
		b.glow(fx0+dx, fireY-2, logCols)
		b.glow(fx0+dx, fireY-1, mixIndex(p, cBrown, p.base, 0.25))
	}
	for dx := -3; dx <= 3; dx += 1 {
		if dx%2 == 0 {
			b.glow(fx0+dx, fireY-3, mixIndex(p, cBrown, p.base, 0.3))
		}
	}

	// Flame tongues: columns of rising noise-shaped cells
	flame := newPerlin(state.Seed, 719)
	flameTop := fireY - 2 - sh(b)*34/100
	for x := fx0 - 3; x <= fx0+3; x++ {
		lateral := absF(float64(x-fx0)) / 3.5
		for y := fireY - 3; y > flameTop; y-- {
			height := float64(fireY-3-y) / float64(maxInt(fireY-flameTop, 1))
			colN := flame.At(tSec*1.35+float64(x)*0.71, float64(y)*0.16+31.0)
			density := 1.35 - height*1.15 - lateral*0.8 + colN*0.65
			if density > 0.18 {
				ci := cWarning
				switch {
				case height < 0.33:
					ci = cDanger
				case height < 0.62:
					ci = cWarning
				default:
					ci = cYellow
				}
				b.glow(x, y, mixIndex(p, ci, p.base, height*0.55+0.05))
			}
		}
	}

	// Sparks: slotted pops above the flame
	sparkSlot := int64(wrapT(t, 24*time.Hour)/time.Millisecond) / 700
	for k := 0; k < 3; k++ {
		slot := sparkSlot - int64(k)
		sr := newRNG(state.Seed, uint64(slot)*383+13)
		frac := (float64(wrapT(t, 24*time.Hour)/time.Millisecond) / 1000.0) - float64(slot)*0.7
		if frac < 0 || frac >= 0.7 {
			continue
		}
		spx := float64(fx0) + sr.rangeF(-2.5, 2.5) + math.Sin(frac*9+float64(slot))*1.2
		spy := float64(fireY-4) - frac/0.7*float64(sh(b))*0.3
		ci := cYellow
		if frac > 0.4 {
			ci = mixIndex(p, cWarning, p.base, 0.35)
		}
		b.mark(int(spx), int(spy), '*', ci)
	}

	// Smoke: rising, leaning column of fading dots
	smoke := newPerlin(state.Seed, 733)
	for k := 0; k < 6; k++ {
		ff := float64(wrapT(t, 24*time.Hour)/time.Millisecond)*0.24 + float64(k)*0.17
		fp := math.Mod(ff, 1.7) / 1.7
		py := float64(flameTop) - fp*float64(sh(b))*0.38
		lean := smoke.At(fp*2.2+float64(k)*1.3, 0)*3.4*fp + fp*2.2
		ci := mixIndex(p, cText, p.base, 0.35+fp*0.55)
		b.mark(int(float64(fx0)+lean), int(py), '·', ci)
	}
}
