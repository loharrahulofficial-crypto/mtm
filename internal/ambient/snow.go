package ambient

import (
	"math"
	"time"
)

// snowScene paints a quiet snowfall: a ramp sky with a dim moon, a
// two-depth pine treeline, snow accumulating on the ground, a soft
// ground glow pooling under the moon, and three parallax layers of
// flakes — far flakes dim and tiny, near flakes big and bright —
// drifting on a slow wind.
type snowScene struct {
	mode string
}

func (s *snowScene) Name() string { return "snow" }

func (s *snowScene) Frame(t time.Duration, w, h int, state SceneState) []Line {
	return renderFrame(s, s.mode, t, w, h, state, false)
}

func (s *snowScene) Reward(t time.Duration, w, h int, state SceneState) []Line {
	return renderFrame(s, s.mode, t, w, h, state, true)
}

func (s *snowScene) paint(b *buffer, p *paletteModel, t time.Duration, w, h int, state SceneState) {
	W := float64(sw(b))
	H := float64(sh(b))
	base := state.Palette.Base
	rmp := BuildRamps(state.Palette)
	ts := tSecF(t)

	// --- sky: the night ramp, dimmer (moonlit haze)
	b.fillGradientHex(0, 0, W, H, blendHex(rmp.SkyTop, base, 0.25), blendHex(rmp.SkyHorizon, base, 0.35), 1, opNormal)

	// --- stars, sparse and faint
	if starT := cachedPerlin(state.Seed, 641); starT != nil {
		for _, st := range nightStars(state.Seed, sw(b), sh(b)) {
			if st.mag < 0.55 {
				continue
			}
			tw := 0.5 + 0.5*math.Sin(ts*st.speed+st.phase)
			if tw < 0.3 {
				continue
			}
			b.fillCircleHexF(st.x, st.y, 0.26, blendHex(state.Palette.BrightText, base, 0.6), tw*0.8, opNormal)
		}
	}

	// --- dim moon with a soft halo, high and to the right
	moonX := W * 0.72
	moonY := H * 0.17
	moonR := H * 0.075
	if moonR < 2 {
		moonR = 2
	}
	phase := moonPhase(state.Now)
	b.fillCircleHexF(moonX, moonY, moonR*1.7, blendHex(state.Palette.BrightText, base, 0.78), 0.45, opAdd)
	b.fillCircleHexF(moonX, moonY, moonR, blendHex(state.Palette.BrightText, base, 0.5), 1, opNormal)
	b.fillCircleHexF(moonX, moonY-moonR*0.15, moonR*0.72, blendHex(state.Palette.BrightText, base, 0.26), 1, opNormal)
	term := math.Cos(2*math.Pi*phase) * moonR * 1.05
	shadowCol := blendHex(state.Palette.Surface, base, 0.3)
	if moonWaxing(phase) {
		b.fillCircleHexF(moonX-term, moonY, moonR, shadowCol, 0.95, opNormal)
	} else {
		b.fillCircleHexF(moonX+term, moonY, moonR, shadowCol, 0.95, opNormal)
	}

	// --- treeline: far hazy, near darker
	sil := cachedPerlin(state.Seed, 641)
	groundY := H * 0.72
	b.beginLayer(layerLand, 0, int(H*0.42), sw(b), sh(b))
	// far pines: sparse, faint, atmospheric haze
	for x := 0.0; x < W; x += 0.8 {
		gy := groundY + sil.At(x*0.028+9.1, 4.2)*1.6
		treeH := H * 0.10 * (sil.At(x*0.06+21.4, 3.3) + 1.5)
		farCol := rmp.ApplyHaze(state.Palette.SurfaceStrong, 0.74)
		b.fillRectHexF(x, gy-treeH, x+0.8, gy+1.2, farCol, 1, opNormal)
	}
	// near pines: taller, darker
	for x := 0.0; x < W; x += 1.0 {
		gy := groundY + sil.At(x*0.035+13.7, 7.9)*1.9 + 2.5
		treeH := H * 0.17 * (sil.At(x*0.08+31.3, 5.1) + 1.45)
		nearCol := blendHex(base, state.Palette.SurfaceStrong, 0.18)
		for i := 0; i < 4; i++ {
			tb := float64(i) / 4
			width := treeH * 0.34 * (tb*0.9 + 0.24)
			yC := gy - treeH*(1-tb) + treeH*0.15
			b.fillRectHexF(x+0.5-width, yC, x+0.5+width, yC+treeH*0.22, nearCol, 1, opNormal)
		}
	}
	b.endLayer()

	// --- snow accumulation: creeps up with progress, soft top edge
	accum := int(clampf(state.Progress, 0, 1) * H * 0.09)
	accumY := H - 1 - float64(accum)
	snowTop := blendHex(state.Palette.BrightText, base, 0.45)
	snowBot := blendHex(state.Palette.BrightText, base, 0.22)
	b.beginLayer(layerLand, 0, int(accumY)-1, sw(b), sh(b))
	b.fillGradientHex(0, accumY, W, H+1, snowTop, snowBot, 1, opNormal)
	// sparkle on the snow: sparse glints
	glintN := cachedPerlin(state.Seed, 661)
	for x := 0.5; x < W; x += 1.15 {
		n := glintN.At(x*0.4, ts*0.25)
		if n > 0.52 {
			b.fillCircleHexF(x, accumY+H*0.045+math.Sin(x*1.7)*0.8, 0.22, state.Palette.BrightText, 0.55, opNormal)
		}
	}
	// ground glow: moonlight pooling on the snow
	b.fillEllipseHexF(moonX, H-0.8, W*0.42, H*0.09, state.Palette.BrightText, 0.10, opAdd)
	b.endLayer()

	// --- flakes: three parallax layers on a slow wind
	wind := cachedPerlin(state.Seed, 661).At(ts*0.05, 0)
	flakes := []struct {
		count int
		size  [2]float64
		fall  [2]float64
		alpha [2]float64
		ci    string
	}{
		{int(W * 0.55), [2]float64{0.16, 0.24}, [2]float64{3.2, 4.6}, [2]float64{0.25, 0.38}, blendHex(state.Palette.BrightText, base, 0.35)},
		{int(W * 0.3), [2]float64{0.26, 0.4}, [2]float64{5.2, 6.8}, [2]float64{0.4, 0.6}, blendHex(state.Palette.BrightText, base, 0.18)},
		{int(W * 0.14), [2]float64{0.42, 0.62}, [2]float64{8.2, 10.0}, [2]float64{0.6, 0.85}, state.Palette.BrightText},
	}
	for li, L := range flakes {
		for i := 0; i < L.count; i++ {
			r := newRNG(state.Seed, uint64(li)*1000+uint64(i)*37+7)
			xBase := r.float64() * W
			yOff := r.float64() * (H + 8)
			size := L.size[0] + r.float64()*(L.size[1]-L.size[0])
			fall := L.fall[0] + r.float64()*(L.fall[1]-L.fall[0])
			alpha := L.alpha[0] + r.float64()*(L.alpha[1]-L.alpha[0])
			y := math.Mod(yOff+ts*fall, H+6) - 3
			x := xBase + wind*3.2 + math.Sin(ts*0.75+float64(i)*1.7+float64(li)*2.3)*0.9
			if x < -2 {
				x += W + 4
			}
			if x > W+2 {
				x -= W + 4
			}
			b.fillCircleHexF(x, y, size, L.ci, alpha, opNormal)
		}
	}
}
