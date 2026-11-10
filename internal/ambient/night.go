package ambient

import (
	"math"
	"time"
)

// nightScene paints a parallax starfield: magnitude-scaled stars, a moon in
// its real phase (computed locally), an occasional shooting star, and an
// aurora of layered noise ribbons in the upper third.
type nightScene struct {
	mode string
}

func (s *nightScene) Name() string { return "night" }

func (s *nightScene) Frame(t time.Duration, w, h int, state SceneState) []Line {
	return renderFrame(s, s.mode, t, w, h, state, false)
}

func (s *nightScene) Reward(t time.Duration, w, h int, state SceneState) []Line {
	return renderFrame(s, s.mode, t, w, h, state, true)
}

// star is one deterministic star.
type star struct {
	x, y  int
	mag   float64
	phase float64
	speed float64
	drift float64 // parallax drift per minute
	layer int
}

func sceneStars(seed int64, w, h int) []star {
	r := newRNG(seed, 17)
	n := clampf(float64(w*h)/9, 18, 110)
	out := make([]star, 0, int(n))
	for i := 0; i < int(n); i++ {
		layer := r.intn(3) // 0 far, 1 mid, 2 near
		mag := r.float64()
		mag = mag * mag // bias toward dim stars
		out = append(out, star{
			x:     r.intn(maxInt(w, 1)),
			y:     r.intn(maxInt(h*7/10, 1)),
			mag:   mag,
			phase: r.rangeF(0, 2*math.Pi),
			speed: r.rangeF(0.5, 1.6),
			drift: []float64{0.004, 0.011, 0.021}[layer],
			layer: layer,
		})
	}
	return out
}

