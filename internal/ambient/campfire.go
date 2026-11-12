package ambient

import (
	"math"
	"time"
)

// campfireScene paints a big bold campfire: layered noise-driven flame
// tongues (outer ember red, mid orange, inner gold with a white-hot
// core) reaching high, an ember bed, crossed logs in a stone ring,
// rising sparks with twinkle, and a warm pool of light cast on the
// surroundings that breathes with the fire.
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

func (s *campfireScene) FramePixels(t time.Duration, w, h int, state SceneState) Raster {
	return renderRaster(s, t, w, h, state)
}

func (s *campfireScene) paint(b *buffer, p *paletteModel, t time.Duration, w, h int, state SceneState) {
	W := float64(sw(b))
	H := float64(sh(b))
	lay := layoutForState(state, W, H)
	base := state.Palette.Base
	rmp := BuildRamps(state.Palette)
	ts := tSecF(t)

	// --- sky: a dark ramp gradient with a dim moon and stars
	b.fillGradientHex(0, 0, W, H, blendHex(rmp.SkyTop, base, 0.2), blendHex(rmp.SkyHorizon, base, 0.45), 1, opNormal)

	moonR := lay.MinDim * 0.085
	if moonR < 1.6 {
		moonR = 1.6
	}
	moonX := clampf(W*0.2, moonR+0.5, W-moonR-0.5)
	moonY := math.Min(H*0.14, lay.Horizon-moonR*2)
	if moonY < moonR+0.5 {
		moonY = moonR + 0.5
	}
	b.anchor("moon", moonX-moonR, moonY-moonR, moonX+moonR, moonY+moonR)
	phase := moonPhase(state.Now)
	b.fillCircleHexF(moonX, moonY, moonR*1.6, blendHex(state.Palette.BrightText, base, 0.8), 0.4, opAdd)
	b.fillCircleHexF(moonX, moonY, moonR, blendHex(state.Palette.BrightText, base, 0.62), 1, opNormal)
	shadowCol := blendHex(state.Palette.Surface, base, 0.35)
	term := math.Cos(2*math.Pi*phase) * moonR
	if moonWaxing(phase) {
		b.fillCircleHexF(moonX-term, moonY, moonR, shadowCol, 0.9, opNormal)
	} else {
		b.fillCircleHexF(moonX+term, moonY, moonR, shadowCol, 0.9, opNormal)
	}
	for _, st := range nightStars(state.Seed, sw(b), sh(b)) {
		if st.mag < 0.55 {
			continue
		}
		tw := 0.5 + 0.5*math.Sin(ts*st.speed+st.phase)
		if tw < 0.3 {
			continue
		}
		b.fillCircleHexF(st.x, st.y, 0.26, blendHex(state.Palette.BrightText, base, 0.55), tw*0.85, opNormal)
	}

	// --- ground: dark band at the base
	groundY := lay.Horizon
	b.anchor("horizon", 0, lay.Horizon-0.01, W, lay.Horizon+0.01)
	b.fillGradientHex(0, groundY, W, H+1, blendHex(base, state.Palette.SurfaceStrong, 0.35), blendHex(base, "#000000", 0.25), 1, opNormal)

	// --- the fire pit
	fx := W * 0.5
	fireY := lay.Horizon + (H-lay.Horizon)*0.38
	// light cast: a breathing warm pool over the whole lower scene
	glowN := cachedPerlin(state.Seed, 701)
	breath := 0.78 + reducedAmp(state, 0.3)*glowN.At(ts*0.5, 0.5)
	b.beginLayer(layerFire, 0, int(H*0.28), sw(b), sh(b))
	// ground pool of light
	b.fillEllipseHexF(fx, groundY+H*0.1, W*0.52, H*0.17, state.Palette.Warning, 0.16*breath, opAdd)
	b.fillEllipseHexF(fx, fireY, W*0.3, H*0.10, state.Palette.Orange, 0.13*breath, opAdd)
	// sky glow directly above the fire
	b.fillEllipseHexF(fx, fireY-H*0.10, W*0.17, H*0.16, state.Palette.Warning, 0.05*breath, opAdd)

	// --- stone ring: a dozen rocks with lit inner edges
	stoneScale := lay.MinDim * 0.011
	stones := []struct {
		dx, dy, r float64
	}{
		{-4.6, 0.9, 0.8}, {-3.9, 1.6, 0.62}, {-2.9, 1.9, 0.7}, {-1.8, 2.1, 0.55},
		{1.8, 2.1, 0.55}, {2.9, 1.9, 0.7}, {3.9, 1.6, 0.62}, {4.6, 0.9, 0.8},
		{-1.0, 2.2, 0.5}, {0.0, 2.25, 0.58}, {1.0, 2.2, 0.5},
	}
	for _, st := range stones {
		sx, sy := fx+st.dx*stoneScale*4.4, fireY+st.dy*stoneScale*2.6
		st.r = st.r * stoneScale * 1.9
		// rock: dark body with a warm rim on the fire side
		b.fillCircleHexF(sx, sy, st.r, blendHex(base, state.Palette.SurfaceRaised, 0.35), 1, opNormal)
		b.fillCircleHexF(sx*0.94+fx*0.06, sy-st.r*0.35, st.r*0.62, blendHex(state.Palette.Warning, base, 0.68), 0.5*breath, opAdd)
	}

	// --- crossed logs: three strokes with a lit top edge
	logHalf := lay.MinDim * 0.055
	log := blendHex(base, state.Palette.Brown, 0.5)
	logTop := blendHex(state.Palette.Brown, state.Palette.BrightText, 0.28)
	b.strokeHexF(fx-logHalf, fireY-0.7*stoneScale*2.6, fx+logHalf*0.9, fireY+1.15*stoneScale*2.6, 0.85, log, 1, opNormal)
	b.strokeHexF(fx+logHalf, fireY-0.7*stoneScale*2.6, fx-logHalf*0.9, fireY+1.15*stoneScale*2.6, 0.85, log, 1, opNormal)
	b.strokeHexF(fx-logHalf*0.66, fireY-1.35*stoneScale*2.6, fx+logHalf*0.79, fireY+0.55*stoneScale*2.6, 0.7, log, 1, opNormal)
	b.strokeHexF(fx-logHalf*0.94, fireY-0.95*stoneScale*2.6, fx+logHalf*0.88, fireY+0.9*stoneScale*2.6, 0.16, logTop, 0.7, opNormal)

	// --- ember bed: pulsing coals along the pit
	for i := 0; i < 9; i++ {
		lr := newRNG(state.Seed, uint64(i)*383+7)
		ex := fx + (float64(i)-4)*lay.MinDim*0.007 + lr.rangeF(-0.15, 0.15)
		pulse := 0.5 + 0.45*math.Sin(ts*1.35+float64(i)*1.7+lr.float64()*3)
		col := state.Palette.Orange
		if i%3 == 1 {
			col = state.Palette.Danger
		}
		if i%4 == 3 {
			col = state.Palette.Yellow
		}
		b.fillCircleHexF(ex, fireY+0.35+lr.rangeF(-0.15, 0.15), 0.3+lr.float64()*0.16, col, pulse, opNormal)
	}

	// --- flames: three layered tongues of noise-driven columns
	flame := cachedPerlin(state.Seed, 719)
	flameScale := lay.MinDim / 60.0
	layers := []struct {
		halfW  float64
		height float64
		ci     string
		alpha  float64
		noiseX float64
	}{
		{2.6 * flameScale, H * 0.30, state.Palette.Danger, 0.9, 0.9},
		{1.9 * flameScale, H * 0.225, state.Palette.Orange, 0.9, 1.3},
		{1.15 * flameScale, H * 0.155, state.Palette.Yellow, 0.95, 1.75},
		{0.55 * flameScale, H * 0.085, state.Palette.BrightText, 0.95, 2.3},
	}
	for li, layer := range layers {
		for x := fx - layer.halfW*2.1; x <= fx+layer.halfW*2.1; x += 0.45 {
			center := 1 - math.Abs(x-fx)/(layer.halfW*2.1)
			if center <= 0 {
				continue
			}
			// per-column flame height: parabolic envelope + noise wobble
			colNoise := flame.At(ts*1.5+x*0.55+float64(li)*11.3, float64(li)*3.7)*0.32 + flame.At(ts*0.8+x*0.23+float64(li)*5.1, 31.0)*0.22
			height := (center * center) * (1 - center*0.15) * layer.height * (0.8 + colNoise) * (0.9 + 0.22*math.Sin(ts*2.1+x*1.4+float64(li)*2.6))
			if height < 1.0 {
				continue
			}
			top := fireY - height
			if top < 0 {
				top = 0
			}
			// column: alpha fades toward the tip
			for y := fireY; y > top; y -= 0.5 {
				frac := (fireY - y) / height
				a := layer.alpha * (1 - frac*frac*0.85)
				width := layer.halfW * (1 - frac*0.72) * (0.9 + 0.2*math.Sin(x*3.1+ts*3.3))
				if width <= 0.1 {
					continue
				}
				b.fillEllipseHexF(x, y, width, 0.62, layer.ci, a, opNormal)
			}
		}
	}

	// --- sparks: slotted pops that rise and twinkle out
	sparkSlot := int64(wrapT(t, 24*time.Hour)/time.Millisecond) / 700
	for k := 0; k < 5; k++ {
		slot := sparkSlot - int64(k)
		sr := newRNG(state.Seed, uint64(slot)*383+13)
		frac := (float64(wrapT(t, 24*time.Hour)/time.Millisecond) / 1000.0) - float64(slot)*0.7
		if frac < 0 || frac >= 0.7 {
			continue
		}
		prog := frac / 0.7
		spx := float64(fx) + sr.rangeF(-2.6, 2.6) + math.Sin(prog*9+float64(slot))*1.1
		spy := fireY - 3 - prog*H*0.42
		tw := 0.5 + 0.5*math.Sin(prog*22+float64(slot)*3.1)
		fade := (1 - prog) * (0.35 + 0.65*tw)
		col := state.Palette.BrightText
		if prog > 0.55 {
			col = state.Palette.Warning
		}
		b.fillCircleHexF(spx, spy, 0.26, col, fade, opNormal)
		if tw > 0.8 {
			b.fillCircleHexF(spx, spy, 0.42, col, fade*0.5, opAdd)
		}
	}
	b.endLayer()
}
