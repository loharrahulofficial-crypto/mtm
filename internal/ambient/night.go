package ambient

import (
	"math"
	"time"
)

// nightScene paints a moonlit sky: a vertical gradient (deep zenith to
// warm horizon haze), a filled moon with limb shading, correct lit phase
// and a soft halo, magnitude-varied stars with twinkle and cross-glow on
// the brightest ones, occasional shooting stars, aurora ribbons as
// additive noise curtains with slow vertical shimmer, and a silhouetted
// horizon in two depths (far haze ridge, near pine line).
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

// starv is one deterministic star with parallax drift.
type starv struct {
	x, y  float64
	mag   float64
	phase float64
	speed float64
}

func nightStars(seed int64, w, h int) []starv {
	r := newRNG(seed, 17)
	n := clampf(float64(w*h)/11, 25, 130)
	out := make([]starv, 0, int(n))
	for i := 0; i < int(n); i++ {
		mag := r.float64()
		mag = mag * mag // bias toward dim stars
		out = append(out, starv{
			x:     r.float64() * float64(w),
			y:     r.float64() * float64(h) * 0.72,
			mag:   mag,
			phase: r.rangeF(0, 2*math.Pi),
			speed: r.rangeF(0.6, 1.7),
		})
	}
	return out
}

// glyphFor maps a twinkle value to a legibility glyph (ascii mode).
func glyphFor(tw float64) rune {
	switch {
	case tw > 0.82:
		return '✦'
	case tw > 0.6:
		return '+'
	case tw > 0.38:
		return '·'
	case tw > 0.2:
		return ':'
	default:
		return '.'
	}
}

