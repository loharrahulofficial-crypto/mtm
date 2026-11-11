package ambient

import (
	"math"
	"time"
)

// meteorScene paints a meteor-shower night: a deep gradient sky denser
// with stars than the night scene, a faint diagonal milky-way wash, and
// a continuous rain of meteors — thin bright streaks with long fading
// gradient tails, several in flight at once, with a brighter bolide
// every third wave. A rugged ridge and pine line anchor the bottom.
type meteorScene struct {
	mode string
}

func (s *meteorScene) Name() string { return "meteor" }

func (s *meteorScene) Frame(t time.Duration, w, h int, state SceneState) []Line {
	return renderFrame(s, s.mode, t, w, h, state, false)
}

func (s *meteorScene) Reward(t time.Duration, w, h int, state SceneState) []Line {
	return renderFrame(s, s.mode, t, w, h, state, true)
}

func (s *meteorScene) FramePixels(t time.Duration, w, h int, state SceneState) Raster {
	return renderRaster(s, t, w, h, state)
}

// meteorStars is a denser, dimmer-biased star field than the night
// scene's: a shower night reads best against a crowded sky.
func meteorStars(seed int64, w, h int) []starv {
	r := newRNG(seed, 23)
	n := clampf(float64(w*h)/8, 30, 190)
	out := make([]starv, 0, int(n))
	for i := 0; i < int(n); i++ {
		mag := r.float64()
		mag = mag * mag * mag // strongly biased toward dim stars
		out = append(out, starv{
			x:     r.float64() * float64(w),
			y:     r.float64() * float64(h) * 0.78,
			mag:   mag,
			phase: r.rangeF(0, 2*math.Pi),
			speed: r.rangeF(0.5, 1.6),
		})
	}
	return out
}

