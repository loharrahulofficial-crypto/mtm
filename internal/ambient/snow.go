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

func (s *snowScene) FramePixels(t time.Duration, w, h int, state SceneState) Raster {
	return renderRaster(s, t, w, h, state)
}

func (s *snowScene) paint(b *buffer, p *paletteModel, t time.Duration, w, h int, state SceneState) {
	W := float64(sw(b))
	H := float64(sh(b))
	lay := layoutForState(state, W, H)
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
	moonR := lay.MinDim * 0.125
	if moonR < 1.6 {
		moonR = 1.6
	}
	moonX := clampf(W*0.72, moonR+0.5, W-moonR-0.5)
	moonY := math.Min(H*0.17, lay.Horizon*0.62)
	if moonY < moonR+0.5 {
		moonY = moonR + 0.5
	}
	b.anchor("moon", moonX-moonR, moonY-moonR, moonX+moonR, moonY+moonR)
	phase := moonPhase(state.Now)
	skyAtSnow := blendHex(rmp.SkyTop, rmp.SkyHorizon, clampf(moonY/H, 0, 1))
	snowCols := sphereCols(state.Palette.BrightText, base, skyAtSnow)
	b.beginLayer(layerCelestial, int(moonX-moonR-2), int(moonY-moonR-2), int(moonX+moonR+3), int(moonY+moonR+3))
	paintMoonSphere(b, moonX, moonY, moonR, phase, ts, snowCols, haloCol(state.Palette.BrightText, state.Palette.Accent))
	b.endLayer()

	// --- treeline: three depth layers — a hazier far line and the near
	// pines, continuous serrated ridgelines with a vertical absolute-y
	// gradient; a touch of mist sits at the base.
	sil := cachedPerlin(state.Seed, 641)
	groundY := lay.Horizon
	b.anchor("horizon", 0, lay.Horizon-0.01, W, lay.Horizon+0.01)
	b.beginLayer(layerLand, 0, int(lay.Horizon-H*0.28), sw(b), sh(b))
	nearCol := blendHex(base, state.Palette.SurfaceStrong, 0.18)
	farCrest := rmp.ApplyHaze(state.Palette.SurfaceStrong, 0.74)
	snowMidCrest := blendHex(blendHex(base, state.Palette.SurfaceStrong, 0.24), farCrest, 0.4)
	layers := []treelineLayer{
		{baseY: groundY + H*0.006, yBot: groundY + H*0.014, params: ridgeParams{swellAmp: H * 0.034, swellFreq: 0.028, spireAmp: H * 0.075, spireFreq: 0.150, phase: 21.4}, crest: farCrest, base: blendHex(farCrest, base, 0.42)},
		{baseY: groundY + H*0.010, yBot: groundY + H*0.020, params: ridgeParams{swellAmp: H * 0.024, swellFreq: 0.032, spireAmp: H * 0.090, spireFreq: 0.250, phase: 15.0}, crest: snowMidCrest, base: blendHex(snowMidCrest, base, 0.42)},
		{baseY: groundY + H*0.016, yBot: groundY + H*0.030, params: ridgeParams{swellAmp: H * 0.020, swellFreq: 0.035, spireAmp: H * 0.115, spireFreq: 0.340, phase: 31.3}, crest: nearCol, base: blendHex(nearCol, base, 0.42)},
	}
	for _, layer := range layers {
		paintRidgeLayer(b, sil, W, layer)
		if layer.baseY == layers[1].baseY {
			paintMistBase(b, cachedPerlin(state.Seed, 661), ts, W, H, groundY+H*0.012, blendHex(rmp.Haze, base, 0.3), 5.1)
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
	// foreground drifts: two soft banks silhouetted against the deep snow,
	// catching light on their crests
	driftSil := cachedPerlin(state.Seed, 733)
	for d := 0; d < 2; d++ {
		y0 := H * (0.84 + float64(d)*0.09)
		amp := H * (0.030 + float64(d)*0.012)
		crest := blendHex(state.Palette.BrightText, base, 0.22-float64(d)*0.07)
		body := blendHex(state.Palette.BrightText, base, 0.36-float64(d)*0.10)
		for x := 0.0; x <= W; x += 0.75 {
			yA := y0 + driftSil.At(x*0.085+float64(d)*31.7, 3.3)*amp
			b.fillRectHexF(x, yA, x+0.75, H+1, body, 1, opNormal)
			b.strokeHexF(x, yA, x+0.75, yA, 0.30, crest, 0.85, opNormal)
		}
	}
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
			x := xBase + wind*3.2 + math.Sin(ts*0.75+float64(i)*1.7+float64(li)*2.3)*reducedAmp(state, 0.9)
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