func (s *nightScene) paint(b *buffer, p *paletteModel, t time.Duration, w, h int, state SceneState) {
	// Sky: quantized gradient bands, deep zenith to a faint horizon glow.
	for y := 0; y < sh(b); y++ {
		f := float64(y) / float64(maxInt(sh(b)-1, 1))
		var row int
		switch {
		case f < 0.88:
			continue // sky stays the untouched base color
		default:
			row = mixIndex(p, cSurfaceStrong, p.base, 0.72)
		}
		b.glowBlock(0, y, sw(b), y+1, row)
	}

	// Aurora: two thin ribbons hugging the upper sky, dim and slow.
	b.beginLayer(layerSky, 0, 0, sw(b), minInt(sh(b), sh(b)/2+3))
	aura := cachedPerlin(state.Seed, 71)
	ts := float64(wrapT(t, 8*time.Minute)) / float64(time.Minute)
	for x := 0; x < sw(b); x++ {
		for r := 0; r < 2; r++ {
			fx := float64(x) / float64(sw(b))
			cen := sh(b)*14/100 + int(aura.At(fx*3.1+float64(r)*13.7, ts*0.25+float64(r)*7.3)*float64(sh(b))/7)
			thick := 1.0 + aura.At(fx*5.2+float64(r)*29.1, ts*0.35+3.0+float64(r)*17.0)*1.1
			ints := 0.30 + 0.42*(0.5+0.5*aura.At(fx*2.3+float64(r)*41.0, ts*0.18+float64(r)*23.0))
			for y := 0; y < sh(b); y++ {
				d := absF(float64(y) - float64(cen))
				if d > thick {
					continue
				}
				a := (1 - d/thick) * ints
				col := cCyan
				if r == 1 {
					col = cSuccess
				}
				b.glow(x, y, mixIndex(p, col, p.base, 1-a*0.75))
			}
		}
	}

	b.endLayer()

	// Stars: two layers with drift + twinkle.
	b.beginLayer(layerStars, 0, 0, sw(b), sh(b)*7/10)
	for _, st := range sceneStars(state.Seed, sw(b), sh(b)) {
		driftX := int(float64(st.x) + st.drift*float64(wrapT(t, 3*time.Minute))/float64(time.Minute)*float64(sw(b))/80)
		x := driftX % sw(b)
		if x < 0 {
			x += sw(b)
		}
		tw := 0.55 + 0.45*math.Sin(ts*st.speed+st.phase)
		bright := st.mag * tw
		ch := '.'
		switch {
		case bright > 0.82:
			ch = '✦'
		case bright > 0.60:
			ch = '*'
		case bright > 0.42:
			ch = '+'
		case bright > 0.24:
			ch = ':'
		case bright > 0.12:
			ch = '·'
		default:
			continue // too dim to draw this beat
		}
		ci := mixIndex(p, cBrightText, p.base, 0.85-bright*0.75)
		b.mark(x, st.y, ch, ci)
	}
	b.endLayer()

	// Moon: real phase from the date; progress walks it across the sky.
	phase := moonPhase(state.Now)
	_ = moonIllumination(phase)
	_ = moonAge(state.Now)
	mx := int(float64(sw(b)) * (0.10 + 0.80*clampf(state.Progress, 0, 1)))
	my := int(float64(sh(b)) * (0.58 - 0.42*math.Sin(math.Pi*clampf(state.Progress, 0, 1))))
	mri := minInt(mr0(sw(b), sh(b)), 5)
	if mri < 2 {
		mri = 2
	}
	b.beginLayer(layerCelestial, mx-mri-1, my-mri-1, mx+mri+2, my+mri+2)
	mr := mri
	lightSide := 1 // waxing: light from the right
	if !moonWaxing(phase) {
		lightSide = -1
	}
	term := math.Cos(2*math.Pi*phase) * float64(mr) // terminator position
	for dy := -mr; dy <= mr; dy++ {
		for dx := -mr; dx <= mr; dx++ {
			fx := float64(dx) / float64(mr)
			fy := float64(dy) / float64(mr)
			d2 := fx*fx + fy*fy
			if d2 > 1 {
				continue
			}
			lit := false
			if moonWaxing(phase) {
				lit = float64(dx) > -term*float64(lightSide)
			} else {
				lit = float64(dx) < term*float64(lightSide)
			}
			// Inside the disc: pick lit or dark shade; limb dims toward the edge.
			shade := math.Sqrt(d2)
			var ci int
			if lit {
				ci = mixIndex(p, cBrightText, p.base, 0.12+0.35*shade)
			} else {
				ci = mixIndex(p, cSurface, p.base, 0.25+0.45*shade)
			}
			ch := rune('●')
			if !lit && s.mode == ModeASCII {
				ch = '·'
			}
			b.mark(mx+dx, my+dy, ch, ci)
		}
	}

	b.endLayer()

	// Shooting stars: deterministic slot every ~5.5s with a seeded chance.
	b.beginLayer(layerStars, 0, 0, sw(b), sh(b)/2)
	slotSec := int64(5)
	slot := int64(wrapT(t, 24*time.Hour)/time.Second) / slotSec
	slotR := newRNG(state.Seed, uint64(slot)*977+13)
	if slotR.float64() < 0.55 {
		slotT := float64(wrapT(t, 24*time.Hour)/time.Millisecond) / 1000.0
		frac := (slotT - float64(slot*slotSec)) / 0.8 // 0.8s streak
		if frac >= 0 && frac < 1 {
			dir := 1.0
			if slotR.float64() < 0.5 {
				dir = -1.0
			}
			x0 := slotR.rangeF(0.1, 0.6) * float64(sw(b))
			y0 := slotR.rangeF(0.02, 0.3) * float64(sh(b))
			headX := x0 + dir*frac*float64(sw(b))/5
			headY := y0 + frac*float64(sh(b))/8
			for k := 0; k < 7; k++ {
				kx := headX - dir*frac*float64(sw(b))/5*float64(k)/6
				ky := headY - frac*float64(sh(b))/8*float64(k)/6
				ci := mixIndex(p, cBrightText, p.base, float64(k)/6.5)
				ch := '·'
				if k == 0 {
					ch = '✦'
				} else if k < 3 {
					ch = '─'
				}
				b.mark(int(kx), int(ky), ch, ci)
			}
		}
	}

	b.endLayer()

	// Horizon silhouettes: a jagged pine line at the base.
	ground := sh(b) * 82 / 100
	beginLand := maxInt(0, ground-9)
	b.beginLayer(layerLand, 0, beginLand, sw(b), sh(b))
	sil := cachedPerlin(state.Seed, 101)
	for x := 0; x < sw(b); x++ {
		gy := ground + int(sil.At(float64(x)*0.21, 7.3)*2.2)
		for y := gy; y < sh(b); y++ {
			tree := sil.At(float64(x)*0.45+100, 3.1)
			ci := mixIndex(p, cSurfaceStrong, p.base, 0.6)
			if tree > 0.32 {
				ci = mixIndex(p, cSurfaceStrong, p.base, 0.42)
				if y < gy+3 {
					b.mark(x, y, '▲', ci)
					continue
				}
			}
			b.glow(x, y, ci)
		}
	}
	b.endLayer()
}

func mr0(w, h int) int {
	return minInt(minInt(w, h), minInt(w, h*2)/8)
}

func sw(b *buffer) int { return b.sw }
func sh(b *buffer) int { return b.sh }