func (s *meteorScene) paint(b *buffer, p *paletteModel, t time.Duration, w, h int, state SceneState) {
	W := float64(sw(b))
	H := float64(sh(b))
	lay := layoutForSub(W, H)
	rmp := BuildRamps(state.Palette)
	ts := float64(wrapT(t, 10*time.Minute)) / float64(time.Second)
	base := state.Palette.Base
	bright := state.Palette.BrightText
	cyan := state.Palette.Cyan
	skyTop := blendHex(rmp.SkyTop, base, 0.22)

	// --- sky: deep zenith, faint cool glow at the horizon
	b.fillGradientHex(0, 0, W, H, skyTop, blendHex(rmp.SkyHorizon, base, 0.12), 1, opNormal)

	// --- milky way: a faint diagonal band of additive noise wash
	mw := cachedPerlin(state.Seed, 313)
	b.beginLayer(layerSky, 0, 0, sw(b), int(lay.Horizon)+2)
	mwCol := blendHex(bright, cyan, 0.22)
	for y := 0.0; y < lay.Horizon; y += 0.5 {
		v := y / lay.Horizon
		cx := W * (0.72 - 0.42*v)
		hw := W*0.05 + float64(mw.At(v*4.2+1.7, ts*0.02))*W*0.03 + lay.MinDim*0.02
		for x := cx - hw; x < cx+hw; x += 0.5 {
			d := 1 - math.Abs(x-cx)/hw
			g := 0.5 + 0.5*float64(mw.At(x*0.05+3.1, y*0.06+ts*0.015))
			cov := d * g * 0.14
			if cov > 0.02 {
				b.glowHexAt(x, y, mwCol, cov)
			}
		}
	}
	b.endLayer()

	// --- stars: dense field, twinkle, cross-glow on the brightest
	b.beginLayer(layerStars, 0, 0, sw(b), int(H*0.78)+2)
	for _, st := range meteorStars(state.Seed, sw(b), sh(b)) {
		tw := 0.55 + 0.45*math.Sin(ts*st.speed+st.phase)
		brightness := st.mag * tw
		if brightness < 0.05 {
			continue
		}
		fade := 1 - brightness*0.82
		if s.mode == ModeASCII {
			b.mark(int(st.x), int(st.y), glyphFor(brightness), mixIndex(p, cBrightText, p.base, fade))
			continue
		}
		r := 0.18 + brightness*0.5
		col := blendHex(bright, base, fade)
		switch {
		case brightness > 0.78:
			b.fillCircleHexF(st.x, st.y, r, col, 1, opNormal)
			b.strokeHexF(st.x-1.5, st.y, st.x+1.5, st.y, 0.22, col, 0.35, opAdd)
			b.strokeHexF(st.x, st.y-1.5, st.x, st.y+1.5, 0.22, col, 0.35, opAdd)
		case brightness > 0.4:
			b.fillCircleHexF(st.x, st.y, r, col, 1, opNormal)
		default:
			b.fillCircleHexF(st.x, st.y, 0.26, col, brightness, opNormal)
		}
	}
	b.endLayer()

	// --- meteor rain: 8-second waves; each wave seeds several meteors
	// with staggered delays and thin gradient tails, plus a bright
	// bolide every third wave that ends in a sparkle pop.
	slotSec := int64(8)
	sec := float64(wrapT(t, 24*time.Hour)/time.Millisecond) / 1000.0
	slot := int64(sec) / slotSec
	slotR := newRNG(state.Seed, uint64(slot)*613+37)
	count := 3 + slotR.intn(3)
	bolide := slot%3 == 0
	if bolide {
		count++
	}
	type meteorv struct {
		delay, dur, ang, x0, y0 float64
		dir                     float64
		bolide                  bool
	}
	meteors := make([]meteorv, 0, count)
	for i := 0; i < count; i++ {
		dir := 1.0
		if slotR.float64() < 0.5 {
			dir = -1.0
		}
		m := meteorv{
			delay:  slotR.rangeF(0, 6.2),
			dur:    slotR.rangeF(0.7, 1.15),
			ang:    slotR.rangeF(0.30, 0.72), // radians below horizontal
			x0:     slotR.rangeF(0.08, 0.85),
			y0:     slotR.rangeF(0.04, 0.34),
			dir:    dir,
			bolide: bolide && i == 0,
		}
		meteors = append(meteors, m)
	}
	b.beginLayer(layerStars, 0, 0, sw(b), int(H*0.78)+2)
	for _, m := range meteors {
		if m.bolide {
			m.dur = 1.3
		}
		frac := (sec - float64(slot*slotSec) - m.delay) / m.dur
		if frac < 0 || frac >= 1 {
			continue
		}
		travel := W * 0.34
		if m.bolide {
			travel = W * 0.52
		}
		e := EaseInOutCubic(frac)
		headX := m.x0*W + m.dir*e*travel
		headY := m.y0*H + e*travel*math.Tan(m.ang)*0.55
		if headY > lay.Horizon-lay.MinDim*0.03 {
			continue // stays in the sky
		}
		dx := m.dir * travel * math.Cos(m.ang) / m.dur
		dy := travel * math.Sin(m.ang) / m.dur
		if m.bolide {
			dx *= 1.6
		}
		tailLen := 13
		if m.bolide {
			tailLen = 18
		}
		headR := 0.4
		if m.bolide {
			headR = 0.58
		}
		headCol := blendHex(bright, cyan, 0.12)
		trailCol := blendHex(bright, cyan, 0.55)
		if s.mode == ModeASCII {
			glyph := '/'
			if m.dir < 0 {
				glyph = '\\'
			}
			col := mixIndex(p, cBrightText, p.base, 1-frac*0.6)
			b.mark(int(clampf(headX, 0, W-1)), int(clampf(headY, 0, H-1)), glyph, col)
			continue
		}
		// head with a hot core
		b.fillCircleHexF(headX, headY, headR, headCol, 1, opNormal)
		b.glowHexAt(headX, headY, bright, 0.4)
		// tail: thin fading segments back along the flight vector
		for k := 1; k < tailLen; k++ {
			fade := 1 - float64(k)/float64(tailLen)
			kx := headX - dx*float64(k)*0.09*m.dur
			ky := headY - dy*float64(k)*0.09*m.dur
			if ky >= lay.Horizon {
				break
			}
			b.fillCircleHexF(kx, ky, 0.34*fade+0.08, blendHex(trailCol, base, 1-fade*0.85), fade*0.9, opNormal)
		}
		// bolide sparkle pop as it finishes
		if m.bolide && frac > 0.82 {
			pop := Envelope(frac, 0.82, 0.9, 0.97, 1.0)
			if pop > 0 {
				for _, d := range [][2]float64{{1, 0}, {-1, 0}, {0, 1}, {0, -1}, {0.7, 0.7}, {-0.7, -0.7}} {
					b.fillCircleHexF(headX+d[0]*pop*1.3, headY+d[1]*pop*1.3, 0.3, bright, pop*0.7, opNormal)
				}
			}
		}
	}
	b.endLayer()

	// --- horizon: rugged far ridge and a jagged pine line, silhouetted
	sil := cachedPerlin(state.Seed, 511)
	farY := lay.Horizon
	nearY := lay.Horizon + H*0.10
	b.anchor("horizon", 0, lay.Horizon-0.01, W, lay.Horizon+0.01)
	b.beginLayer(layerLand, 0, int(lay.Horizon-H*0.20), sw(b), sh(b))
	pine := blendHex(base, state.Palette.SurfaceStrong, 0.06)
	for x := 0.0; x < W; x += 0.75 {
		fy := farY + sil.At(x*0.013+9.7, 4.4)*H*0.075
		ny := nearY + sil.At(x*0.024+3.9, 8.8)*H*0.04
		fc := rmp.ApplyHaze(state.Palette.SurfaceStrong, 0.7)
		b.fillRectHexF(x, fy, x+0.75, H, fc, 1, opNormal)
		tree := sil.At(x*0.085+21, 6.6)
		hgt := H * 0.026
		if tree > 0.28 {
			hgt += (tree - 0.28) * (tree - 0.28) * H * 0.85
		}
		hgt += sil.At(x*0.33+55, 2.2) * H * 0.02
		b.fillRectHexF(x, ny-hgt, x+0.75, H, pine, 1, opNormal)
	}
	b.endLayer()
}