func (s *nightScene) paint(b *buffer, p *paletteModel, t time.Duration, w, h int, state SceneState) {
	W := float64(sw(b))
	H := float64(sh(b))
	rmp := BuildRamps(state.Palette)
	ts := float64(wrapT(t, 10*time.Minute)) / float64(time.Second)
	base := state.Palette.Base
	surfaceStrong := state.Palette.SurfaceStrong
	cyan := state.Palette.Cyan
	bright := state.Palette.BrightText

	// --- sky: deep zenith fading to a warm haze at the horizon
	b.fillGradientHex(0, 0, W, H, rmp.SkyTop, rmp.SkyHorizon, 1, opNormal)

	// --- aurora ribbons: two additive noise curtains, slow vertical shimmer
	b.beginLayer(layerSky, 0, 0, sw(b), minInt(sh(b), sh(b)/2+2))
	aura := cachedPerlin(state.Seed, 71)
	for ribbon := 0; ribbon < 2; ribbon++ {
		ribbonCol := cyan
		if ribbon == 1 {
			// second ribbon leans emerald by blending toward success
			ribbonCol = blendHex(state.Palette.Success, state.Palette.Cyan, 0.35)
		}
		for x := 0.0; x < W; x += 0.5 {
			fx := x / W
			cen := H*0.16 + float64(ribbon)*H*0.10 +
				aura.At(fx*3.4+float64(ribbon)*13.7, ts*0.07+float64(ribbon)*7.3)*H*0.13
			thick := 0.7 + aura.At(fx*5.3+float64(ribbon)*29.1, ts*0.11+3.0+float64(ribbon)*17.0)*0.85
			intensity := 0.16 + 0.26*(0.5+0.5*aura.At(fx*2.3+float64(ribbon)*41.0, ts*0.05+float64(ribbon)*23.0))
			for y := cen - thick; y < cen+thick; y += 0.5 {
				d := math.Abs(y-cen) / thick
				// vertical shimmer: the curtain breathes edge-to-edge
				shimmer := 0.55 + 0.45*math.Sin(ts*1.35+y*1.7+float64(ribbon)*2.1+x*0.06)
				if shimmer <= 0.05 {
					continue
				}
				cov := (1 - d) * intensity * shimmer
				if cov <= 0.02 {
					continue
				}
				covPx := clampf(cov, 0, 0.75)
				b.glowHexAt(x, y, ribbonCol, covPx)
			}
		}
	}
	b.endLayer()

	// --- stars: varied magnitudes, twinkle, and cross-glow on the brightest
	b.beginLayer(layerStars, 0, 0, sw(b), int(H*0.74)+1)
	for _, st := range nightStars(state.Seed, sw(b), sh(b)) {
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
		case brightness > 0.78: // brighter star with a soft cross glow
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

	// --- moon: phase from the date; progress walks it across the sky
	prog := clampf(state.Progress, 0, 1)
	phase := moonPhase(state.Now)
	mX := W * (0.10 + 0.80*prog)
	mY := H*0.60 - math.Sin(math.Pi*prog)*H*0.44
	mR := math.Min(W, H*1.6) * 0.072
	if mR < 1.6 {
		mR = 1.6
	}
	b.beginLayer(layerCelestial, int(mX-mR-2), int(mY-mR-2), int(mX+mR+3), int(mY+mR+3))
	// halo: slow glow pulse (idle breathing)
	haloPulse := 0.5 + reducedAmp(state, 0.14)*math.Sin(ts*0.42)
	b.fillCircleHexF(mX, mY, mR*1.9, blendHex(bright, base, 0.82), haloPulse, opAdd) // halo
	// disc: limb shading in three tones
	b.fillCircleHexF(mX, mY, mR, blendHex(bright, base, 0.62), 1, opNormal)
	b.fillCircleHexF(mX, mY-mR*0.18, mR*0.78, blendHex(bright, base, 0.32), 1, opNormal)
	b.fillCircleHexF(mX, mY-mR*0.3, mR*0.45, blendHex(bright, base, 0.12), 1, opNormal)
	// terminator: the shadow disc slides across with the phase
	term := math.Cos(2*math.Pi*phase) * mR * 1.12
	shadowCol := blendHex(state.Palette.Surface, base, 0.35)
	if moonWaxing(phase) {
		b.fillCircleHexF(mX-term, mY, mR, shadowCol, 0.97, opNormal)
	} else {
		b.fillCircleHexF(mX+term, mY, mR, shadowCol, 0.97, opNormal)
	}
	// craters: fixed fractional dimples on the lit side
	lit := 1.0
	if !moonWaxing(phase) {
		lit = -1.0
	}
	b.fillCircleHexF(mX+lit*mR*0.35, mY-mR*0.22, mR*0.16, blendHex(bright, base, 0.55), 0.8, opNormal)
	b.fillCircleHexF(mX+lit*mR*0.05, mY+mR*0.28, mR*0.11, blendHex(bright, base, 0.5), 0.8, opNormal)
	b.fillCircleHexF(mX-lit*mR*0.25, mY+mR*0.02, mR*0.07, blendHex(bright, base, 0.48), 0.8, opNormal)
	b.endLayer()

	// --- shooting stars: slotted, fading tails
	b.beginLayer(layerStars, 0, 0, sw(b), sh(b)/2)
	slotSec := int64(5)
	slot := int64(wrapT(t, 24*time.Hour)/time.Second) / slotSec
	slotR := newRNG(state.Seed, uint64(slot)*977+13)
	if slotR.float64() < 0.55 {
		frac := (float64(wrapT(t, 24*time.Hour)/time.Millisecond)/1000.0 - float64(slot*slotSec)) / 0.8
		if frac >= 0 && frac < 1 {
			dir := 1.0
			if slotR.float64() < 0.5 {
				dir = -1.0
			}
			x0 := slotR.rangeF(0.15, 0.65) * W
			y0 := slotR.rangeF(0.03, 0.24) * H
			headX := x0 + dir*frac*W*0.22
			headY := y0 + frac*H*0.16
			for k := 0; k < 8; k++ {
				fade := 1 - float64(k)/8
				kx := headX - dir*frac*W*0.22*(float64(k)/7)
				ky := headY - frac*H*0.16*(float64(k)/7)
				col := blendHex(bright, base, 1-fade*0.85)
				if k == 0 {
					b.fillCircleHexF(kx, ky, 0.5, col, 1, opNormal)
					b.glowHexAt(kx, ky, bright, 0.5)
					continue
				}
				b.fillCircleHexF(kx, ky, 0.42*fade+0.1, col, fade, opNormal)
			}
		}
	}
	b.endLayer()

	// --- horizon: far hazy ridge, then the near pine line
	sil := cachedPerlin(state.Seed, 101)
	farY := H * 0.74
	nearY := H * 0.84
	b.beginLayer(layerLand, 0, int(H*0.55), sw(b), sh(b))
	for x := 0.0; x < W; x += 0.75 {
		fy := farY + sil.At(x*0.016+3.1, 7.3)*H*0.055
		ny := nearY + sil.At(x*0.021+11.7, 1.9)*H*0.045
		fc := rmp.ApplyHaze(surfaceStrong, 0.72)
		nc := blendHex(base, surfaceStrong, 0.28)
		// far ridge fill
		b.fillRectHexF(x, fy, x+0.75, H, fc, 1, opNormal)
		// near ground fill
		b.fillRectHexF(x, ny, x+0.75, H, nc, 1, opNormal)
		// pines on the near line
		tree := sil.At(x*0.09+40, 3.3)
		if tree > 0.28 {
			pineH := H*0.13*(tree-0.28) + H*0.055
			pw := pineH * 0.36
			pc := blendHex(base, surfaceStrong, 0.08)
			// stepped pine: three stacked widening triangles
			for i := 0; i < 3; i++ {
				tb := float64(i) / 3
				width := pw * (tb*0.9 + 0.28)
				yC := ny - pineH*(1-tb) + pineH*0.16
				b.fillRectHexF(x+0.35-width, yC, x+0.35+width, yC+pineH*0.24, pc, 1, opNormal)
			}
		}
	}
	b.endLayer()
}

// glowHexAt adds a small additive glow dab at float position.
func (b *buffer) glowHexAt(cx, cy float64, hex string, a float64) {
	r, g, bl, ok := hexF(hex)
	if !ok {
		return
	}
	b.fb.fillCircleF(cx, cy, 0.62, r, g, bl, a, opAdd)
}
